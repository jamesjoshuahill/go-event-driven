package message

import (
	"fmt"
	"tickets/event"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
)

func NewRouter(
	logger watermill.LoggerAdapter,
	receiptsSub message.Subscriber,
	trackerConfirmedSub message.Subscriber,
	trackerCanceledSub message.Subscriber,
	receiptIssuer ReceiptIssuer,
	spreadsheetAppender SpreadsheetAppender,
) (*message.Router, error) {
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

	return router, nil
}
