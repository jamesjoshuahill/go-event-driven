package main

import (
	"context"

	"github.com/ThreeDotsLabs/watermill/components/cqrs"
)

type FollowRequestSent struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type EventsCounter interface {
	CountEvent() error
}

type FollowRequestSentHandler struct {
	counter EventsCounter
}

func (h FollowRequestSentHandler) CountEvent(_ context.Context, event *FollowRequestSent) error {
	return h.counter.CountEvent()
}

func NewFollowRequestSentHandler(counter EventsCounter) cqrs.EventHandler {
	h := FollowRequestSentHandler{
		counter: counter,
	}

	return cqrs.NewEventHandler(
		"FollowRequestSent",
		h.CountEvent,
	)
}
