package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

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
)

const (
	TopicIssueReceipt    = "issue-receipt"
	TopicAppendToTracker = "append-to-tracker"
)

type TicketsConfirmationRequest struct {
	Tickets []string `json:"tickets"`
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
		ConsumerGroup: TopicIssueReceipt,
	}, logger)
	if err != nil {
		return fmt.Errorf("creating receipts subscriber: %w", err)
	}

	receiptsMessages, err := receiptsSub.Subscribe(context.Background(), TopicIssueReceipt)
	if err != nil {
		return fmt.Errorf("subscribing to topic '%s': %w", TopicIssueReceipt, err)
	}
	defer func() {
		if err := receiptsSub.Close(); err != nil {
			logger.Error("failed to close receipts subscriber", err, nil)
		}
	}()

	go processReceipts(receiptsClient, receiptsMessages, logger)

	trackerSub, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{
		Client:        rdb,
		ConsumerGroup: TopicAppendToTracker,
	}, logger)
	if err != nil {
		return fmt.Errorf("creating tracker subscriber: %w", err)
	}
	defer func() {
		if err := trackerSub.Close(); err != nil {
			logger.Error("failed to close tracker subscriber", err, nil)
		}
	}()

	trackerMessages, err := trackerSub.Subscribe(context.Background(), TopicAppendToTracker)
	if err != nil {
		return fmt.Errorf("subscribing to topic '%s': %w", TopicAppendToTracker, err)
	}

	go processTracker(spreadsheetsClient, trackerMessages, logger)

	publisher, err := redisstream.NewPublisher(redisstream.PublisherConfig{
		Client: rdb,
	}, logger)
	if err != nil {
		return fmt.Errorf("creating publisher: %w", err)
	}

	e := commonHTTP.NewEcho()

	e.POST("/tickets-confirmation", func(c echo.Context) error {
		var request TicketsConfirmationRequest
		if err := c.Bind(&request); err != nil {
			return &echo.HTTPError{
				Code:     http.StatusBadRequest,
				Message:  "failed to parse request",
				Internal: fmt.Errorf("failed to bind request: %w", err),
			}
		}

		for _, ticketID := range request.Tickets {
			msg := message.NewMessage(watermill.NewUUID(), []byte(ticketID))

			if err := publisher.Publish(TopicIssueReceipt, msg); err != nil {
				return &echo.HTTPError{
					Message:  http.StatusText(http.StatusInternalServerError),
					Internal: fmt.Errorf("publishing message to topic '%s': %w", TopicIssueReceipt, err),
				}
			}

			if err := publisher.Publish(TopicAppendToTracker, msg); err != nil {
				return &echo.HTTPError{
					Message:  http.StatusText(http.StatusInternalServerError),
					Internal: fmt.Errorf("publishing message to topic '%s': %w", TopicAppendToTracker, err),
				}
			}
		}

		return c.NoContent(http.StatusOK)
	})

	logrus.Info("Server starting...")

	err = e.Start(":8080")
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("starting http server: %w", err)
	}

	return nil
}

func processReceipts(client ReceiptsClient, messages <-chan *message.Message, logger watermill.LoggerAdapter) {
	for msg := range messages {
		ticketID := string(msg.Payload)
		if err := client.IssueReceipt(msg.Context(), ticketID); err != nil {
			logger.Error("failed to issue receipt: %w", err, nil)
			msg.Nack()
			continue
		}

		msg.Ack()
	}
}
func processTracker(client SpreadsheetsClient, messages <-chan *message.Message, logger watermill.LoggerAdapter) {
	for msg := range messages {
		ticketID := string(msg.Payload)
		if err := client.AppendRow(msg.Context(), "tickets-to-print", []string{ticketID}); err != nil {
			logger.Error("failed to append row to tracker: %w", err, nil)
			msg.Nack()
			continue
		}

		msg.Ack()
	}
}

type ReceiptsClient struct {
	clients *clients.Clients
}

func NewReceiptsClient(clients *clients.Clients) ReceiptsClient {
	return ReceiptsClient{
		clients: clients,
	}
}

func (c ReceiptsClient) IssueReceipt(ctx context.Context, ticketID string) error {
	body := receipts.PutReceiptsJSONRequestBody{
		TicketId: ticketID,
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
