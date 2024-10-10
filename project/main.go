package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/ThreeDotsLabs/go-event-driven/common/clients"
	"github.com/ThreeDotsLabs/go-event-driven/common/clients/receipts"
	"github.com/ThreeDotsLabs/go-event-driven/common/clients/spreadsheets"
	commonHTTP "github.com/ThreeDotsLabs/go-event-driven/common/http"
	"github.com/ThreeDotsLabs/go-event-driven/common/log"
	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const (
	TopicTicketBookingConfirmed = "TicketBookingConfirmed"
)

type TicketBookingConfirmedEvent struct {
	Header        EventHeader `json:"header"`
	TicketID      string      `json:"ticket_id"`
	CustomerEmail string      `json:"customer_email"`
	Price         Price       `json:"price"`
}

type EventHeader struct {
	ID          string    `json:"id"`
	PublishedAt time.Time `json:"published_at"`
}

type TicketsStatusRequest struct {
	Tickets []Ticket `json:"tickets"`
}

type Ticket struct {
	ID            string `json:"ticket_id"`
	Status        string `json:"status"`
	CustomerEmail string `json:"customer_email"`
	Price         Price  `json:"price"`
}

type Price struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

func main() {
	log.Init(logrus.InfoLevel)
	logger := watermill.NewStdLogger(false, false)

	if err := run(logger); err != nil {
		logger.Error("failed to run", err, nil)
	}
}

func run(logger watermill.LoggerAdapter) error {
	c, err := clients.NewClients(os.Getenv("GATEWAY_ADDR"), nil)
	if err != nil {
		return fmt.Errorf("creating gateway client: %w", err)
	}

	receiptsClient := NewReceiptsClient(c)
	spreadsheetsClient := NewSpreadsheetsClient(c)

	rdb := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_ADDR"),
	})

	receiptsSub, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{
		Client:        rdb,
		ConsumerGroup: "issue-receipt",
	}, logger)
	if err != nil {
		return fmt.Errorf("creating receipts subscriber: %w", err)
	}
	defer func() {
		if err := receiptsSub.Close(); err != nil {
			logger.Error("failed to close receipts subscriber", err, nil)
		}
	}()

	router, err := message.NewRouter(message.RouterConfig{}, logger)
	if err != nil {
		return fmt.Errorf("creating router: %w", err)
	}

	router.AddNoPublisherHandler("issue-receipt", TopicTicketBookingConfirmed, receiptsSub,
		processIssueReceipt(receiptsClient))

	trackerSub, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{
		Client:        rdb,
		ConsumerGroup: "append-to-tracker",
	}, logger)
	if err != nil {
		return fmt.Errorf("creating tracker subscriber: %w", err)
	}
	defer func() {
		if err := trackerSub.Close(); err != nil {
			logger.Error("failed to close tracker subscriber", err, nil)
		}
	}()

	router.AddNoPublisherHandler("append-to-tracker", TopicTicketBookingConfirmed, trackerSub,
		processAppendToTracker(spreadsheetsClient))

	publisher, err := redisstream.NewPublisher(redisstream.PublisherConfig{
		Client: rdb,
	}, logger)
	if err != nil {
		return fmt.Errorf("creating publisher: %w", err)
	}

	e := commonHTTP.NewEcho()

	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	e.POST("/tickets-status", func(c echo.Context) error {
		var request TicketsStatusRequest
		if err := c.Bind(&request); err != nil {
			return &echo.HTTPError{
				Code:     http.StatusBadRequest,
				Message:  "failed to parse request",
				Internal: fmt.Errorf("failed to bind request: %w", err),
			}
		}

		for _, ticket := range request.Tickets {
			if err := publishTicketBookingConfirmed(publisher, ticket); err != nil {
				return err
			}
		}

		return c.NoContent(http.StatusOK)
	})

	sigCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	g, runCtx := errgroup.WithContext(sigCtx)

	g.Go(func() error {
		if err := router.Run(runCtx); err != nil {
			return fmt.Errorf("running messaging router: %w", err)
		}

		return nil
	})

	g.Go(func() error {
		// Wait for router
		<-router.Running()

		logrus.Info("Starting HTTP server...")
		err = e.Start(":8080")
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("starting http server: %w", err)
		}

		return nil
	})

	g.Go(func() error {
		<-runCtx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		logrus.Info("Shutting down HTTP server...")
		if err := e.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutting down http server: %w", err)
		}

		return nil
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("waiting for shutdown: %w", err)
	}
	logrus.Info("Shutdown complete.")

	return nil
}

