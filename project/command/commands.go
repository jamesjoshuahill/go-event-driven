package command

import (
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

type RefundTicket struct {
	TicketID string `json:"ticket_id"`
	Header   header
}

func NewRefundTicket(ticketID, idempotencyKey string) RefundTicket {
	return RefundTicket{
		Header:   newHeader(idempotencyKey),
		TicketID: ticketID,
	}
}
