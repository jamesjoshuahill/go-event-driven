package event

import (
	"tickets/entity"
	"time"

	"github.com/ThreeDotsLabs/watermill"
)

const (
	TopicTicketBookingConfirmed = "TicketBookingConfirmed"
	TopicTicketBookingCanceled  = "TicketBookingCanceled"
)

type Header struct {
	ID          string
	PublishedAt time.Time
}

func NewHeader() Header {
	return Header{
		ID:          watermill.NewUUID(),
		PublishedAt: time.Now().UTC(),
	}
}

type TicketBookingConfirmed struct {
	Header        Header       `json:"header"`
	TicketID      string       `json:"ticket_id"`
	CustomerEmail string       `json:"customer_email"`
	Price         entity.Money `json:"price"`
}

func (e TicketBookingConfirmed) Type() string {
	return "TicketBookingConfirmed"
}

type TicketBookingCanceled struct {
	Header        Header       `json:"header"`
	TicketID      string       `json:"ticket_id"`
	CustomerEmail string       `json:"customer_email"`
	Price         entity.Money `json:"price"`
}

func (e TicketBookingCanceled) Type() string {
	return "TicketBookingCanceled"
}
