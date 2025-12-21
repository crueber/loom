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
	selfhstIconsService  = "https://cdn.jsdelivr.net/gh/selfhst/icons@main/webp"
	simpleIconsService   = "https://cdn.simpleicons.org"
	faviconSize          = "32"
	requestTimeout       = 2 * time.Second
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

// FetchFromDomain fetches favicon from the website's domain using Google's service
func (f *Fetcher) FetchFromDomain(domain string) (*string, error) {
	faviconURL := fmt.Sprintf("%s?domain=%s&sz=%s", googleFaviconService, domain, faviconSize)
	return f.fetchAndEncode(faviconURL)
}

// FetchFromCustomURL fetches an icon from a user-provided URL
func (f *Fetcher) FetchFromCustomURL(iconURL string) (*string, error) {
	return f.fetchAndEncode(iconURL)
}

// FetchFromService fetches an icon from an icon service using a slug
// Tries selfh.st/icons first, then falls back to Simple Icons
func (f *Fetcher) FetchFromService(customURL string) (*string, error) {
	// If user provides full URL, use it directly
	if strings.HasPrefix(customURL, "http://") || strings.HasPrefix(customURL, "https://") {
		return f.FetchFromCustomURL(customURL)
	}

	// Otherwise treat as slug
	slug := strings.TrimSpace(customURL)
	if slug == "" {
		return nil, fmt.Errorf("empty icon slug")
	}

	// Try selfh.st first
	selfhstURL := fmt.Sprintf("%s/%s.webp", selfhstIconsService, slug)
	icon, err := f.fetchAndEncode(selfhstURL)
	if err == nil && icon != nil {
		return icon, nil
	}

	// Fallback to Simple Icons
	simpleIconsURL := fmt.Sprintf("%s/%s", simpleIconsService, slug)
	return f.fetchAndEncode(simpleIconsURL)
}

// FetchIcon determines which fetch method to use based on icon source
func (f *Fetcher) FetchIcon(iconSource string, customURL *string, domain string) (*string, error) {
	switch iconSource {
	case "loom":
		iconURL := "/static/favicon-32x32.png"
		return &iconURL, nil
	case "custom":
		if customURL == nil || *customURL == "" {
			return nil, fmt.Errorf("custom icon URL required")
		}
		return f.FetchFromCustomURL(*customURL)
	case "service":
		if customURL == nil || *customURL == "" {
			return nil, fmt.Errorf("icon service slug required")
		}
		return f.FetchFromService(*customURL)
	case "auto":
		fallthrough
	default:
		if domain == "" {
			return nil, fmt.Errorf("domain required for auto icon source")
		}
		return f.FetchFromDomain(domain)
	}
}

// fetchAndEncode fetches an icon from a URL and returns it as a Base64 data URI
func (f *Fetcher) fetchAndEncode(iconURL string) (*string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", iconURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch icon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch icon: status %d", resp.StatusCode)
	}

	// Read the icon bytes
	iconBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read icon: %w", err)
	}

	// Don't cache if the response is empty or suspiciously small
	if len(iconBytes) < 100 {
		return nil, fmt.Errorf("icon too small: %d bytes", len(iconBytes))
	}

	// Encode to Base64 and create data URI
	encoded := base64.StdEncoding.EncodeToString(iconBytes)

	// Determine content type from response header, default to png
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/png"
	}

	dataURI := fmt.Sprintf("data:%s;base64,%s", contentType, encoded)
	return &dataURI, nil
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
