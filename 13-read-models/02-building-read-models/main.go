package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/components/cqrs"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/shopspring/decimal"
)

type InvoiceIssued struct {
	InvoiceID    string
	CustomerName string
	Amount       decimal.Decimal
	IssuedAt     time.Time
}

type InvoicePaymentReceived struct {
	PaymentID  string
	InvoiceID  string
	PaidAmount decimal.Decimal
	PaidAt     time.Time

	FullyPaid bool
}

type InvoiceVoided struct {
	InvoiceID string
	VoidedAt  time.Time
}

type InvoiceReadModel struct {
	InvoiceID    string
	CustomerName string
	Amount       decimal.Decimal
	IssuedAt     time.Time

	FullyPaid     bool
	PaidAmount    decimal.Decimal
	LastPaymentAt time.Time

	Voided   bool
	VoidedAt time.Time
}

type InvoiceReadModelStorage struct {
	invoices     map[string]InvoiceReadModel
	invoicesLock *sync.RWMutex
}

func NewInvoiceReadModelStorage() *InvoiceReadModelStorage {
	return &InvoiceReadModelStorage{
		invoices:     make(map[string]InvoiceReadModel),
		invoicesLock: &sync.RWMutex{},
	}
}

func (s *InvoiceReadModelStorage) Invoices() []InvoiceReadModel {
	s.invoicesLock.Lock()
	invoices := make([]InvoiceReadModel, 0, len(s.invoices))
	for _, invoice := range s.invoices {
		invoices = append(invoices, invoice)
	}
	s.invoicesLock.Unlock()
	return invoices
}

func (s *InvoiceReadModelStorage) InvoiceByID(id string) (InvoiceReadModel, bool) {
	s.invoicesLock.RLock()
	invoice, exists := s.invoices[id]
	s.invoicesLock.RUnlock()
	return invoice, exists
}

func (s *InvoiceReadModelStorage) OnInvoiceIssued(_ context.Context, event *InvoiceIssued) error {
	s.invoicesLock.Lock()
	s.invoices[event.InvoiceID] = InvoiceReadModel{
		InvoiceID:    event.InvoiceID,
		CustomerName: event.CustomerName,
		Amount:       event.Amount,
		IssuedAt:     event.IssuedAt,
		PaidAmount:   decimal.Zero,
	}
	s.invoicesLock.Unlock()
	return nil
}

func (s *InvoiceReadModelStorage) OnInvoicePaymentReceived(_ context.Context, event *InvoicePaymentReceived) error {
	s.invoicesLock.Lock()
	defer s.invoicesLock.Unlock()

	invoice, exists := s.invoices[event.InvoiceID]
	if !exists {
		return fmt.Errorf("invoice not found: %s", event.InvoiceID)
	}

	if event.FullyPaid {
		invoice.FullyPaid = true
	}
	invoice.PaidAmount = invoice.PaidAmount.Add(event.PaidAmount)
	if event.PaidAt.After(invoice.LastPaymentAt) {
		invoice.LastPaymentAt = event.PaidAt
	}

	s.invoices[event.InvoiceID] = invoice

	return nil
}

func (s *InvoiceReadModelStorage) OnInvoiceVoided(_ context.Context, event *InvoiceVoided) error {
	s.invoicesLock.Lock()
	defer s.invoicesLock.Unlock()

	invoice, exists := s.invoices[event.InvoiceID]
	if !exists {
		return fmt.Errorf("invoice not found: %s", event.InvoiceID)
	}

	invoice.Voided = true
	if event.VoidedAt.After(invoice.VoidedAt) {
		invoice.VoidedAt = event.VoidedAt
	}

	s.invoices[event.InvoiceID] = invoice

	return nil
}

func NewRouter(storage *InvoiceReadModelStorage, eventProcessorConfig cqrs.EventProcessorConfig, watermillLogger watermill.LoggerAdapter) (*message.Router, error) {
	router, err := message.NewRouter(message.RouterConfig{}, watermillLogger)
	if err != nil {
		return nil, fmt.Errorf("could not create router: %w", err)
	}

	eventProcessor, err := cqrs.NewEventProcessorWithConfig(router, eventProcessorConfig)
	if err != nil {
		return nil, fmt.Errorf("could not create command processor: %w", err)
	}

	err = eventProcessor.AddHandlers(
		cqrs.NewEventHandler("InvoiceIssued", storage.OnInvoiceIssued),
		cqrs.NewEventHandler("InvoicePaymentReceived", storage.OnInvoicePaymentReceived),
		cqrs.NewEventHandler("InvoiceVoided", storage.OnInvoiceVoided),
	)
	if err != nil {
		return nil, fmt.Errorf("could not add event handlers: %w", err)
	}

	return router, nil
}
