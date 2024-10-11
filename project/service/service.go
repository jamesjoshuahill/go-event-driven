package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

func Run(msgRouter *message.Router, httpRouter *echo.Echo) error {
	sigCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	g, runCtx := errgroup.WithContext(sigCtx)

	g.Go(func() error {
		if err := msgRouter.Run(runCtx); err != nil {
			return fmt.Errorf("running messaging router: %w", err)
		}

		return nil
	})

	g.Go(func() error {
		// Wait for router
		<-msgRouter.Running()

		logrus.Info("Starting HTTP server...")
		err := httpRouter.Start(":8080")
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
		if err := httpRouter.Shutdown(shutdownCtx); err != nil {
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
