package main

import (
	"github.com/ThreeDotsLabs/watermill/message"
	"os"
	"strconv"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/redis/go-redis/v9"
)

func main() {
	logger := watermill.NewStdLogger(false, false)

	rdb := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_ADDR"),
	})

	publisher, err := redisstream.NewPublisher(redisstream.PublisherConfig{
		Client: rdb,
	}, logger)
	if err != nil {
		panic(err)
	}

	err = publisher.Publish("progress", newProgressMsg(50), newProgressMsg(100))
	if err != nil {
		panic(err)
	}
}

func newProgressMsg(percentage int) *message.Message {
	payload := strconv.Itoa(percentage)
	return message.NewMessage(watermill.NewUUID(), []byte(payload))
}
