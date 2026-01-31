package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dovakiin0/proxy-m3u8/config"
	"github.com/dovakiin0/proxy-m3u8/internal/utils"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// Redis client for caching responses
var (
	ctx = config.Ctx
)

// CachedResponse represents a cached response structure
type CachedResponse struct {
	StatusCode int         `json:"status_code"`
	Headers    http.Header `json:"headers"`
	Body       []byte      `json:"body"`
}

const cacheTTL = 1 * time.Hour // Cache expiration time

// generateCacheKey creates a unique cache key based on the target URL and header parameters
func generateCacheKey(targetURL string) string {
	return "m3u8proxy_cache:" + targetURL
}

func M3U8ProxyHandler(c echo.Context) error {
	if config.Client == nil {
		log.Println("Redis client is not initialized. Cache will be ignored.")
	}

	/*
		########################################################################################
		#              Get the target URL and referer from query parameters
		########################################################################################
	*/
	targetURL := c.QueryParam("url")
	if targetURL == "" {
		return c.String(http.StatusBadRequest, "Missing 'url' query parameter")
	}

	referer := c.QueryParam("referer")
	var refererHeader string
	if referer != "" {
		unscaped, err := url.QueryUnescape(referer)
		if err != nil {
			log.Printf("Error unescaping referer: %v", err)
			return c.String(http.StatusBadRequest, "Invalid 'referer' query parameter")
		}
		refererHeader = unscaped
	}

	/*
		########################################################################################
		#                          Check cache for existing response
		########################################################################################
	*/
	// if cache exists, we will use it
	cacheKey := generateCacheKey(targetURL)
	var cachedData CachedResponse

	if config.IsAvailable {
		val, err := config.Client.Get(ctx, cacheKey).Result()
		if err == nil { // Cache hit
			err = json.Unmarshal([]byte(val), &cachedData)
			if err == nil {
				log.Printf("CACHE HIT for %s", targetURL)
				// Apply cached headers
				for key, values := range cachedData.Headers {
					for _, value := range values {
						c.Response().Header().Add(key, value)
					}
				}
				c.Response().WriteHeader(cachedData.StatusCode)
				_, err = c.Response().Writer.Write(cachedData.Body)
				if err != nil {
					log.Printf("Error writing cached response body for %s: %v", targetURL, err)
				}
				return nil // Served from cache
			}
			log.Printf("Error unmarshalling cached data for %s: %v. Fetching from origin.", targetURL, err)
			// Proceed to fetch from origin if unmarshal fails
		} else if err != redis.Nil {
			log.Printf("Redis GET error for key %s: %v. Fetching from origin.", cacheKey, err)
			// Proceed to fetch from origin on other Redis errors
		} else {
			log.Printf("CACHE MISS for %s", targetURL)
		}
	}

	_, err := url.ParseRequestURI(targetURL)
	if err != nil {
		log.Printf("Invalid target URL: %s, error: %v", targetURL, err)
		return c.String(http.StatusBadRequest, "Invalid 'url' query parameter")
	}
	isM3U8 := strings.HasSuffix(strings.ToLower(targetURL), ".m3u8")
	isTS := strings.HasSuffix(strings.ToLower(targetURL), ".ts")
	isOtherStatic := utils.IsStaticFileExtension(targetURL)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		log.Printf("Error creating request to target %s: %v", targetURL, err)
		return c.String(http.StatusInternalServerError, "Failed to create request to target server")
	}

	// Parse target URL to extract origin
	parsedURL, _ := url.Parse(targetURL)
	targetOrigin := parsedURL.Scheme + "://" + parsedURL.Host

	// Set browser-like headers to bypass Cloudflare
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	// Don't set Accept-Encoding manually - let Go's HTTP client handle compression automatically
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Ch-Ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)

	// if the referer is provided, set it in the request headers
	if refererHeader != "" {
		req.Header.Set("Referer", refererHeader)
		req.Header.Set("Origin", refererHeader)
	} else {
		// Use the target's own origin as referer to appear as same-origin request
		req.Header.Set("Referer", targetOrigin+"/")
		req.Header.Set("Origin", targetOrigin)
	}

	upstreamResp, err := utils.ProxyHTTPClient.Do(req)
	if err != nil {
		log.Printf("Error fetching target URL %s: %v", targetURL, err)
		// Check for timeout or other specific errors if needed
		if urlErr, ok := err.(*url.Error); ok && urlErr.Timeout() {
			return c.String(http.StatusGatewayTimeout, "Upstream server timed out")
		}
		return c.String(http.StatusBadGateway, "Failed to fetch content from upstream server")
	}
	defer upstreamResp.Body.Close()

	rawBodyBytes, err := io.ReadAll(upstreamResp.Body)
	if err != nil {
		log.Printf("Error reading response body from upstream %s: %v", targetURL, err)
		return c.String(http.StatusInternalServerError, "Failed to read response from upstream server")
	}

	var responseBodyBytes []byte
	responseHeadersToClient := http.Header{}

	// Whitelist headers to copy
	headerWhitelist := []string{
		"Content-Type", "Content-Disposition", "Accept-Ranges", "Content-Range",
	}
	if upstreamResp.StatusCode == http.StatusOK || upstreamResp.StatusCode == http.StatusPartialContent {
		headerWhitelist = append(headerWhitelist, "ETag", "Last-Modified")
	}

	for _, hName := range headerWhitelist {
		if hVal := upstreamResp.Header.Get(hName); hVal != "" {
			// Add the header to the response headers to be sent to the client
			responseHeadersToClient.Set(hName, hVal)
		}
	}

	if (isM3U8 || isTS) && upstreamResp.StatusCode == http.StatusOK {
		var transformedBodyBuffer bytes.Buffer
		proxyRoutePath := strings.TrimPrefix(c.Path(), "/")
		urlPrefix := proxyRoutePath + "?url="

		err = utils.ProcessM3U8Stream(bytes.NewReader(rawBodyBytes), &transformedBodyBuffer, targetURL, urlPrefix)
		if err != nil {
			log.Printf("Error processing M3U8 stream for %s: %v", targetURL, err)
			return c.String(http.StatusInternalServerError, "Error transforming M3U8 content")
		}
		responseBodyBytes = transformedBodyBuffer.Bytes()
		// Content-Length is not set from upstream as body is transformed
	} else {
		// No transformation or non-OK status
		responseBodyBytes = rawBodyBytes
		// Set Content-Length from upstream if it's a static file or non-OK response and CL is present
		if (isOtherStatic || upstreamResp.StatusCode != http.StatusOK) && upstreamResp.Header.Get("Content-Length") != "" {
			responseHeadersToClient.Set("Content-Length", upstreamResp.Header.Get("Content-Length"))
		}
	}

	// Prepare bodyToServe for sending to client
	bodyToServe := bytes.NewReader(responseBodyBytes)
	for key, values := range responseHeadersToClient {
		for _, value := range values {
			c.Response().Header().Set(key, value)
		}
	}

	c.Response().WriteHeader(upstreamResp.StatusCode)

	_, err = io.Copy(c.Response().Writer, bodyToServe)
	if err != nil {
		log.Printf("Error writing response body to client for %s: %v", targetURL, err)
	}

	/*
		########################################################################################
		#                          Cache the response if Redis is available
		########################################################################################
	*/
	if config.IsAvailable && (upstreamResp.StatusCode == http.StatusOK || upstreamResp.StatusCode == http.StatusPartialContent) {
		// We need to cache the headers that we sent to the client.
		// So, use c.Response().Header() (after they've been set, but before body is fully written).
		// Or more reliably, use the `responseHeadersToClient` we constructed.

		cacheEntry := CachedResponse{
			StatusCode: upstreamResp.StatusCode,
			Headers:    responseHeadersToClient, // Use the headers we decided to send
			Body:       responseBodyBytes,       // This is the (potentially transformed) body
		}
		jsonData, err := json.Marshal(cacheEntry)
		if err != nil {
			log.Printf("Error marshalling data for Redis cache for %s: %v", targetURL, err)
		} else {
			err = config.Client.Set(ctx, cacheKey, jsonData, cacheTTL).Err()
			if err != nil {
				log.Printf("Redis SET error for key %s: %v", cacheKey, err)
			} else {
				log.Printf("CACHED %s (key: %s)", targetURL, cacheKey)
			}
		}
	}

	return nil
}
