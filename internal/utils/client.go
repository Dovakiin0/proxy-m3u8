package utils

import (
	"maps"
	"net/http"
	"time"
)

var ProxyHTTPClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		DisableCompression:    false, // Enable automatic decompression
		ForceAttemptHTTP2:     true,  // Try HTTP/2 for better compatibility
		MaxConnsPerHost:       10,
		ResponseHeaderTimeout: 20 * time.Second,
	},
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		// Follow up to 10 redirects
		if len(via) >= 10 {
			return http.ErrUseLastResponse
		}
		// Copy headers to redirected request
		maps.Copy(req.Header, via[0].Header)
		return nil
	},
}
