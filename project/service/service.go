package service

import (
	"context"
	"errors"
	"fmt"
	"tickets/http"
	"tickets/message"
	"tickets/postgres"
	"time"

	"github.com/ThreeDotsLabs/go-event-driven/common/log"
	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/ThreeDotsLabs/watermill/components/cqrs"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type ServiceDeps struct {
	DB                  *sqlx.DB
	Logger              watermill.LoggerAdapter
	RedisClient         *redis.Client
	DeadNationBooker    message.DeadNationBooker
	TicketGenerator     message.TicketGenerator
	ReceiptIssuer       message.ReceiptIssuer
	SpreadsheetAppender message.SpreadsheetAppender
}

type Service struct {
	db           *sqlx.DB
	msgForwarder *message.Forwarder
	msgRouter    *message.Router
	httpRouter   *echo.Echo
}

func New(deps ServiceDeps) (*Service, error) {
	publisher, err := redisstream.NewPublisher(redisstream.PublisherConfig{
		Client: deps.RedisClient,
	}, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("creating publisher: %w", err)
	}
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
		return nil, fmt.Errorf("creating event bus: %w", err)
	}

	bookingRepo := postgres.NewBookingRepo(deps.DB, deps.Logger)
	showRepo := postgres.NewShowRepo(deps.DB)
	ticketRepo := postgres.NewTicketRepo(deps.DB)

	msgRouter, err := message.NewRouter(message.RouterDeps{
		DeadNationBooker:    deps.DeadNationBooker,
		Logger:              deps.Logger,
		Publisher:           eventBus,
		ReceiptIssuer:       deps.ReceiptIssuer,
		RedisClient:         deps.RedisClient,
		ShowRepo:            showRepo,
		SpreadsheetAppender: deps.SpreadsheetAppender,
		TicketGenerator:     deps.TicketGenerator,
		TicketRepo:          ticketRepo,
	})
	if err != nil {
		return nil, fmt.Errorf("creating message router: %w", err)
	}

	msgForwarder, err := message.NewForwarder(deps.DB, deps.RedisClient, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("creating message forwarder: %w", err)
	}

	httpRouter := http.NewRouter(deps.DB, bookingRepo, deps.Logger, eventBus, showRepo, ticketRepo)

	return &Service{
		db:           deps.DB,
		msgForwarder: msgForwarder,
		msgRouter:    msgRouter,
		httpRouter:   httpRouter,
	}, nil
}

func (s Service) Run(ctx context.Context) error {
	if err := postgres.InitialiseDB(ctx, s.db); err != nil {
		return fmt.Errorf("initialising db: %w", err)
	}

	g, runCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := s.msgRouter.Run(runCtx); err != nil {
			return fmt.Errorf("running message router: %w", err)
		}

		return nil
	})

	g.Go(func() error {
		if err := s.msgForwarder.Run(runCtx); err != nil {
			return fmt.Errorf("running message forwarder: %w", err)
		}

		return nil
	})

	g.Go(func() error {
		// Wait for message components
		<-s.msgRouter.Running()
		<-s.msgForwarder.Running()

		logrus.Info("starting http server...")
		err := s.httpRouter.Start(":8080")
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("starting http server: %w", err)
		}

		return nil
	})

	g.Go(func() error {
		<-runCtx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		logrus.Info("shutting down http server...")
		if err := s.httpRouter.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutting down http server: %w", err)
		}

		return nil
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("waiting for shutdown: %w", err)
	}
	logrus.Info("shutdown complete")

	return nil
}
