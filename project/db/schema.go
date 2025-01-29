package db

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func InitialiseDB(ctx context.Context, db *sqlx.DB) error {
	if err := CreateBookingsTable(ctx, db); err != nil {
		return fmt.Errorf("creating bookings table: %w", err)
	}

	if err := CreateShowsTable(ctx, db); err != nil {
		return fmt.Errorf("creating shows table: %w", err)
	}

	if err := CreateTicketsTable(ctx, db); err != nil {
		return fmt.Errorf("creating tickets table: %w", err)
	}

	return nil
}
