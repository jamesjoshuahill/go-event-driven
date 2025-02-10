package message

import (
	"context"
	"fmt"

	"tickets/command"
	"tickets/entity"
	"tickets/event"
)

type ShowRepo interface {
	Get(ctx context.Context, showID string) (entity.Show, error)
}

type DeadNationBooker interface {
	CreateBooking(ctx context.Context, deadNationID string, booking entity.Booking) error
}

type Publisher interface {
	Publish(ctx context.Context, event any) error
}

type ReceiptsClient interface {
	IssueReceipt(ctx context.Context, idempotencyKey, ticketID string, price entity.Money) error
	VoidReceipt(ctx context.Context, idempotencyKey, ticketID string) error
}

type SpreadsheetAppender interface {
	AppendRow(ctx context.Context, spreadsheetName string, row []string) error
}

type TicketGenerator interface {
	GenerateTicket(ctx context.Context, ticketID string, price entity.Money) (string, error)
}

type PaymentRefunder interface {
	RefundPayment(ctx context.Context, idempotencyKey string, ticketID string) error
}

type TicketRepo interface {
	Add(ctx context.Context, ticket entity.Ticket) error
	Delete(ctx context.Context, ticketID string) error
}

func handleCreateDeadNationBooking(r ShowRepo, b DeadNationBooker) func(ctx context.Context, e *event.BookingMade) error {
	return func(ctx context.Context, e *event.BookingMade) error {
		show, err := r.Get(ctx, e.ShowID)
		if err != nil {
			return fmt.Errorf("getting show: %w", err)
		}

		booking := entity.Booking{
			BookingID:       e.BookingID,
			CustomerEmail:   e.CustomerEmail,
			NumberOfTickets: e.NumberOfTickets,
			ShowID:          e.ShowID,
		}
		if err := b.CreateBooking(ctx, show.DeadNationID, booking); err != nil {
			return fmt.Errorf("creating dead nation booking: %w", err)
		}

		return nil
	}
}

func handleIssueReceipt(r ReceiptsClient) func(ctx context.Context, e *event.TicketBookingConfirmed) error {
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

func handleRefundTicket(p PaymentRefunder, r ReceiptsClient) func(ctx context.Context, cmd *command.RefundTicket) error {
	return func(ctx context.Context, cmd *command.RefundTicket) error {
		if err := p.RefundPayment(ctx, cmd.Header.IdempotencyKey, cmd.TicketID); err != nil {
			return fmt.Errorf("refunding payment: %w", err)
		}

		if err := r.VoidReceipt(ctx, cmd.Header.IdempotencyKey, cmd.TicketID); err != nil {
			return fmt.Errorf("voiding ticket receipt: %w", err)
		}

		return nil
	}
}
