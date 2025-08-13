package internal

import (
	"errors"
	"net/http"
	"strconv"
	"time"
)

const retryAfterHTTPHeader = "Retry-After"

var errCouldNotParseRetryAfterHeader = errors.New("could not parse" + retryAfterHTTPHeader + "header")

type OptionalDuration struct {
	Duration time.Duration
	// true if duration field is defined.
	Defined bool
}

func parseDelaySeconds(s string) (time.Duration, error) {
	// Verify duration parsed properly
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, errCouldNotParseRetryAfterHeader
	}

	// If n > 0 return n seconds, otherwise return 0
	if n > 0 {
		duration := time.Duration(n) * time.Second
		return duration, nil
	}
	return 0, nil
}

func parseHTTPDate(s string) (time.Duration, error) {
	// Verify duration parsed properly
	t, err := http.ParseTime(s)
	if err != nil {
		return 0, errCouldNotParseRetryAfterHeader
	}

	// If the date is in the future return that duration, otherwise return 0
	if duration := time.Until(t); duration > 0 {
		return duration, nil
	}
	return 0, nil
}

// ExtractRetryAfterHeader extracts Retry-After response header if the status
// is 503 or 429. Returns 0 duration if the header is not found or the status
// is different.
func ExtractRetryAfterHeader(resp *http.Response) OptionalDuration {
	if resp.StatusCode == http.StatusServiceUnavailable ||
		resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := resp.Header.Get(retryAfterHTTPHeader)
		if retryAfter != "" {
			// Parse delay-seconds https://datatracker.ietf.org/doc/html/rfc7231#section-7.1.3
			retryInterval, err := parseDelaySeconds(retryAfter)
			if err == nil {
				return OptionalDuration{Defined: true, Duration: retryInterval}
			}
			// Parse HTTP-date https://datatracker.ietf.org/doc/html/rfc7231#section-7.1.3
			retryInterval, err = parseHTTPDate(retryAfter)
			if err == nil {
				return OptionalDuration{Defined: true, Duration: retryInterval}
			}
		}
	}
	return OptionalDuration{Defined: false}
}
