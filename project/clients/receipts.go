package clients

import (
	"context"
	"fmt"
	"net/http"
	"tickets/entity"

	"github.com/ThreeDotsLabs/go-event-driven/common/clients"
	"github.com/ThreeDotsLabs/go-event-driven/common/clients/receipts"
)

type ReceiptsClient struct {
	clients *clients.Clients
}

func NewReceiptsClient(clients *clients.Clients) ReceiptsClient {
	return ReceiptsClient{
		clients: clients,
	}
}

func (c ReceiptsClient) IssueReceipt(ctx context.Context, ticketID string, price entity.Money) error {
	body := receipts.CreateReceipt{
		TicketId: ticketID,
		Price: receipts.Money{
			MoneyAmount:   price.Amount,
			MoneyCurrency: price.Currency,
		},
	}

	res, err := c.clients.Receipts.PutReceiptsWithResponse(ctx, body)
	if err != nil {
		return fmt.Errorf("put receipt request: %w", err)
	}

	if res.StatusCode() != http.StatusOK {
		return fmt.Errorf("unexpected status code: %v", res.StatusCode())
	}

	return nil
}
