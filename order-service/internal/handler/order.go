package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/auth"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/config"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/repository"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/service"
)

// CreateOrder godoc
// @Summary      Create a new order
// @Description  Places an order for a car and reserves the car for it in car-service. 201 means a new order was created and confirmed. 200 means the customer already had a live order for this car, so this request was a retry: the existing order is returned, and completed first if it was still pending. 409 means the car is not available. 503 means the order exists but car-service could not confirm the reservation yet; it stays pending, and repeating the request completes it.
// @Tags         orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        order  body      CreateOrderRequest  true  "Order Payload"
// @Success      201    {object}  CreateOrderResponse
// @Success      200    {object}  CreateOrderResponse
// @Failure      400    {object}  ErrorResponse
// @Failure      401    {object}  ErrorResponse
// @Failure      404    {object}  ErrorResponse
// @Failure      409    {object}  ErrorResponse
// @Failure      500    {object}  ErrorResponse
// @Failure      503    {object}  ErrorResponse
// @Router       /orders [post]
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	userId, ok := r.Context().Value(UserIDKey).(uint)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{
			Error: "unauthorized",
		})
		return
	}

	userName, ok := r.Context().Value(UserNameKey).(string)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{
			Error: "unauthorized",
		})
		return
	}

	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid request body",
		})
		return
	}

	ctx := r.Context()
	order, created, err := h.service.CreateOrder(ctx, userId, userName, req.CarID)
	if err != nil {
		var pending *service.PendingOrderError
		if errors.As(err, &pending) {
			log.Printf("order %d is pending: %v", pending.OrderID, pending.Err)
			writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{
				Error: fmt.Sprintf("order %d was created but the car could not be reserved yet; it is pending", pending.OrderID),
			})
			return
		}
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

	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}
	writeJSON(w, status, CreateOrderResponse{
		Message: "success",
		ID:      order.ID,
		Status:  string(order.Status),
	})
}

// GetOrders godoc
// @Summary      List orders
// @Description  Returns a paginated list of orders. Requires Admin role.
// @Tags         orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        orderBy     query  string false "field to sort by (car_id, user_id, created_at); unknown values fall back to created_at"
// @Param        sortIn      query  string false "sort direction (asc, desc); unknown values fall back to asc"
// @Param        car_ids     query  string false "comma-separated car IDs to filter"
// @Param        page        query  int    false "page number"
// @Param        limit       query  int    false "items per page"
// @Success      200         {object}  GetOrdersResponse
// @Failure      400         {object}  ErrorResponse
// @Failure      401         {object}  ErrorResponse
// @Failure      403         {object}  ErrorResponse
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
		items[i] = toOrderItem(order)
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

	role, roleRetrieved := r.Context().Value(RoleKey).(auth.UserRole)
	userId, userIdRetrieved := r.Context().Value(UserIDKey).(uint)
	if !roleRetrieved || !userIdRetrieved {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error: "internal error",
		})
		return
	}

	order, err := h.service.GetOrderById(id)
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

	if role != auth.AdminRole && order.UserID != userId {
		writeJSON(w, http.StatusForbidden, ErrorResponse{
			Error: "forbidden",
		})
		return
	}

	writeJSON(w, http.StatusOK, GetOrderResponse{
		Item: toOrderItem(order),
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
// @Failure      403    {object}  ErrorResponse
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

	order, err := h.service.UpdateOrder(repository.UpdateOrderInput{ID: id, CarName: payload.CarName, PriceCents: payload.PriceCents})
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

	writeJSON(w, http.StatusOK, UpdateOrderResponse{Item: toOrderItem(order)})
}

// CancelOrder godoc
// @Summary      Cancel an order
// @Description  Cancels a pending or confirmed order and releases its car in car-service (Admin only). Repeating the request for an order that is already cancelled returns 200 with the order. 409 means the order is failed (nothing to cancel), car-service refused to release the car, or the order changed while the request was running. 503 means car-service could not confirm the release: the order is unchanged and the request can be repeated.
// @Tags         orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Order ID"
// @Success      200  {object}  CancelOrderResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      409  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Failure      503  {object}  ErrorResponse
// @Router       /orders/{id}/cancel [post]
func (h *Handler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	idString := r.PathValue("id")

	id64, err := strconv.ParseInt(idString, 10, 64)
	if err != nil || id64 <= 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid ID",
		})
		return
	}
	id := uint(id64)

	order, err := h.service.CancelOrder(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrItemNotFound):
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "order not found"})
		case errors.Is(err, service.ErrOrderNotCancellable):
			writeJSON(w, http.StatusConflict, ErrorResponse{Error: "order cannot be cancelled (it failed, so it never held a car)"})
		case errors.Is(err, service.ErrCarNotReleasable):
			log.Printf("order %d: %v", id, err)
			writeJSON(w, http.StatusConflict, ErrorResponse{Error: "car could not be released (car may be sold or held by a different order)"})
		case errors.Is(err, repository.ErrOrderStatusConflict):
			writeJSON(w, http.StatusConflict, ErrorResponse{Error: "order changed while it was being cancelled; try again"})
		case errors.Is(err, service.ErrCarReleaseUnconfirmed):
			log.Printf("order %d: %v", id, err)
			writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "car-service could not confirm the release; the order is unchanged, try again"})
		default:
			log.Printf("order %d: failed to cancel: %v", id, err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal error"})
		}
		return
	}

	writeJSON(w, http.StatusOK, CancelOrderResponse{Item: toOrderItem(order)})
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
// @Failure      403  {object}  ErrorResponse
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

func toOrderItem(order domain.Order) OrderItem {
	return OrderItem{
		ID:         order.ID,
		CarName:    order.CarName,
		UserName:   order.UserName,
		PriceCents: order.PriceCents,
		Status:     string(order.Status),
	}
}
