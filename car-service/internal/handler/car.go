package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/config"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/repository"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/service"
)

// GetCars godoc
// @Summary      List cars
// @Description  Returns a paginated list of cars with optional filtering and sorting.
// @Tags         cars
// @Accept       json
// @Produce      json
// @Param        orderBy  query  string false "field to sort by (name, price_cents)"
// @Param        sortIn   query  string false "sort direction (asc, desc)"
// @Param        name     query  string false "filter by car name (partial match)"
// @Param        minPrice query  int    false "minimum price in cents"
// @Param        maxPrice query  int    false "maximum price in cents"
// @Param        page     query  int    false "page number"
// @Param        limit    query  int    false "items per page"
// @Success      200      {object}  GetCarsResponse
// @Failure      400      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Router       /cars [get]
func (h *Handler) GetCars(w http.ResponseWriter, r *http.Request) {
	orderBy := r.URL.Query().Get("orderBy")
	sortIn := r.URL.Query().Get("sortIn")
	name := r.URL.Query().Get("name")
	minPrice := r.URL.Query().Get("minPrice")
	maxPrice := r.URL.Query().Get("maxPrice")
	page := r.URL.Query().Get("page")
	limit := r.URL.Query().Get("limit")

	minPriceInt, err := parseOptionalInt64(minPrice, 0)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid minPrice",
		})
		return
	}

	maxPriceInt, err := parseOptionalInt64(maxPrice, config.DefaultMaxCarPrice)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid maxPrice",
		})
		return
	}

	pageInt, err := parseOptionalInt(page, 1)
	if err != nil || pageInt <= 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "page must be a positive integer",
		})
		return
	}

	limitInt, err := parseOptionalInt(limit, config.DefaultPageLimit)
	if err != nil || limitInt <= 0 || limitInt > config.MaxPageLimit {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "limit must be a positive integer and not exceed " + strconv.Itoa(config.MaxPageLimit),
		})
		return
	}

	ctx := r.Context()
	params := service.GetCarsParameters{
		OrderBy:  orderBy,
		SortIn:   sortIn,
		Name:     name,
		MinPrice: minPriceInt,
		MaxPrice: maxPriceInt,
		Page:     pageInt,
		Limit:    limitInt,
	}
	cars, total, err := h.service.GetCarsWithTotalCount(ctx, params)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error: "failed to get cars",
		})
		return
	}
	response := GetCarsResponse{
		Items:   mapCarsToCarItems(cars),
		Page:    pageInt,
		Limit:   limitInt,
		Total:   int(total),
		HasNext: pageInt*limitInt < int(total),
	}

	writeJSON(w, http.StatusOK, response)
}

func mapCarsToCarItems(cars []domain.Car) []CarItem {
	items := make([]CarItem, len(cars))
	for i, car := range cars {
		items[i] = CarItem{
			ID:         car.ID,
			Name:       car.Name,
			PriceCents: car.PriceCents,
		}
	}

	return items
}

// GetCarById godoc
// @Summary      Get a car by ID
// @Description  Returns a single car.
// @Tags         cars
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Car ID"
// @Success      200  {object}  GetCarResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /cars/{id} [get]
func (h *Handler) GetCarById(w http.ResponseWriter, r *http.Request) {
	idString := r.PathValue("id")

	id64, err := strconv.ParseInt(idString, 10, 64)
	if err != nil || id64 <= 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid ID",
		})
		return
	}
	id := uint(id64)

	ctx := r.Context()
	car, err := h.service.GetCarById(ctx, id)
	if err != nil {
		if err == repository.ErrItemNotFound {
			writeJSON(w, http.StatusNotFound, ErrorResponse{
				Error: "item not found",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error: "internal error",
		})
		return
	}

	carItem := CarItem{
		ID:         car.ID,
		Name:       car.Name,
		PriceCents: car.PriceCents,
	}
	writeJSON(w, http.StatusOK, GetCarResponse{
		Item: carItem,
	})
}

