package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/config"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/repository"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/service"
)

// CreateOrder godoc
// @Summary      Create a new order
// @Description  Places an order for a specific product. Requires Customer role.
// @Tags         orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order  body      CreateOrderRequest  true  "Order Payload"
// @Success      201    {object}  CreateOrderResponse
// @Failure      400    {object}  ErrorResponse
// @Failure      401    {object}  ErrorResponse
// @Failure      409    {object}  ErrorResponse
// @Router       /orders [post]
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	// TODO: userId is supposed to be given via a JWT token.
	// The token will be checked by API gateway.
	// If the token is deemed valid, API gateway should retrieve a userId from a parsed token.
	// After that, the userId will be sent from API gateway to order-service in a request header.
	// Replace the following line, so that userId is retreived from a request header.
	userId := uint(1)

	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid request body",
		})
		return
	}

	ctx := r.Context()
	err := h.service.CreateOrder(ctx, userId, req.CarID)
	if err != nil {
		if errors.Is(err, repository.ErrItemNotFound) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{
				Error: "car not found",
			})
			return
		}
		if errors.Is(err, repository.ErrItemNotAvailable) {
			writeJSON(w, http.StatusConflict, ErrorResponse{
				Error: "car not available",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error: "internal error",
		})
		return
	}

	writeJSON(w, http.StatusCreated, CreateOrderResponse{Message: "success"})
}

// GetOrders godoc
// @Summary      List orders
// @Description  Returns a paginated list of orders. Requires Admin role.
// @Tags         orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        orderBy     query  string false "field to sort by (id, price_cents, created_at)"
// @Param        sortIn      query  string false "sort direction (asc, desc)"
// @Param        car_ids query  string false "comma-separated car IDs to filter"
// @Param        page        query  int    false "page number"
// @Param        limit       query  int    false "items per page"
// @Success      200         {object}  GetOrdersResponse
// @Failure      400         {object}  ErrorResponse
// @Failure      401         {object}  ErrorResponse
// @Failure      500         {object}  ErrorResponse
// @Router       /orders [get]
func (h *Handler) GetOrders(w http.ResponseWriter, r *http.Request) {
	orderBy := r.URL.Query().Get("orderBy")
	sortIn := r.URL.Query().Get("sortIn")
	carIDs := r.URL.Query().Get("car_ids")
	page := r.URL.Query().Get("page")
	limit := r.URL.Query().Get("limit")

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

	carIDsSlice := make([]string, 0)
	if carIDs != "" {
		carIDsSlice = strings.Split(carIDs, ",")
	}

	carIDIntegers := make([]uint, 0)
	for _, carIDString := range carIDsSlice {
		id, err := strconv.ParseUint(carIDString, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{
				Error: "car_ids must be integers",
			})
			return
		}
		carIDIntegers = append(carIDIntegers, uint(id))
	}

	ctx := r.Context()
	params := service.GetOrderParameters{
		OrderBy: orderBy,
		SortIn:  sortIn,
		CarIDs:  carIDIntegers,
		Page:    pageInt,
		Limit:   limitInt,
	}
	orders, total, err := h.service.GetOrdersWithTotalCount(ctx, params)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error: "failed to get orders",
		})
		return
	}
	items := make([]OrderItem, len(orders))
	for i, order := range orders {
		items[i] = OrderItem{
			ID:         order.ID,
			PriceCents: order.PriceCents,
		}
	}

	writeJSON(w, http.StatusOK, GetOrdersResponse{
		Items:   items,
		Page:    pageInt,
		Limit:   limitInt,
		Total:   int(total),
		HasNext: pageInt*limitInt < int(total),
	})
}

// GetOrderById godoc
// @Summary      Get an order by ID
// @Description  Returns a single order. Admins can view any order; customers can view their own orders.
// @Tags         orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Order ID"
// @Success      200  {object}  GetOrderResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /orders/{id} [get]
func (h *Handler) GetOrderById(w http.ResponseWriter, r *http.Request) {
	idString := r.PathValue("id")

	id64, err := strconv.ParseInt(idString, 10, 64)
	if err != nil || id64 <= 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid ID",
		})
		return
	}
	id := uint(id64)

	// TODO: Make sure the userId is passed from API gateway via a request header.
	// If userId isn't provided, return an error here so that `h.service.GetOrderById(id)` won't be called.
	userId := uint(1)

	order, err := h.service.GetOrderById(id)
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

	// TODO: Update the following logic.
	// An order's detail is available for the customer who made the order and admin users.
	if order.UserID != userId {
		writeJSON(w, http.StatusForbidden, ErrorResponse{
			Error: "forbidden",
		})
		return
	}

	orderItem := OrderItem{
		ID:         order.ID,
		PriceCents: order.PriceCents,
	}
	writeJSON(w, http.StatusOK, GetOrderResponse{
		Item: orderItem,
	})
}

// UpdateOrder godoc
// @Summary      Update an order
// @Description  Updates an order's fields (Admin only).
// @Tags         orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id     path      int                 true  "Order ID"
// @Param        order  body      UpdateOrderRequest  true  "Order update payload"
// @Success      200    {object}  UpdateOrderResponse
// @Failure      400    {object}  ErrorResponse
// @Failure      401    {object}  ErrorResponse
// @Failure      404    {object}  ErrorResponse
// @Failure      500    {object}  ErrorResponse
// @Router       /orders/{id} [patch]
func (h *Handler) UpdateOrder(w http.ResponseWriter, r *http.Request) {
	idString := r.PathValue("id")

	id64, err := strconv.ParseInt(idString, 10, 64)
	if err != nil || id64 <= 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid ID",
		})
		return
	}
	id := uint(id64)

	var payload UpdateOrderRequest

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid request body",
		})
		return
	}

	order, err := h.service.UpdateOrder(repository.UpdateOrderInput{ID: id, PriceCents: payload.PriceCents})
	if err != nil {
		if errors.Is(err, repository.ErrItemNotFound) {
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

	item := OrderItem{
		ID:         order.ID,
		PriceCents: order.PriceCents,
	}
	writeJSON(w, http.StatusOK, UpdateOrderResponse{Item: item})
}

// DeleteOrder godoc
// @Summary      Delete an order
// @Description  Deletes an order by ID (Admin only).
// @Tags         orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Order ID"
// @Success      200  {object}  DeleteOrderResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /orders/{id} [delete]
func (h *Handler) DeleteOrder(w http.ResponseWriter, r *http.Request) {
	idString := r.PathValue("id")

	id64, err := strconv.ParseInt(idString, 10, 64)
	if err != nil || id64 <= 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid ID",
		})
		return
	}
	id := uint(id64)

	err = h.service.DeleteOrder(id)
	if err != nil {
		if errors.Is(err, repository.ErrItemNotFound) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{
				Error: "order not found",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error: "internal error",
		})
		return
	}

	writeJSON(w, http.StatusOK, DeleteOrderResponse{Message: "success"})
}
