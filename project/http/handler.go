package http

import (
	"context"
	"fmt"
	"net/http"
	"tickets/entity"
	"tickets/event"

	"github.com/labstack/echo/v4"
)

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

type Publisher interface {
	Publish(ctx context.Context, event any) error
}

type handler struct {
	publisher Publisher
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
		ticket := entity.Ticket{
			ID:            ticketStatus.ID,
			CustomerEmail: ticketStatus.CustomerEmail,
			Price: entity.Money{
				Amount:   ticketStatus.Price.Amount,
				Currency: ticketStatus.Price.Currency,
			},
		}

		var e any
		switch ticketStatus.Status {
		case entity.StatusConfirmed:
			e = event.NewTicketBookingConfirmed(ticket)
		case entity.StatusCanceled:
			e = event.NewTicketBookingCanceled(ticket)
		}

		if err := h.publisher.Publish(c.Request().Context(), e); err != nil {
			return &echo.HTTPError{
				Message:  http.StatusText(http.StatusInternalServerError),
				Internal: fmt.Errorf("publishing event: %w", err),
			}
		}
	}

	return c.NoContent(http.StatusOK)
}
