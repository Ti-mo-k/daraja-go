package daraja

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// STKCallback is the body Daraja POSTs to the merchant callback URL.
type STKCallback struct {
	Body STKCallbackBody `json:"Body"`
}

// STKCallbackBody wraps the STK result.
type STKCallbackBody struct {
	StkCallback STKCallbackResult `json:"stkCallback"`
}

// STKCallbackResult is the STK result payload.
type STKCallbackResult struct {
	MerchantRequestID string            `json:"MerchantRequestID"`
	CheckoutRequestID string            `json:"CheckoutRequestID"`
	ResultCode        int               `json:"ResultCode"`
	ResultDesc        string            `json:"ResultDesc"`
	CallbackMetadata  *CallbackMetadata `json:"CallbackMetadata"`
}

// CallbackMetadata is present on successful STK completions.
type CallbackMetadata struct {
	Item []CallbackMetadataItem `json:"Item"`
}

// CallbackMetadataItem is a single named value from Daraja.
// Value is kept as raw JSON because Daraja mixes strings and numbers.
type CallbackMetadataItem struct {
	Name  string          `json:"Name"`
	Value json.RawMessage `json:"Value"`
}

// ParseSTKCallback decodes a Daraja STK callback body.
func ParseSTKCallback(body io.Reader) (*STKCallback, error) {
	var callback STKCallback
	if err := json.NewDecoder(body).Decode(&callback); err != nil {
		return nil, fmt.Errorf("decode STK callback: %w", err)
	}
	return &callback, nil
}

// IsSuccessful reports whether Daraja processed the payment (ResultCode 0).
func (c *STKCallback) IsSuccessful() bool {
	return c.Body.StkCallback.ResultCode == 0
}

func (c *STKCallback) metadataValue(name string) (json.RawMessage, error) {
	if c.Body.StkCallback.CallbackMetadata == nil {
		return nil, fmt.Errorf("%s not found: callback metadata is missing", name)
	}

	for _, item := range c.Body.StkCallback.CallbackMetadata.Item {
		if item.Name == name {
			if len(item.Value) == 0 || string(item.Value) == "null" {
				return nil, fmt.Errorf("%s is empty", name)
			}
			return item.Value, nil
		}
	}

	return nil, fmt.Errorf("%s not found in callback metadata", name)
}

func metadataString(raw json.RawMessage) (string, error) {
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString, nil
	}

	var asNumber json.Number
	if err := json.Unmarshal(raw, &asNumber); err == nil {
		return asNumber.String(), nil
	}

	return "", fmt.Errorf("unsupported metadata value %s", strings.TrimSpace(string(raw)))
}

// Amount returns the paid amount.
//
// Daraja often encodes this as a JSON number (including 1.00). Integer and
// string values are also accepted.
func (c *STKCallback) Amount() (float64, error) {
	value, err := c.metadataValue("Amount")
	if err != nil {
		return 0, err
	}

	text, err := metadataString(value)
	if err != nil {
		return 0, fmt.Errorf("decode amount: %w", err)
	}

	amount, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, fmt.Errorf("decode amount: %w", err)
	}

	return amount, nil
}

// MpesaReceiptNumber returns the M-Pesa receipt (for example NLJ7RT61SV).
func (c *STKCallback) MpesaReceiptNumber() (string, error) {
	value, err := c.metadataValue("MpesaReceiptNumber")
	if err != nil {
		return "", err
	}

	receipt, err := metadataString(value)
	if err != nil {
		return "", fmt.Errorf("decode M-Pesa receipt number: %w", err)
	}
	if receipt == "" {
		return "", fmt.Errorf("M-Pesa receipt number is empty")
	}

	return receipt, nil
}

// TransactionDate returns Daraja's timestamp as YYYYMMDDHHmmss.
//
// The value is left in Daraja's native form (JSON number or string) rather
// than converted to time.Time: Daraja does not send a timezone, so a time.Time
// would have to assume Africa/Nairobi. Keeping the original integer is also
// easier to compare with Daraja docs and logs. Callers who need time.Time can
// parse this value with layout 20060102150405 in the timezone they choose.
func (c *STKCallback) TransactionDate() (int64, error) {
	value, err := c.metadataValue("TransactionDate")
	if err != nil {
		return 0, err
	}

	text, err := metadataString(value)
	if err != nil {
		return 0, fmt.Errorf("decode transaction date: %w", err)
	}

	date, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("decode transaction date: %w", err)
	}

	return date, nil
}

// PhoneNumber returns the customer MSISDN.
func (c *STKCallback) PhoneNumber() (string, error) {
	value, err := c.metadataValue("PhoneNumber")
	if err != nil {
		return "", err
	}

	phone, err := metadataString(value)
	if err != nil {
		return "", fmt.Errorf("decode phone number: %w", err)
	}
	if phone == "" {
		return "", fmt.Errorf("phone number is empty")
	}

	return phone, nil
}

// ResultCode returns Daraja's numeric result code.
func (c *STKCallback) ResultCode() int {
	return c.Body.StkCallback.ResultCode
}

// ResultDescription returns Daraja's result description.
func (c *STKCallback) ResultDescription() string {
	return c.Body.StkCallback.ResultDesc
}

// MerchantRequestID returns the merchant request ID from the callback.
func (c *STKCallback) MerchantRequestID() string {
	return c.Body.StkCallback.MerchantRequestID
}

// CheckoutRequestID returns the checkout request ID from the callback.
func (c *STKCallback) CheckoutRequestID() string {
	return c.Body.StkCallback.CheckoutRequestID
}
