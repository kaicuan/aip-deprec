package upstream

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"aip/internal/config"
	"aip/internal/db"
	"aip/internal/proxy"
	"aip/internal/ratelimit"
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

var usageRegex = regexp.MustCompile(`"usage"\s*:\s*\{([^}]+)\}`)
var promptTokensRegex = regexp.MustCompile(`"(?:prompt_tokens|input_tokens)"\s*:\s*(\d+)`)
var compTokensRegex = regexp.MustCompile(`"(?:completion_tokens|output_tokens)"\s*:\s*(\d+)`)
var cachedTokensRegex = regexp.MustCompile(`"(?:cached_tokens|cache_read_input_tokens)"\s*:\s*(\d+)`)

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

	sessionID := r.Header.Get("x-opencode-session")
	if sessionID == "" {
		sessionID = r.Header.Get("x-session-affinity")
	}
	if sessionID == "" {
		sessionID = r.Header.Get("X-Session-Id")
	}
	targetURL := strings.TrimRight(u.baseURL, "/") + "/" + strings.TrimLeft(pathSuffix, "/")

	startTime := time.Now()
	var lastProxy *db.Proxy
	var lastResp *http.Response
	var lastErr error
	var finalBodyBytes []byte

	// Try with failover
	maxRetries := 3
	proxies, _ := u.db.ListProxies()
	if len(proxies) > 0 && len(proxies) < maxRetries {
		maxRetries = len(proxies)
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		var selectedProxy *db.Proxy
		if attempt == 0 {
			selectedProxy, err = u.pool.GetNextProxy()
		} else {
			candidates := u.pool.GetOnlineCandidates(lastProxy.ID)
			if len(candidates) > 0 {
				selectedProxy = candidates[attempt%len(candidates)]
			} else {
				break
			}
		}

		if err != nil || selectedProxy == nil {
			break
		}
		lastProxy = selectedProxy

		transport, trErr := proxy.CreateTransport(selectedProxy)
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
		for k, vv := range r.Header {
			// Skip hop-by-hop headers
			if strings.EqualFold(k, "Transfer-Encoding") || strings.EqualFold(k, "Connection") {
				continue
			}
			for _, v := range vv {
				outReq.Header.Add(k, v)
			}
		}

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
			cooldown := ratelimit.ParseResetDuration(resp.Header, string(respBytes))
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
			ErrorMessage: errMsg,
			SessionID:    sessionID,
			RequestBody:  string(bodyBytes),
		})
		http.Error(w, errMsg, http.StatusBadGateway)
		return
	}

	defer lastResp.Body.Close()

	// If it ended on 429 and all retries were exhausted
	if lastResp.StatusCode == http.StatusTooManyRequests && len(finalBodyBytes) > 0 {
		durationMs := int(time.Since(startTime).Milliseconds())
		for k, vv := range lastResp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
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
			ErrorMessage: string(finalBodyBytes),
			SessionID:    sessionID,
			RequestBody:  string(bodyBytes),
			ResponseBody: string(finalBodyBytes),
		})
		return
	}

	// Copy response headers
	for k, vv := range lastResp.Header {
		if strings.EqualFold(k, "Transfer-Encoding") || strings.EqualFold(k, "Connection") {
			continue
		}
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(lastResp.StatusCode)

	flusher, isFlusher := w.(http.Flusher)
	buf := make([]byte, 4096)
	var capturedOutput bytes.Buffer
	var tailOutput bytes.Buffer
	var promptTokens, compTokens, cachedTokens int

	for {
		n, rErr := lastResp.Body.Read(buf)
		if n > 0 {
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
	capturedStr := capturedOutput.String()
	tailStr := tailOutput.String()

	checkStr := tailStr
	if checkStr == "" {
		checkStr = capturedStr
	}

	// Extract token metrics if present
	if m := promptTokensRegex.FindStringSubmatch(checkStr); len(m) > 1 {
		fmt.Sscanf(m[1], "%d", &promptTokens)
	} else if m := promptTokensRegex.FindStringSubmatch(capturedStr); len(m) > 1 {
		fmt.Sscanf(m[1], "%d", &promptTokens)
	}

	if m := compTokensRegex.FindStringSubmatch(checkStr); len(m) > 1 {
		fmt.Sscanf(m[1], "%d", &compTokens)
	} else if m := compTokensRegex.FindStringSubmatch(capturedStr); len(m) > 1 {
		fmt.Sscanf(m[1], "%d", &compTokens)
	}

	if m := cachedTokensRegex.FindStringSubmatch(checkStr); len(m) > 1 {
		fmt.Sscanf(m[1], "%d", &cachedTokens)
	} else if m := cachedTokensRegex.FindStringSubmatch(capturedStr); len(m) > 1 {
		fmt.Sscanf(m[1], "%d", &cachedTokens)
	}

	isStream := 0
	if reqSnippet.Stream || strings.Contains(lastResp.Header.Get("Content-Type"), "text/event-stream") {
		isStream = 1
	}

	proxyURL := "direct"
	if lastProxy != nil {
		proxyURL = lastProxy.URL
	}

	_ = u.db.InsertRequestLog(&db.RequestLog{
		Timestamp:        startTime.Unix(),
		Model:            reqSnippet.Model,
		ProxyURL:         proxyURL,
		StatusCode:       lastResp.StatusCode,
		DurationMs:       durationMs,
		PromptTokens:     promptTokens,
		CompletionTokens: compTokens,
		CacheReadTokens:  cachedTokens,
		IsStream:         isStream,
		SessionID:        sessionID,
		RequestBody:      string(bodyBytes),
		ResponseBody:     capturedStr,
	})
}