func processIssueReceipt(client ReceiptsClient) func(msg *message.Message) error {
	return func(msg *message.Message) error {
		var body TicketBookingConfirmedEvent
		err := json.Unmarshal(msg.Payload, &body)
		if err != nil {
			return fmt.Errorf("failed to unmarshal payload: %w", err)
		}

		req := IssueReceiptRequest{
			TicketID: body.TicketID,
			Price: receipts.Money{
				MoneyAmount:   body.Price.Amount,
				MoneyCurrency: body.Price.Currency,
			},
		}

		if err := client.IssueReceipt(msg.Context(), req); err != nil {
			return fmt.Errorf("failed to issue receipt: %w", err)
		}

		return nil
	}
}

func processAppendToTracker(client SpreadsheetsClient) func(msg *message.Message) error {
	return func(msg *message.Message) error {
		var body TicketBookingConfirmedEvent
		err := json.Unmarshal(msg.Payload, &body)
		if err != nil {
			return fmt.Errorf("failed to unmarshal payload: %w", err)
		}

		row := []string{body.TicketID, body.CustomerEmail, body.Price.Amount, body.Price.Currency}
		if err := client.AppendRow(msg.Context(), "tickets-to-print", row); err != nil {
			return fmt.Errorf("failed to append row to tracker: %w", err)
		}

		return nil
	}
}

func publishTicketBookingConfirmed(publisher message.Publisher, ticket Ticket) error {
	body := TicketBookingConfirmedEvent{
		Header: EventHeader{
			ID:          watermill.NewUUID(),
			PublishedAt: time.Now().UTC(),
		},
		TicketID:      ticket.ID,
		CustomerEmail: ticket.CustomerEmail,
		Price: Price{
			Amount:   ticket.Price.Amount,
			Currency: ticket.Price.Currency,
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return &echo.HTTPError{
			Message:  http.StatusText(http.StatusInternalServerError),
			Internal: fmt.Errorf("failed to marshal body: %w", err),
		}
	}

	msg := message.NewMessage(body.Header.ID, payload)

	if err := publisher.Publish(TopicTicketBookingConfirmed, msg); err != nil {
		return &echo.HTTPError{
			Message:  http.StatusText(http.StatusInternalServerError),
			Internal: fmt.Errorf("publishing message to topic '%s': %w", TopicTicketBookingConfirmed, err),
		}
	}

	return nil
}

type IssueReceiptRequest struct {
	TicketID string
	Price    receipts.Money
}

type ReceiptsClient struct {
	clients *clients.Clients
}

func NewReceiptsClient(clients *clients.Clients) ReceiptsClient {
	return ReceiptsClient{
		clients: clients,
	}
}

func (c ReceiptsClient) IssueReceipt(ctx context.Context, request IssueReceiptRequest) error {
	body := receipts.PutReceiptsJSONRequestBody{
		TicketId: request.TicketID,
		Price: receipts.Money{
			MoneyAmount:   request.Price.MoneyAmount,
			MoneyCurrency: request.Price.MoneyCurrency,
		},
	}

	receiptsResp, err := c.clients.Receipts.PutReceiptsWithResponse(ctx, body)
	if err != nil {
		return err
	}
	if receiptsResp.StatusCode() != http.StatusOK {
		return fmt.Errorf("unexpected status code: %v", receiptsResp.StatusCode())
	}

	return nil
}

type SpreadsheetsClient struct {
	clients *clients.Clients
}

func NewSpreadsheetsClient(clients *clients.Clients) SpreadsheetsClient {
	return SpreadsheetsClient{
		clients: clients,
	}
}

func (c SpreadsheetsClient) AppendRow(ctx context.Context, spreadsheetName string, row []string) error {
	request := spreadsheets.PostSheetsSheetRowsJSONRequestBody{
		Columns: row,
	}

	sheetsResp, err := c.clients.Spreadsheets.PostSheetsSheetRowsWithResponse(ctx, spreadsheetName, request)
	if err != nil {
		return err
	}
	if sheetsResp.StatusCode() != http.StatusOK {
		return fmt.Errorf("unexpected status code: %v", sheetsResp.StatusCode())
	}

	return nil
}
