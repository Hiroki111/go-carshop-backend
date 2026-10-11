package carclient

const (
	StatusAvailable = "available"
	StatusReserved  = "reserved"
	StatusSold      = "sold"
	StatusWithdrawn = "withdrawn"
)

type Car struct {
	ID         uint
	Name       string
	PriceCents uint
	Status     string
}