// CreateCar godoc
// @Summary      Create a car
// @Description  Creates a new car (Admin only).
// @Tags         cars
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        car  body      CreateCarRequest  true  "Car payload"
// @Success      201      {object}  CreateCarResponse
// @Failure      400      {object}  ErrorResponse
// @Failure      401      {object}  ErrorResponse
// @Failure      409      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Router       /cars [post]
func (h *Handler) CreateCar(w http.ResponseWriter, r *http.Request) {
	var payload CreateCarRequest

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid request body",
		})
		return
	}

	payload.Name = strings.TrimSpace(payload.Name)
	if err := h.validate.Struct(payload); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: h.formatValidationError(err),
		})
		return
	}

	car, err := h.service.CreateCar(domain.Car{
		Name:       payload.Name,
		PriceCents: payload.PriceCents,
	})
	if err != nil {
		if errors.Is(err, repository.ErrCarAlreadyExists) {
			writeJSON(w, http.StatusConflict, ErrorResponse{
				Error: "duplicate car name",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error: "internal error",
		})
		return
	}

	item := CarItem{
		ID:         car.ID,
		Name:       car.Name,
		PriceCents: car.PriceCents,
	}
	writeJSON(w, http.StatusCreated, CreateCarResponse{
		Item: item,
	})
}

// UpdateCar godoc
// @Summary      Update a car
// @Description  Updates car fields (Admin only).
// @Tags         cars
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      int                  true  "Car ID"
// @Param        car  body      UpdateCarRequest  true  "Car update payload"
// @Success      200      {object}  UpdateCarResponse
// @Failure      400      {object}  ErrorResponse
// @Failure      401      {object}  ErrorResponse
// @Failure      404      {object}  ErrorResponse
// @Failure      409      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Router       /cars/{id} [patch]
func (h *Handler) UpdateCar(w http.ResponseWriter, r *http.Request) {
	idString := r.PathValue("id")

	id64, err := strconv.ParseInt(idString, 10, 64)
	if err != nil || id64 <= 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid ID",
		})
		return
	}
	id := uint(id64)

	var payload UpdateCarRequest

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid request body",
		})
		return
	}

	if payload.Name != nil {
		trimmedName := strings.TrimSpace(*payload.Name)
		payload.Name = &trimmedName
		if err := h.validate.Struct(payload); err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{
				Error: h.formatValidationError(err),
			})
			return
		}
	}

	ctx := r.Context()
	car, err := h.service.UpdateCar(
		ctx,
		repository.UpdateCarsInput{
			ID:         id,
			Name:       payload.Name,
			PriceCents: payload.PriceCents,
		},
	)
	if err != nil {
		if errors.Is(err, repository.ErrItemNotFound) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{
				Error: "item not found",
			})
			return
		}
		if errors.Is(err, repository.ErrCarAlreadyExists) {
			writeJSON(w, http.StatusConflict, ErrorResponse{
				Error: "duplicate car data",
			})
			return
		}
		if errors.Is(err, repository.ErrOptimisticLockFailed) {
			writeJSON(w, http.StatusConflict, ErrorResponse{
				Error: "car data locked",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error: "internal error",
		})
		return
	}

	item := CarItem{
		ID:         car.ID,
		Name:       car.Name,
		PriceCents: car.PriceCents,
	}
	writeJSON(w, http.StatusOK, UpdateCarResponse{Item: item})
}

func (h *Handler) DeleteCar(w http.ResponseWriter, r *http.Request) {
	idString := r.PathValue("id")

	id64, err := strconv.ParseInt(idString, 10, 64)
	if err != nil || id64 <= 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid ID",
		})
		return
	}
	id := uint(id64)

	ctx := r.Context()
	err = h.service.DeleteCar(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrItemNotFound) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{
				Error: "car not found",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error: "internal error",
		})
		return
	}

	writeJSON(w, http.StatusOK, DeleteCarResponse{Message: "success"})
}
