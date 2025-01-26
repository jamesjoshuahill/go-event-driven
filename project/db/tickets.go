package db

import (
	"context"
	"fmt"
	"tickets/entity"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func CreateTicketsTable(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS tickets (
		ticket_id UUID PRIMARY KEY,
		price_amount NUMERIC(10, 2) NOT NULL,
		price_currency CHAR(3) NOT NULL,
		customer_email VARCHAR(255) NOT NULL
		);`)
	return err
}

type TicketRepo struct {
	db *sqlx.DB
}

func NewTicketRepo(db *sqlx.DB) TicketRepo {
	return TicketRepo{
		db: db,
	}
}

func (r TicketRepo) Add(ctx context.Context, ticket entity.Ticket) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO tickets
		(ticket_id, price_amount, price_currency, customer_email)
		VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING;`,
		ticket.ID, ticket.Price.Amount, ticket.Price.Currency, ticket.CustomerEmail)
	return err
}

func (r TicketRepo) Delete(ctx context.Context, ticketID string) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM tickets WHERE ticket_id = $1", ticketID)
	if err != nil {
		return fmt.Errorf("executing delete query: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("getting rows affected: %w", err)
	}
	if n != 1 {
		return fmt.Errorf("unexpected exec result: %d rows affected", n)
	}

	return nil
}

func (r TicketRepo) List(ctx context.Context) ([]entity.Ticket, error) {
	rows, err := r.db.QueryxContext(ctx, "SELECT ticket_id, price_amount, price_currency, customer_email FROM tickets")
	if err != nil {
		return nil, fmt.Errorf("querying db: %w", err)
	}

	var tickets []entity.Ticket
	for rows.Next() {
		var t entity.Ticket
		if err := rows.Scan(&t.ID, &t.Price.Amount, &t.Price.Currency, &t.CustomerEmail); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		tickets = append(tickets, t)
	}

	return tickets, nil
}
