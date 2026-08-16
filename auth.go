package daraja

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Refresh tokens this long before they expire so concurrent callers never
// observe an already-expired token.
const tokenExpirySkew = 30 * time.Second

// AuthResponse is the OAuth client-credentials payload returned by Daraja.
type AuthResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   string `json:"expires_in"`
}

func (c *Client) basicAuthHeader() string {
	credentials := c.consumerKey + ":" + c.consumerSecret
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(credentials))
}

func (c *Client) cachedTokenLocked() *AuthResponse {
	if c.accessToken == "" {
		return nil
	}
	if !c.currentTime().Add(tokenExpirySkew).Before(c.tokenExpiry) {
		return nil
	}
	return &AuthResponse{
		AccessToken: c.accessToken,
		ExpiresIn:   c.tokenExpires,
	}
}

func (c *Client) storeTokenLocked(auth *AuthResponse, fetchedAt time.Time) error {
	expiresIn, err := strconv.Atoi(auth.ExpiresIn)
	if err != nil {
		return fmt.Errorf("parse expires_in: %w", err)
	}
	if expiresIn <= 0 {
		return fmt.Errorf("expires_in must be greater than 0")
	}
	if auth.AccessToken == "" {
		return fmt.Errorf("access_token is empty")
	}

	c.accessToken = auth.AccessToken
	c.tokenExpires = auth.ExpiresIn
	c.tokenExpiry = fetchedAt.Add(time.Duration(expiresIn) * time.Second)
	return nil
}

func (c *Client) fetchAccessToken(ctx context.Context) (*AuthResponse, error) {
	if c.consumerKey == "" {
		return nil, fmt.Errorf("consumer key is required")
	}
	if c.consumerSecret == "" {
		return nil, fmt.Errorf("consumer secret is required")
	}
	if c.baseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	authURL := c.endpoint("/oauth/v1/generate?grant_type=client_credentials")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create auth request: %w", err)
	}

	req.Header.Set("Authorization", c.basicAuthHeader())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request access token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("daraja returned status %s", resp.Status)
	}

	var authResponse AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResponse); err != nil {
		return nil, fmt.Errorf("decode access token response: %w", err)
	}

	return &authResponse, nil
}

// GetAccessToken returns a cached OAuth token, or fetches a new one.
//
// Concurrent callers share one mutex: a valid cached token is returned
// immediately, and a refresh is performed while holding the lock so only one
// HTTP request is in flight. That is simpler than singleflight and avoids a
// stampede of identical token requests.
func (c *Client) GetAccessToken(ctx context.Context) (*AuthResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cached := c.cachedTokenLocked(); cached != nil {
		return cached, nil
	}

	fetchedAt := c.currentTime()
	authResponse, err := c.fetchAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.storeTokenLocked(authResponse, fetchedAt); err != nil {
		return nil, err
	}

	return authResponse, nil
}
