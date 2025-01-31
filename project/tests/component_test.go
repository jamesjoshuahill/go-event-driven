package tests_test

import (
	"testing"
	"tickets/service"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/google/uuid"
)

func TestComponent(t *testing.T) {
	db := setupDB(t)
	redisClient := setupRedis(t)
	deadNationBooker := &MockDeadNationBooker{}
	receiptIssuer := &MockReceiptIssuer{}
	spreadsheetAppender := &MockSpreadsheetAppender{}
	ticketGenerator := &MockTicketGenerator{}

	deps := service.ServiceDeps{
		DB:                  db,
		Logger:              watermill.NewStdLogger(false, false),
		RedisClient:         redisClient,
		DeadNationBooker:    deadNationBooker,
		ReceiptIssuer:       receiptIssuer,
		SpreadsheetAppender: spreadsheetAppender,
		TicketGenerator:     ticketGenerator,
	}
	startService(t, deps)

	t.Run("confimed ticket", func(t *testing.T) {
		ticket := TicketStatus{
			TicketID:      uuid.NewString(),
			Status:        "confirmed",
			CustomerEmail: "someone@example.com",
			Price: Money{
				Amount:   "42.00",
				Currency: "GBP",
			},
		}
		req := TicketsStatusRequest{
			Tickets: []TicketStatus{
				ticket,
			},
		}
		idempotencyKey := uuid.NewString()

		sendTicketsStatus(t, req, idempotencyKey)
		sendTicketsStatus(t, req, idempotencyKey)
		sendTicketsStatus(t, req, idempotencyKey)
		assertReceiptForTicketIssued(t, receiptIssuer, ticket)
		assertTicketToPrintRowForTicketAppended(t, spreadsheetAppender, ticket)
		assertStoredTicketInDB(t, db, ticket)
		assertTicketGenerated(t, ticketGenerator, ticket)
		assertTicketPrintedEventPublished(t, redisClient, ticket)
	})

	t.Run("canceled ticket", func(t *testing.T) {
		ticket := TicketStatus{
			TicketID:      uuid.NewString(),
			Status:        "canceled",
			CustomerEmail: "someone@example.com",
			Price: Money{
				Amount:   "42.00",
				Currency: "GBP",
			},
		}
		req := TicketsStatusRequest{
			Tickets: []TicketStatus{
				ticket,
			},
		}

		sendTicketsStatus(t, req, uuid.NewString())
		assertTicketToRefundRowForTicketAppended(t, spreadsheetAppender, ticket)
	})
}
