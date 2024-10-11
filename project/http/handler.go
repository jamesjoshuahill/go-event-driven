package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"tickets/entity"
	"tickets/event"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/labstack/echo/v4"
	"github.com/lithammer/shortuuid/v3"
)

const (
	TopicTicketBookingConfirmed = "TicketBookingConfirmed"
	TopicTicketBookingCanceled  = "TicketBookingCanceled"
)

type TicketsStatusRequest struct {
	Tickets []entity.Ticket `json:"tickets"`
}

type handler struct {
	publisher message.Publisher
}

func (h handler) PostTicketStatus(c echo.Context) error {
	var request TicketsStatusRequest
	if err := c.Bind(&request); err != nil {
		return &echo.HTTPError{
			Code:     http.StatusBadRequest,
			Message:  "failed to parse request",
			Internal: fmt.Errorf("failed to bind request: %w", err),
		}
	}

	for _, ticket := range request.Tickets {
		correlationID := c.Request().Header.Get("Correlation-ID")
		if correlationID == "" {
			correlationID = "gen_" + shortuuid.New()
		}

		publishFunc := publishTicketBookingConfirmed
		if ticket.Status == entity.StatusCanceled {
			publishFunc = publishTicketBookingCanceled
		}

		if err := publishFunc(correlationID, h.publisher, ticket); err != nil {
			return err
		}
	}

	return c.NoContent(http.StatusOK)
}

func publishTicketBookingConfirmed(correlationID string, publisher message.Publisher, ticket entity.Ticket) error {
	body := event.TicketBookingConfirmed{
		Header:        event.NewHeader(),
		TicketID:      ticket.ID,
		CustomerEmail: ticket.CustomerEmail,
		Price: entity.Money{
			Amount:   ticket.Price.Amount,
			Currency: ticket.Price.Currency,
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return &echo.HTTPError{
			Message:  http.StatusText(http.StatusInternalServerError),
			Internal: fmt.Errorf("failed to marshal body: %w", err),
		}
	}

	msg := message.NewMessage(body.Header.ID, payload)
	middleware.SetCorrelationID(correlationID, msg)
	msg.Metadata.Set("type", "TicketBookingConfirmed")

	if err := publisher.Publish(TopicTicketBookingConfirmed, msg); err != nil {
		return &echo.HTTPError{
			Message:  http.StatusText(http.StatusInternalServerError),
			Internal: fmt.Errorf("publishing message to topic '%s': %w", TopicTicketBookingConfirmed, err),
		}
	}

	return nil
}

func publishTicketBookingCanceled(correlationID string, publisher message.Publisher, ticket entity.Ticket) error {
	body := event.TicketBookingCanceled{
		Header:        event.NewHeader(),
		TicketID:      ticket.ID,
		CustomerEmail: ticket.CustomerEmail,
		Price: entity.Money{
			Amount:   ticket.Price.Amount,
			Currency: ticket.Price.Currency,
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return &echo.HTTPError{
			Message:  http.StatusText(http.StatusInternalServerError),
			Internal: fmt.Errorf("failed to marshal body: %w", err),
		}
	}

	msg := message.NewMessage(body.Header.ID, payload)
	middleware.SetCorrelationID(correlationID, msg)
	msg.Metadata.Set("type", "TicketBookingCanceled")

	if err := publisher.Publish(TopicTicketBookingCanceled, msg); err != nil {
		return &echo.HTTPError{
			Message:  http.StatusText(http.StatusInternalServerError),
			Internal: fmt.Errorf("publishing message to topic '%s': %w", TopicTicketBookingCanceled, err),
		}
	}

	return nil
}
