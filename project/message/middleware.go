package message

import (
	"github.com/ThreeDotsLabs/go-event-driven/common/log"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/lithammer/shortuuid/v3"
	"github.com/sirupsen/logrus"
)

func correlationIDMiddleware(next message.HandlerFunc) message.HandlerFunc {
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

func loggerMiddleware(next message.HandlerFunc) message.HandlerFunc {
	return func(msg *message.Message) ([]*message.Message, error) {
		correlationID := log.CorrelationIDFromContext(msg.Context())
		ctx := log.ToContext(msg.Context(), logrus.WithFields(logrus.Fields{
			"message_uuid":   msg.UUID,
			"correlation_id": correlationID}))
		msg.SetContext(ctx)

		return next(msg)
	}
}

func handlerLogMiddleware(next message.HandlerFunc) message.HandlerFunc {
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

func skipInvalidEventsMiddleware(next message.HandlerFunc) message.HandlerFunc {
	return func(msg *message.Message) ([]*message.Message, error) {
		logger := log.FromContext(msg.Context())

		if msg.UUID == "2beaf5bc-d5e4-4653-b075-2b36bbf28949" {
			logger.Info("Skipping message with uuid 2beaf5bc-d5e4-4653-b075-2b36bbf28949")
			return nil, nil
		}

		return next(msg)
	}
}
