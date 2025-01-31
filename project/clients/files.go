package clients

import (
	"context"
	"fmt"
	"net/http"
	"tickets/entity"

	"github.com/ThreeDotsLabs/go-event-driven/common/clients/files"
	"github.com/ThreeDotsLabs/go-event-driven/common/log"
)

const printTicketFileTemplate = `<html><body>
Ticket ID: %s
Price amount: %s
Price currency: %s
</body></html>`

type FilesClient struct {
	client files.ClientWithResponsesInterface
}

func NewFilesClient(c *Clients) FilesClient {
	return FilesClient{
		client: c.Files,
	}
}

func (c FilesClient) GenerateTicket(ctx context.Context, ticketID string, price entity.Money) (string, error) {
	fileID := fmt.Sprintf("%s-ticket.html", ticketID)
	fileContent := fmt.Sprintf(printTicketFileTemplate, ticketID, price.Amount, price.Currency)

	res, err := c.client.PutFilesFileIdContentWithTextBodyWithResponse(ctx, fileID, fileContent)
	if err != nil {
		return "", fmt.Errorf("put file request: %w", err)
	}

	if res.StatusCode() == http.StatusConflict {
		log.FromContext(ctx).Infof("file %s already exists", fileID)
		return fileID, nil
	}

	if res.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", res.StatusCode())
	}

	return fileID, nil
}
