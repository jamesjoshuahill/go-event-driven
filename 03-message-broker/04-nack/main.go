package main

import (
	"context"
	"fmt"

	"github.com/ThreeDotsLabs/watermill/message"
)

type AlarmClient interface {
	StartAlarm() error
	StopAlarm() error
}

func ConsumeMessages(sub message.Subscriber, alarmClient AlarmClient) {
	messages, err := sub.Subscribe(context.Background(), "smoke_sensor")
	if err != nil {
		panic(err)
	}

	for msg := range messages {
		switch string(msg.Payload) {
		case "0":
			if err := alarmClient.StopAlarm(); err != nil {
				fmt.Printf("failed to stop alarm: %s\n", err)
				msg.Nack()
				continue
			}
		case "1":
			if err := alarmClient.StartAlarm(); err != nil {
				fmt.Printf("failed to start alarm: %s\n", err)
				msg.Nack()
				continue
			}
		default:
			fmt.Printf("unknown payload: %s\n", string(msg.Payload))

		}

		msg.Ack()
	}
}
