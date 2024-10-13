package message

import (
	"encoding/json"
	"fmt"
	"tickets/entity"
	"tickets/event"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
)

func NewTicketBookingConfirmed(ticket entity.Ticket, correlationID string) (*message.Message, error) {
	e := event.TicketBookingConfirmed{
		Header:        event.NewHeader(),
		TicketID:      ticket.ID,
		CustomerEmail: ticket.CustomerEmail,
		Price: entity.Money{
			Amount:   ticket.Price.Amount,
			Currency: ticket.Price.Currency,
		},
	}

	payload, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("marshalling event: %w", err)
	}

	msg := message.NewMessage(watermill.NewUUID(), payload)
	middleware.SetCorrelationID(correlationID, msg)
	msg.Metadata.Set("type", e.Type())

	return msg, nil
}

func NewTicketBookingCanceled(ticket entity.Ticket, correlationID string) (*message.Message, error) {
	e := event.TicketBookingCanceled{
		Header:        event.NewHeader(),
		TicketID:      ticket.ID,
		CustomerEmail: ticket.CustomerEmail,
		Price: entity.Money{
			Amount:   ticket.Price.Amount,
			Currency: ticket.Price.Currency,
		},
	}

	payload, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("marshalling event: %w", err)
	}

	msg := message.NewMessage(watermill.NewUUID(), payload)
	middleware.SetCorrelationID(correlationID, msg)
	msg.Metadata.Set("type", e.Type())

	return msg, nil
}
