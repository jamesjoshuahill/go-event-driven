package db

import (
	"context"
	"database/sql"
	"tickets/entity"

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

type BookingRepo struct{}

func NewBookingRepo() BookingRepo {
	return BookingRepo{}
}

func (r BookingRepo) AddInTx(ctx context.Context, tx *sql.Tx, booking entity.Booking) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO bookings
		(booking_id, show_id, number_of_tickets, customer_email)
		VALUES ($1, $2, $3, $4);`,
		booking.BookingID, booking.ShowID, booking.NumberOfTickets, booking.CustomerEmail)
	return err
}
