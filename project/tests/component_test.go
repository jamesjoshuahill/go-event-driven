package tests_test

import (
	"context"
	"testing"
	"tickets/db"
	"tickets/service"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComponent(t *testing.T) {
	logger := watermill.NewStdLogger(false, false)

	redisAddr := getEnvOrDefault("REDIS_ADDR", "localhost:6379")
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	t.Cleanup(func() {
		assert.NoError(t, rdb.Conn().Close())
	})

	dsn := getEnvOrDefault("POSTGRES_URL", "postgres://user:password@localhost:5432/db?sslmode=disable")
	dbConn, err := sqlx.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, dbConn.Close())
	})

	require.NoError(t, db.CreateTicketsTable(context.Background(), dbConn))

	ticketGenerator := &MockTicketGenerator{}
	receiptIssuer := &MockReceiptIssuer{}
	spreadsheetAppender := &MockSpreadsheetAppender{}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go func() {
		svc, err := service.New(logger, rdb, ticketGenerator, receiptIssuer, spreadsheetAppender, dbConn)
		assert.NoError(t, err)

		assert.NoError(t, svc.Run(ctx))
	}()

	waitForHttpServer(t)
	t.Run("confimed ticket", func(t *testing.T) {
		ticket := TicketStatus{
			TicketID:      uuid.NewString(),
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
			TicketID:      uuid.NewString(),
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
		assertTicketToRefundRowForTicketAppended(t, spreadsheetAppender, ticket)
	})
}
