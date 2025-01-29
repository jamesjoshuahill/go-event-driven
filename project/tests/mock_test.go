package tests_test

import (
	"context"
	"sync"
	"tickets/entity"
)

type MockTicketGenerator struct{}

func (m *MockTicketGenerator) GenerateTicket(_ context.Context, ticketID string, price entity.Money) error {
	return nil
}

type MockReceiptIssuer struct {
	lock           sync.Mutex
	IssuedReceipts []IssueReceiptRequest
}

type IssueReceiptRequest struct {
	ticketID string
	price    entity.Money
}

func (m *MockReceiptIssuer) IssueReceipt(_ context.Context, ticketID string, price entity.Money) error {
	m.lock.Lock()
	m.IssuedReceipts = append(m.IssuedReceipts, IssueReceiptRequest{ticketID: ticketID, price: price})
	m.lock.Unlock()

	return nil
}

type MockSpreadsheetAppender struct {
	lock         sync.Mutex
	RowsAppended []AppendRowRequest
}

type AppendRowRequest struct {
	spreadsheetName string
	row             []string
}

func (m *MockSpreadsheetAppender) AppendRow(ctx context.Context, spreadsheetName string, row []string) error {
	m.lock.Lock()
	m.RowsAppended = append(m.RowsAppended, AppendRowRequest{spreadsheetName: spreadsheetName, row: row})
	m.lock.Unlock()

	return nil
}
