package event

import (
	"tickets/entity"
	"time"

	"github.com/ThreeDotsLabs/watermill"
)

type header struct {
	ID             string    `json:"id"`
	PublishedAt    time.Time `json:"published_at"`
	IdempotencyKey string    `json:"idempotency_key"`
}

func newHeader(idempotencyKey string) header {
	return header{
		ID:             watermill.NewUUID(),
		PublishedAt:    time.Now().UTC(),
		IdempotencyKey: idempotencyKey,
	}
}

type TicketBookingConfirmed struct {
	Header        header       `json:"header"`
	TicketID      string       `json:"ticket_id"`
	CustomerEmail string       `json:"customer_email"`
	Price         entity.Money `json:"price"`
}

func NewTicketBookingConfirmed(idempotencyKey string, ticket entity.Ticket) TicketBookingConfirmed {
	return TicketBookingConfirmed{
		Header:        newHeader(idempotencyKey),
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

func NewTicketBookingCanceled(idempotencyKey string, ticket entity.Ticket) TicketBookingCanceled {
	return TicketBookingCanceled{
		Header:        newHeader(idempotencyKey),
		TicketID:      ticket.ID,
		CustomerEmail: ticket.CustomerEmail,
		Price: entity.Money{
			Amount:   ticket.Price.Amount,
			Currency: ticket.Price.Currency,
		},
	}
}

type TicketPrinted struct {
	Header   header `json:"header"`
	TicketID string `json:"ticket_id"`
	FileName string `json:"file_name"`
}

func NewTicketPrinted(idempotencyKey, ticketID, fileName string) TicketPrinted {
	return TicketPrinted{
		Header:   newHeader(idempotencyKey),
		TicketID: ticketID,
		FileName: fileName,
	}
}

type BookingMade struct {
	Header          header `json:"header"`
	BookingID       string `json:"booking_id"`
	ShowID          string `json:"show_id"`
	NumberOfTickets uint   `json:"number_of_tickets"`
	CustomerEmail   string `json:"customer_email"`
}

func NewBookingMade(idempotencyKey string, booking entity.Booking) BookingMade {
	return BookingMade{
		Header:          newHeader(idempotencyKey),
		BookingID:       booking.BookingID,
		ShowID:          booking.ShowID,
		NumberOfTickets: booking.NumberOfTickets,
		CustomerEmail:   booking.CustomerEmail,
	}
}
