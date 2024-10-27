package tests_test

import (
	"context"
	"os"
	"testing"
	"tickets/service"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestComponent(t *testing.T) {
	logger := watermill.NewStdLogger(false, false)
	rdb := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_ADDR"),
	})
	t.Cleanup(func() {
		assert.NoError(t, rdb.Conn().Close())
	})

	receiptIssuer := &MockReceiptIssuer{}
	spreadsheetAppender := &MockSpreadsheetAppender{}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go func() {
		svc, err := service.New(logger, rdb, receiptIssuer, spreadsheetAppender)
		assert.NoError(t, err)

		assert.NoError(t, svc.Run(ctx))

	}()

	waitForHttpServer(t)
	t.Run("confimed ticket", func(t *testing.T) {
		ticket := TicketStatus{
			TicketID:      "some ticket id",
			Status:        "confirmed",
			CustomerEmail: "someone@example.com",
			Price: Money{
				Amount:   "42",
				Currency: "GBP",
			},
		}
		req := TicketsStatusRequest{
			Tickets: []TicketStatus{
				ticket,
			},
		}

		sendTicketsStatus(t, req)
		assertReceiptForTicketIssued(t, receiptIssuer, ticket)
		assertTicketToPrintRowForTicketAppended(t, spreadsheetAppender, ticket)
	})

	t.Run("canceled ticket", func(t *testing.T) {
		ticket := TicketStatus{
			TicketID:      "some ticket id",
			Status:        "canceled",
			CustomerEmail: "someone@example.com",
			Price: Money{
				Amount:   "42",
				Currency: "GBP",
			},
		}
		req := TicketsStatusRequest{
			Tickets: []TicketStatus{
				ticket,
			},
		}

		sendTicketsStatus(t, req)
		assertTicketToPrintRowForTicketAppended(t, spreadsheetAppender, ticket)
	})
}
