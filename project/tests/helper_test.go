package tests_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/lithammer/shortuuid/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getEnvOrDefault(key string, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func waitForHttpServer(t *testing.T) {
	t.Helper()

	require.EventuallyWithT(
		t,
		func(t *assert.CollectT) {
			resp, err := http.Get("http://localhost:8080/health")
			if !assert.NoError(t, err) {
				return
			}
			defer resp.Body.Close()

			if assert.Less(t, resp.StatusCode, 300, "API not ready, http status: %d", resp.StatusCode) {
				return
			}
		},
		time.Second*10,
		time.Millisecond*50,
	)
}

type TicketsStatusRequest struct {
	Tickets []TicketStatus `json:"tickets"`
}

type TicketStatus struct {
	TicketID      string `json:"ticket_id"`
	Status        string `json:"status"`
	Price         Money  `json:"price"`
	Email         string `json:"email"`
	BookingID     string `json:"booking_id"`
	CustomerEmail string `json:"customer_email"`
}

type Money struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

func sendTicketsStatus(t *testing.T, req TicketsStatusRequest) {
	t.Helper()

	payload, err := json.Marshal(req)
	require.NoError(t, err)

	correlationID := shortuuid.New()

	httpReq, err := http.NewRequest(
		http.MethodPost,
		"http://localhost:8080/tickets-status",
		bytes.NewBuffer(payload),
	)
	require.NoError(t, err)

	httpReq.Header.Set("Correlation-ID", correlationID)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func assertReceiptForTicketIssued(t *testing.T, receiptIssuer *MockReceiptIssuer, ticket TicketStatus) {
	assert.EventuallyWithT(
		t,
		func(collectT *assert.CollectT) {
			issuedReceipts := len(receiptIssuer.IssuedReceipts)
			t.Log("issued receipts", issuedReceipts)

			assert.Greater(collectT, issuedReceipts, 0, "no receipts issued")
		},
		10*time.Second,
		100*time.Millisecond,
	)

	var receipt IssueReceiptRequest
	var ok bool
	for _, issuedReceipt := range receiptIssuer.IssuedReceipts {
		if issuedReceipt.ticketID != ticket.TicketID {
			continue
		}
		receipt = issuedReceipt
		ok = true
		break
	}
	require.Truef(t, ok, "receipt for ticket %s not found", ticket.TicketID)

	assert.Equal(t, ticket.TicketID, receipt.ticketID)
	assert.Equal(t, ticket.Price.Amount, receipt.price.Amount)
	assert.Equal(t, ticket.Price.Currency, receipt.price.Currency)
}

func assertTicketToPrintRowForTicketAppended(t *testing.T, spreadsheetAppender *MockSpreadsheetAppender, ticket TicketStatus) {
	assert.EventuallyWithT(
		t,
		func(collectT *assert.CollectT) {
			rowsAppended := len(spreadsheetAppender.RowsAppended)
			t.Log("rows appended", rowsAppended)

			assert.Greater(collectT, rowsAppended, 0, "no rows appended")
		},
		10*time.Second,
		100*time.Millisecond,
	)

	var req AppendRowRequest
	var ok bool
	for _, rowAppended := range spreadsheetAppender.RowsAppended {
		if rowAppended.spreadsheetName != "tickets-to-print" {
			continue
		}
		if len(rowAppended.row) == 0 || rowAppended.row[0] != ticket.TicketID {
			continue
		}
		req = rowAppended
		ok = true
		break
	}
	require.Truef(t, ok, "row for ticket %s not found", ticket.TicketID)

	assert.Len(t, req.row, 4)
	assert.Equal(t, ticket.TicketID, req.row[0])
	assert.Equal(t, ticket.CustomerEmail, req.row[1])
	assert.Equal(t, ticket.Price.Amount, req.row[2])
	assert.Equal(t, ticket.Price.Currency, req.row[3])
}

func assertTicketToRefundRowForTicketAppended(t *testing.T, spreadsheetAppender *MockSpreadsheetAppender, ticket TicketStatus) {
	assert.EventuallyWithT(
		t,
		func(collectT *assert.CollectT) {
			rowsAppended := len(spreadsheetAppender.RowsAppended)
			t.Log("rows appended", rowsAppended)

			assert.Greater(collectT, rowsAppended, 0, "no rows appended")
		},
		10*time.Second,
		100*time.Millisecond,
	)

	var req AppendRowRequest
	var ok bool
	for _, rowAppended := range spreadsheetAppender.RowsAppended {
		if rowAppended.spreadsheetName != "tickets-to-refund" {
			continue
		}
		if len(rowAppended.row) == 0 || rowAppended.row[0] != ticket.TicketID {
			continue
		}
		req = rowAppended
		ok = true
		break
	}
	require.Truef(t, ok, "row for ticket %s not found", ticket.TicketID)

	assert.Len(t, req.row, 4)
	assert.Equal(t, ticket.TicketID, req.row[0])
	assert.Equal(t, ticket.CustomerEmail, req.row[1])
	assert.Equal(t, ticket.Price.Amount, req.row[2])
	assert.Equal(t, ticket.Price.Currency, req.row[3])
}
