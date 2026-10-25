package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/cache"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/repository"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type spyCarsCache struct {
	mu            sync.Mutex
	invalidations int
	storedCars    map[string]domain.Car
}

func newSpyCarsCache() *spyCarsCache {
	return &spyCarsCache{storedCars: map[string]domain.Car{}}
}

func (c *spyCarsCache) GetCar(ctx context.Context, key string) (*domain.Car, bool, error) {
	return nil, false, nil
}

func (c *spyCarsCache) GetPage(ctx context.Context, key string) (*cache.CarsPage, bool, error) {
	return nil, false, nil
}

func (c *spyCarsCache) SetCar(ctx context.Context, key string, car *domain.Car, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.storedCars[key] = *car
	return nil
}

func (c *spyCarsCache) SetPage(ctx context.Context, key string, page *cache.CarsPage, ttl time.Duration) error {
	return nil
}

func (c *spyCarsCache) InvalidateCars(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.invalidations++
	return nil
}

func (c *spyCarsCache) InvalidateCar(ctx context.Context, cacheKey string) error {
	return nil
}

func (c *spyCarsCache) snapshot() (int, map[string]domain.Car) {
	c.mu.Lock()
	defer c.mu.Unlock()

	stored := make(map[string]domain.Car, len(c.storedCars))
	for key, car := range c.storedCars {
		stored[key] = car
	}
	return c.invalidations, stored
}

type spyCarsCacheWarmer struct {
	listWarmed chan struct{}
}

func newSpyCarsCacheWarmer() *spyCarsCacheWarmer {
	return &spyCarsCacheWarmer{listWarmed: make(chan struct{}, 10)}
}

func (w *spyCarsCacheWarmer) WarmCar(id uint, ttl time.Duration) {}

func (w *spyCarsCacheWarmer) WarmCarList(ttl time.Duration) {
	w.listWarmed <- struct{}{}
}

type cacheFixture struct {
	svc    *service.Service
	repo   *repository.Repository
	cache  *spyCarsCache
	warmer *spyCarsCacheWarmer
	car    domain.Car // an available car
}

func newCacheFixture(t *testing.T) cacheFixture {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		TranslateError: true,
		Logger:         logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	repo := repository.NewRepository(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	car, err := repo.CreateCar(domain.Car{Name: "test", PriceCents: 5})
	if err != nil {
		t.Fatal(err)
	}

	spyCache := newSpyCarsCache()
	spyWarmer := newSpyCarsCacheWarmer()

	return cacheFixture{
		svc:    service.NewService(repo, spyCache, spyWarmer),
		repo:   repo,
		cache:  spyCache,
		warmer: spyWarmer,
		car:    car,
	}
}

func (f cacheFixture) holdCar(t *testing.T, orderID uint) {
	t.Helper()
	if _, err := f.repo.ReserveCar(f.car.ID, orderID); err != nil {
		t.Fatal(err)
	}
}

func TestReserveAndReleaseCar_RefreshTheCaches(t *testing.T) {
	const orderID = 42

	tests := []struct {
		testName       string
		heldBefore     bool
		act            func(f cacheFixture) error
		expectedStatus domain.CarStatus // what the cached car must say afterwards
	}{
		{
			testName: "reserving an available car",
			act: func(f cacheFixture) error {
				_, err := f.svc.ReserveCar(context.Background(), f.car.ID, orderID)
				return err
			},
			expectedStatus: domain.CarStatusReserved,
		},
		{
			testName:   "releasing a reserved car",
			heldBefore: true,
			act: func(f cacheFixture) error {
				_, err := f.svc.ReleaseCar(context.Background(), f.car.ID, orderID)
				return err
			},
			expectedStatus: domain.CarStatusAvailable,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			f := newCacheFixture(t)
			if test.heldBefore {
				f.holdCar(t, orderID)
			}

			if err := test.act(f); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			invalidations, stored := f.cache.snapshot()
			if invalidations != 1 {
				t.Fatalf("expected the car lists to be invalidated once, got %d", invalidations)
			}
			cached, ok := stored[cache.CarCacheKey(f.car.ID)]
			if !ok {
				t.Fatal("the car was not written to the cache")
			}
			if cached.Status != test.expectedStatus {
				t.Fatalf("expected the cached car to be %q, got %q", test.expectedStatus, cached.Status)
			}

			select {
			case <-f.warmer.listWarmed:
			case <-time.After(time.Second):
				t.Fatal("the car list was not warmed")
			}
		})
	}
}

func TestReserveAndReleaseCar_LeaveTheCachesAloneOnFailure(t *testing.T) {
	const orderID = 42

	tests := []struct {
		testName    string
		heldBy      uint
		act         func(f cacheFixture) error
		expectedErr error
	}{
		{
			testName: "reserving a car that another order holds",
			heldBy:   7,
			act: func(f cacheFixture) error {
				_, err := f.svc.ReserveCar(context.Background(), f.car.ID, orderID)
				return err
			},
			expectedErr: repository.ErrItemNotAvailable,
		},
		{
			testName: "releasing a car that another order holds",
			heldBy:   7,
			act: func(f cacheFixture) error {
				_, err := f.svc.ReleaseCar(context.Background(), f.car.ID, orderID)
				return err
			},
			expectedErr: repository.ErrItemNotAvailable,
		},
		{
			testName: "reserving a car that does not exist",
			act: func(f cacheFixture) error {
				_, err := f.svc.ReserveCar(context.Background(), f.car.ID+1000, orderID)
				return err
			},
			expectedErr: repository.ErrItemNotFound,
		},
		{
			testName: "releasing a car that does not exist",
			act: func(f cacheFixture) error {
				_, err := f.svc.ReleaseCar(context.Background(), f.car.ID+1000, orderID)
				return err
			},
			expectedErr: repository.ErrItemNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			f := newCacheFixture(t)
			if test.heldBy != 0 {
				f.holdCar(t, test.heldBy)
			}

			err := test.act(f)
			if !errors.Is(err, test.expectedErr) {
				t.Fatalf("expected error %v, got %v", test.expectedErr, err)
			}

			invalidations, stored := f.cache.snapshot()
			if invalidations != 0 {
				t.Fatalf("expected no invalidation, got %d", invalidations)
			}
			if len(stored) != 0 {
				t.Fatalf("expected nothing to be written to the cache, got %+v", stored)
			}

			select {
			case <-f.warmer.listWarmed:
				t.Fatal("the car list must not be warmed")
			case <-time.After(50 * time.Millisecond):
			}
		})
	}
}
