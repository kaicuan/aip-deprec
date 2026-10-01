package upstream

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	// regex for "resets in 2 hours 15 minutes", "resets in 45m", "try again in 1h 30m", "resets in 45 seconds"
	hourMinSecRegex = regexp.MustCompile(`(?i)(?:resets? in|try again in|wait|retry after)\s*(?:(\d+)\s*(?:hours?|hrs?|h))?\s*(?:(\d+)\s*(?:minutes?|mins?|m))?\s*(?:(\d+)\s*(?:seconds?|secs?|s))?`)
)

// ParseResetDuration parses headers and body of a 429 response to find cooldown seconds
func ParseResetDuration(respHeaders http.Header, body string) time.Duration {
	defaultDuration := 5 * time.Minute

	if respHeaders != nil {
		// 1. Check Retry-After
		if retryAfter := respHeaders.Get("Retry-After"); retryAfter != "" {
			if secs, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && secs > 0 {
				return time.Duration(secs) * time.Second
			}
			if targetTime, err := http.ParseTime(retryAfter); err == nil {
				d := time.Until(targetTime)
				if d > 0 {
					return d
				}
			}
		}

		// 2. Check x-ratelimit-reset (can be unix timestamp or seconds)
		for _, key := range []string{"x-ratelimit-reset", "ratelimit-reset", "x-ratelimit-reset-requests"} {
			if val := respHeaders.Get(key); val != "" {
				if num, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64); err == nil && num > 0 {
					now := time.Now().Unix()
					if num > now {
						// Likely unix timestamp in seconds
						return time.Duration(num-now) * time.Second
					} else if num > now*1000 {
						// Likely unix timestamp in milliseconds
						return time.Duration((num - now*1000) / 1000) * time.Second
					} else if num < 86400*30 {
						// Likely relative seconds
						return time.Duration(num) * time.Second
					}
				}
			}
		}
	}

	// 3. Scan body text
	if body != "" {
		if d := parseDurationFromBody(body); d > 0 {
			return d
		}
	}

	return defaultDuration
}

func parseDurationFromBody(body string) time.Duration {
	matches := hourMinSecRegex.FindStringSubmatch(body)
	if len(matches) == 0 {
		return 0
	}

	var hours, mins, secs int
	if matches[1] != "" {
		hours, _ = strconv.Atoi(matches[1])
	}
	if matches[2] != "" {
		mins, _ = strconv.Atoi(matches[2])
	}
	if matches[3] != "" {
		secs, _ = strconv.Atoi(matches[3])
	}

	totalSecs := hours*3600 + mins*60 + secs
	if totalSecs > 0 {
		return time.Duration(totalSecs) * time.Second
	}

	return 0
}
