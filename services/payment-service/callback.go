package main

import (
	"encoding/json"
	"github.com/ZnayMed/znaymed-backend/services/payment-service/db"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type yooWebhook struct {
	Event  string        `json:"event"`
	Object yooPaymentObj `json:"object"`
}

type yooPaymentObj struct {
	ID       string            `json:"id"`
	Status   string            `json:"status"`
	Metadata map[string]string `json:"metadata"`
	Amount   struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"amount"`
}

func (s *paymentServer) PSPCallback(w http.ResponseWriter, r *http.Request) {

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if !strings.Contains(string(body), "payment.") {
		w.WriteHeader(http.StatusOK)
		return
	}

	var hook yooWebhook
	if err := json.Unmarshal(body, &hook); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	internalID := hook.Object.Metadata["internal_payment_id"]
	if internalID == "" {
		http.Error(w, "no internal payment id", http.StatusBadRequest)
		return
	}

	tx := s.db.DB.Begin()
	if tx.Error != nil {
		http.Error(w, "tx begin failed", http.StatusInternalServerError)
		return
	}
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			http.Error(w, "panic", http.StatusInternalServerError)
		}
	}()

	var payment db.Payment
	if err := tx.First(&payment, "id = ?", internalID).Error; err != nil {
		_ = tx.Rollback()
		http.Error(w, "payment not found", http.StatusNotFound)
		return
	}

	switch hook.Event {
	case "payment.succeeded":
		if err := s.db.MarkPaymentAsPaid(tx, payment.ID); err != nil {
			_ = tx.Rollback()
			http.Error(w, "status update failed", http.StatusInternalServerError)
			return
		}
		err := s.db.AddOutboxEventTx(tx, "PaymentConfirmed", map[string]interface{}{
			"version":    1,
			"payment_id": payment.ID,
			"tgid":       payment.TgID,
			"course_id":  payment.CourseID,
			"amount":     payment.Amount,
			"currency":   payment.Currency,
		})
		if err != nil {
			_ = tx.Rollback()
			http.Error(w, "outbox insert failed", http.StatusInternalServerError)
			return
		}
	case "payment.canceled":
		if err := s.db.MarkPaymentAsCanceled(tx, payment.ID); err != nil {
			_ = tx.Rollback()
			http.Error(w, "status update failed", http.StatusInternalServerError)
			return
		}
	default:
	}

	if err := tx.Commit().Error; err != nil {
		_ = tx.Rollback()
		http.Error(w, "tx commit failed", http.StatusInternalServerError)
		return
	}

	log.Printf("Webhook %s обработан для payment=%s (provider=%s)", hook.Event, payment.ID, hook.Object.ID)
	w.WriteHeader(http.StatusOK)
}

func (s *paymentServer) PSPCallbackTest(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("ENV") == "prod" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	s.PSPCallback(w, r)
}
