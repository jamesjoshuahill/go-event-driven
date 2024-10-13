package http

import (
	"fmt"
	"net/http"
	"tickets/entity"
	"tickets/event"
	"tickets/message"

	"github.com/labstack/echo/v4"
	"github.com/lithammer/shortuuid/v3"
)

const headerKeyCorrelationID = "Correlation-ID"

type ticketsStatusRequest struct {
	Tickets []ticketStatus `json:"tickets"`
}

type ticketStatus struct {
	ID            string `json:"ticket_id"`
	Status        string `json:"status"`
	CustomerEmail string `json:"customer_email"`
	Price         money  `json:"price"`
}

type money struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

func (h handler) PostTicketStatus(c echo.Context) error {
	var request ticketsStatusRequest
	if err := c.Bind(&request); err != nil {
		return &echo.HTTPError{
			Code:     http.StatusBadRequest,
			Message:  "failed to parse request",
			Internal: fmt.Errorf("failed to bind request: %w", err),
		}
	}

	for _, ticketStatus := range request.Tickets {
		correlationID := c.Request().Header.Get(headerKeyCorrelationID)
		if correlationID == "" {
			correlationID = "gen_" + shortuuid.New()
		}

		ticket := entity.Ticket{
			ID:            ticketStatus.ID,
			Status:        ticketStatus.Status,
			CustomerEmail: ticketStatus.CustomerEmail,
			Price: entity.Money{
				Amount:   ticketStatus.Price.Amount,
				Currency: ticketStatus.Price.Currency,
			},
		}

		switch ticket.Status {
		case entity.StatusConfirmed:
			msg, err := message.NewTicketBookingConfirmed(ticket, correlationID)
			if err != nil {
				return &echo.HTTPError{
					Message:  http.StatusText(http.StatusInternalServerError),
					Internal: fmt.Errorf("creating event: %w", err),
				}
			}

			topic := event.TopicTicketBookingConfirmed
			if err := h.publisher.Publish(topic, msg); err != nil {
				return &echo.HTTPError{
					Message:  http.StatusText(http.StatusInternalServerError),
					Internal: fmt.Errorf("publishing message to topic '%s': %w", topic, err),
				}
			}
		case entity.StatusCanceled:
			msg, err := message.NewTicketBookingCanceled(ticket, correlationID)
			if err != nil {
				return &echo.HTTPError{
					Message:  http.StatusText(http.StatusInternalServerError),
					Internal: fmt.Errorf("creating event: %w", err),
				}
			}

			topic := event.TopicTicketBookingCanceled
			if err := h.publisher.Publish(topic, msg); err != nil {
				return &echo.HTTPError{
					Message:  http.StatusText(http.StatusInternalServerError),
					Internal: fmt.Errorf("publishing message to topic '%s': %w", topic, err),
				}
			}
		}
	}

	return c.NoContent(http.StatusOK)
}
