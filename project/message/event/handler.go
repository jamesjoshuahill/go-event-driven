package event

import (
	"context"
	"fmt"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/ThreeDotsLabs/watermill/components/cqrs"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/redis/go-redis/v9"
	"tickets/entity"
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

func NewProcessorConfig(logger watermill.LoggerAdapter, redisClient *redis.Client) cqrs.EventProcessorConfig {
	return cqrs.EventProcessorConfig{
		SubscriberConstructor: func(params cqrs.EventProcessorSubscriberConstructorParams) (message.Subscriber, error) {
			return redisstream.NewSubscriber(redisstream.SubscriberConfig{
				Client:        redisClient,
				ConsumerGroup: "svc-users." + params.HandlerName,
			}, logger)
		},
		GenerateSubscribeTopic: func(params cqrs.EventProcessorGenerateSubscribeTopicParams) (string, error) {
			return params.EventName, nil
		},
		Marshaler: cqrs.JSONMarshaler{
			GenerateName: cqrs.StructName,
		},
		Logger: logger,
	}
}

type Handler struct {
	deadNationBooker    DeadNationBooker
	publisher           Publisher
	receiptsClient      ReceiptsClient
	showRepo            ShowRepo
	spreadsheetAppender SpreadsheetAppender
	ticketGenerator     TicketGenerator
	ticketRepo          TicketRepo
}

func NewHandler(
	d DeadNationBooker,
	p Publisher,
	r ReceiptsClient,
	sr ShowRepo,
	sa SpreadsheetAppender,
	tg TicketGenerator,
	tr TicketRepo,
) Handler {
	return Handler{
		deadNationBooker:    d,
		publisher:           p,
		receiptsClient:      r,
		showRepo:            sr,
		spreadsheetAppender: sa,
		ticketGenerator:     tg,
		ticketRepo:          tr,
	}
}

func (h Handler) CreateDeadNationBooking(ctx context.Context, e *BookingMade) error {
	show, err := h.showRepo.Get(ctx, e.ShowID)
	if err != nil {
		return fmt.Errorf("getting show: %w", err)
	}

	booking := entity.Booking{
		BookingID:       e.BookingID,
		CustomerEmail:   e.CustomerEmail,
		NumberOfTickets: e.NumberOfTickets,
		ShowID:          e.ShowID,
	}
	if err := h.deadNationBooker.CreateBooking(ctx, show.DeadNationID, booking); err != nil {
		return fmt.Errorf("creating dead nation booking: %w", err)
	}

	return nil
}

func (h Handler) IssueReceipt(ctx context.Context, e *TicketBookingConfirmed) error {
	currency := e.Price.Currency
	if currency == "" {
		currency = "USD"
	}

	price := entity.Money{
		Amount:   e.Price.Amount,
		Currency: currency,
	}

	if err := h.receiptsClient.IssueReceipt(ctx, e.Header.IdempotencyKey, e.TicketID, price); err != nil {
		return err
	}

	return nil
}

func (h Handler) AppendToTrackerConfirmed(ctx context.Context, e *TicketBookingConfirmed) error {
	currency := e.Price.Currency
	if currency == "" {
		currency = "USD"
	}

	row := []string{e.TicketID, e.CustomerEmail, e.Price.Amount, currency}
	if err := h.spreadsheetAppender.AppendRow(ctx, "tickets-to-print", row); err != nil {
		return fmt.Errorf("failed to append row to tracker: %w", err)
	}

	return nil
}

func (h Handler) AppendToTrackerCanceled(ctx context.Context, e *TicketBookingCanceled) error {
	row := []string{e.TicketID, e.CustomerEmail, e.Price.Amount, e.Price.Currency}
	if err := h.spreadsheetAppender.AppendRow(ctx, "tickets-to-refund", row); err != nil {
		return fmt.Errorf("failed to append row to tracker: %w", err)
	}

	return nil
}

func (h Handler) StoreInDB(ctx context.Context, e *TicketBookingConfirmed) error {
	t := entity.Ticket{
		ID:            e.TicketID,
		CustomerEmail: e.CustomerEmail,
		Price: entity.Money{
			Amount:   e.Price.Amount,
			Currency: e.Price.Currency,
		},
	}
	return h.ticketRepo.Add(ctx, t)
}

func (h Handler) RemoveCanceledFromDB(ctx context.Context, e *TicketBookingCanceled) error {
	return h.ticketRepo.Delete(ctx, e.TicketID)
}

func (h Handler) PrintTicket(ctx context.Context, e *TicketBookingConfirmed) error {
	fileID, err := h.ticketGenerator.GenerateTicket(ctx, e.TicketID, e.Price)
	if err != nil {
		return fmt.Errorf("generating ticket: %w", err)
	}

	ticketPrinted := NewTicketPrinted(e.Header.IdempotencyKey, e.TicketID, fileID)

	if err := h.publisher.Publish(ctx, ticketPrinted); err != nil {
		return fmt.Errorf("publishing ticket printed event: %w", err)
	}

	return nil
}
