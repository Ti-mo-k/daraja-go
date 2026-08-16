package daraja

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	stkPushPath               = "/mpesa/stkpush/v1/processrequest"
	stkTransactionTypePayBill = "CustomerPayBillOnline"
	stkTimestampLayout        = "20060102150405"
)

// STKPushRequest is the caller-supplied input for an STK Push.
type STKPushRequest struct {
	Amount           int    `json:"Amount"`
	PartyA           string `json:"PartyA"`
	PartyB           string `json:"PartyB"`
	PhoneNumber      string `json:"PhoneNumber"`
	AccountReference string `json:"AccountReference"`
	TransactionDesc  string `json:"TransactionDesc"`
}

// STKPushResponse is Daraja's immediate response to an STK Push request.
type STKPushResponse struct {
	MerchantRequestID   string `json:"MerchantRequestID"`
	CheckoutRequestID   string `json:"CheckoutRequestID"`
	ResponseCode        string `json:"ResponseCode"`
	ResponseDescription string `json:"ResponseDescription"`
	CustomerMessage     string `json:"CustomerMessage"`
}

type stkPushPayload struct {
	BusinessShortCode string `json:"BusinessShortCode"`
	Password          string `json:"Password"`
	Timestamp         string `json:"Timestamp"`
	TransactionType   string `json:"TransactionType"`
	Amount            int    `json:"Amount"`
	PartyA            string `json:"PartyA"`
	PartyB            string `json:"PartyB"`
	PhoneNumber       string `json:"PhoneNumber"`
	CallBackURL       string `json:"CallBackURL"`
	AccountReference  string `json:"AccountReference"`
	TransactionDesc   string `json:"TransactionDesc"`
}

func (c *Client) generateSTKPassword(timestamp string) string {
	password := c.businessShortCode + c.passkey + timestamp
	return base64.StdEncoding.EncodeToString([]byte(password))
}

func validateMSISDN(field, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	if strings.HasPrefix(value, "+") || strings.ContainsAny(value, " \t") {
		return fmt.Errorf("%s must be digits in international format, e.g. 2547xxxxxxxx", field)
	}
	return nil
}

func (c *Client) validateSTKPush(request STKPushRequest) error {
	if c.businessShortCode == "" {
		return fmt.Errorf("business short code is required")
	}
	if c.passkey == "" {
		return fmt.Errorf("passkey is required")
	}
	if c.callbackURL == "" {
		return fmt.Errorf("callback URL is required")
	}
	if _, err := url.ParseRequestURI(c.callbackURL); err != nil {
		return fmt.Errorf("callback URL is invalid: %w", err)
	}

	if request.Amount <= 0 {
		return fmt.Errorf("amount must be greater than 0")
	}
	if err := validateMSISDN("PartyA", request.PartyA); err != nil {
		return err
	}
	if request.PartyB == "" {
		return fmt.Errorf("PartyB is required")
	}
	if err := validateMSISDN("PhoneNumber", request.PhoneNumber); err != nil {
		return err
	}
	if request.AccountReference == "" {
		return fmt.Errorf("account reference is required")
	}
	if request.TransactionDesc == "" {
		return fmt.Errorf("transaction description is required")
	}

	return nil
}

// STKPush initiates an M-Pesa STK Push (Lipa Na M-Pesa Online).
func (c *Client) STKPush(ctx context.Context, request STKPushRequest) (*STKPushResponse, error) {
	if err := c.validateSTKPush(request); err != nil {
		return nil, err
	}

	timestamp := c.currentTime().Format(stkTimestampLayout)
	payload := stkPushPayload{
		BusinessShortCode: c.businessShortCode,
		Password:          c.generateSTKPassword(timestamp),
		Timestamp:         timestamp,
		TransactionType:   stkTransactionTypePayBill,
		Amount:            request.Amount,
		PartyA:            request.PartyA,
		PartyB:            request.PartyB,
		PhoneNumber:       request.PhoneNumber,
		CallBackURL:       c.callbackURL,
		AccountReference:  request.AccountReference,
		TransactionDesc:   request.TransactionDesc,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal STK push payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(stkPushPath), bytes.NewReader(payloadJSON))
	if err != nil {
		return nil, fmt.Errorf("create STK push request: %w", err)
	}

	authResponse, err := c.GetAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+authResponse.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send STK push request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("daraja returned status %s", resp.Status)
	}

	var stkResponse STKPushResponse
	if err := json.NewDecoder(resp.Body).Decode(&stkResponse); err != nil {
		return nil, fmt.Errorf("decode STK push response: %w", err)
	}

	return &stkResponse, nil
}
