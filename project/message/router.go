package message

import (
	"fmt"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/components/cqrs"
	"github.com/ThreeDotsLabs/watermill/message"
	"tickets/message/command"
	"tickets/message/event"
)

type Router struct {
	*message.Router
}

func NewRouter(
	commandHandler command.Handler,
	commandProcessorConfig cqrs.CommandProcessorConfig,
	eventHandler event.Handler,
	eventProcessorConfig cqrs.EventProcessorConfig,
	logger watermill.LoggerAdapter,
) (*Router, error) {
	router, err := message.NewRouter(message.RouterConfig{}, logger)
	if err != nil {
		return nil, fmt.Errorf("creating router: %w", err)
	}

	addMiddlewares(router, logger)

	eventProcessor, err := cqrs.NewEventProcessorWithConfig(router, eventProcessorConfig)
	if err != nil {
		return nil, fmt.Errorf("creating event processor: %w", err)
	}

	eventHandlers := []cqrs.EventHandler{
		cqrs.NewEventHandler("create-dead-nation-booking", eventHandler.CreateDeadNationBooking),
		cqrs.NewEventHandler("issue-receipt", eventHandler.IssueReceipt),
		cqrs.NewEventHandler("append-to-tracker-confirmed", eventHandler.AppendToTrackerConfirmed),
		cqrs.NewEventHandler("append-to-tracker-canceled", eventHandler.AppendToTrackerCanceled),
		cqrs.NewEventHandler("store-confirmed-in-db", eventHandler.StoreInDB),
		cqrs.NewEventHandler("remove-canceled-from-db", eventHandler.RemoveCanceledFromDB),
		cqrs.NewEventHandler("print-ticket", eventHandler.PrintTicket),
	}

	if err := eventProcessor.AddHandlers(eventHandlers...); err != nil {
		return nil, fmt.Errorf("adding event handlers: %w", err)
	}

	cmdProcessor, err := cqrs.NewCommandProcessorWithConfig(router, commandProcessorConfig)
	if err != nil {
		return nil, fmt.Errorf("creating command processor: %w", err)
	}

	cmdHandlers := []cqrs.CommandHandler{
		cqrs.NewCommandHandler("refund-ticket", commandHandler.RefundTicket),
	}

	if err := cmdProcessor.AddHandlers(cmdHandlers...); err != nil {
		return nil, fmt.Errorf("adding command handlers: %w", err)
	}

	return &Router{router}, nil
}
