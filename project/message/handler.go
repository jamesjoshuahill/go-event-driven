package message

import (
	"context"
	"encoding/json"
	"fmt"
	"tickets/entity"
	"tickets/event"

	"github.com/ThreeDotsLabs/watermill/message"
)

type ReceiptIssuer interface {
	IssueReceipt(ctx context.Context, ticketID string, money entity.Money) error
}

type SpreadsheetAppender interface {
	AppendRow(ctx context.Context, spreadsheetName string, row []string) error
}

func handleIssueReceipt(client ReceiptIssuer) func(msg *message.Message) error {
	return func(msg *message.Message) error {
		var body event.TicketBookingConfirmed
		err := json.Unmarshal(msg.Payload, &body)
		if err != nil {
			return fmt.Errorf("failed to unmarshal payload: %w", err)
		}

		currency := body.Price.Currency
		if currency == "" {
			currency = "USD"
		}

		price := entity.Money{
			Amount:   body.Price.Amount,
			Currency: currency,
		}

		if err := client.IssueReceipt(msg.Context(), body.TicketID, price); err != nil {
			return err
		}

		return nil
	}
}

func handleAppendToTrackerConfirmed(client SpreadsheetAppender) func(msg *message.Message) error {
	return func(msg *message.Message) error {
		var body event.TicketBookingConfirmed
		err := json.Unmarshal(msg.Payload, &body)
		if err != nil {
			return fmt.Errorf("failed to unmarshal payload: %w", err)
		}

		currency := body.Price.Currency
		if currency == "" {
			currency = "USD"
		}

		row := []string{body.TicketID, body.CustomerEmail, body.Price.Amount, currency}
		if err := client.AppendRow(msg.Context(), "tickets-to-print", row); err != nil {
			return fmt.Errorf("failed to append row to tracker: %w", err)
		}

		return nil
	}
}

func handleAppendToTrackerCanceled(client SpreadsheetAppender) func(msg *message.Message) error {
	return func(msg *message.Message) error {
		var body event.TicketBookingCanceled
		err := json.Unmarshal(msg.Payload, &body)
		if err != nil {
			return fmt.Errorf("failed to unmarshal payload: %w", err)
		}

		row := []string{body.TicketID, body.CustomerEmail, body.Price.Amount, body.Price.Currency}
		if err := client.AppendRow(msg.Context(), "tickets-to-refund", row); err != nil {
			return fmt.Errorf("failed to append row to tracker: %w", err)
		}

		return nil
	}
}
