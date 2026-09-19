package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/config"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/repository"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/service"
)

// GetProducts godoc
// @Summary      List products
// @Description  Returns a paginated list of products with optional filtering and sorting.
// @Tags         products
// @Accept       json
// @Produce      json
// @Param        orderBy  query  string false "field to sort by (name, price_cents)"
// @Param        sortIn   query  string false "sort direction (asc, desc)"
// @Param        name     query  string false "filter by product name (partial match)"
// @Param        minPrice query  int    false "minimum price in cents"
// @Param        maxPrice query  int    false "maximum price in cents"
// @Param        page     query  int    false "page number"
// @Param        limit    query  int    false "items per page"
// @Success      200      {object}  GetProductsResponse
// @Failure      400      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Router       /cars [get]
func (h *Handler) GetProducts(w http.ResponseWriter, r *http.Request) {
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

	maxPriceInt, err := parseOptionalInt64(maxPrice, config.DefaultMaxProductPrice)
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
	params := service.GetProductsParameters{
		OrderBy:  orderBy,
		SortIn:   sortIn,
		Name:     name,
		MinPrice: minPriceInt,
		MaxPrice: maxPriceInt,
		Page:     pageInt,
		Limit:    limitInt,
	}
	products, total, err := h.service.GetProductsWithTotalCount(ctx, params)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error: "failed to get products",
		})
		return
	}
	response := GetProductsResponse{
		Items:   mapProductsToProductItems(products),
		Page:    pageInt,
		Limit:   limitInt,
		Total:   int(total),
		HasNext: pageInt*limitInt < int(total),
	}

	writeJSON(w, http.StatusOK, response)
}

func mapProductsToProductItems(products []domain.Product) []ProductItem {
	items := make([]ProductItem, len(products))
	for i, product := range products {
		items[i] = ProductItem{
			ID:         product.ID,
			Name:       product.Name,
			PriceCents: product.PriceCents,
		}
	}

	return items
}

// GetProductById godoc
// @Summary      Get a product by ID
// @Description  Returns a single product.
// @Tags         products
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Product ID"
// @Success      200  {object}  GetProductResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /cars/{id} [get]
func (h *Handler) GetProductById(w http.ResponseWriter, r *http.Request) {
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
	product, err := h.service.GetProductById(ctx, id)
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

	productItem := ProductItem{
		ID:         product.ID,
		Name:       product.Name,
		PriceCents: product.PriceCents,
	}
	writeJSON(w, http.StatusOK, GetProductResponse{
		Item: productItem,
	})
}

// CreateProduct godoc
// @Summary      Create a product
// @Description  Creates a new product (Admin only).
// @Tags         products
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        product  body      CreateProductRequest  true  "Product payload"
// @Success      201      {object}  CreateProductResponse
// @Failure      400      {object}  ErrorResponse
// @Failure      401      {object}  ErrorResponse
// @Failure      409      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Router       /cars [post]
func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var payload CreateProductRequest

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid request body",
		})
		return
	}

	if err := h.validate.Struct(payload); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: h.formatValidationError(err),
		})
		return
	}

	product, err := h.service.CreateProduct(domain.Product{
		Name:       payload.Name,
		PriceCents: payload.PriceCents,
	})
	if err != nil {
		if errors.Is(err, repository.ErrProductAlreadyExists) {
			writeJSON(w, http.StatusConflict, ErrorResponse{
				Error: "duplicate product name",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error: "internal error",
		})
		return
	}

	item := ProductItem{
		ID:         product.ID,
		Name:       product.Name,
		PriceCents: product.PriceCents,
	}
	writeJSON(w, http.StatusCreated, CreateProductResponse{
		Item: item,
	})
}

// UpdateProduct godoc
// @Summary      Update a product
// @Description  Updates product fields (Admin only).
// @Tags         products
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      int                  true  "Product ID"
// @Param        product  body      UpdateProductRequest  true  "Product update payload"
// @Success      200      {object}  UpdateProductResponse
// @Failure      400      {object}  ErrorResponse
// @Failure      401      {object}  ErrorResponse
// @Failure      404      {object}  ErrorResponse
// @Failure      409      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Router       /cars/{id} [patch]
func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	idString := r.PathValue("id")

	id64, err := strconv.ParseInt(idString, 10, 64)
	if err != nil || id64 <= 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid ID",
		})
		return
	}
	id := uint(id64)

	var payload UpdateProductRequest

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid request body",
		})
		return
	}

	if payload.Name != nil {
		if err := h.validate.Struct(payload); err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{
				Error: h.formatValidationError(err),
			})
			return
		}
	}

	ctx := r.Context()
	product, err := h.service.UpdateProduct(
		ctx,
		repository.UpdateProductsInput{
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
		if errors.Is(err, repository.ErrProductAlreadyExists) {
			writeJSON(w, http.StatusConflict, ErrorResponse{
				Error: "duplicate product data",
			})
			return
		}
		if errors.Is(err, repository.ErrOptimisticLockFailed) {
			writeJSON(w, http.StatusConflict, ErrorResponse{
				Error: "product data locked",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error: "internal error",
		})
		return
	}

	item := ProductItem{
		ID:         product.ID,
		Name:       product.Name,
		PriceCents: product.PriceCents,
	}
	writeJSON(w, http.StatusOK, UpdateProductResponse{Item: item})
}

func (h *Handler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
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
	err = h.service.DeleteProduct(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrItemNotFound) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{
				Error: "product not found",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error: "internal error",
		})
		return
	}

	writeJSON(w, http.StatusOK, DeleteProductResponse{Message: "success"})
}
