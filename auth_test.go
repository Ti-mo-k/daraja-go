package daraja

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.Handler, opts ...ClientOption) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	allOpts := append([]ClientOption{WithHTTPClient(server.Client())}, opts...)
	return NewClient("key", "secret", server.URL, allOpts...)
}

func authHandler(t *testing.T, token, expiresIn string, calls *atomic.Int32) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if calls != nil {
			calls.Add(1)
		}
		if r.Method != http.MethodGet {
			t.Errorf("auth method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/oauth/v1/generate" {
			t.Errorf("auth path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("grant_type") != "client_credentials" {
			t.Errorf("grant_type = %s", r.URL.Query().Get("grant_type"))
		}
		if got := r.Header.Get("Authorization"); got == "" || got[:6] != "Basic " {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AuthResponse{
			AccessToken: token,
			ExpiresIn:   expiresIn,
		})
	}
}

func TestGetAccessTokenSuccess(t *testing.T) {
	client := newTestClient(t, authHandler(t, "token-1", "3599", nil))

	got, err := client.GetAccessToken(context.Background())
	if err != nil {
		t.Fatalf("GetAccessToken: %v", err)
	}
	if got.AccessToken != "token-1" {
		t.Fatalf("AccessToken = %q, want token-1", got.AccessToken)
	}
	if got.ExpiresIn != "3599" {
		t.Fatalf("ExpiresIn = %q, want 3599", got.ExpiresIn)
	}
}

func TestGetAccessTokenMalformedJSON(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{not-json"))
	}))

	_, err := client.GetAccessToken(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestGetAccessTokenNon200(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))

	_, err := client.GetAccessToken(context.Background())
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestGetAccessTokenReuse(t *testing.T) {
	var calls atomic.Int32
	client := newTestClient(t, authHandler(t, "token-1", "3599", &calls))

	if _, err := client.GetAccessToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetAccessToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("auth calls = %d, want 1", got)
	}
}

func TestGetAccessTokenRefreshBeforeExpiry(t *testing.T) {
	var calls atomic.Int32
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	client := newTestClient(t, authHandler(t, "token-1", "3600", &calls))
	client.now = func() time.Time { return now }

	if _, err := client.GetAccessToken(context.Background()); err != nil {
		t.Fatal(err)
	}

	now = now.Add(3600*time.Second - tokenExpirySkew + time.Second)
	if _, err := client.GetAccessToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("auth calls = %d, want 2 (refresh before expiry)", got)
	}
}

func TestGetAccessTokenNeverUsesExpiredToken(t *testing.T) {
	var calls atomic.Int32
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	client := newTestClient(t, authHandler(t, "token-1", "60", &calls))
	client.now = func() time.Time { return now }

	if _, err := client.GetAccessToken(context.Background()); err != nil {
		t.Fatal(err)
	}

	now = now.Add(60 * time.Second)
	if _, err := client.GetAccessToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("auth calls = %d, want 2 after expiry", got)
	}
}

func TestGetAccessTokenConcurrentSingleFetch(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			close(started)
			<-release
		}
		json.NewEncoder(w).Encode(AuthResponse{
			AccessToken: "shared",
			ExpiresIn:   "3599",
		})
	}))

	var wg sync.WaitGroup
	errCh := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.GetAccessToken(context.Background())
			errCh <- err
		}()
	}

	<-started
	close(release)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("auth calls = %d, want 1", got)
	}
}
