package domain

import "testing"

func TestCarStatus_CanTransitionTo(t *testing.T) {
	statuses := []CarStatus{CarStatusAvailable, CarStatusReserved, CarStatusSold, CarStatusWithdrawn}

	allowed := map[CarStatus]map[CarStatus]bool{
		CarStatusAvailable: {CarStatusReserved: true, CarStatusWithdrawn: true},
		CarStatusReserved:  {CarStatusAvailable: true, CarStatusSold: true},
		CarStatusWithdrawn: {CarStatusAvailable: true},
		CarStatusSold:      {},
	}

	for _, from := range statuses {
		for _, to := range statuses {
			t.Run(string(from)+" -> "+string(to), func(t *testing.T) {
				if got := from.CanTransitionTo(to); got != allowed[from][to] {
					t.Fatalf("CanTransitionTo = %v, want %v", got, allowed[from][to])
				}
			})
		}
	}
}

func TestCarStatus_CanTransitionTo_UnknownStatus(t *testing.T) {
	unknown := CarStatus("banana")

	if unknown.IsValid() {
		t.Fatal("an unknown status must not be valid")
	}
	if unknown.CanTransitionTo(CarStatusAvailable) {
		t.Fatal("cannot move out of an unknown status")
	}
	if CarStatusAvailable.CanTransitionTo(unknown) {
		t.Fatal("cannot move into an unknown status")
	}
}

func TestCarStatus_IsValid(t *testing.T) {
	for _, s := range []CarStatus{CarStatusAvailable, CarStatusReserved, CarStatusSold, CarStatusWithdrawn} {
		if !s.IsValid() {
			t.Fatalf("%q should be valid", s)
		}
	}
	for _, s := range []CarStatus{"", "banana", "Available", "unavailable"} {
		if s.IsValid() {
			t.Fatalf("%q should not be valid", s)
		}
	}
}

func TestCarStatus_CanBeChangedByDealerTo(t *testing.T) {
	tests := []struct {
		from, to CarStatus
		want     bool
	}{
		// A dealer can withdraw an available car ...
		{CarStatusAvailable, CarStatusWithdrawn, true},
		// ... and relist a withdrawn car.
		{CarStatusWithdrawn, CarStatusAvailable, true},
		// Sold cars' statuses can't be changed.
		{CarStatusSold, CarStatusWithdrawn, false},
		{CarStatusSold, CarStatusAvailable, false},
		// A reserved car has an order in flight; it must be released first.
		{CarStatusReserved, CarStatusWithdrawn, false},
		// Not changing status is invalid.
		{CarStatusAvailable, CarStatusAvailable, false},
		{CarStatusWithdrawn, CarStatusWithdrawn, false},
		// Withdran cars can't be directly reserved or sold.
		{CarStatusWithdrawn, CarStatusReserved, false},
		{CarStatusWithdrawn, CarStatusSold, false},
		// Everything else belongs to the order flow.
		{CarStatusAvailable, CarStatusReserved, false},
		{CarStatusReserved, CarStatusAvailable, false},
		{CarStatusReserved, CarStatusSold, false},
	}

	for _, test := range tests {
		t.Run(string(test.from)+" -> "+string(test.to), func(t *testing.T) {
			if got := test.from.CanBeChangedByDealerTo(test.to); got != test.want {
				t.Fatalf("CanBeChangedByDealerTo = %v, want %v", got, test.want)
			}
		})
	}
}
