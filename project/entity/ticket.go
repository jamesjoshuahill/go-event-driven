package entity

const (
	StatusConfirmed = "confirmed"
	StatusCanceled  = "canceled"
)

type Ticket struct {
	ID            string `json:"ticket_id"`
	CustomerEmail string `json:"customer_email"`
	Price         Money  `json:"price"`
}

type Money struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}
