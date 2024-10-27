package tests_test

import (
	"context"
	"os"
	"testing"
	"tickets/service"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestComponent(t *testing.T) {
	logger := watermill.NewStdLogger(false, false)
	rdb := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_ADDR"),
	})
	t.Cleanup(func() {
		assert.NoError(t, rdb.Conn().Close())
	})

	receiptIssuer := &MockReceiptIssuer{}
	spreadsheetAppender := &MockSpreadsheetAppender{}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go func() {
		svc, err := service.New(logger, rdb, receiptIssuer, spreadsheetAppender)
		assert.NoError(t, err)

		assert.NoError(t, svc.Run(ctx))
	}()

	waitForHttpServer(t)
}
