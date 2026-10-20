package proxy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aip/internal/db"
	"golang.org/x/net/proxy"
)

type Pool struct {
	db         *db.DB
	rrIndex    uint64
	mu         sync.Mutex
	transports sync.Map // map[string]*http.Transport
}

func NewPool(database *db.DB) *Pool {
	return &Pool{
		db: database,
	}
}

// ParseProxy normalizes input into a db.Proxy model
func ParseProxy(raw string, defaultProto string) (*db.Proxy, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty proxy input")
	}

	if defaultProto == "" {
		defaultProto = "http"
	}
	defaultProto = strings.ToLower(defaultProto)

	// Direct connection
	if raw == "direct" || raw == "direct://localhost" {
		return &db.Proxy{
			RawInput:  raw,
			URL:       "direct://localhost",
			Protocol:  "direct",
			Host:      "localhost",
			Port:      0,
			Status:    "online",
			CreatedAt: time.Now().Unix(),
		}, nil
	}

	// Case 1: host:port:username:password
	parts := strings.Split(raw, ":")
	if len(parts) == 4 && !strings.Contains(parts[0], "/") && !strings.Contains(parts[0], "@") {
		host := parts[0]
		port, err := strconv.Atoi(parts[1])
		if err == nil && port > 0 && port <= 65535 {
			user := parts[2]
			pass := parts[3]
			parsedURL := fmt.Sprintf("%s://%s:%s@%s:%d", defaultProto, url.QueryEscape(user), url.QueryEscape(pass), host, port)
			return &db.Proxy{
				RawInput:  raw,
				URL:       parsedURL,
				Protocol:  defaultProto,
				Host:      host,
				Port:      port,
				Username:  user,
				Password:  pass,
				Status:    "online",
				CreatedAt: time.Now().Unix(),
			}, nil
		}
	}

	// Ensure protocol scheme
	norm := raw
	if strings.HasPrefix(strings.ToLower(norm), "relay://") {
		norm = "https://" + norm[len("relay://"):]
		defaultProto = "relay"
	} else if !strings.Contains(norm, "://") {
		norm = defaultProto + "://" + norm
	}

	u, err := url.Parse(norm)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proxy URL: %w", err)
	}

	protocol := strings.ToLower(u.Scheme)
	if defaultProto == "relay" || protocol == "relay" {
		protocol = "relay"
	} else if strings.HasSuffix(strings.ToLower(u.Hostname()), ".workers.dev") || strings.HasSuffix(strings.ToLower(u.Hostname()), ".vercel.app") {
		protocol = "relay"
	}

	if protocol != "http" && protocol != "https" && protocol != "socks5" && protocol != "socks5h" && protocol != "direct" && protocol != "relay" {
		return nil, fmt.Errorf("unsupported protocol %q", protocol)
	}

	host := u.Hostname()
	if host == "" {
		return nil, errors.New("missing host in proxy")
	}

	port := 0
	if pStr := u.Port(); pStr != "" {
		port, _ = strconv.Atoi(pStr)
	} else if protocol == "http" {
		port = 80
	} else if protocol == "https" || protocol == "relay" {
		port = 443
	} else if protocol == "socks5" || protocol == "socks5h" {
		port = 1080
	}

	var username, password string
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}

	canonicalURL := u.String()
	if protocol == "relay" {
		canonicalURL = fmt.Sprintf("https://%s", host)
		if port != 443 && port != 0 {
			canonicalURL = fmt.Sprintf("https://%s:%d", host, port)
		}
	}

	return &db.Proxy{
		RawInput:  raw,
		URL:       canonicalURL,
		Protocol:  protocol,
		Host:      host,
		Port:      port,
		Username:  username,
		Password:  password,
		Status:    "online",
		CreatedAt: time.Now().Unix(),
	}, nil
}

// CreateTransport configures an http.Transport with the proxy
func CreateTransport(p *db.Proxy) (*http.Transport, error) {
	if p == nil || p.Protocol == "direct" || p.Protocol == "relay" {
		return &http.Transport{
			Proxy: nil,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}, nil
	}

	if p.Protocol == "http" || p.Protocol == "https" {
		parsedURL, err := url.Parse(p.URL)
		if err != nil {
			return nil, err
		}
		return &http.Transport{
			Proxy: http.ProxyURL(parsedURL),
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}, nil
	}

	if p.Protocol == "socks5" || p.Protocol == "socks5h" {
		var auth *proxy.Auth
		if p.Username != "" || p.Password != "" {
			auth = &proxy.Auth{
				User:     p.Username,
				Password: p.Password,
			}
		}

		dialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("%s:%d", p.Host, p.Port), auth, proxy.Direct)
		if err != nil {
			return nil, err
		}

		return &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}, nil
	}

	return nil, fmt.Errorf("unsupported protocol: %s", p.Protocol)
}

