package http

import (
	"net/http"

	commonHTTP "github.com/ThreeDotsLabs/go-event-driven/common/http"
	"github.com/labstack/echo/v4"
)

var ErrServerClosed = http.ErrServerClosed

func NewRouter(publisher Publisher) *echo.Echo {
	server := commonHTTP.NewEcho()

	server.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	handler := handler{
		publisher: publisher,
	}

	server.POST("/tickets-status", handler.PostTicketStatus)

	return server
}
