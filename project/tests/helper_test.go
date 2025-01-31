package tests_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"tickets/service"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/lithammer/shortuuid/v3"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getEnvOrDefault(key string, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func setupRedis(t *testing.T) *redis.Client {
	addr := getEnvOrDefault("REDIS_ADDR", "localhost:6379")
	c := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	t.Cleanup(func() {
		assert.NoError(t, c.Conn().Close())
	})

	return c
}

func setupDB(t *testing.T) *sqlx.DB {
	dsn := getEnvOrDefault("POSTGRES_URL", "postgres://user:password@localhost:5432/db?sslmode=disable")
	db, err := sqlx.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, db.Close())
	})

	return db
}

func startService(t *testing.T, deps service.ServiceDeps) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go func() {
		svc, err := service.New(deps)
		assert.NoError(t, err)

		assert.NoError(t, svc.Run(ctx))
	}()

	waitForHTTPServer(t)
}

func waitForHTTPServer(t *testing.T) {
	t.Helper()

	require.EventuallyWithT(
		t,
		func(t *assert.CollectT) {
			resp, err := http.Get("http://localhost:8080/health")
			if !assert.NoError(t, err) {
				return
			}
			defer resp.Body.Close()

			assert.Equal(t, 200, resp.StatusCode, "API not ready, http status: %d", resp.StatusCode)
		},
		1*time.Second,
		10*time.Millisecond,
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

func sendTicketsStatus(t *testing.T, req TicketsStatusRequest, idempotencyKey string) {
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
	httpReq.Header.Set("Idempotency-Key", idempotencyKey)

	resp, err := http.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func assertReceiptForTicketIssued(t *testing.T, receiptIssuer *MockReceiptIssuer, ticket TicketStatus) {
	t.Helper()

	assert.EventuallyWithT(
		t,
		func(c *assert.CollectT) {
			req, ok := receiptIssuer.RequestForTicketID(ticket.TicketID)
			require.True(c, ok)

			assert.Equal(t, ticket.TicketID, req.ticketID)
			assert.Equal(t, ticket.Price.Amount, req.price.Amount)
			assert.Equal(t, ticket.Price.Currency, req.price.Currency)
		},
		1*time.Second,
		10*time.Millisecond,
	)
}

func assertTicketToPrintRowForTicketAppended(t *testing.T, spreadsheetAppender *MockSpreadsheetAppender, ticket TicketStatus) {
	t.Helper()

	assert.EventuallyWithT(
		t,
		func(c *assert.CollectT) {
			req, ok := spreadsheetAppender.RequestFor("tickets-to-print", ticket.TicketID)
			require.True(c, ok)

			assert.Len(t, req.row, 4)
			assert.Equal(t, ticket.TicketID, req.row[0])
			assert.Equal(t, ticket.CustomerEmail, req.row[1])
			assert.Equal(t, ticket.Price.Amount, req.row[2])
			assert.Equal(t, ticket.Price.Currency, req.row[3])
		},
		1*time.Second,
		10*time.Millisecond,
	)
}

func assertTicketToRefundRowForTicketAppended(t *testing.T, spreadsheetAppender *MockSpreadsheetAppender, ticket TicketStatus) {
	t.Helper()

	assert.EventuallyWithT(
		t,
		func(c *assert.CollectT) {
			req, ok := spreadsheetAppender.RequestFor("tickets-to-refund", ticket.TicketID)
			require.True(c, ok)

			require.Len(c, req.row, 4)
			assert.Equal(c, ticket.TicketID, req.row[0])
			assert.Equal(c, ticket.CustomerEmail, req.row[1])
			assert.Equal(c, ticket.Price.Amount, req.row[2])
			assert.Equal(c, ticket.Price.Currency, req.row[3])
		},
		1*time.Second,
		10*time.Millisecond,
	)
}

type Ticket struct {
	ID            string
	CustomerEmail string
	Price         struct {
		Amount   string
		Currency string
	}
}

func assertStoredTicketInDB(t *testing.T, dbConn *sqlx.DB, ticket TicketStatus) {
	t.Helper()

	assert.EventuallyWithT(
		t,
		func(c *assert.CollectT) {
			row := dbConn.QueryRowx("SELECT ticket_id, price_amount, price_currency, customer_email FROM tickets WHERE ticket_id = $1", ticket.TicketID)

			var actual Ticket
			err := row.Scan(&actual.ID, &actual.Price.Amount, &actual.Price.Currency, &actual.CustomerEmail)
			require.NoError(c, err)

			assert.Equal(c, ticket.TicketID, actual.ID)
			assert.Equal(c, ticket.Price.Amount, actual.Price.Amount)
			assert.Equal(c, ticket.Price.Currency, actual.Price.Currency)
			assert.Equal(c, ticket.CustomerEmail, actual.CustomerEmail)
		},
		1*time.Second,
		10*time.Millisecond,
	)
}

func assertTicketGenerated(t *testing.T, ticketGenerator *MockTicketGenerator, ticket TicketStatus) {
	t.Helper()

	assert.EventuallyWithT(
		t,
		func(c *assert.CollectT) {
			req, ok := ticketGenerator.RequestForTicketID(ticket.TicketID)
			require.True(c, ok)

			assert.Equal(c, ticket.TicketID, req.ticketID)
			assert.Equal(c, ticket.Price.Amount, req.price.Amount)
			assert.Equal(c, ticket.Price.Currency, req.price.Currency)
		},
		1*time.Second,
		10*time.Millisecond,
	)
}

type TicketPrinted struct {
	TicketID string `json:"ticket_id"`
	FileName string `json:"file_name"`
}

func assertTicketPrintedEventPublished(t *testing.T, redisClient *redis.Client, ticket TicketStatus) {
	t.Helper()

	assert.EventuallyWithT(
		t,
		func(c *assert.CollectT) {
			res, err := redisClient.XRead(context.Background(), &redis.XReadArgs{
				Streams: []string{"TicketPrinted", "0"},
				Count:   100,
			}).Result()
			require.NoError(c, err)

			require.Len(c, res, 1)
			require.NotEmpty(c, res[0].Messages)

			var match bool
			for _, m := range res[0].Messages {
				payload, ok := m.Values["payload"].(string)
				require.True(c, ok)

				var actual TicketPrinted
				err = json.Unmarshal([]byte(payload), &actual)
				require.NoError(c, err)

				if actual.TicketID != ticket.TicketID {
					continue
				}

				assert.Equal(c, ticket.TicketID, actual.TicketID)
				expectedFileName := fmt.Sprintf("%s-ticket.html", ticket.TicketID)
				assert.Equal(c, expectedFileName, actual.FileName)
				match = true
			}
			assert.True(c, match, "no matching message")
		},
		1*time.Second,
		10*time.Millisecond,
	)
}
