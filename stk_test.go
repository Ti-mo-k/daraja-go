package daraja

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func validSTKRequest() STKPushRequest {
	return STKPushRequest{
		Amount:           1,
		PartyA:           "254708021200",
		PartyB:           "174379",
		PhoneNumber:      "254708021200",
		AccountReference: "Test",
		TransactionDesc:  "Payment",
	}
}

func TestGenerateSTKPassword(t *testing.T) {
	client := NewClient("key", "secret", "https://example.com",
		WithBusinessShortCode("174379"),
		WithPasskey("bfb279f9aa9bdbcf158e97dd71a467cd2e0c893059b10f78e6b72ada1ed2c919"),
	)

	timestamp := "20131219161930"
	got := client.generateSTKPassword(timestamp)
	want := base64.StdEncoding.EncodeToString([]byte("174379bfb279f9aa9bdbcf158e97dd71a467cd2e0c893059b10f78e6b72ada1ed2c91920131219161930"))
	if got != want {
		t.Fatalf("password = %s, want %s", got, want)
	}
}

func TestSTKPushSuccessAndRequestShape(t *testing.T) {
	var authCalls atomic.Int32
	now := time.Date(2026, 8, 16, 15, 4, 5, 0, time.UTC)

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/v1/generate", authHandler(t, "access-token", "3599", &authCalls))
	mux.HandleFunc("/mpesa/stkpush/v1/processrequest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/mpesa/stkpush/v1/processrequest" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}

		var payload stkPushPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("payload JSON: %v", err)
		}

		if payload.BusinessShortCode != "174379" {
			t.Errorf("BusinessShortCode = %q", payload.BusinessShortCode)
		}
		if payload.TransactionType != stkTransactionTypePayBill {
			t.Errorf("TransactionType = %q", payload.TransactionType)
		}
		if payload.Timestamp != "20260816150405" {
			t.Errorf("Timestamp = %q", payload.Timestamp)
		}
		wantPassword := base64.StdEncoding.EncodeToString([]byte("174379passkey20260816150405"))
		if payload.Password != wantPassword {
			t.Errorf("Password = %q, want %q", payload.Password, wantPassword)
		}
		if payload.Amount != 1 {
			t.Errorf("Amount = %d", payload.Amount)
		}
		if payload.PartyA != "254708021200" || payload.PhoneNumber != "254708021200" {
			t.Errorf("PartyA/PhoneNumber = %q/%q", payload.PartyA, payload.PhoneNumber)
		}
		if payload.PartyB != "174379" {
			t.Errorf("PartyB = %q", payload.PartyB)
		}
		if payload.CallBackURL != "https://example.com/mpesa/callback" {
			t.Errorf("CallBackURL = %q", payload.CallBackURL)
		}
		if payload.AccountReference != "Test" || payload.TransactionDesc != "Payment" {
			t.Errorf("reference/desc = %q/%q", payload.AccountReference, payload.TransactionDesc)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(STKPushResponse{
			MerchantRequestID:   "m-id",
			CheckoutRequestID:   "c-id",
			ResponseCode:        "0",
			ResponseDescription: "Success",
			CustomerMessage:     "Success",
		})
	})

	client := newTestClient(t, mux,
		WithBusinessShortCode("174379"),
		WithPasskey("passkey"),
		WithCallbackURL("https://example.com/mpesa/callback"),
	)
	client.now = func() time.Time { return now }

	got, err := client.STKPush(context.Background(), validSTKRequest())
	if err != nil {
		t.Fatalf("STKPush: %v", err)
	}
	if got.CheckoutRequestID != "c-id" || got.MerchantRequestID != "m-id" {
		t.Fatalf("response = %+v", got)
	}
	if authCalls.Load() != 1 {
		t.Fatalf("auth calls = %d, want 1", authCalls.Load())
	}
}

func TestSTKPushErrorStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/v1/generate", authHandler(t, "access-token", "3599", nil))
	mux.HandleFunc("/mpesa/stkpush/v1/processrequest", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fail", http.StatusInternalServerError)
	})

	client := newTestClient(t, mux,
		WithBusinessShortCode("174379"),
		WithPasskey("passkey"),
		WithCallbackURL("https://example.com/callback"),
	)

	_, err := client.STKPush(context.Background(), validSTKRequest())
	if err == nil {
		t.Fatal("expected error for non-200 STK response")
	}
}

func TestSTKPushValidation(t *testing.T) {
	client := NewClient("key", "secret", "https://example.com",
		WithBusinessShortCode("174379"),
		WithPasskey("passkey"),
		WithCallbackURL("https://example.com/callback"),
	)

	tests := []struct {
		name    string
		mut     func(*STKPushRequest)
		client  *Client
		wantErr string
	}{
		{
			name:    "amount",
			mut:     func(r *STKPushRequest) { r.Amount = 0 },
			wantErr: "amount must be greater than 0",
		},
		{
			name:    "party A",
			mut:     func(r *STKPushRequest) { r.PartyA = "" },
			wantErr: "PartyA is required",
		},
		{
			name:    "party B",
			mut:     func(r *STKPushRequest) { r.PartyB = "" },
			wantErr: "PartyB is required",
		},
		{
			name:    "phone",
			mut:     func(r *STKPushRequest) { r.PhoneNumber = "+254708021200" },
			wantErr: "PhoneNumber must be digits",
		},
		{
			name:    "account reference",
			mut:     func(r *STKPushRequest) { r.AccountReference = "" },
			wantErr: "account reference is required",
		},
		{
			name:    "transaction desc",
			mut:     func(r *STKPushRequest) { r.TransactionDesc = "" },
			wantErr: "transaction description is required",
		},
		{
			name:    "short code",
			client:  NewClient("key", "secret", "https://example.com", WithPasskey("p"), WithCallbackURL("https://example.com/c")),
			wantErr: "business short code is required",
		},
		{
			name:    "passkey",
			client:  NewClient("key", "secret", "https://example.com", WithBusinessShortCode("1"), WithCallbackURL("https://example.com/c")),
			wantErr: "passkey is required",
		},
		{
			name:    "callback",
			client:  NewClient("key", "secret", "https://example.com", WithBusinessShortCode("1"), WithPasskey("p")),
			wantErr: "callback URL is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := client
			if tt.client != nil {
				c = tt.client
			}
			req := validSTKRequest()
			if tt.mut != nil {
				tt.mut(&req)
			}
			_, err := c.STKPush(context.Background(), req)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}
