package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type YooKassa struct {
	apiURL    string
	shopID    string
	secretKey string
	client    *http.Client

	vatCode        int    // напр. 1 = без НДС
	taxSystemCode  *int   // может быть nil
	paymentSubject string // "service"
	paymentMode    string // "full_payment"
}

func NewYooKassa() *YooKassa {
	y := &YooKassa{
		apiURL:    getenv("YOOKASSA_API", "https://api.yookassa.ru/v3"),
		shopID:    os.Getenv("YOOKASSA_SHOP_ID"),
		secretKey: os.Getenv("YOOKASSA_SECRET"),
		client:    &http.Client{Timeout: 10 * time.Second},

		vatCode:        getenvInt("YOOKASSA_VAT_CODE", 1),
		taxSystemCode:  getenvIntPtr("YOOKASSA_TAX_SYSTEM_CODE"),
		paymentSubject: getenv("YOOKASSA_PAYMENT_SUBJECT", "service"),
		paymentMode:    getenv("YOOKASSA_PAYMENT_MODE", "full_payment"),
	}
	log.Printf("[YooKassa] api=%s shop=%s*** mock=%v return_url=%s",
		y.apiURL, head(y.shopID), os.Getenv("YOOKASSA_MOCK") == "1", os.Getenv("PUBLIC_RETURN_URL"))
	return y
}

func head(s string) string {
	if len(s) < 3 {
		return s
	}
	return s[:3]
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getenvInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

func getenvIntPtr(k string) *int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return &n
		}
	}
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

type Receipt struct {
	Customer struct {
		Email string `json:"email,omitempty"`
		Phone string `json:"phone,omitempty"`
	} `json:"customer"`
	Items         []ReceiptItem `json:"items"`
	TaxSystemCode *int          `json:"tax_system_code,omitempty"`
}

type ReceiptItem struct {
	Description string `json:"description"`
	Quantity    string `json:"quantity"` // "1.00"
	Amount      struct {
		Value    string `json:"value"`    // "1.00"
		Currency string `json:"currency"` // "RUB"
	} `json:"amount"`
	VatCode        int    `json:"vat_code"`                  // 1=без НДС (проверь под свой магазин)
	PaymentSubject string `json:"payment_subject,omitempty"` // "service"
	PaymentMode    string `json:"payment_mode,omitempty"`    // "full_payment"
}

type CreatePaymentReq struct {
	Amount struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"amount"`
	Capture      bool   `json:"capture"`
	Description  string `json:"description,omitempty"`
	Confirmation struct {
		Type      string `json:"type"`
		ReturnURL string `json:"return_url"`
	} `json:"confirmation"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Receipt  *Receipt          `json:"receipt,omitempty"` // <— НОВОЕ
}

type CreatePaymentResp struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Confirmation struct {
		Type string `json:"type"`
		URL  string `json:"confirmation_url"`
	} `json:"confirmation"`
	ExpiresAt *time.Time `json:"expires_at"`
}

func (y *YooKassa) CreatePayment(ctx context.Context, amountRub string, description, returnURL, idemKey string, metadata map[string]string) (providerID, confirmationURL, status string, expiresAt *time.Time, err error) {

	//if os.Getenv("YOOKASSA_MOCK") == "1" {
	//	pid := "mock_" + idemKey
	//	url := getenv("PUBLIC_RETURN_URL", "http://localhost:8081/return")
	//	if strings.Contains(url, "?") {
	//		url = url + "&pid=" + pid
	//	} else {
	//		url = url + "?pid=" + pid
	//	}
	//	return pid, url, "pending", nil, nil
	//}

	reqBody := CreatePaymentReq{
		Capture:     true,
		Description: description,
		Metadata:    metadata,
	}
	reqBody.Amount.Value = amountRub
	reqBody.Amount.Currency = "RUB"
	reqBody.Confirmation.Type = "redirect"
	reqBody.Confirmation.ReturnURL = returnURL

	b, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/payments", y.apiURL), bytes.NewReader(b))
	if err != nil {
		return "", "", "", nil, err
	}

	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(y.shopID+":"+y.secretKey)))
	req.Header.Set("Content-Type", "application/json")
	// Идемпотентность
	req.Header.Set("Idempotence-Key", idemKey)

	resp, err := y.client.Do(req)
	if err != nil {
		return "", "", "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var body bytes.Buffer
		_, _ = body.ReadFrom(resp.Body)
		return "", "", "", nil, fmt.Errorf("yookassa http %d: %s", resp.StatusCode, body.String())
	}

	var out CreatePaymentResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", "", nil, err
	}

	return out.ID, out.Confirmation.URL, out.Status, out.ExpiresAt, nil
}
