package event

import (
	"tickets/entity"
	"time"

	"github.com/ThreeDotsLabs/watermill"
)

type header struct {
	ID          string
	PublishedAt time.Time
}

func newHeader() header {
	return header{
		ID:          watermill.NewUUID(),
		PublishedAt: time.Now().UTC(),
	}
}

type TicketBookingConfirmed struct {
	Header        header       `json:"header"`
	TicketID      string       `json:"ticket_id"`
	CustomerEmail string       `json:"customer_email"`
	Price         entity.Money `json:"price"`
}

func NewTicketBookingConfirmed(ticket entity.Ticket) TicketBookingConfirmed {
	return TicketBookingConfirmed{
		Header:        newHeader(),
		TicketID:      ticket.ID,
		CustomerEmail: ticket.CustomerEmail,
		Price: entity.Money{
			Amount:   ticket.Price.Amount,
			Currency: ticket.Price.Currency,
		},
	}
}

type TicketBookingCanceled struct {
	Header        header       `json:"header"`
	TicketID      string       `json:"ticket_id"`
	CustomerEmail string       `json:"customer_email"`
	Price         entity.Money `json:"price"`
}

func NewTicketBookingCanceled(ticket entity.Ticket) TicketBookingCanceled {
	return TicketBookingCanceled{
		Header:        newHeader(),
		TicketID:      ticket.ID,
		CustomerEmail: ticket.CustomerEmail,
		Price: entity.Money{
			Amount:   ticket.Price.Amount,
			Currency: ticket.Price.Currency,
		},
	}
}
