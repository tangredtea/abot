package channels

import (
	"net/http"
	"time"
)

// DefaultHTTPClient is a shared HTTP client with reasonable timeout settings.
// Used by all channel implementations to prevent goroutine leaks from hanging requests.
var DefaultHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	},
}
