package main

import (
	"github.com/ThreeDotsLabs/watermill/components/cqrs"
	"github.com/ThreeDotsLabs/watermill/message"
)

func NewEventBus(pub message.Publisher) (*cqrs.EventBus, error) {
	return cqrs.NewEventBus(pub, generateTopic, cqrs.JSONMarshaler{})
}

func generateTopic(eventName string) string {
	return eventName
}
