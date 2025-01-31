package message

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ThreeDotsLabs/go-event-driven/common/log"
	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	watermillSQL "github.com/ThreeDotsLabs/watermill-sql/v2/pkg/sql"
	"github.com/ThreeDotsLabs/watermill/components/cqrs"
	"github.com/ThreeDotsLabs/watermill/components/forwarder"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

const outboxTopic = "events_to_forward"

type Forwarder struct {
	*forwarder.Forwarder
}

func NewForwarder(
	db *sqlx.DB,
	rdb *redis.Client,
	logger watermill.LoggerAdapter,
) (*Forwarder, error) {
	subscriber, err := watermillSQL.NewSubscriber(db, watermillSQL.SubscriberConfig{
		SchemaAdapter:  watermillSQL.DefaultPostgreSQLSchema{},
		OffsetsAdapter: watermillSQL.DefaultPostgreSQLOffsetsAdapter{},
	}, logger)
	if err != nil {
		return nil, fmt.Errorf("creating subscriber: %w", err)
	}

	if err := subscriber.SubscribeInitialize(outboxTopic); err != nil {
		return nil, fmt.Errorf("initialising subscriber: %w", err)
	}

	publisher, err := redisstream.NewPublisher(redisstream.PublisherConfig{
		Client: rdb,
	}, logger)
	if err != nil {
		return nil, fmt.Errorf("creating publisher: %w", err)
	}

	decoratedPublisher := log.CorrelationPublisherDecorator{Publisher: publisher}

	f, err := forwarder.NewForwarder(subscriber, decoratedPublisher, logger, forwarder.Config{
		ForwarderTopic: outboxTopic,
	})
	if err != nil {
		return nil, fmt.Errorf("creating forwarder: %w", err)
	}

	return &Forwarder{f}, nil
}

func PublishInTx(
	ctx context.Context,
	event any,
	tx *sql.Tx,
	logger watermill.LoggerAdapter,
) error {
	sqlPublisher, err := watermillSQL.NewPublisher(
		tx,
		watermillSQL.PublisherConfig{
			SchemaAdapter: watermillSQL.DefaultPostgreSQLSchema{},
		},
		logger,
	)
	if err != nil {
		return fmt.Errorf("creating sql publisher: %w", err)
	}

	publisher := forwarder.NewPublisher(sqlPublisher, forwarder.PublisherConfig{
		ForwarderTopic: outboxTopic,
	})

	decoratedPublisher := log.CorrelationPublisherDecorator{Publisher: publisher}

	eventBus, err := cqrs.NewEventBusWithConfig(decoratedPublisher, cqrs.EventBusConfig{
		GeneratePublishTopic: func(params cqrs.GenerateEventPublishTopicParams) (string, error) {
			return params.EventName, nil
		},
		Marshaler: cqrs.JSONMarshaler{
			GenerateName: cqrs.StructName,
		},
	})
	if err != nil {
		return fmt.Errorf("creating sql event bus: %w", err)
	}

	if err := eventBus.Publish(ctx, event); err != nil {
		return fmt.Errorf("publishing event: %w", err)
	}

	return nil
}
