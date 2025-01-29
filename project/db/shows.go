package db

import (
	"context"
	"tickets/entity"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func CreateShowsTable(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS shows (
		show_id UUID PRIMARY KEY,
		dead_nation_id UUID NOT NULL,
		number_of_tickets INTEGER NOT NULL,
		start_time TIMESTAMP WITH TIME ZONE NOT NULL,
		title VARCHAR(255) NOT NULL,
		venue VARCHAR(255) NOT NULL
	);`)
	return err
}

type ShowRepo struct {
	db *sqlx.DB
}

func NewShowRepo(db *sqlx.DB) ShowRepo {
	return ShowRepo{
		db: db,
	}
}

func (r ShowRepo) Add(ctx context.Context, show entity.Show) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO shows
		(show_id, dead_nation_id, number_of_tickets, start_time, title, venue)
		VALUES ($1, $2, $3, $4, $5, $6);`,
		show.ShowID, show.DeadNationID, show.NumberOfTickets, show.StartTime, show.Title, show.Venue)
	return err
}
