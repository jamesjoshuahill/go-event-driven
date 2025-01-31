package clients

import (
	"context"
	"fmt"
	"net/http"
	"tickets/entity"

	"github.com/ThreeDotsLabs/go-event-driven/common/clients/dead_nation"
	"github.com/google/uuid"
)

type DeadNationClient struct {
	client dead_nation.ClientWithResponsesInterface
}

func NewDeadNationClient(c *Clients) DeadNationClient {
	return DeadNationClient{
		client: c.DeadNation,
	}
}

func (c DeadNationClient) CreateBooking(ctx context.Context, deadNationID string, booking entity.Booking) error {
	bookingID, err := uuid.Parse(booking.BookingID)
	if err != nil {
		return fmt.Errorf("parsing booking id: %w", err)
	}

	eventID, err := uuid.Parse(deadNationID)
	if err != nil {
		return fmt.Errorf("parsing show id: %w", err)
	}

	body := dead_nation.PostTicketBookingRequest{
		BookingId:       bookingID,
		CustomerAddress: booking.CustomerEmail,
		EventId:         eventID,
		NumberOfTickets: int(booking.NumberOfTickets),
	}
	res, err := c.client.PostTicketBookingWithResponse(ctx, body)
	if err != nil {
		return fmt.Errorf("sending create ticking booking request: %w", err)
	}

	if res.StatusCode() == http.StatusCreated {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode())
	}

	return nil
}
