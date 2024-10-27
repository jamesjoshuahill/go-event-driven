package service

import (
	"context"
	"errors"
	"fmt"
	"tickets/http"
	"tickets/message"
	"time"

	"github.com/ThreeDotsLabs/go-event-driven/common/log"
	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type Service struct {
	msgRouter  *message.Router
	httpRouter *echo.Echo
}

func New(
	logger watermill.LoggerAdapter,
	redisClient *redis.Client,
	receiptIssuer message.ReceiptIssuer,
	spreadsheetAppender message.SpreadsheetAppender,
) (*Service, error) {
	publisher, err := redisstream.NewPublisher(redisstream.PublisherConfig{
		Client: redisClient,
	}, logger)
	if err != nil {
		return nil, fmt.Errorf("creating publisher: %w", err)
	}
	decoratedPublisher := log.CorrelationPublisherDecorator{Publisher: publisher}

	msgRouter, err := message.NewRouter(logger, redisClient, receiptIssuer, spreadsheetAppender)
	if err != nil {
		return nil, fmt.Errorf("creating message router: %w", err)
	}

	httpRouter := http.NewRouter(decoratedPublisher)

	return &Service{
		msgRouter:  msgRouter,
		httpRouter: httpRouter,
	}, nil
}

func (s Service) Run(ctx context.Context) error {
	g, runCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := s.msgRouter.Run(runCtx); err != nil {
			return fmt.Errorf("running messaging router: %w", err)
		}

		return nil
	})

	g.Go(func() error {
		// Wait for message router
		<-s.msgRouter.Running()

		logrus.Info("Starting HTTP server...")
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

		logrus.Info("Shutting down HTTP server...")
		if err := s.httpRouter.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutting down http server: %w", err)
		}

		return nil
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("waiting for shutdown: %w", err)
	}
	logrus.Info("Shutdown complete.")

	return nil
}
