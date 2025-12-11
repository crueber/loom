package oauth

import (
	"context"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Client handles OAuth2/OIDC authentication
type Client struct {
	provider *oidc.Provider
	config   *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// UserInfo contains user information extracted from ID token
type UserInfo struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Sub           string `json:"sub"`
}

// NewClient creates a new OAuth2/OIDC client with auto-discovery
func NewClient(issuerURL, clientID, clientSecret, redirectURL string) (*Client, error) {
	ctx := context.Background()

	// Strip .well-known/openid-configuration if accidentally included
	// (Authentik UI provides the full URL, but the library appends it automatically)
	issuerURL = strings.TrimSuffix(issuerURL, "/.well-known/openid-configuration")
	issuerURL = strings.TrimSuffix(issuerURL, ".well-known/openid-configuration")

	// OIDC Discovery - automatically discovers endpoints from .well-known/openid-configuration
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get OIDC provider: %w", err)
	}

	// OAuth2 config
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	// ID token verifier
	verifier := provider.Verifier(&oidc.Config{ClientID: clientID})

	return &Client{
		provider: provider,
		config:   config,
		verifier: verifier,
	}, nil
}

// AuthCodeURL returns the OAuth2 authorization URL with state parameter
func (c *Client) AuthCodeURL(state string) string {
	return c.config.AuthCodeURL(state)
}

// Exchange exchanges the authorization code for an OAuth2 token
func (c *Client) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return c.config.Exchange(ctx, code)
}

// VerifyIDToken verifies and returns the ID token
func (c *Client) VerifyIDToken(ctx context.Context, rawIDToken string) (*oidc.IDToken, error) {
	return c.verifier.Verify(ctx, rawIDToken)
}

// GetUserInfo extracts user information from ID token claims
func (c *Client) GetUserInfo(ctx context.Context, idToken *oidc.IDToken) (*UserInfo, error) {
	var claims UserInfo
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}
	return &claims, nil
}
