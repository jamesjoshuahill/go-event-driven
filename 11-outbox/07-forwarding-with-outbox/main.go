package main

import (
	"context"
	"fmt"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	watermillSQL "github.com/ThreeDotsLabs/watermill-sql/v2/pkg/sql"
	"github.com/ThreeDotsLabs/watermill/components/forwarder"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

func RunForwarder(
	db *sqlx.DB,
	rdb *redis.Client,
	outboxTopic string,
	logger watermill.LoggerAdapter,
) error {
	subscriber, err := watermillSQL.NewSubscriber(db, watermillSQL.SubscriberConfig{
		SchemaAdapter:  watermillSQL.DefaultPostgreSQLSchema{},
		OffsetsAdapter: watermillSQL.DefaultPostgreSQLOffsetsAdapter{},
	}, logger)
	if err != nil {
		return fmt.Errorf("creating subscriber: %w", err)
	}

	if err := subscriber.SubscribeInitialize(outboxTopic); err != nil {
		return fmt.Errorf("initialising subscriber: %w", err)
	}

	publisher, err := redisstream.NewPublisher(redisstream.PublisherConfig{
		Client: rdb,
	}, logger)
	if err != nil {
		return fmt.Errorf("creating publisher: %w", err)
	}

	fwd, err := forwarder.NewForwarder(subscriber, publisher, logger, forwarder.Config{
		ForwarderTopic: outboxTopic,
	})
	if err != nil {
		return fmt.Errorf("creating forwarder: %w", err)
	}

	go func() {
		if err := fwd.Run(context.Background()); err != nil {
			panic(err)
		}
	}()

	<-fwd.Running()
	return nil
}
