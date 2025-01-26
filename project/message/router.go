package message

import (
	"fmt"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/ThreeDotsLabs/watermill/components/cqrs"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/redis/go-redis/v9"
)

type Router struct {
	*message.Router
}

func NewRouter(
	logger watermill.LoggerAdapter,
	rdb *redis.Client,
	ticketGenerator TicketGenerator,
	receiptIssuer ReceiptIssuer,
	spreadsheetAppender SpreadsheetAppender,
	ticketRepo TicketRepo,
) (*Router, error) {
	router, err := message.NewRouter(message.RouterConfig{}, logger)
	if err != nil {
		return nil, fmt.Errorf("creating router: %w", err)
	}

	router.AddMiddleware(correlationIDMiddleware)
	router.AddMiddleware(loggerMiddleware)
	router.AddMiddleware(handlerLogMiddleware)
	router.AddMiddleware(middleware.Retry{
		MaxRetries:      10,
		InitialInterval: time.Millisecond * 100,
		MaxInterval:     time.Second,
		Multiplier:      2,
		Logger:          logger,
	}.Middleware)
	router.AddMiddleware(skipInvalidEventsMiddleware)

	config := cqrs.EventProcessorConfig{
		SubscriberConstructor: func(params cqrs.EventProcessorSubscriberConstructorParams) (message.Subscriber, error) {
			return redisstream.NewSubscriber(redisstream.SubscriberConfig{
				Client:        rdb,
				ConsumerGroup: "svc-users." + params.HandlerName,
			}, logger)
		},
		GenerateSubscribeTopic: func(params cqrs.EventProcessorGenerateSubscribeTopicParams) (string, error) {
			return params.EventName, nil
		},
		Marshaler: cqrs.JSONMarshaler{
			GenerateName: cqrs.StructName,
		},
		Logger: logger,
	}

	ep, err := cqrs.NewEventProcessorWithConfig(router, config)
	if err != nil {
		return nil, fmt.Errorf("creating event processor: %w", err)
	}

	handlers := []cqrs.EventHandler{
		cqrs.NewEventHandler("issue-receipt", handleIssueReceipt(receiptIssuer)),
		cqrs.NewEventHandler("append-to-tracker-confirmed", handleAppendToTrackerConfirmed(spreadsheetAppender)),
		cqrs.NewEventHandler("append-to-tracker-canceled", handleAppendToTrackerCanceled(spreadsheetAppender)),
		cqrs.NewEventHandler("store-confirmed-in-db", handleStoreInDB(ticketRepo)),
		cqrs.NewEventHandler("remove-canceled-from-db", handleRemoveCanceledFromDB(ticketRepo)),
		cqrs.NewEventHandler("store-file-to-print", handleStoreFileToPrint(ticketGenerator)),
	}

	ep.AddHandlers(handlers...)

	return &Router{router}, nil
}
