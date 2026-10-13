package repository_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/repository"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/testutil"
)

// These tests need a real Postgres (see testutil.NewPostgresDB). They are
// skipped when TEST_POSTGRES_DSN is not set.

const concurrentAttempts = 20

func newRepoWithCar(t *testing.T) (*repository.Repository, domain.Car) {
	t.Helper()

	db := testutil.NewPostgresDB(t)

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(concurrentAttempts + 5)

	repo := repository.NewRepository(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	car, err := repo.CreateCar(domain.Car{Name: "contested car", PriceCents: 1000})
	if err != nil {
		t.Fatal(err)
	}

	car, err = repo.GetCarById(car.ID)
	if err != nil {
		t.Fatal(err)
	}

	return repo, car
}

func runConcurrently(n int, attempt func(i int) error) []error {
	results := make([]error, n)
	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i] = attempt(i)
		}(i)
	}

	close(start)
	wg.Wait()

	return results
}

func countSuccesses(results []error) (count, firstIndex int) {
	firstIndex = -1
	for i, err := range results {
		if err == nil {
			count++
			if firstIndex == -1 {
				firstIndex = i
			}
		}
	}
	return count, firstIndex
}

func TestReserveCar_ConcurrentOrdersReserveExactlyOnce(t *testing.T) {
	repo, car := newRepoWithCar(t)

	results := runConcurrently(concurrentAttempts, func(i int) error {
		orderID := uint(i + 1)
		_, err := repo.ReserveCar(car.ID, orderID)
		return err
	})

	successes, winner := countSuccesses(results)
	if successes != 1 {
		t.Fatalf("expected exactly 1 successful reservation, got %d", successes)
	}
	for i, err := range results {
		if i == winner {
			continue
		}
		if !errors.Is(err, repository.ErrItemNotAvailable) {
			t.Errorf("attempt %d: expected ErrItemNotAvailable, got %v", i, err)
		}
	}

	got, err := repo.GetCarById(car.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.CarStatusReserved {
		t.Errorf("expected status %q, got %q", domain.CarStatusReserved, got.Status)
	}
	if got.ReservedByOrderID == nil || *got.ReservedByOrderID != uint(winner+1) {
		t.Errorf("expected the car to be held by the winning order %d, got %s", winner+1, testutil.DescribeUint(got.ReservedByOrderID))
	}
	if got.Version != car.Version+1 {
		t.Errorf("expected version %d (bumped once), got %d", car.Version+1, got.Version)
	}
}

func TestReserveCar_ConcurrentRetriesOfSameOrderAllSucceed(t *testing.T) {
	repo, car := newRepoWithCar(t)
	const orderID = 7

	results := runConcurrently(concurrentAttempts, func(int) error {
		_, err := repo.ReserveCar(car.ID, orderID)
		return err
	})

	for i, err := range results {
		if err != nil {
			t.Errorf("attempt %d: unexpected error: %v", i, err)
		}
	}

	got, err := repo.GetCarById(car.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.CarStatusReserved {
		t.Errorf("expected status %q, got %q", domain.CarStatusReserved, got.Status)
	}
	if got.Version != car.Version+1 {
		t.Errorf("expected version %d (bumped once), got %d", car.Version+1, got.Version)
	}
}
