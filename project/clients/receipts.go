package clients

import (
	"context"
	"fmt"
	"net/http"
	"tickets/entity"

	"github.com/ThreeDotsLabs/go-event-driven/common/clients/receipts"
)

type ReceiptsClient struct {
	client receipts.ClientWithResponsesInterface
}

func NewReceiptsClient(c *Clients) ReceiptsClient {
	return ReceiptsClient{
		client: c.Receipts,
	}
}

func (c ReceiptsClient) IssueReceipt(ctx context.Context, idempotencyKey, ticketID string, price entity.Money) error {
	body := receipts.CreateReceipt{
		IdempotencyKey: &idempotencyKey,
		TicketId:       ticketID,
		Price: receipts.Money{
			MoneyAmount:   price.Amount,
			MoneyCurrency: price.Currency,
		},
	}

	res, err := c.client.PutReceiptsWithResponse(ctx, body)
	if err != nil {
		return fmt.Errorf("put receipt request: %w", err)
	}

	if res.StatusCode() != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode())
	}

	return nil
}