// TestProxy probes https://cloudflare.com/cdn-cgi/trace for normal proxies,
// or tests the relay directly with x-relay-target/x-relay-path for relays.
func TestProxy(p *db.Proxy, timeout time.Duration) (int, string, error) {
	if timeout <= 0 {
		timeout = 8 * time.Second
	}

	if p != nil && p.Protocol == "relay" {
		client := &http.Client{
			Timeout: timeout,
		}
		req, err := http.NewRequestWithContext(context.Background(), "GET", p.URL, nil)
		if err != nil {
			return 0, "", err
		}
		req.Header.Set("x-relay-target", "https://opencode.ai")
		req.Header.Set("x-relay-path", "/zen/v1/models")
		req.Header.Set("Authorization", "Bearer public")
		req.Header.Set("User-Agent", "curl/8.5.0")
		req.Host = p.Host

		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			return 0, "", err
		}
		defer resp.Body.Close()

		duration := int(time.Since(start).Milliseconds())
		if resp.StatusCode != http.StatusOK {
			return duration, "", fmt.Errorf("unexpected status %d", resp.StatusCode)
		}

		egress := "relay-edge"
		if ray := resp.Header.Get("cf-ray"); ray != "" {
			parts := strings.Split(ray, "-")
			if len(parts) > 1 {
				egress = "cf-" + parts[1]
			} else {
				egress = "cloudflare"
			}
		} else if vID := resp.Header.Get("x-vercel-id"); vID != "" {
			parts := strings.Split(vID, "::")
			if len(parts) > 0 && parts[0] != "" {
				egress = "vercel-" + parts[0]
			} else {
				egress = "vercel"
			}
		}
		return duration, egress, nil
	}

	tr, err := CreateTransport(p)
	if err != nil {
		return 0, "", err
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", "https://cloudflare.com/cdn-cgi/trace", nil)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("User-Agent", "curl/8.5.0")

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	duration := int(time.Since(start).Milliseconds())

	if resp.StatusCode != http.StatusOK {
		return duration, "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	egressIP := ""
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ip=") {
			egressIP = strings.TrimPrefix(line, "ip=")
			break
		}
	}

	return duration, egressIP, nil
}

// GetNextProxy returns the next available online proxy using round-robin
func (pool *Pool) GetNextProxy() (*db.Proxy, error) {
	proxies, err := pool.db.ListProxies()
	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	var candidates []*db.Proxy

	for _, p := range proxies {
		// Auto recover if cooldown expired
		if p.Status == "rate_limited" && p.CooldownUntil > 0 && now >= p.CooldownUntil {
			p.Status = "online"
			p.CooldownUntil = 0
			_ = pool.db.UpdateProxyStatus(p.ID, "online", p.LatencyMs, p.EgressIP, 0, "")
		}

		if p.Status == "online" {
			candidates = append(candidates, p)
		}
	}

	if len(candidates) == 0 {
		return nil, errors.New("no online proxies available in pool")
	}

	idx := atomic.AddUint64(&pool.rrIndex, 1) - 1
	selected := candidates[idx%uint64(len(candidates))]
	return selected, nil
}

// GetTransport returns a cached *http.Transport for the given proxy, or creates and caches a new one.
func (pool *Pool) GetTransport(p *db.Proxy) (*http.Transport, error) {
	key := "direct"
	if p != nil && p.URL != "" {
		key = p.URL
	}

	if val, ok := pool.transports.Load(key); ok {
		return val.(*http.Transport), nil
	}

	tr, err := CreateTransport(p)
	if err != nil {
		return nil, err
	}

	actual, _ := pool.transports.LoadOrStore(key, tr)
	return actual.(*http.Transport), nil
}

// GetOnlineCandidates returns all currently eligible proxies for failover excluding excludeID
func (pool *Pool) GetOnlineCandidates(excludeID int64) []*db.Proxy {
	return pool.GetOnlineCandidatesExcluding(map[int64]bool{excludeID: true})
}

// GetOnlineCandidatesExcluding returns all eligible online proxies excluding any IDs in the excludeIDs set
func (pool *Pool) GetOnlineCandidatesExcluding(excludeIDs map[int64]bool) []*db.Proxy {
	proxies, err := pool.db.ListProxies()
	if err != nil {
		return nil
	}

	now := time.Now().Unix()
	var candidates []*db.Proxy
	for _, p := range proxies {
		if excludeIDs != nil && excludeIDs[p.ID] {
			continue
		}
		if p.Status == "rate_limited" && p.CooldownUntil > 0 && now >= p.CooldownUntil {
			p.Status = "online"
			p.CooldownUntil = 0
			_ = pool.db.UpdateProxyStatus(p.ID, "online", p.LatencyMs, p.EgressIP, 0, "")
		}
		if p.Status == "online" {
			candidates = append(candidates, p)
		}
	}
	return candidates
}
