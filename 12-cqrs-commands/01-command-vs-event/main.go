package main

import (
	"context"
	"fmt"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/components/cqrs"
	"github.com/ThreeDotsLabs/watermill/message"
)

type SendNotification struct {
	NotificationID string
	Email          string
	Message        string
}

type Sender interface {
	SendNotification(ctx context.Context, notificationID, email, message string) error
}

func NewProcessor(router *message.Router, sender Sender, sub message.Subscriber, watermillLogger watermill.LoggerAdapter) *cqrs.CommandProcessor {
	commandProcessor, err := cqrs.NewCommandProcessorWithConfig(router, cqrs.CommandProcessorConfig{
		GenerateSubscribeTopic: func(params cqrs.CommandProcessorGenerateSubscribeTopicParams) (string, error) {
			return "commands", nil
		},
		SubscriberConstructor: func(params cqrs.CommandProcessorSubscriberConstructorParams) (message.Subscriber, error) {
			return sub, nil
		},
		Marshaler: cqrs.JSONMarshaler{
			GenerateName: cqrs.StructName,
		},
		Logger: watermillLogger,
	})
	if err != nil {
		panic(err)
	}

	err = commandProcessor.AddHandlers(cqrs.NewCommandHandler(
		"send_notification",
		func(ctx context.Context, cmd *SendNotification) error {
			fmt.Println("Sending notification", cmd.NotificationID, cmd.Email, cmd.Message)
			return sender.SendNotification(ctx, cmd.NotificationID, cmd.Email, cmd.Message)
		},
	))
	if err != nil {
		panic(err)
	}

	return commandProcessor
}
