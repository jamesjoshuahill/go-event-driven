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

type RouterDeps struct {
	DeadNationBooker    DeadNationBooker
	Logger              watermill.LoggerAdapter
	Publisher           Publisher
	ReceiptIssuer       ReceiptIssuer
	RedisClient         *redis.Client
	ShowRepo            ShowRepo
	SpreadsheetAppender SpreadsheetAppender
	TicketGenerator     TicketGenerator
	TicketRepo          TicketRepo
}

type Router struct {
	*message.Router
}

func NewRouter(deps RouterDeps) (*Router, error) {
	router, err := message.NewRouter(message.RouterConfig{}, deps.Logger)
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
		Logger:          deps.Logger,
	}.Middleware)
	router.AddMiddleware(skipInvalidEventsMiddleware)

	config := cqrs.EventProcessorConfig{
		SubscriberConstructor: func(params cqrs.EventProcessorSubscriberConstructorParams) (message.Subscriber, error) {
			return redisstream.NewSubscriber(redisstream.SubscriberConfig{
				Client:        deps.RedisClient,
				ConsumerGroup: "svc-users." + params.HandlerName,
			}, deps.Logger)
		},
		GenerateSubscribeTopic: func(params cqrs.EventProcessorGenerateSubscribeTopicParams) (string, error) {
			return params.EventName, nil
		},
		Marshaler: cqrs.JSONMarshaler{
			GenerateName: cqrs.StructName,
		},
		Logger: deps.Logger,
	}

	ep, err := cqrs.NewEventProcessorWithConfig(router, config)
	if err != nil {
		return nil, fmt.Errorf("creating event processor: %w", err)
	}

	handlers := []cqrs.EventHandler{
		cqrs.NewEventHandler("create-dead-nation-booking", handleCreateDeadNationBooking(deps.ShowRepo, deps.DeadNationBooker)),
		cqrs.NewEventHandler("issue-receipt", handleIssueReceipt(deps.ReceiptIssuer)),
		cqrs.NewEventHandler("append-to-tracker-confirmed", handleAppendToTrackerConfirmed(deps.SpreadsheetAppender)),
		cqrs.NewEventHandler("append-to-tracker-canceled", handleAppendToTrackerCanceled(deps.SpreadsheetAppender)),
		cqrs.NewEventHandler("store-confirmed-in-db", handleStoreInDB(deps.TicketRepo)),
		cqrs.NewEventHandler("remove-canceled-from-db", handleRemoveCanceledFromDB(deps.TicketRepo)),
		cqrs.NewEventHandler("print-ticket", handlePrintTicket(deps.TicketGenerator, deps.Publisher)),
	}

	if err := ep.AddHandlers(handlers...); err != nil {
		return nil, fmt.Errorf("adding handlers: %w", err)
	}

	return &Router{router}, nil
}
