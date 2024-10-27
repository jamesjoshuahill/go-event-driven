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
		return fmt.Errorf("creating client: %w", err)
	}

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

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	svc, err := service.New(logger, rdb, receiptsClient, spreadsheetsClient)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	return svc.Run(ctx)
}
