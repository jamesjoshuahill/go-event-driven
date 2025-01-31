package tests_test

import (
	"context"
	"fmt"
	"sync"
	"tickets/entity"
)

type MockTicketGenerator struct {
	lock     sync.Mutex
	Requests []GenerateTicketRequest
}

type GenerateTicketRequest struct {
	ticketID string
	price    entity.Money
}

func (m *MockTicketGenerator) GenerateTicket(_ context.Context, ticketID string, price entity.Money) (string, error) {
	m.lock.Lock()
	m.Requests = append(m.Requests, GenerateTicketRequest{
		ticketID: ticketID,
		price:    price,
	})
	m.lock.Unlock()

	fileID := fmt.Sprintf("%s-ticket.html", ticketID)

	return fileID, nil
}

func (m *MockTicketGenerator) RequestForTicketID(ticketID string) (GenerateTicketRequest, bool) {
	m.lock.Lock()
	var match GenerateTicketRequest
	var ok bool
	for _, req := range m.Requests {
		if req.ticketID == ticketID {
			match = req
			ok = true
		}
	}
	m.lock.Unlock()

	return match, ok
}

type MockReceiptIssuer struct {
	lock     sync.Mutex
	Requests []IssueReceiptRequest
}

type IssueReceiptRequest struct {
	idempotencyKey string
	ticketID       string
	price          entity.Money
}

func (m *MockReceiptIssuer) IssueReceipt(_ context.Context, idempotencyKey, ticketID string, price entity.Money) error {
	m.lock.Lock()
	m.Requests = append(m.Requests, IssueReceiptRequest{
		idempotencyKey: idempotencyKey,
		ticketID:       ticketID,
		price:          price,
	})
	m.lock.Unlock()

	return nil
}

func (m *MockReceiptIssuer) RequestForTicketID(ticketID string) (IssueReceiptRequest, bool) {
	m.lock.Lock()
	var match IssueReceiptRequest
	var ok bool
	for _, req := range m.Requests {
		if req.ticketID == ticketID {
			match = req
			ok = true
		}
	}
	m.lock.Unlock()

	return match, ok
}

type MockSpreadsheetAppender struct {
	lock     sync.Mutex
	Requests []AppendRowRequest
}

type AppendRowRequest struct {
	spreadsheetName string
	row             []string
}

func (m *MockSpreadsheetAppender) AppendRow(ctx context.Context, spreadsheetName string, row []string) error {
	m.lock.Lock()
	m.Requests = append(m.Requests, AppendRowRequest{spreadsheetName: spreadsheetName, row: row})
	m.lock.Unlock()

	return nil
}

func (m *MockSpreadsheetAppender) RequestFor(spreadsheetName, ticketID string) (AppendRowRequest, bool) {
	m.lock.Lock()
	var match AppendRowRequest
	var ok bool
	for _, req := range m.Requests {
		if req.spreadsheetName == spreadsheetName && req.row[0] == ticketID {
			match = req
			ok = true
		}
	}
	m.lock.Unlock()

	return match, ok
}

type MockDeadNationBooker struct{}

func (m *MockDeadNationBooker) CreateBooking(ctx context.Context, booking entity.Booking) error {
	return nil
}
