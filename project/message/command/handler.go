package command

import (
	"context"
	"fmt"
)

type PaymentsClient interface {
	RefundPayment(ctx context.Context, idempotencyKey string, ticketID string) error
}

type ReceiptsClient interface {
	VoidReceipt(ctx context.Context, idempotencyKey, ticketID string) error
}

type Handler struct {
	payments PaymentsClient
	receipts ReceiptsClient
}

func NewHandler(p PaymentsClient, r ReceiptsClient) Handler {
	return Handler{
		payments: p,
		receipts: r,
	}
}

func (h Handler) RefundTicket(ctx context.Context, cmd *RefundTicket) error {
	if err := h.payments.RefundPayment(ctx, cmd.Header.IdempotencyKey, cmd.TicketID); err != nil {
		return fmt.Errorf("refunding payment: %w", err)
	}

	if err := h.receipts.VoidReceipt(ctx, cmd.Header.IdempotencyKey, cmd.TicketID); err != nil {
		return fmt.Errorf("voiding ticket receipt: %w", err)
	}

	return nil
}
