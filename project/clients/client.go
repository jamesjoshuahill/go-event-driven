package clients

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ThreeDotsLabs/go-event-driven/common/clients"
	"github.com/ThreeDotsLabs/go-event-driven/common/log"
)

type Clients struct {
	*clients.Clients
}

func New(gatewayAddress string) (*Clients, error) {
	c, err := clients.NewClients(gatewayAddress, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Correlation-ID", log.CorrelationIDFromContext(ctx))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("creating gateway client: %w", err)
	}

	return &Clients{c}, nil
}
