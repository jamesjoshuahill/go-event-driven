package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"tickets/event"
	libHTTP "tickets/http"
	"tickets/service"
	"time"

	"github.com/ThreeDotsLabs/go-event-driven/common/clients"
	"github.com/ThreeDotsLabs/go-event-driven/common/clients/receipts"
	"github.com/ThreeDotsLabs/go-event-driven/common/clients/spreadsheets"
	"github.com/ThreeDotsLabs/go-event-driven/common/log"
	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/lithammer/shortuuid/v3"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

const (
	TopicTicketBookingConfirmed = "TicketBookingConfirmed"
	TopicTicketBookingCanceled  = "TicketBookingCanceled"
)

func main() {
	log.Init(logrus.InfoLevel)
	logger := watermill.NewStdLogger(false, false)

	if err := run(logger); err != nil {
		logger.Error("failed to run", err, nil)
	}
}

func run(logger watermill.LoggerAdapter) error {
	c, err := clients.NewClients(os.Getenv("GATEWAY_ADDR"), func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Correlation-ID", log.CorrelationIDFromContext(ctx))
		return nil
	})
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

	router.AddMiddleware(CorrelationIDMiddleware)
	router.AddMiddleware(LoggerMiddleware)
	router.AddMiddleware(HandlerLogMiddleware)
	router.AddMiddleware(middleware.Retry{
		MaxRetries:      10,
		InitialInterval: time.Millisecond * 100,
		MaxInterval:     time.Second,
		Multiplier:      2,
		Logger:          logger,
	}.Middleware)
	router.AddMiddleware(SkipInvalidEventsMiddleware)

	router.AddNoPublisherHandler("issue-receipt", TopicTicketBookingConfirmed, receiptsSub,
		processIssueReceipt(receiptsClient))

	trackerConfirmedSub, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{
		Client:        rdb,
		ConsumerGroup: "append-to-tracker-confirmed",
	}, logger)
	if err != nil {
		return fmt.Errorf("creating tracker confirmed subscriber: %w", err)
	}
	defer func() {
		if err := trackerConfirmedSub.Close(); err != nil {
			logger.Error("failed to close tracker confirmed subscriber", err, nil)
		}
	}()

	router.AddNoPublisherHandler("append-to-tracker-confirmed", TopicTicketBookingConfirmed, trackerConfirmedSub,
		processAppendToTrackerConfirmed(spreadsheetsClient))

	trackerCanceledSub, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{
		Client:        rdb,
		ConsumerGroup: "append-to-tracker-canceled",
	}, logger)
	if err != nil {
		return fmt.Errorf("creating tracker canceled subscriber: %w", err)
	}
	defer func() {
		if err := trackerCanceledSub.Close(); err != nil {
			logger.Error("failed to close tracker canceled subscriber", err, nil)
		}
	}()

	router.AddNoPublisherHandler("append-to-tracker-canceled", TopicTicketBookingCanceled, trackerCanceledSub,
		processAppendToTrackerCanceled(spreadsheetsClient))

	publisher, err := redisstream.NewPublisher(redisstream.PublisherConfig{
		Client: rdb,
	}, logger)
	if err != nil {
		return fmt.Errorf("creating publisher: %w", err)
	}

	httpRouter := libHTTP.NewRouter(publisher)

	return service.Run(router, httpRouter)
}

func CorrelationIDMiddleware(next message.HandlerFunc) message.HandlerFunc {
	return func(msg *message.Message) ([]*message.Message, error) {
		correlationID := middleware.MessageCorrelationID(msg)
		if correlationID == "" {
			correlationID = "gen_" + shortuuid.New()
		}

		ctx := log.ContextWithCorrelationID(msg.Context(), correlationID)
		msg.SetContext(ctx)

		return next(msg)
	}
}

func LoggerMiddleware(next message.HandlerFunc) message.HandlerFunc {
	return func(msg *message.Message) ([]*message.Message, error) {
		correlationID := log.CorrelationIDFromContext(msg.Context())
		ctx := log.ToContext(msg.Context(), logrus.WithFields(logrus.Fields{
			"message_uuid":   msg.UUID,
			"correlation_id": correlationID}))
		msg.SetContext(ctx)

		return next(msg)
	}
}

func HandlerLogMiddleware(next message.HandlerFunc) message.HandlerFunc {
	return func(msg *message.Message) ([]*message.Message, error) {
		logger := log.FromContext(msg.Context())
		logger.Info("Handling a message")

		msgs, err := next(msg)

		if err != nil {
			logger.WithError(err).Error("Message handling error")
		}

		return msgs, err
	}
}

func SkipInvalidEventsMiddleware(next message.HandlerFunc) message.HandlerFunc {
	return func(msg *message.Message) ([]*message.Message, error) {
		logger := log.FromContext(msg.Context())

		msgType := msg.Metadata.Get("type")
		if msgType == "" {
			logger.Info("Skipping message with no type")
			return nil, nil
		}

		if msg.UUID == "2beaf5bc-d5e4-4653-b075-2b36bbf28949" {
			logger.Info("Skipping message with uuid 2beaf5bc-d5e4-4653-b075-2b36bbf28949")
			return nil, nil
		}

		return next(msg)
	}
}

func processIssueReceipt(client ReceiptsClient) func(msg *message.Message) error {
	return func(msg *message.Message) error {
		var body event.TicketBookingConfirmed
		err := json.Unmarshal(msg.Payload, &body)
		if err != nil {
			return fmt.Errorf("failed to unmarshal payload: %w", err)
		}

		currency := body.Price.Currency
		if currency == "" {
			currency = "USD"
		}

		req := IssueReceiptRequest{
			TicketID: body.TicketID,
			Price: receipts.Money{
				MoneyAmount:   body.Price.Amount,
				MoneyCurrency: currency,
			},
		}

		if err := client.IssueReceipt(msg.Context(), req); err != nil {
			return err
		}

		return nil
	}
}

func processAppendToTrackerConfirmed(client SpreadsheetsClient) func(msg *message.Message) error {
	return func(msg *message.Message) error {
		var body event.TicketBookingConfirmed
		err := json.Unmarshal(msg.Payload, &body)
		if err != nil {
			return fmt.Errorf("failed to unmarshal payload: %w", err)
		}

		currency := body.Price.Currency
		if currency == "" {
			currency = "USD"
		}

		row := []string{body.TicketID, body.CustomerEmail, body.Price.Amount, currency}
		if err := client.AppendRow(msg.Context(), "tickets-to-print", row); err != nil {
			return fmt.Errorf("failed to append row to tracker: %w", err)
		}

		return nil
	}
}

func processAppendToTrackerCanceled(client SpreadsheetsClient) func(msg *message.Message) error {
	return func(msg *message.Message) error {
		var body event.TicketBookingCanceled
		err := json.Unmarshal(msg.Payload, &body)
		if err != nil {
			return fmt.Errorf("failed to unmarshal payload: %w", err)
		}

		row := []string{body.TicketID, body.CustomerEmail, body.Price.Amount, body.Price.Currency}
		if err := client.AppendRow(msg.Context(), "tickets-to-refund", row); err != nil {
			return fmt.Errorf("failed to append row to tracker: %w", err)
		}

		return nil
	}
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
