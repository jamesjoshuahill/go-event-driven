package message

import (
	"context"
	"fmt"
	"tickets/entity"
	"tickets/event"
)

type ReceiptIssuer interface {
	IssueReceipt(ctx context.Context, ticketID string, money entity.Money) error
}

type SpreadsheetAppender interface {
	AppendRow(ctx context.Context, spreadsheetName string, row []string) error
}

type TicketRepo interface {
	Create(ctx context.Context, ticket entity.Ticket) error
}

func handleIssueReceipt(r ReceiptIssuer) func(ctx context.Context, e *event.TicketBookingConfirmed) error {
	return func(ctx context.Context, e *event.TicketBookingConfirmed) error {
		currency := e.Price.Currency
		if currency == "" {
			currency = "USD"
		}

		price := entity.Money{
			Amount:   e.Price.Amount,
			Currency: currency,
		}

		if err := r.IssueReceipt(ctx, e.TicketID, price); err != nil {
			return err
		}

		return nil
	}
}

func handleAppendToTrackerConfirmed(s SpreadsheetAppender) func(context.Context, *event.TicketBookingConfirmed) error {
	return func(ctx context.Context, e *event.TicketBookingConfirmed) error {
		currency := e.Price.Currency
		if currency == "" {
			currency = "USD"
		}

		row := []string{e.TicketID, e.CustomerEmail, e.Price.Amount, currency}
		if err := s.AppendRow(ctx, "tickets-to-print", row); err != nil {
			return fmt.Errorf("failed to append row to tracker: %w", err)
		}

		return nil
	}
}

func handleAppendToTrackerCanceled(s SpreadsheetAppender) func(context.Context, *event.TicketBookingCanceled) error {
	return func(ctx context.Context, e *event.TicketBookingCanceled) error {
		row := []string{e.TicketID, e.CustomerEmail, e.Price.Amount, e.Price.Currency}
		if err := s.AppendRow(ctx, "tickets-to-refund", row); err != nil {
			return fmt.Errorf("failed to append row to tracker: %w", err)
		}

		return nil
	}
}

func handleStoreInDB(repo TicketRepo) func(context.Context, *event.TicketBookingConfirmed) error {
	return func(ctx context.Context, e *event.TicketBookingConfirmed) error {
		t := entity.Ticket{
			ID:            e.TicketID,
			Status:        entity.StatusConfirmed,
			CustomerEmail: e.CustomerEmail,
			Price: entity.Money{
				Amount:   e.Price.Amount,
				Currency: e.Price.Currency,
			},
		}
		return repo.Create(ctx, t)
	}
}
