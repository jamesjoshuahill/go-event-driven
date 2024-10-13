package http

import (
	"net/http"

	commonHTTP "github.com/ThreeDotsLabs/go-event-driven/common/http"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/labstack/echo/v4"
)

type handler struct {
	publisher message.Publisher
}

func NewRouter(publisher message.Publisher) *echo.Echo {
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
