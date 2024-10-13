package main

import (
	"fmt"
	"os"
	"tickets/clients"
	"tickets/http"
	"tickets/message"
	"tickets/service"

	"github.com/ThreeDotsLabs/go-event-driven/common/log"
	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
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

	receiptsSub, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{
		Client:        rdb,
		ConsumerGroup: "issue-receipt",
	}, logger)
	if err != nil {
		return fmt.Errorf("creating receipts subscriber: %w", err)
	}
	defer func() {
		if err := receiptsSub.Close(); err != nil {
			logger.Error("failed to close receipts subscriber", err, nil)
		}
	}()

	trackerConfirmedSub, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{
		Client:        rdb,
		ConsumerGroup: "append-to-tracker-confirmed",
	}, logger)
	if err != nil {
		return fmt.Errorf("creating tracker confirmed subscriber: %w", err)
	}
	defer func() {
		if err := trackerConfirmedSub.Close(); err != nil {
			logger.Error("failed to close tracker confirmed subscriber", err, nil)
		}
	}()

	trackerCanceledSub, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{
		Client:        rdb,
		ConsumerGroup: "append-to-tracker-canceled",
	}, logger)
	if err != nil {
		return fmt.Errorf("creating tracker canceled subscriber: %w", err)
	}
	defer func() {
		if err := trackerCanceledSub.Close(); err != nil {
			logger.Error("failed to close tracker canceled subscriber", err, nil)
		}
	}()

	msgRouter, err := message.NewRouter(logger, receiptsSub, trackerConfirmedSub, trackerCanceledSub, receiptsClient, spreadsheetsClient)
	if err != nil {
		return fmt.Errorf("creating message router: %w", err)
	}

	publisher, err := redisstream.NewPublisher(redisstream.PublisherConfig{
		Client: rdb,
	}, logger)
	if err != nil {
		return fmt.Errorf("creating publisher: %w", err)
	}

	httpRouter := http.NewRouter(publisher)

	return service.Run(msgRouter, httpRouter)
}
