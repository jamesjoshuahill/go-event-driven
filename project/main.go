package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"tickets/clients"
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
	gatewayClient, err := clients.New(os.Getenv("GATEWAY_ADDR"))
	if err != nil {
		return fmt.Errorf("creating gateway client: %w", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_ADDR"),
	})
	defer func() {
		if err := redisClient.Conn().Close(); err != nil {
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

	deadNationClient := clients.NewDeadNationClient(gatewayClient)
	filesClient := clients.NewFilesClient(gatewayClient)
	paymentsClient := clients.NewPaymentsClient(gatewayClient)
	receiptsClient := clients.NewReceiptsClient(gatewayClient)
	spreadsheetsClient := clients.NewSpreadsheetsClient(gatewayClient)

	svc, err := service.New(service.Deps{
		DB:                 dbConn,
		DeadNationBooker:   deadNationClient,
		Logger:             logger,
		PaymentsClient:     paymentsClient,
		ReceiptsClient:     receiptsClient,
		RedisClient:        redisClient,
		SpreadsheetsClient: spreadsheetsClient,
		FilesClient:        filesClient,
	})
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	return svc.Run(ctx)
}
