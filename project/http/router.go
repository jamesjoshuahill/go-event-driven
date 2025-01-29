package http

import (
	"net/http"

	commonHTTP "github.com/ThreeDotsLabs/go-event-driven/common/http"
	"github.com/ThreeDotsLabs/watermill"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

var ErrServerClosed = http.ErrServerClosed

func NewRouter(
	db *sqlx.DB,
	bookingRepo BookingRepo,
	logger watermill.LoggerAdapter,
	publisher Publisher,
	showRepo ShowRepo,
	ticketRepo TicketRepo,
) *echo.Echo {
	server := commonHTTP.NewEcho()

	server.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	handler := handler{
		db:          db,
		bookingRepo: bookingRepo,
		logger:      logger,
		publisher:   publisher,
		showRepo:    showRepo,
		ticketRepo:  ticketRepo,
	}

	server.POST("/shows", handler.CreateShow)
	server.POST("/book-tickets", handler.CreateBooking)
	server.POST("/tickets-status", handler.CreateTicketStatus)
	server.GET("/tickets", handler.ListTickets)

	return server
}
