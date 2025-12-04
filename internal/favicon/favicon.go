package favicon

import (
	"context"
	"fmt"
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

// FetchFaviconURL fetches the favicon URL for a given website URL
// Returns the favicon URL or nil if not available
func (f *Fetcher) FetchFaviconURL(websiteURL string) *string {
	domain, err := extractDomain(websiteURL)
	if err != nil {
		return nil
	}

	faviconURL := fmt.Sprintf("%s?domain=%s&sz=%s", googleFaviconService, domain, faviconSize)

	// Test if the favicon URL is accessible
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

	if resp.StatusCode == http.StatusOK {
		return &faviconURL
	}

	return nil
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
