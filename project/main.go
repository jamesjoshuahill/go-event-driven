package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"tickets/clients"
	"tickets/db"
	"tickets/service"

	"github.com/ThreeDotsLabs/go-event-driven/common/log"
	"github.com/ThreeDotsLabs/watermill"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

func main() {
	log.Init(logrus.InfoLevel)
	logger := watermill.NewStdLogger(false, false)

	if err := run(logger); err != nil {
		logger.Error("failed to run", err, nil)
	}
}

func run(logger watermill.LoggerAdapter) error {
	c, err := clients.New(os.Getenv("GATEWAY_ADDR"))
	if err != nil {
		return fmt.Errorf("creating gateway client: %w", err)
	}

	ticketGenerator := clients.NewFilesClient(c)
	receiptsClient := clients.NewReceiptsClient(c)
	spreadsheetsClient := clients.NewSpreadsheetsClient(c)

	rdb := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_ADDR"),
	})
	defer func() {
		if err := rdb.Conn().Close(); err != nil {
			logger.Error("failed to close redis connection", err, nil)
		}
	}()

	dbConn, err := sqlx.Open("postgres", os.Getenv("POSTGRES_URL"))
	if err != nil {
		return fmt.Errorf("connecting to db: %w", err)
	}
	defer func() {
		if err := dbConn.Close(); err != nil {
			logger.Error("failed to close db connection", err, nil)
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := db.CreateTicketsTable(ctx, dbConn); err != nil {
		return fmt.Errorf("creating tickets table: %w", err)
	}

	if err := db.CreateShowsTable(ctx, dbConn); err != nil {
		return fmt.Errorf("creating shows table: %w", err)
	}

	svc, err := service.New(logger, rdb, dbConn, ticketGenerator, receiptsClient, spreadsheetsClient)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	return svc.Run(ctx)
}
