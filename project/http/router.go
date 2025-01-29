package http

import (
	"net/http"

	commonHTTP "github.com/ThreeDotsLabs/go-event-driven/common/http"
	"github.com/labstack/echo/v4"
)

var ErrServerClosed = http.ErrServerClosed

func NewRouter(publisher Publisher, showRepo ShowRepo, ticketRepo TicketRepo) *echo.Echo {
	server := commonHTTP.NewEcho()

	server.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	handler := handler{
		publisher:  publisher,
		showRepo:   showRepo,
		ticketRepo: ticketRepo,
	}

	server.POST("/shows", handler.CreateShow)
	server.POST("/tickets-status", handler.CreateTicketStatus)
	server.GET("/tickets", handler.ListTickets)

	return server
}
