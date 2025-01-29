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
	ShowID          string
	DeadNationID    string
	NumberOfTickets uint
	StartTime       time.Time
	Title           string
	Venue           string
}

type Booking struct {
	BookingID       string
	ShowID          string
	NumberOfTickets uint
	CustomerEmail   string
}
