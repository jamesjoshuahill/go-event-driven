package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"tickets/entity"
	"tickets/message/command"
	"tickets/message/event"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

const headerKeyIdempotencyKey = "Idempotency-Key"

type createTicketsStatusRequest struct {
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

type createShowRequest struct {
	DeadNationID    string    `json:"dead_nation_id"`
	NumberOfTickets uint      `json:"number_of_tickets"`
	StartTime       time.Time `json:"start_time"`
	Title           string    `json:"title"`
	Venue           string    `json:"venue"`
}

type createShowResponse struct {
	ShowID string `json:"show_id"`
}

type createBookingRequest struct {
	ShowID          string `json:"show_id"`
	NumberOfTickets uint   `json:"number_of_tickets"`
	CustomerEmail   string `json:"customer_email"`
}

type createBookingResponse struct {
	BookingID string `json:"booking_id"`
}

type BookingRepo interface {
	Add(ctx context.Context, ticketsAvailable uint, booking entity.Booking) error
}

type CommandSender interface {
	Send(ctx context.Context, cmd any) error
}

type EventPublisher interface {
	Publish(ctx context.Context, event any) error
}

type ShowRepo interface {
	Add(ctx context.Context, show entity.Show) error
	Get(ctx context.Context, showID string) (entity.Show, error)
}

type TicketRepo interface {
	List(ctx context.Context) ([]entity.Ticket, error)
}

type notEnoughTicketsError interface {
	error
	NotEnoughTickets() bool
}

type handler struct {
	bookingRepo       BookingRepo
	commandSender     CommandSender
	db                *sqlx.DB
	eventPublisher    EventPublisher
	newEventPublisher EventPublisher
	logger            watermill.LoggerAdapter
	showRepo          ShowRepo
	ticketRepo        TicketRepo
}

func (h handler) CreateTicketStatus(c echo.Context) error {
	var body createTicketsStatusRequest
	if err := c.Bind(&body); err != nil {
		return &echo.HTTPError{
			Code:     http.StatusBadRequest,
			Message:  "failed to parse request",
			Internal: fmt.Errorf("failed to bind request: %w", err),
		}
	}

	idempotencyKey, err := getIdempotencyKey(c)
	if err != nil {
		return err
	}

	for _, ticketStatus := range body.Tickets {
		ticket := entity.Ticket{
			ID:            ticketStatus.ID,
			CustomerEmail: ticketStatus.CustomerEmail,
			Price: entity.Money{
				Amount:   ticketStatus.Price.Amount,
				Currency: ticketStatus.Price.Currency,
			},
		}

		ticketIdempotencyKey := idempotencyKey + ticketStatus.ID

		var e any
		switch ticketStatus.Status {
		case entity.StatusConfirmed:
			e = event.NewTicketBookingConfirmed(ticketIdempotencyKey, ticket)
		case entity.StatusCanceled:
			e = event.NewTicketBookingCanceled(ticketIdempotencyKey, ticket)
		}

		if err := h.eventPublisher.Publish(c.Request().Context(), e); err != nil {
			return &echo.HTTPError{
				Code:     http.StatusInternalServerError,
				Message:  http.StatusText(http.StatusInternalServerError),
				Internal: fmt.Errorf("publishing event to old topic: %w", err),
			}
		}

		if err := h.newEventPublisher.Publish(c.Request().Context(), e); err != nil {
			return &echo.HTTPError{
				Code:     http.StatusInternalServerError,
				Message:  http.StatusText(http.StatusInternalServerError),
				Internal: fmt.Errorf("publishing event to new topic: %w", err),
			}
		}
	}

	return c.NoContent(http.StatusOK)
}

func (h handler) ListTickets(c echo.Context) error {
	tickets, err := h.ticketRepo.List(c.Request().Context())
	if err != nil {
		return &echo.HTTPError{
			Code:     http.StatusInternalServerError,
			Message:  http.StatusText(http.StatusInternalServerError),
			Internal: fmt.Errorf("listing tickets: %w", err),
		}
	}

	return c.JSON(http.StatusOK, tickets)
}

func (h handler) CreateShow(c echo.Context) error {
	var reqBody createShowRequest
	if err := c.Bind(&reqBody); err != nil {
		return &echo.HTTPError{
			Code:     http.StatusBadRequest,
			Message:  "failed to parse request",
			Internal: fmt.Errorf("failed to bind request: %w", err),
		}
	}

	show := entity.Show{
		ShowID:          uuid.NewString(),
		DeadNationID:    reqBody.DeadNationID,
		NumberOfTickets: reqBody.NumberOfTickets,
		StartTime:       reqBody.StartTime.UTC(),
		Title:           reqBody.Title,
		Venue:           reqBody.Venue,
	}

	if err := h.showRepo.Add(c.Request().Context(), show); err != nil {
		return &echo.HTTPError{
			Code:     http.StatusInternalServerError,
			Message:  http.StatusText(http.StatusInternalServerError),
			Internal: fmt.Errorf("adding show: %w", err),
		}
	}

	return c.JSON(http.StatusCreated, createShowResponse{
		ShowID: show.ShowID,
	})
}

func (h handler) CreateBooking(c echo.Context) error {
	var reqBody createBookingRequest
	if err := c.Bind(&reqBody); err != nil {
		return &echo.HTTPError{
			Code:     http.StatusBadRequest,
			Message:  "failed to parse request",
			Internal: fmt.Errorf("failed to bind request: %w", err),
		}
	}

	show, err := h.showRepo.Get(c.Request().Context(), reqBody.ShowID)
	if err != nil {
		return fmt.Errorf("getting show: %w", err)
	}

	booking := entity.Booking{
		BookingID:       uuid.NewString(),
		CustomerEmail:   reqBody.CustomerEmail,
		NumberOfTickets: reqBody.NumberOfTickets,
		ShowID:          reqBody.ShowID,
	}

	err = h.bookingRepo.Add(c.Request().Context(), show.NumberOfTickets, booking)
	var notEnoughTicketsErr notEnoughTicketsError
	if errors.As(err, &notEnoughTicketsErr) {
		return &echo.HTTPError{
			Code:     http.StatusBadRequest,
			Message:  "Not enough tickets",
			Internal: fmt.Errorf("adding booking: %w", err),
		}
	}

	if err != nil {
		return &echo.HTTPError{
			Code:     http.StatusInternalServerError,
			Message:  http.StatusText(http.StatusInternalServerError),
			Internal: fmt.Errorf("adding booking: %w", err),
		}
	}

	return c.JSON(http.StatusCreated, createBookingResponse{
		BookingID: booking.BookingID,
	})
}

func (h handler) RefundTicket(c echo.Context) error {
	idempotencyKey := getOrGenerateIdempotencyKey(c)
	cmd := command.NewRefundTicket(c.Param("ticket_id"), idempotencyKey)

	if err := h.commandSender.Send(c.Request().Context(), cmd); err != nil {
		return &echo.HTTPError{
			Code:     http.StatusInternalServerError,
			Message:  http.StatusText(http.StatusInternalServerError),
			Internal: fmt.Errorf("publishing refund ticket command: %w", err),
		}
	}

	return c.NoContent(http.StatusAccepted)
}

func getIdempotencyKey(c echo.Context) (string, error) {
	idempotencyKey := c.Request().Header.Get(headerKeyIdempotencyKey)
	if idempotencyKey == "" {
		return "", &echo.HTTPError{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("missing required header %s", headerKeyIdempotencyKey),
		}
	}

	return idempotencyKey, nil
}

func getOrGenerateIdempotencyKey(c echo.Context) string {
	idempotencyKey := c.Request().Header.Get(headerKeyIdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = uuid.NewString()
	}

	return idempotencyKey
}
