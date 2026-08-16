package daraja

import (
	"strings"
	"testing"
)

const successfulCallback = `{
  "Body": {
    "stkCallback": {
      "MerchantRequestID": "29115-34620561-1",
      "CheckoutRequestID": "ws_CO_191220191020363925",
      "ResultCode": 0,
      "ResultDesc": "The service request is processed successfully.",
      "CallbackMetadata": {
        "Item": [
          { "Name": "Amount", "Value": 1.00 },
          { "Name": "MpesaReceiptNumber", "Value": "NLJ7RT61SV" },
          { "Name": "TransactionDate", "Value": 20191106225345 },
          { "Name": "PhoneNumber", "Value": 254708374149 }
        ]
      }
    }
  }
}`

const successfulCallbackStrings = `{
  "Body": {
    "stkCallback": {
      "MerchantRequestID": "m",
      "CheckoutRequestID": "c",
      "ResultCode": 0,
      "ResultDesc": "ok",
      "CallbackMetadata": {
        "Item": [
          { "Name": "Amount", "Value": "10" },
          { "Name": "MpesaReceiptNumber", "Value": "ABC123" },
          { "Name": "TransactionDate", "Value": "20191106225345" },
          { "Name": "PhoneNumber", "Value": "254708374149" }
        ]
      }
    }
  }
}`

const cancelledCallback = `{
  "Body": {
    "stkCallback": {
      "MerchantRequestID": "m",
      "CheckoutRequestID": "c",
      "ResultCode": 1032,
      "ResultDesc": "Request cancelled by user"
    }
  }
}`

const failedCallback = `{
  "Body": {
    "stkCallback": {
      "MerchantRequestID": "m",
      "CheckoutRequestID": "c",
      "ResultCode": 1037,
      "ResultDesc": "DS timeout."
    }
  }
}`

func TestParseSTKCallbackSuccess(t *testing.T) {
	cb, err := ParseSTKCallback(strings.NewReader(successfulCallback))
	if err != nil {
		t.Fatal(err)
	}
	if !cb.IsSuccessful() {
		t.Fatal("expected successful callback")
	}
	if cb.ResultCode() != 0 {
		t.Fatalf("ResultCode = %d", cb.ResultCode())
	}
	if cb.MerchantRequestID() != "29115-34620561-1" {
		t.Fatalf("MerchantRequestID = %s", cb.MerchantRequestID())
	}
	if cb.CheckoutRequestID() != "ws_CO_191220191020363925" {
		t.Fatalf("CheckoutRequestID = %s", cb.CheckoutRequestID())
	}

	amount, err := cb.Amount()
	if err != nil {
		t.Fatal(err)
	}
	if amount != 1 {
		t.Fatalf("Amount = %v, want 1", amount)
	}

	receipt, err := cb.MpesaReceiptNumber()
	if err != nil {
		t.Fatal(err)
	}
	if receipt != "NLJ7RT61SV" {
		t.Fatalf("receipt = %s", receipt)
	}

	date, err := cb.TransactionDate()
	if err != nil {
		t.Fatal(err)
	}
	if date != 20191106225345 {
		t.Fatalf("TransactionDate = %d", date)
	}

	phone, err := cb.PhoneNumber()
	if err != nil {
		t.Fatal(err)
	}
	if phone != "254708374149" {
		t.Fatalf("PhoneNumber = %s", phone)
	}
}

func TestParseSTKCallbackStringMetadata(t *testing.T) {
	cb, err := ParseSTKCallback(strings.NewReader(successfulCallbackStrings))
	if err != nil {
		t.Fatal(err)
	}

	amount, err := cb.Amount()
	if err != nil {
		t.Fatal(err)
	}
	if amount != 10 {
		t.Fatalf("Amount = %v", amount)
	}

	receipt, err := cb.MpesaReceiptNumber()
	if err != nil {
		t.Fatal(err)
	}
	if receipt != "ABC123" {
		t.Fatalf("receipt = %s", receipt)
	}

	date, err := cb.TransactionDate()
	if err != nil {
		t.Fatal(err)
	}
	if date != 20191106225345 {
		t.Fatalf("date = %d", date)
	}

	phone, err := cb.PhoneNumber()
	if err != nil {
		t.Fatal(err)
	}
	if phone != "254708374149" {
		t.Fatalf("phone = %s", phone)
	}
}

func TestParseSTKCallbackCancelled(t *testing.T) {
	cb, err := ParseSTKCallback(strings.NewReader(cancelledCallback))
	if err != nil {
		t.Fatal(err)
	}
	if cb.IsSuccessful() {
		t.Fatal("cancelled callback must not be successful")
	}
	if cb.ResultCode() != 1032 {
		t.Fatalf("ResultCode = %d", cb.ResultCode())
	}
	if cb.ResultDescription() != "Request cancelled by user" {
		t.Fatalf("ResultDesc = %s", cb.ResultDescription())
	}

	if _, err := cb.Amount(); err == nil {
		t.Fatal("expected error when metadata is missing")
	}
}

func TestParseSTKCallbackFailed(t *testing.T) {
	cb, err := ParseSTKCallback(strings.NewReader(failedCallback))
	if err != nil {
		t.Fatal(err)
	}
	if cb.IsSuccessful() {
		t.Fatal("failed callback must not be successful")
	}
	if cb.ResultCode() != 1037 {
		t.Fatalf("ResultCode = %d", cb.ResultCode())
	}

	_, err = cb.MpesaReceiptNumber()
	if err == nil {
		t.Fatal("expected missing metadata error")
	}
}

func TestParseSTKCallbackMalformedJSON(t *testing.T) {
	_, err := ParseSTKCallback(strings.NewReader("{"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseSTKCallbackMissingMetadataFields(t *testing.T) {
	body := `{
	  "Body": {
	    "stkCallback": {
	      "MerchantRequestID": "m",
	      "CheckoutRequestID": "c",
	      "ResultCode": 0,
	      "ResultDesc": "ok",
	      "CallbackMetadata": { "Item": [] }
	    }
	  }
	}`
	cb, err := ParseSTKCallback(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if !cb.IsSuccessful() {
		t.Fatal("ResultCode 0 should be successful even without items")
	}

	if _, err := cb.Amount(); err == nil {
		t.Fatal("expected amount error")
	}
	if _, err := cb.MpesaReceiptNumber(); err == nil {
		t.Fatal("expected receipt error")
	}
	if _, err := cb.TransactionDate(); err == nil {
		t.Fatal("expected date error")
	}
	if _, err := cb.PhoneNumber(); err == nil {
		t.Fatal("expected phone error")
	}
}

func TestParseSTKCallbackNilMetadataDoesNotPanic(t *testing.T) {
	body := `{
	  "Body": {
	    "stkCallback": {
	      "ResultCode": 0,
	      "ResultDesc": "ok"
	    }
	  }
	}`
	cb, err := ParseSTKCallback(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("panic: %v", rec)
		}
	}()

	_, err = cb.Amount()
	if err == nil {
		t.Fatal("expected error for nil metadata")
	}
}
