package internal

import (
	"math/rand"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func response503() *http.Response {
	return &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Header:     map[string][]string{},
	}
}

func assertUndefinedDuration(t *testing.T, d OptionalDuration) {
	assert.NotNil(t, d)
	assert.Equal(t, false, d.Defined)
	assert.Equal(t, time.Duration(0), d.Duration)
}

func assertDuration(t *testing.T, duration OptionalDuration, expected time.Duration) {
	assert.NotNil(t, duration)
	assert.Equal(t, true, duration.Defined)

	// LessOrEqual to consider the time passes during the tests (actual duration would decrease in HTTP-date tests)
	assert.LessOrEqual(t, duration.Duration, expected)
}

func TestExtractRetryAfterHeaderDelaySeconds(t *testing.T) {
	// Generate random n > 0 int
	retryIntervalSec := rand.Intn(9999)

	// Generate a 503 status code response with Retry-After = n header
	resp := response503()
	resp.Header.Add(retryAfterHTTPHeader, strconv.Itoa(retryIntervalSec))

	expectedDuration := time.Second * time.Duration(retryIntervalSec)
	assertDuration(t, ExtractRetryAfterHeader(resp), expectedDuration)

	// Verify status code 429
	resp.StatusCode = http.StatusTooManyRequests
	assertDuration(t, ExtractRetryAfterHeader(resp), expectedDuration)

	// Verify different status code than {429, 503}
	resp.StatusCode = http.StatusBadGateway
	assertUndefinedDuration(t, ExtractRetryAfterHeader(resp))

	// Verify a zero duration is created for n < 0
	resp.StatusCode = http.StatusTooManyRequests
	resp.Header.Set(retryAfterHTTPHeader, strconv.Itoa(-1))
	assertDuration(t, ExtractRetryAfterHeader(resp), 0)
}

func TestExtractRetryAfterHeaderHttpDate(t *testing.T) {
	// Generate a random n > 0 second duration
	now := time.Now()
	retryIntervalSec := rand.Intn(9999)
	expectedDuration := time.Second * time.Duration(retryIntervalSec)

	// Set a response with Retry-After header = random n > 0 int
	resp := response503()
	retryAfter := now.Add(time.Second * time.Duration(retryIntervalSec)).UTC()

	// Verify HTTP-date TimeFormat format is being parsed correctly
	resp.Header.Set(retryAfterHTTPHeader, retryAfter.Format(http.TimeFormat))
	assertDuration(t, ExtractRetryAfterHeader(resp), expectedDuration)

	// Verify ANSI time format
	resp.Header.Set(retryAfterHTTPHeader, retryAfter.Format(time.ANSIC))
	assertDuration(t, ExtractRetryAfterHeader(resp), expectedDuration)

	// Verify RFC850 time format
	resp.Header.Set(retryAfterHTTPHeader, retryAfter.Format(time.RFC850))
	assertDuration(t, ExtractRetryAfterHeader(resp), expectedDuration)

	// Verify non HTTP-date RFC1123 format isn't being parsed
	resp.Header.Set(retryAfterHTTPHeader, retryAfter.Format(time.RFC1123))
	assertUndefinedDuration(t, ExtractRetryAfterHeader(resp))

	// Verify a zero duration is created for n = 0
	resp.Header.Set(retryAfterHTTPHeader, now.UTC().Format(http.TimeFormat))
	assertDuration(t, ExtractRetryAfterHeader(resp), 0)

	// Verify a zero duration is created for n < 0
	resp.Header.Set(retryAfterHTTPHeader, now.Add(-1*time.Second).UTC().Format(http.TimeFormat))
	assertDuration(t, ExtractRetryAfterHeader(resp), 0)
}
