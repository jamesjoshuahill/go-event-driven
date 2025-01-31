package postgres

import (
	"context"
	"fmt"
	"tickets/entity"
	"tickets/event"
	"tickets/message"

	"github.com/ThreeDotsLabs/watermill"
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

	if err := message.PublishInTx(ctx, e, tx, r.logger); err != nil {
		return fmt.Errorf("publishing event in transaction: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}
