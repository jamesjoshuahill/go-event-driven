package main

import (
	"context"
	"fmt"

	"github.com/ThreeDotsLabs/watermill"
	watermillSQL "github.com/ThreeDotsLabs/watermill-sql/v2/pkg/sql"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func SubscribeForMessages(
	db *sqlx.DB,
	topic string,
	logger watermill.LoggerAdapter,
) (<-chan *message.Message, error) {
	subscriber, err := watermillSQL.NewSubscriber(db, watermillSQL.SubscriberConfig{
		InitializeSchema: true,
		SchemaAdapter:    watermillSQL.DefaultPostgreSQLSchema{},
		OffsetsAdapter:   watermillSQL.DefaultPostgreSQLOffsetsAdapter{},
	}, logger)
	if err != nil {
		return nil, fmt.Errorf("creating subscriber: %w", err)
	}

	return subscriber.Subscribe(context.Background(), "ItemAddedToCart")
}
