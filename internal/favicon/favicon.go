package favicon

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	googleFaviconService = "https://www.google.com/s2/favicons"
	faviconSize         = "32"
	requestTimeout      = 2 * time.Second
)

// Fetcher handles favicon fetching
type Fetcher struct {
	client *http.Client
}

// New creates a new favicon fetcher
func New() *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: requestTimeout,
		},
	}
}

// FetchFaviconURL fetches the favicon for a given website URL and returns it as a Base64 data URI
// Returns the data URI or nil if not available
func (f *Fetcher) FetchFaviconURL(websiteURL string) *string {
	domain, err := extractDomain(websiteURL)
	if err != nil {
		return nil
	}

	faviconURL := fmt.Sprintf("%s?domain=%s&sz=%s", googleFaviconService, domain, faviconSize)

	// Fetch the favicon bytes
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", faviconURL, nil)
	if err != nil {
		return nil
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	// Read the favicon bytes
	faviconBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	// Don't cache if the response is empty or suspiciously small
	if len(faviconBytes) < 100 {
		return nil
	}

	// Encode to Base64 and create data URI
	encoded := base64.StdEncoding.EncodeToString(faviconBytes)

	// Determine content type from response header, default to png
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/png"
	}

	dataURI := fmt.Sprintf("data:%s;base64,%s", contentType, encoded)
	return &dataURI
}

// extractDomain extracts the domain from a URL
func extractDomain(rawURL string) (string, error) {
	// Add scheme if missing
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	domain := parsedURL.Hostname()
	if domain == "" {
		return "", fmt.Errorf("no domain found in URL")
	}

	return domain, nil
}
