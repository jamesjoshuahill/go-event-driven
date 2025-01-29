package message

import (
	"fmt"

	"github.com/ThreeDotsLabs/go-event-driven/common/log"
	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	watermillSQL "github.com/ThreeDotsLabs/watermill-sql/v2/pkg/sql"
	"github.com/ThreeDotsLabs/watermill/components/forwarder"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

const OutboxTopic = "events_to_forward"

type Forwarder struct {
	*forwarder.Forwarder
}

func NewForwarder(
	db *sqlx.DB,
	rdb *redis.Client,
	outboxTopic string,
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
