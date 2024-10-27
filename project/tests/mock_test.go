package tests_test

import (
	"context"
	"tickets/entity"
)

type MockReceiptIssuer struct{}

func (m *MockReceiptIssuer) IssueReceipt(_ context.Context, ticketID string, price entity.Money) error {
	return nil
}

type MockSpreadsheetAppender struct{}

func (m *MockSpreadsheetAppender) AppendRow(ctx context.Context, spreadsheetName string, row []string) error {
	return nil
}
