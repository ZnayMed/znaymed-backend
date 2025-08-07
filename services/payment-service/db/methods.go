package db

import (
	"encoding/json"
	"gorm.io/gorm"
)

func (d *Database) FindPayment(tgid, courseID string) (*Payment, error) {
	var payment Payment
	err := d.DB.
		Where("tgid = ? AND course_id = ?", tgid, courseID).
		First(&payment).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

func (d *Database) CreatePayment(p *Payment) error {
	return d.DB.Create(p).Error
}

func (d *Database) MarkPaymentAsPaid(tx *gorm.DB, paymentID string) error {
	return tx.Model(&Payment{}).
		Where("id = ? AND status <> ?", paymentID, "PAID").
		Update("status", "PAID").Error
}

func (d *Database) AddOutboxEventTx(tx *gorm.DB, eventType string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	event := Outbox{
		EventType: eventType,
		Payload:   data,
		Sent:      false,
	}
	return tx.Create(&event).Error
}

func (d *Database) GetUnsentOutboxEvents() ([]Outbox, error) {
	var events []Outbox
	err := d.DB.Where("sent = ?", false).Order("id ASC").Find(&events).Error
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (d *Database) MarkOutboxEventAsSent(id uint) error {
	return d.DB.Model(&Outbox{}).Where("id = ?", id).Update("sent", true).Error
}
