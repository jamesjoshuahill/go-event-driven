package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"tickets/entity"
	"tickets/event"
	"tickets/message"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type notEnoughTicketsError struct {
	ticketsAvailable uint
	ticketsRequested uint
}

func (e notEnoughTicketsError) Error() string {
	return fmt.Sprintf("not enough tickets: tickets available %d, tickets requested %d", e.ticketsAvailable, e.ticketsRequested)
}

func (e notEnoughTicketsError) NotEnoughTickets() bool {
	return true
}

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
	db *sqlx.DB
}

func NewBookingRepo(db *sqlx.DB) BookingRepo {
	return BookingRepo{
		db: db,
	}
}

func (r BookingRepo) Add(ctx context.Context, totalTickets uint, booking entity.Booking) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	if err := add(ctx, tx, totalTickets, booking); err != nil {
		return errors.Join(err, tx.Rollback())
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

func add(ctx context.Context, tx *sql.Tx, totalTickets uint, booking entity.Booking) error {
	row := tx.QueryRowContext(ctx, `SELECT coalesce(SUM(number_of_tickets), 0)
		FROM bookings WHERE show_id = $1`, booking.ShowID)
	var ticketsBooked uint
	if err := row.Scan(&ticketsBooked); err != nil {
		return fmt.Errorf("counting tickets booked: %w", err)
	}

	ticketsAvailable := totalTickets - ticketsBooked
	if booking.NumberOfTickets > ticketsAvailable {
		return notEnoughTicketsError{
			ticketsAvailable: ticketsAvailable,
			ticketsRequested: booking.NumberOfTickets,
		}
	}

	_, err := tx.ExecContext(ctx, `INSERT INTO bookings
		(booking_id, show_id, number_of_tickets, customer_email)
		VALUES ($1, $2, $3, $4);`,
		booking.BookingID, booking.ShowID, booking.NumberOfTickets, booking.CustomerEmail)
	if err != nil {
		return fmt.Errorf("inserting booking: %w", err)
	}

	e := event.NewBookingMade(uuid.NewString(), booking)

	if err := message.PublishInTx(ctx, e, tx); err != nil {
		return fmt.Errorf("publishing event in transaction: %w", err)
	}

	return nil
}
