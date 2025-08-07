package main

import (
	"encoding/json"
	"github.com/ZnayMed/znaymed-backend/services/payment-service/db"
	"log"
	"net/http"
)

func (s *paymentServer) PSPCallback(w http.ResponseWriter, r *http.Request) {
	var cb struct {
		PaymentID string `json:"payment_id"`
		Status    string `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&cb); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if cb.Status != "success" {
		log.Printf("⚠️ Платёж %s не успешен, игнорируем", cb.PaymentID)
		w.WriteHeader(http.StatusOK)
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
	if err := tx.First(&payment, "id = ?", cb.PaymentID).Error; err != nil {
		_ = tx.Rollback()
		http.Error(w, "payment not found", http.StatusNotFound)
		return
	}
	if payment.Status == "PAID" {
		log.Println("✅ Повторный callback, платёж уже PAID")
		_ = tx.Rollback()
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := s.db.MarkPaymentAsPaid(tx, cb.PaymentID); err != nil {
		_ = tx.Rollback()
		http.Error(w, "status update failed", http.StatusInternalServerError)
		return
	}

	err := s.db.AddOutboxEventTx(tx, "PaymentConfirmed", map[string]string{
		"payment_id": payment.ID,
		"tgid":       payment.TgID,
		"course_id":  payment.CourseID,
	})
	if err != nil {
		_ = tx.Rollback()
		http.Error(w, "outbox insert failed", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		_ = tx.Rollback()
		http.Error(w, "tx commit failed", http.StatusInternalServerError)
		return
	}

	log.Printf("🎉 Платёж %s отмечен как PAID и отправлен в outbox", cb.PaymentID)
	w.WriteHeader(http.StatusOK)
}
