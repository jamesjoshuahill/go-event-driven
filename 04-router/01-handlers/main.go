package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/redis/go-redis/v9"
)

func main() {
	logger := watermill.NewStdLogger(false, false)

	router, err := message.NewRouter(message.RouterConfig{}, logger)
	if err != nil {
		panic(err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_ADDR"),
	})

	sub, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{
		Client: rdb,
	}, logger)
	if err != nil {
		panic(err)
	}

	pub, err := redisstream.NewPublisher(redisstream.PublisherConfig{
		Client: rdb,
	}, logger)
	if err != nil {
		panic(err)
	}

	router.AddHandler(
		"celsius-to-fahrenheit",
		"temperature-celsius",
		sub,
		"temperature-fahrenheit",
		pub,
		func(msg *message.Message) ([]*message.Message, error) {
			f, err := celsiusToFahrenheit(string(msg.Payload))
			if err != nil {
				return nil, fmt.Errorf("converting celsius to fahrenheit: %w", err)
			}

			fMsg := message.NewMessage(watermill.NewUUID(), message.Payload(f))

			return []*message.Message{fMsg}, nil
		},
	)

	err = router.Run(context.Background())
	if err != nil {
		panic(err)
	}
}

func celsiusToFahrenheit(temperature string) (string, error) {
	celsius, err := strconv.Atoi(temperature)
	if err != nil {
		return "", err
	}

	return strconv.Itoa(celsius*9/5 + 32), nil
}
