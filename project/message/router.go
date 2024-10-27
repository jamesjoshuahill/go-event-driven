package message

import (
	"fmt"
	"tickets/event"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
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
	receiptIssuer ReceiptIssuer,
	spreadsheetAppender SpreadsheetAppender,
) (*Router, error) {
	receiptsSub, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{
		Client:        rdb,
		ConsumerGroup: "issue-receipt",
	}, logger)
	if err != nil {
		return nil, fmt.Errorf("creating receipts subscriber: %w", err)
	}
	// defer func() {
	// 	if err := receiptsSub.Close(); err != nil {
	// 		logger.Error("failed to close receipts subscriber", err, nil)
	// 	}
	// }()

	trackerConfirmedSub, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{
		Client:        rdb,
		ConsumerGroup: "append-to-tracker-confirmed",
	}, logger)
	if err != nil {
		return nil, fmt.Errorf("creating tracker confirmed subscriber: %w", err)
	}
	// defer func() {
	// 	if err := trackerConfirmedSub.Close(); err != nil {
	// 		logger.Error("failed to close tracker confirmed subscriber", err, nil)
	// 	}
	// }()

	trackerCanceledSub, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{
		Client:        rdb,
		ConsumerGroup: "append-to-tracker-canceled",
	}, logger)
	if err != nil {
		return nil, fmt.Errorf("creating tracker canceled subscriber: %w", err)
	}
	// defer func() {
	// 	if err := trackerCanceledSub.Close(); err != nil {
	// 		logger.Error("failed to close tracker canceled subscriber", err, nil)
	// 	}
	// }()

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

	router.AddNoPublisherHandler(
		"issue-receipt",
		event.TopicTicketBookingConfirmed,
		receiptsSub,
		handleIssueReceipt(receiptIssuer))

	router.AddNoPublisherHandler(
		"append-to-tracker-confirmed",
		event.TopicTicketBookingConfirmed,
		trackerConfirmedSub,
		handleAppendToTrackerConfirmed(spreadsheetAppender))

	router.AddNoPublisherHandler(
		"append-to-tracker-canceled",
		event.TopicTicketBookingCanceled,
		trackerCanceledSub,
		handleAppendToTrackerCanceled(spreadsheetAppender))

	return &Router{router}, nil
}
