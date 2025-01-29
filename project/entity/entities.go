package entity

import "time"

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

type Show struct {
	ShowID          string    `json:"show_id"`
	DeadNationID    string    `json:"dead_nation_id"`
	NumberOfTickets uint      `json:"number_of_tickets"`
	StartTime       time.Time `json:"start_time"`
	Title           string    `json:"title"`
	Venue           string    `json:"venue"`
}
