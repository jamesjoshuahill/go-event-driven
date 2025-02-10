package clients

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ThreeDotsLabs/go-event-driven/common/clients/payments"
)

type PaymentsClient struct {
	client payments.ClientWithResponsesInterface
}

func NewPaymentsClient(c *Clients) PaymentsClient {
	return PaymentsClient{
		client: c.Payments,
	}
}

func (c PaymentsClient) RefundPayment(ctx context.Context, idempotencyKey string, ticketID string) error {
	res, err := c.client.PutRefundsWithResponse(ctx, payments.PaymentRefundRequest{
		PaymentReference: ticketID,
		Reason:           "customer requested refund",
		DeduplicationId:  &idempotencyKey,
	})
	if err != nil {
		return fmt.Errorf("put refund request: %w", err)
	}

	if res.StatusCode() != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode())
	}

	return nil
}
