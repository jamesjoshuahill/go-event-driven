package message

import (
	"context"
	"fmt"
	"tickets/entity"
	"tickets/event"
)

type Publisher interface {
	Publish(ctx context.Context, event any) error
}

type ReceiptIssuer interface {
	IssueReceipt(ctx context.Context, idempotencyKey, ticketID string, price entity.Money) error
}

type SpreadsheetAppender interface {
	AppendRow(ctx context.Context, spreadsheetName string, row []string) error
}

type TicketGenerator interface {
	GenerateTicket(ctx context.Context, ticketID string, price entity.Money) (string, error)
}

type TicketRepo interface {
	Add(ctx context.Context, ticket entity.Ticket) error
	Delete(ctx context.Context, ticketID string) error
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

		if err := r.IssueReceipt(ctx, e.Header.IdempotencyKey, e.TicketID, price); err != nil {
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
			CustomerEmail: e.CustomerEmail,
			Price: entity.Money{
				Amount:   e.Price.Amount,
				Currency: e.Price.Currency,
			},
		}
		return repo.Add(ctx, t)
	}
}

func handleRemoveCanceledFromDB(repo TicketRepo) func(context.Context, *event.TicketBookingCanceled) error {
	return func(ctx context.Context, e *event.TicketBookingCanceled) error {
		return repo.Delete(ctx, e.TicketID)
	}
}

func handlePrintTicket(g TicketGenerator, p Publisher) func(context.Context, *event.TicketBookingConfirmed) error {
	return func(ctx context.Context, e *event.TicketBookingConfirmed) error {
		fileID, err := g.GenerateTicket(ctx, e.TicketID, e.Price)
		if err != nil {
			return fmt.Errorf("generating ticket: %w", err)
		}

		ticketPrinted := event.NewTicketPrinted(e.Header.IdempotencyKey, e.TicketID, fileID)

		if err := p.Publish(ctx, ticketPrinted); err != nil {
			return fmt.Errorf("publishing ticket printed event: %w", err)
		}

		return nil
	}
}
