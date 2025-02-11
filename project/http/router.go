package http

import (
	"net/http"

	commonHTTP "github.com/ThreeDotsLabs/go-event-driven/common/http"
	"github.com/ThreeDotsLabs/watermill"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

var ErrServerClosed = http.ErrServerClosed

type RouterDeps struct {
	BookingRepo       BookingRepo
	CommandSender     CommandSender
	DB                *sqlx.DB
	EventPublisher    EventPublisher
	NewEventPublisher EventPublisher
	Logger            watermill.LoggerAdapter
	ShowRepo          ShowRepo
	TicketRepo        TicketRepo
}

func NewRouter(deps RouterDeps) *echo.Echo {
	server := commonHTTP.NewEcho()

	server.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	handler := handler{
		bookingRepo:       deps.BookingRepo,
		commandSender:     deps.CommandSender,
		db:                deps.DB,
		eventPublisher:    deps.EventPublisher,
		newEventPublisher: deps.NewEventPublisher,
		logger:            deps.Logger,
		showRepo:          deps.ShowRepo,
		ticketRepo:        deps.TicketRepo,
	}

	server.POST("/shows", handler.CreateShow)
	server.POST("/book-tickets", handler.CreateBooking)
	server.POST("/tickets-status", handler.CreateTicketStatus)
	server.GET("/tickets", handler.ListTickets)
	server.PUT("/ticket-refund/:ticket_id", handler.RefundTicket)

	return server
}
