// Package daraja is a client for the Safaricom Daraja (M-Pesa) APIs.
package daraja

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

const defaultHTTPTimeout = 30 * time.Second

// ClientOption configures a Client.
type ClientOption func(*Client)

// Client is a Daraja API client.
type Client struct {
	consumerKey    string
	consumerSecret string
	baseURL        string

	businessShortCode string
	passkey           string
	callbackURL       string

	httpClient *http.Client

	mu           sync.Mutex
	accessToken  string
	tokenExpiry  time.Time
	tokenExpires string

	now func() time.Time
}

// NewClient returns a Client for the given consumer credentials and Daraja base URL.
//
// baseURL is typically https://sandbox.safaricom.co.ke or https://api.safaricom.co.ke.
func NewClient(consumerKey, consumerSecret, baseURL string, opts ...ClientOption) *Client {
	client := &Client{
		consumerKey:    consumerKey,
		consumerSecret: consumerSecret,
		baseURL:        strings.TrimRight(baseURL, "/"),
		httpClient:     &http.Client{Timeout: defaultHTTPTimeout},
		now:            time.Now,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

// WithBusinessShortCode sets the Lipa Na M-Pesa business short code.
func WithBusinessShortCode(shortCode string) ClientOption {
	return func(c *Client) {
		c.businessShortCode = shortCode
	}
}

// WithPasskey sets the Lipa Na M-Pesa passkey used to generate STK passwords.
func WithPasskey(passkey string) ClientOption {
	return func(c *Client) {
		c.passkey = passkey
	}
}

// WithCallbackURL sets the URL Daraja will POST STK results to.
func WithCallbackURL(callbackURL string) ClientOption {
	return func(c *Client) {
		c.callbackURL = callbackURL
	}
}

// WithHTTPClient replaces the Client's HTTP client.
//
// This is the supported way to set timeouts, transport, or a test server client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

func (c *Client) endpoint(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return c.baseURL + path
}

func (c *Client) currentTime() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}
