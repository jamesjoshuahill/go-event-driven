package db

import (
	"context"
	"database/sql"
	"fmt"
	"tickets/entity"
	"tickets/event"
	"tickets/message"

	"github.com/ThreeDotsLabs/go-event-driven/common/log"
	"github.com/ThreeDotsLabs/watermill"
	watermillSQL "github.com/ThreeDotsLabs/watermill-sql/v2/pkg/sql"
	"github.com/ThreeDotsLabs/watermill/components/cqrs"
	"github.com/ThreeDotsLabs/watermill/components/forwarder"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func CreateBookingsTable(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS bookings (
		booking_id UUID PRIMARY KEY,
		show_id UUID NOT NULL,
		number_of_tickets INTEGER NOT NULL,
		customer_email VARCHAR(255) NOT NULL
	);`)
	return err
}

type BookingRepo struct {
	db     *sqlx.DB
	logger watermill.LoggerAdapter
}

func NewBookingRepo(db *sqlx.DB, logger watermill.LoggerAdapter) BookingRepo {
	return BookingRepo{
		db:     db,
		logger: logger,
	}
}

func (r BookingRepo) Add(ctx context.Context, booking entity.Booking) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO bookings
		(booking_id, show_id, number_of_tickets, customer_email)
		VALUES ($1, $2, $3, $4);`,
		booking.BookingID, booking.ShowID, booking.NumberOfTickets, booking.CustomerEmail)
	if err != nil {
		return fmt.Errorf("inserting booking: %w", err)
	}

	e := event.NewBookingMade(uuid.NewString(), booking)

	if err := PublishInTx(ctx, e, tx, r.logger); err != nil {
		return fmt.Errorf("publishing event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

func PublishInTx(
	ctx context.Context,
	event any,
	tx *sql.Tx,
	logger watermill.LoggerAdapter,
) error {
	sqlPublisher, err := watermillSQL.NewPublisher(
		tx,
		watermillSQL.PublisherConfig{
			SchemaAdapter: watermillSQL.DefaultPostgreSQLSchema{},
		},
		logger,
	)
	if err != nil {
		return fmt.Errorf("creating sql publisher: %w", err)
	}

	publisher := forwarder.NewPublisher(sqlPublisher, forwarder.PublisherConfig{
		ForwarderTopic: message.OutboxTopic,
	})

	decoratedPublisher := log.CorrelationPublisherDecorator{Publisher: publisher}

	eventBus, err := cqrs.NewEventBusWithConfig(decoratedPublisher, cqrs.EventBusConfig{
		GeneratePublishTopic: func(params cqrs.GenerateEventPublishTopicParams) (string, error) {
			return params.EventName, nil
		},
		Marshaler: cqrs.JSONMarshaler{
			GenerateName: cqrs.StructName,
		},
	})
	if err != nil {
		return fmt.Errorf("creating sql event bus: %w", err)
	}

	if err := eventBus.Publish(ctx, event); err != nil {
		return fmt.Errorf("publishing event: %w", err)
	}

	return nil
}
