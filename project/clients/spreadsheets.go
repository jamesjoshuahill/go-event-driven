package clients

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ThreeDotsLabs/go-event-driven/common/clients/spreadsheets"
)

type SpreadsheetsClient struct {
	client spreadsheets.ClientWithResponsesInterface
}

func NewSpreadsheetsClient(c *Clients) SpreadsheetsClient {
	return SpreadsheetsClient{
		client: c.Spreadsheets,
	}
}

func (c SpreadsheetsClient) AppendRow(ctx context.Context, spreadsheetName string, row []string) error {
	request := spreadsheets.PostSheetsSheetRowsJSONRequestBody{
		Columns: row,
	}

	res, err := c.client.PostSheetsSheetRowsWithResponse(ctx, spreadsheetName, request)
	if err != nil {
		return err
	}

	if res.StatusCode() != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode())
	}

	return nil
}
