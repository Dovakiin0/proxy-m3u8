// go-m3u8-proxy/utils/m3u8_modifier.go
package utils

import (
	"bufio"
	"io"
	"net/url"
	"path"
	"strings"
)

// AllowedExtensions defines extensions for files that might be referenced and proxied.
// These are files that, if not m3u8 or ts, are proxied as-is.
var AllowedExtensions = []string{".png", ".jpg", ".webp", ".ico", ".html", ".js", ".css", ".txt"} // .ts and .m3u8 handled separately

// IsAllowedStaticExtension checks if the line ends with one of the non-M3U8/TS static file extensions.
func IsAllowedStaticExtension(line string) bool {
	for _, ext := range AllowedExtensions {
		if strings.HasSuffix(line, ext) {
			return true
		}
	}
	return false
}

func IsStaticFileExtension(path string) bool {
	lowerPath := strings.ToLower(path)
	for _, ext := range AllowedExtensions {
		if strings.HasSuffix(lowerPath, ext) {
			return true
		}
	}
	return false
}

// ProcessM3U8Stream reads an M3U8 stream, transforms relevant lines, and writes to the output stream.
// proxyPrefix is the prefix for rewritten URLs, e.g., "m3u8-proxy?url="
// referer is propagated as a &referer= param on every rewritten URL so downstream requests carry the correct origin
func ProcessM3U8Stream(reader io.Reader, writer io.Writer, originalM3U8URL, proxyPrefix, referer string) error {
	scanner := bufio.NewScanner(reader)
	parsedBaseURL, err := url.Parse(originalM3U8URL)
	if err != nil {
		// If originalM3U8URL is not a valid URL, we might not be able to resolve relative paths correctly.
		// For simplicity, we'll try to proceed, but this could be an issue for malformed original URLs.
		// A robust solution might return an error here or have a fallback.
		// For now, let's assume originalM3U8URL is well-formed for baseUrl calculation.
	}

	// Calculate baseUrl: scheme://host/path/to/
	var baseUrlForRelativePaths string
	if parsedBaseURL != nil {
		// Get the directory part of the URL
		parsedBaseURL.Path = path.Dir(parsedBaseURL.Path)
		if !strings.HasSuffix(parsedBaseURL.Path, "/") {
			parsedBaseURL.Path += "/"
		}
		baseUrlForRelativePaths = parsedBaseURL.String()
	}

	refererSuffix := ""
	if referer != "" {
		refererSuffix = "&referer=" + url.QueryEscape(referer)
	}

	for scanner.Scan() {
		line := scanner.Text()
		modifiedLine := line

		// Trim whitespace from the line for accurate suffix checking
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "#") || trimmedLine == "" {
			// It's a comment or empty line, pass through
			modifiedLine = line
		} else if strings.HasSuffix(trimmedLine, ".m3u8") || strings.HasSuffix(trimmedLine, ".ts") {
			// These are segments or nested playlists, assumed relative to the M3U8's base URL
			if isAbsoluteURL(trimmedLine) {
				modifiedLine = proxyPrefix + url.QueryEscape(trimmedLine) + refererSuffix
			} else {
				// Construct absolute URL from baseUrlForRelativePaths and the relative line
				absoluteSegmentURL := resolveURL(baseUrlForRelativePaths, trimmedLine)
				modifiedLine = proxyPrefix + url.QueryEscape(absoluteSegmentURL) + refererSuffix
			}
		} else if IsAllowedStaticExtension(trimmedLine) {
			if isAbsoluteURL(trimmedLine) {
				modifiedLine = proxyPrefix + url.QueryEscape(trimmedLine) + refererSuffix
			} else {
				absoluteResourceURL := resolveURL(baseUrlForRelativePaths, trimmedLine)
				modifiedLine = proxyPrefix + url.QueryEscape(absoluteResourceURL) + refererSuffix
			}
		}

		if _, err := io.WriteString(writer, modifiedLine+"\n"); err != nil {
			return err
		}
	}

	return scanner.Err()
}

func isAbsoluteURL(line string) bool {
	return strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://")
}

// resolveURL resolves a relative path against a base URL.
// If relativePath is already absolute, it's returned as is.
func resolveURL(baseURLStr, relativePath string) string {
	if isAbsoluteURL(relativePath) {
		return relativePath
	}

	base, err := url.Parse(baseURLStr)
	if err != nil {
		return relativePath
	}

	relative, err := url.Parse(relativePath)
	if err != nil {
		return relativePath
	}

	return base.ResolveReference(relative).String()
}
