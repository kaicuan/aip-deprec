package upstream

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"aip/internal/config"
	"aip/internal/db"
	"aip/internal/proxy"
)

type Upstream struct {
	db      *db.DB
	pool    *proxy.Pool
	baseURL string
}

func NewUpstream(database *db.DB, pool *proxy.Pool) *Upstream {
	return &Upstream{
		db:      database,
		pool:    pool,
		baseURL: config.UpstreamBaseURL,
	}
}

type chatRequestSnippet struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

var promptTokensRegex = regexp.MustCompile(`"(?:prompt_tokens|input_tokens)"\s*:\s*(\d+)`)
var compTokensRegex = regexp.MustCompile(`"(?:completion_tokens|output_tokens)"\s*:\s*(\d+)`)
var cachedTokensRegex = regexp.MustCompile(`"(?:cached_tokens|cache_read_input_tokens)"\s*:\s*(\d+)`)
var reasoningTokensRegex = regexp.MustCompile(`"reasoning_tokens"\s*:\s*(\d+)`)

func copyHeaderWithoutHopByHop(dst, src http.Header) {
	for k, vv := range src {
		if strings.EqualFold(k, "Transfer-Encoding") || strings.EqualFold(k, "Connection") {
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func extractToken(regex *regexp.Regexp, primary, secondary string) int {
	if m := regex.FindStringSubmatch(primary); len(m) > 1 {
		if val, err := strconv.Atoi(m[1]); err == nil {
			return val
		}
	}
	if secondary != "" && secondary != primary {
		if m := regex.FindStringSubmatch(secondary); len(m) > 1 {
			if val, err := strconv.Atoi(m[1]); err == nil {
				return val
			}
		}
	}
	return 0
}

func (u *Upstream) Forward(w http.ResponseWriter, r *http.Request, pathSuffix string) {
	// Read request body into memory for retry capability & logging
	var bodyBytes []byte
	var err error
	if r.Body != nil {
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		_ = r.Body.Close()
	}

	var reqSnippet chatRequestSnippet
	if len(bodyBytes) > 0 {
		_ = json.Unmarshal(bodyBytes, &reqSnippet)
	}

	targetURL := strings.TrimRight(u.baseURL, "/") + "/" + strings.TrimLeft(pathSuffix, "/")

	startTime := time.Now()
	var lastProxy *db.Proxy
	var lastResp *http.Response
	var lastErr error
	var finalBodyBytes []byte

	// Try with failover
	maxRetries := 3
	triedIDs := make(map[int64]bool)

	for attempt := 0; attempt <= maxRetries; attempt++ {
		var selectedProxy *db.Proxy
		if attempt == 0 {
			selectedProxy, err = u.pool.GetNextProxy()
		} else {
			candidates := u.pool.GetOnlineCandidatesExcluding(triedIDs)
			if len(candidates) > 0 {
				selectedProxy = candidates[0]
			} else {
				break
			}
		}

		if err != nil || selectedProxy == nil {
			break
		}
		lastProxy = selectedProxy
		triedIDs[selectedProxy.ID] = true

		transport, trErr := u.pool.GetTransport(selectedProxy)
		if trErr != nil {
			lastErr = trErr
			continue
		}

		client := &http.Client{
			Transport: transport,
			Timeout:   0, // No client-level timeout for streaming
		}

		outReq, reqErr := http.NewRequestWithContext(r.Context(), r.Method, targetURL, bytes.NewReader(bodyBytes))
		if reqErr != nil {
			lastErr = reqErr
			break
		}

		// Copy headers
		copyHeaderWithoutHopByHop(outReq.Header, r.Header)

		// Inject Authorization: Bearer public & Host header
		outReq.Header.Set("Authorization", "Bearer public")
		outReq.Host = "opencode.ai"

		resp, doErr := client.Do(outReq)
		if doErr != nil {
			// Network error with this proxy, mark error and failover
			_ = u.db.RecordProxyFailure(selectedProxy.ID, 0, false)
			lastErr = doErr
			continue
		}

		// If 429 rate limit
		if resp.StatusCode == http.StatusTooManyRequests {
			respBytes, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			cooldown := ParseResetDuration(resp.Header, string(respBytes))
			cooldownUntil := time.Now().Add(cooldown).Unix()
			_ = u.db.RecordProxyFailure(selectedProxy.ID, cooldownUntil, true)
			lastResp = resp
			finalBodyBytes = respBytes
			// Continue to next proxy in retry loop
			continue
		}

		// Success or other status
		_ = u.db.RecordProxySuccess(selectedProxy.ID)
		lastResp = resp
		break
	}

	if lastResp == nil {
		durationMs := int(time.Since(startTime).Milliseconds())
		errMsg := "All proxy attempts failed"
		if lastErr != nil {
			errMsg = lastErr.Error()
		}
		proxyURL := "none"
		if lastProxy != nil {
			proxyURL = lastProxy.URL
		}
		_ = u.db.InsertRequestLog(&db.RequestLog{
			Timestamp:    startTime.Unix(),
			Model:        reqSnippet.Model,
			ProxyURL:     proxyURL,
			StatusCode:   http.StatusBadGateway,
			DurationMs:   durationMs,
			TTFTMs:       0,
			ErrorMessage: errMsg,
			RequestBody:  string(bodyBytes),
		})
		http.Error(w, errMsg, http.StatusBadGateway)
		return
	}

	defer lastResp.Body.Close()

	// If it ended on 429 and all retries were exhausted
	if lastResp.StatusCode == http.StatusTooManyRequests && len(finalBodyBytes) > 0 {
		durationMs := int(time.Since(startTime).Milliseconds())
		copyHeaderWithoutHopByHop(w.Header(), lastResp.Header)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write(finalBodyBytes)

		proxyURL := "none"
		if lastProxy != nil {
			proxyURL = lastProxy.URL
		}
		_ = u.db.InsertRequestLog(&db.RequestLog{
			Timestamp:    startTime.Unix(),
			Model:        reqSnippet.Model,
			ProxyURL:     proxyURL,
			StatusCode:   http.StatusTooManyRequests,
			DurationMs:   durationMs,
			TTFTMs:       durationMs,
			ErrorMessage: string(finalBodyBytes),
			RequestBody:  string(bodyBytes),
			ResponseBody: string(finalBodyBytes),
		})
		return
	}

	// Copy response headers
	copyHeaderWithoutHopByHop(w.Header(), lastResp.Header)
	w.WriteHeader(lastResp.StatusCode)

	flusher, isFlusher := w.(http.Flusher)
	buf := make([]byte, 4096)
	var capturedOutput bytes.Buffer
	var tailOutput bytes.Buffer
	var ttftMs int

	for {
		n, rErr := lastResp.Body.Read(buf)
		if n > 0 {
			if ttftMs == 0 {
				ttftMs = int(time.Since(startTime).Milliseconds())
			}
			chunk := buf[:n]
			_, _ = w.Write(chunk)
			if isFlusher {
				flusher.Flush()
			}
			if capturedOutput.Len() < 64*1024 {
				capturedOutput.Write(chunk)
			}
			tailOutput.Write(chunk)
			if tailOutput.Len() > 32*1024 {
				b := tailOutput.Bytes()
				tailOutput.Reset()
				tailOutput.Write(b[len(b)-16*1024:])
			}
		}
		if rErr != nil {
			break
		}
	}

	durationMs := int(time.Since(startTime).Milliseconds())
	if ttftMs == 0 && lastResp.StatusCode >= 200 && lastResp.StatusCode < 300 {
		ttftMs = durationMs
	}
	capturedStr := capturedOutput.String()
	tailStr := tailOutput.String()

	checkStr := tailStr
	if checkStr == "" {
		checkStr = capturedStr
	}

	promptTokens := extractToken(promptTokensRegex, checkStr, capturedStr)
	compTokens := extractToken(compTokensRegex, checkStr, capturedStr)
	cachedTokens := extractToken(cachedTokensRegex, checkStr, capturedStr)
	reasoningTokens := extractToken(reasoningTokensRegex, checkStr, capturedStr)

	isStream := 0
	if reqSnippet.Stream || strings.Contains(lastResp.Header.Get("Content-Type"), "text/event-stream") {
		isStream = 1
	}

	proxyURL := "direct"
	if lastProxy != nil {
		proxyURL = lastProxy.URL
	}

	savings := u.calculateSavings(reqSnippet.Model, promptTokens, compTokens, cachedTokens)

	_ = u.db.InsertRequestLog(&db.RequestLog{
		Timestamp:        startTime.Unix(),
		Model:            reqSnippet.Model,
		ProxyURL:         proxyURL,
		StatusCode:       lastResp.StatusCode,
		DurationMs:       durationMs,
		TTFTMs:           ttftMs,
		PromptTokens:     promptTokens,
		CompletionTokens: compTokens,
		CachedTokens:     cachedTokens,
		ReasoningTokens:  reasoningTokens,
		IsStream:         isStream,
		RequestBody:      string(bodyBytes),
		ResponseBody:     capturedStr,
		EstimatedSavings: savings,
	})
}

func (u *Upstream) calculateSavings(modelID string, promptTokens, compTokens, cachedTokens int) float64 {
	if promptTokens == 0 && compTokens == 0 {
		return 0.0
	}
	m, err := u.db.GetModel(modelID)
	if err != nil || m == nil || m.CommercialRate == "" || m.CommercialRate == "{}" {
		return 0.0
	}

	var rate struct {
		Input     float64 `json:"input"`
		Output    float64 `json:"output"`
		CacheRead float64 `json:"cache_read"`
	}
	if err := json.Unmarshal([]byte(m.CommercialRate), &rate); err != nil {
		return 0.0
	}

	uncached := promptTokens - cachedTokens
	if uncached < 0 {
		uncached = 0
	}

	savings := (float64(uncached)*rate.Input + float64(cachedTokens)*rate.CacheRead + float64(compTokens)*rate.Output) / 1000000.0
	return savings
}
