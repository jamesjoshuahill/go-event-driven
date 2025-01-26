package db_test

import (
	"context"
	"log"
	"os"
	"testing"
	"tickets/db"
	"tickets/entity"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

var dbConn *sqlx.DB

func TestMain(m *testing.M) {
	dsn := getEnvOrDefault("POSTGRES_URL", "postgres://user:password@localhost:5432/db?sslmode=disable")

	var err error
	dbConn, err = sqlx.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to connect to db: %s", err)
	}

	if err := db.CreateTicketsTable(context.Background(), dbConn); err != nil {
		log.Fatalf("failed to create tickets table: %s", err)
	}

	code := m.Run()

	if err := dbConn.Close(); err != nil {
		log.Fatalf("failed to close db connection: %s", err)
	}

	os.Exit(code)
}

func TestTicketRepo_Add(t *testing.T) {
	ctx := context.Background()
	ticket := entity.Ticket{
		ID: uuid.NewString(),
		Price: entity.Money{
			Amount:   "100",
			Currency: "GBP",
		},
		CustomerEmail: "test@example.com",
	}
	r := db.NewTicketRepo(dbConn)
	require.NoError(t, r.Add(ctx, ticket))
	require.NoError(t, r.Add(ctx, ticket))

	tickets, err := r.List(ctx)
	require.NoError(t, err)
	var matchingTickets []entity.Ticket
	for _, t := range tickets {
		if t.ID == ticket.ID {
			matchingTickets = append(matchingTickets, t)
		}
	}
	require.Len(t, matchingTickets, 1)
}

func getEnvOrDefault(key string, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
