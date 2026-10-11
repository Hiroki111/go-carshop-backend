package domain

import (
	"slices"

	"gorm.io/gorm"
)

type CarStatus string

const (
	CarStatusAvailable CarStatus = "available"
	CarStatusReserved  CarStatus = "reserved"
	CarStatusSold      CarStatus = "sold"
	// CarStatusWithdrawn: the dealer pulled the car from sale (a defect was
	// found, nobody wants it, ...). Unlike reserved and sold this is the
	// dealer's own decision, not part of an order.
	CarStatusWithdrawn CarStatus = "withdrawn"
)

var allowedTransitions = map[CarStatus][]CarStatus{
	CarStatusAvailable: {CarStatusReserved, CarStatusWithdrawn},
	CarStatusReserved:  {CarStatusAvailable, CarStatusSold},
	CarStatusWithdrawn: {CarStatusAvailable},
	CarStatusSold:      {},
}

func (s CarStatus) IsValid() bool {
	_, ok := allowedTransitions[s]
	return ok
}

func (s CarStatus) CanTransitionTo(next CarStatus) bool {
	return slices.Contains(allowedTransitions[s], next)
}

// CanBeChangedByDealerTo reports whether a dealer (an admin) may move a car
// from s to next by hand: withdrawing a car, or relisting a withdrawn one.
// Reserving, releasing and selling belong to the order flow, not to a dealer.
func (s CarStatus) CanBeChangedByDealerTo(next CarStatus) bool {
	return s.CanTransitionTo(next) && (next == CarStatusWithdrawn || s == CarStatusWithdrawn)
}

type Car struct {
	gorm.Model
	Name       string    `gorm:"uniqueIndex;not null"`
	PriceCents uint      `gorm:"not null"`
	Version    uint      `gorm:"not null;default:1"`
	Status     CarStatus `gorm:"not null;default:available;check:chk_cars_status,status IN ('available','reserved','sold','withdrawn')"`
}
