package db

import (
	"context"
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

type BookingRepo struct {
	db *sqlx.DB
}

func NewBookingRepo(db *sqlx.DB) BookingRepo {
	return BookingRepo{
		db: db,
	}
}

func (r BookingRepo) Add(ctx context.Context, booking entity.Booking) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO bookings
		(booking_id, show_id, number_of_tickets, customer_email)
		VALUES ($1, $2, $3, $4);`,
		booking.BookingID, booking.ShowID, booking.NumberOfTickets, booking.CustomerEmail)
	return err
}
