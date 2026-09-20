package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"testing"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/config"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/handler"
)

func TestGetProducts_WithSorting(t *testing.T) {
	products := []domain.Product{
		{Name: "apple", PriceCents: 100},
		{Name: "banana", PriceCents: 300},
		{Name: "cherry", PriceCents: 200},
	}

	tests := []struct {
		orderBy, sortIn             string
		expectedProductNamesInOrder []string
	}{
		{orderBy: "name", sortIn: "asc", expectedProductNamesInOrder: []string{"apple", "banana", "cherry"}},
		{orderBy: "name", sortIn: "desc", expectedProductNamesInOrder: []string{"cherry", "banana", "apple"}},
		{orderBy: "name", sortIn: "", expectedProductNamesInOrder: []string{"apple", "banana", "cherry"}},
		{orderBy: "price_cents", sortIn: "asc", expectedProductNamesInOrder: []string{"apple", "cherry", "banana"}},
		{orderBy: "price_cents", sortIn: "desc", expectedProductNamesInOrder: []string{"banana", "cherry", "apple"}},
		{orderBy: "price_cents", sortIn: "", expectedProductNamesInOrder: []string{"apple", "cherry", "banana"}},
		{orderBy: "created_at", sortIn: "asc", expectedProductNamesInOrder: []string{"apple", "banana", "cherry"}},
		{orderBy: "created_at", sortIn: "desc", expectedProductNamesInOrder: []string{"cherry", "banana", "apple"}},
		{orderBy: "created_at", sortIn: "", expectedProductNamesInOrder: []string{"apple", "banana", "cherry"}},
	}

	for _, test := range tests {
		testName := fmt.Sprintf("order by %s", test.orderBy)
		path := fmt.Sprintf("/cars?orderBy=%s", test.orderBy)
		if test.sortIn != "" {
			testName += fmt.Sprintf(", sorted in %s order", test.sortIn)
			path += fmt.Sprintf("&sortIn=%s", test.sortIn)
		}

		t.Run(testName, func(t *testing.T) {
			app, db := setupTestApp(t)
			seedProducts(t, db, products)

			rec := executeRequest(t, app, http.MethodGet, path, nil)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
			}

			var resp handler.GetProductsResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("invalid json response")
			}

			actualNames := make([]string, 0, len(resp.Items))
			for _, item := range resp.Items {
				actualNames = append(actualNames, item.Name)
			}

			if !reflect.DeepEqual(test.expectedProductNamesInOrder, actualNames) {
				t.Fatalf("expected %v, got %v", test.expectedProductNamesInOrder, actualNames)
			}
		})
	}
}

func TestGetProducts_WithFilteringByName(t *testing.T) {
	products := []domain.Product{
		{Name: "apple"},
		{Name: "banana"},
		{Name: "cherry"},
	}

	tests := []struct {
		name                 string
		keyword              string
		expectedProductNames []string
	}{
		{name: "Matching one word", keyword: "ap", expectedProductNames: []string{"apple"}},
		{name: "Matching one word - case insensitive", keyword: "Ap", expectedProductNames: []string{"apple"}},
		{name: "Matching multiple words", keyword: "a", expectedProductNames: []string{"apple", "banana"}},
		{name: "Matching nothing", keyword: "aa", expectedProductNames: []string{}},
		{name: "Empty keyword", keyword: "", expectedProductNames: []string{"apple", "banana", "cherry"}},
	}

	for _, test := range tests {
		path := fmt.Sprintf("/cars?name=%s", test.keyword)

		t.Run(test.name, func(t *testing.T) {
			app, db := setupTestApp(t)
			seedProducts(t, db, products)

			rec := executeRequest(t, app, http.MethodGet, path, nil)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
			}

			var resp handler.GetProductsResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("invalid json response")
			}

			if len(test.expectedProductNames) != len(resp.Items) {
				t.Fatalf("expected %d items, got %d", len(test.expectedProductNames), len(resp.Items))
			}

			actualNames := make([]string, 0, len(resp.Items))
			for _, item := range resp.Items {
				actualNames = append(actualNames, item.Name)
			}
			sort.Strings(actualNames)
			sort.Strings(test.expectedProductNames)

			if !reflect.DeepEqual(actualNames, test.expectedProductNames) {
				t.Fatalf("expected products %v, got %v", test.expectedProductNames, actualNames)
			}
		})
	}
}

func TestGetProducts_WithFilteringByPrice(t *testing.T) {
	products := []domain.Product{
		{Name: "$1.00 Product", PriceCents: 100},
		{Name: "$1.50 Product", PriceCents: 150},
		{Name: "$2.00 Product", PriceCents: 200},
	}

	tests := []struct {
		name                 string
		minPrice, maxPrice   string
		expectedProductNames []string
		expectedCode         int
	}{
		{name: "Matching items", minPrice: "100", maxPrice: "160", expectedProductNames: []string{"$1.00 Product", "$1.50 Product"}, expectedCode: http.StatusOK},
		{name: "Matching items without minPrice", minPrice: "", maxPrice: "150", expectedProductNames: []string{"$1.00 Product", "$1.50 Product"}, expectedCode: http.StatusOK},
		{name: "Matching items without maxPrice", minPrice: "150", maxPrice: "", expectedProductNames: []string{"$1.50 Product", "$2.00 Product"}, expectedCode: http.StatusOK},
		{name: "Matching items without minPrice and maxPrice", minPrice: "", maxPrice: "", expectedProductNames: []string{"$1.00 Product", "$1.50 Product", "$2.00 Product"}, expectedCode: http.StatusOK},
		{name: "Matching no item when minPrice is larger than maxPrice", minPrice: "200", maxPrice: "100", expectedProductNames: []string{}, expectedCode: http.StatusOK},
		{name: "Bad request with invalid price", minPrice: "abc", maxPrice: "200", expectedProductNames: []string{}, expectedCode: http.StatusBadRequest},
	}

	for _, test := range tests {
		path := fmt.Sprintf("/cars?minPrice=%s&maxPrice=%s", test.minPrice, test.maxPrice)

		t.Run(test.name, func(t *testing.T) {
			app, db := setupTestApp(t)
			seedProducts(t, db, products)

			rec := executeRequest(t, app, http.MethodGet, path, nil)

			if rec.Code != test.expectedCode {
				t.Fatalf("expected %d, got %d", test.expectedCode, rec.Code)
			}

			if test.expectedCode == http.StatusOK {
				var resp handler.GetProductsResponse
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("invalid json response")
				}

				if len(test.expectedProductNames) != len(resp.Items) {
					t.Fatalf("expected %d items, got %d", len(test.expectedProductNames), len(resp.Items))
				}

				actualNames := make([]string, 0, len(resp.Items))
				for _, item := range resp.Items {
					actualNames = append(actualNames, item.Name)
				}
				sort.Strings(actualNames)
				sort.Strings(test.expectedProductNames)

				if !reflect.DeepEqual(actualNames, test.expectedProductNames) {
					t.Fatalf("expected products %v, got %v", test.expectedProductNames, actualNames)
				}
			}
		})
	}
}

func TestGetProducts_WithPagination(t *testing.T) {
	products := make([]domain.Product, 100)
	for i := range products {
		products[i] = domain.Product{Name: strconv.Itoa(i)}
	}

	tests := []struct {
		name                                  string
		page, limit                           string
		expectedItemCount, expectedTotalCount int
		expectedFirstItem                     string
		expectedHasNext                       bool
		expectedCode                          int
	}{
		{
			name: "Use blank page and blank limit",
			page: "", limit: "",
			expectedItemCount: 20, expectedTotalCount: 100, expectedFirstItem: "0", expectedHasNext: true, expectedCode: http.StatusOK,
		},
		{
			name: "Use page and limit",
			page: "2", limit: "5",
			expectedItemCount: 5, expectedTotalCount: 100, expectedFirstItem: "5", expectedHasNext: true, expectedCode: http.StatusOK,
		},
		{
			name: "Use blank page and limit",
			page: "", limit: "5",
			expectedItemCount: 5, expectedTotalCount: 100, expectedFirstItem: "0", expectedHasNext: true, expectedCode: http.StatusOK,
		},
		{
			name: "Use page and blank limit",
			page: "2", limit: "",
			expectedItemCount: 20, expectedTotalCount: 100, expectedFirstItem: "20", expectedHasNext: true, expectedCode: http.StatusOK,
		},
		{
			name: "Page offset exceeds total count",
			page: "2", limit: "101",
			expectedItemCount: 0, expectedTotalCount: 100, expectedCode: http.StatusOK,
		},
		{
			name: "Use non-numeric page",
			page: "abc", limit: "", expectedCode: http.StatusBadRequest,
		},
		{
			name: "Use non-numeric limit",
			page: "", limit: "abc", expectedCode: http.StatusBadRequest,
		},
		{
			name: "Use limit that exceeds the limit",
			page: "", limit: strconv.Itoa(config.MaxPageLimit + 1), expectedCode: http.StatusBadRequest,
		},
		{
			name: "Use page 0",
			page: "0", limit: "", expectedCode: http.StatusBadRequest,
		},
		{
			name: "Use limit 0",
			page: "", limit: "0", expectedCode: http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		path := fmt.Sprintf("/cars?page=%s&limit=%s", test.page, test.limit)
		t.Run(test.name, func(t *testing.T) {
			app, db := setupTestApp(t)
			seedProducts(t, db, products)

			rec := executeRequest(t, app, http.MethodGet, path, nil)

			if rec.Code != test.expectedCode {
				t.Fatalf("expected %d, got %d", test.expectedCode, rec.Code)
			}

			if test.expectedCode == http.StatusOK {
				var resp handler.GetProductsResponse
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("invalid json response")
				}

				if test.expectedItemCount != len(resp.Items) {
					t.Fatalf("expected %d items, got %d", test.expectedItemCount, len(resp.Items))
				}

				if test.expectedTotalCount != resp.Total {
					t.Fatalf("expected %d total items, got %d", test.expectedTotalCount, resp.Total)
				}

				if test.expectedFirstItem != "" {
					if resp.Items[0].Name != test.expectedFirstItem {
						t.Fatalf("expected first item %s, got %s", test.expectedFirstItem, resp.Items[0].Name)
					}
				}

				if test.expectedHasNext != resp.HasNext {
					t.Fatalf("expected HasNext %v, got %v", test.expectedHasNext, resp.HasNext)
				}
			}
		})
	}
}

func TestGetProduct_ById(t *testing.T) {
	tests := []struct {
		name         string
		getId        func(t *testing.T, products []domain.Product) string
		expectedCode int
	}{
		{
			name: "Product found",
			getId: func(t *testing.T, products []domain.Product) string {
				return fmt.Sprint(products[0].ID)
			},
			expectedCode: http.StatusOK,
		},
		{
			name: "Product not found",
			getId: func(t *testing.T, products []domain.Product) string {
				return fmt.Sprint(products[0].ID + 1)
			},
			expectedCode: http.StatusNotFound,
		},
		{
			name: "Non-numeric ID",
			getId: func(t *testing.T, products []domain.Product) string {
				return "abc"
			},
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, db := setupTestApp(t)

			products := seedProducts(t, db, []domain.Product{{Name: "test"}})
			id := test.getId(t, products)
			path := fmt.Sprintf("/cars/%s", id)

			rec := executeRequest(t, app, http.MethodGet, path, nil)

			if rec.Code != test.expectedCode {
				t.Fatalf("expected %d, got %d", test.expectedCode, rec.Code)
			}

			if test.expectedCode == http.StatusOK {
				var resp handler.GetProductResponse
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("invalid json response")
				}

				expectedId, _ := strconv.ParseUint(id, 10, 64)
				if resp.Item.ID != uint(expectedId) {
					t.Fatalf("expected ID %d, got %d", expectedId, resp.Item.ID)
				}

				if resp.Item.Name != "test" {
					t.Fatalf("expected name 'test', got %s", resp.Item.Name)
				}
			}
		})
	}
}

func TestCreateProduct(t *testing.T) {
	type Palyload struct {
		Name       string `json:"name"`
		PriceCents int64  `json:"price_cents"`
	}

	tests := []struct {
		testName            string
		palyload            Palyload
		expectedCode        int
		expectedProductName string
	}{
		{
			testName:            "Product created",
			palyload:            Palyload{Name: "test", PriceCents: 1},
			expectedCode:        http.StatusCreated,
			expectedProductName: "test",
		},
		{
			testName:            "Product created with trimmed name",
			palyload:            Palyload{Name: "   test   ", PriceCents: 1},
			expectedCode:        http.StatusCreated,
			expectedProductName: "test",
		},
		{
			testName:     "Failed to create product - blank name",
			palyload:     Palyload{Name: "   ", PriceCents: 1},
			expectedCode: http.StatusBadRequest,
		},
		{
			testName:     "Failed to create product - negative price",
			palyload:     Palyload{Name: "test", PriceCents: -1},
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			app, db := setupTestApp(t)

			payload := Palyload{
				Name:       test.palyload.Name,
				PriceCents: test.palyload.PriceCents,
			}

			rec := executeRequest(t, app, http.MethodPost, "/cars", payload)

			if rec.Code != test.expectedCode {
				t.Fatalf("expected code %d, got %d", test.expectedCode, rec.Code)
			}

			if rec.Code == http.StatusCreated {
				var resp handler.CreateProductResponse
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if resp.Item.Name != test.expectedProductName {
					t.Fatalf("expected name in response %s, got %s", test.expectedProductName, resp.Item.Name)
				}

				var createdProduct domain.Product
				if err := db.First(&createdProduct, resp.Item.ID).Error; err != nil {
					t.Fatalf("product with ID %d not found in DB", resp.Item.ID)
				}

				if createdProduct.Name != test.expectedProductName {
					t.Fatalf("expected name in DB %s, got %s", test.expectedProductName, createdProduct.Name)
				}
			}
		})
	}
}

func TestUpdateProduct(t *testing.T) {
	type Payload struct {
		Name       *string `json:"name"`
		PriceCents *int64  `json:"price_cents"`
	}
	const currentProductName = "current product name"
	const updatedProductName = "this is the new name"
	const unavailableProductName = "this name is taken"

	tests := []struct {
		testName            string
		payload             Payload
		hasValidId          bool
		expectedCode        int
		expectedProductName string
	}{
		{
			testName:            "success - full update",
			payload:             Payload{Name: strPtr(updatedProductName), PriceCents: int64Ptr(10)},
			hasValidId:          true,
			expectedCode:        http.StatusOK,
			expectedProductName: updatedProductName,
		},
		{
			testName:            "success - update name only",
			payload:             Payload{Name: strPtr(updatedProductName)},
			hasValidId:          true,
			expectedCode:        http.StatusOK,
			expectedProductName: updatedProductName,
		},
		{
			testName:            "success - name is trimmed",
			payload:             Payload{Name: strPtr(" " + updatedProductName + " ")},
			hasValidId:          true,
			expectedCode:        http.StatusOK,
			expectedProductName: updatedProductName,
		},
		{
			testName:     "success - update price_cents only",
			payload:      Payload{PriceCents: int64Ptr(10)},
			hasValidId:   true,
			expectedCode: http.StatusOK,
		},
		{
			testName:     "success - empty payload",
			payload:      Payload{},
			hasValidId:   true,
			expectedCode: http.StatusOK,
		},

		{
			testName:     "fail - invalid ID",
			payload:      Payload{Name: strPtr(updatedProductName), PriceCents: int64Ptr(10)},
			hasValidId:   false,
			expectedCode: http.StatusNotFound,
		},
		{
			testName:     "fail - duplicate name",
			payload:      Payload{Name: strPtr(unavailableProductName), PriceCents: int64Ptr(10)},
			hasValidId:   true,
			expectedCode: http.StatusConflict,
		},
		{
			testName:     "fail - whitespace-only name",
			payload:      Payload{Name: strPtr("   "), PriceCents: int64Ptr(10)},
			hasValidId:   true,
			expectedCode: http.StatusBadRequest,
		},
		{
			testName:     "fail - negative price_cents",
			payload:      Payload{Name: strPtr(updatedProductName), PriceCents: int64Ptr(-10)},
			hasValidId:   true,
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			app, db := setupTestApp(t)

			currentProduct := domain.Product{Name: currentProductName, PriceCents: 5}
			anotherProduct := domain.Product{Name: unavailableProductName, PriceCents: 5}
			db.Create(&currentProduct)
			db.Create(&anotherProduct)

			var path string
			if test.hasValidId {
				path = fmt.Sprintf("/cars/%d", currentProduct.ID)
			} else {
				path = fmt.Sprintf("/cars/%d", currentProduct.ID+anotherProduct.ID)
			}

			rec := executeRequest(t, app, http.MethodPatch, path, test.payload)

			if rec.Code != test.expectedCode {
				t.Fatalf("expected code %d, got %d", test.expectedCode, rec.Code)
			}

			if rec.Code == http.StatusOK {
				var resp handler.UpdateProductResponse
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				var updated domain.Product
				db.First(&updated, currentProduct.ID)

				if test.payload.Name != nil {
					if updated.Name != test.expectedProductName {
						t.Fatalf("expected name in DB %s, got %s", test.expectedProductName, updated.Name)
					}
					if resp.Item.Name != test.expectedProductName {
						t.Fatalf("expected name in response %s, got %s", test.expectedProductName, resp.Item.Name)
					}
				} else {
					if updated.Name != currentProduct.Name {
						t.Fatalf("expected name %s, got %s", currentProduct.Name, updated.Name)
					}
				}

				if test.payload.PriceCents != nil {
					if updated.PriceCents != uint(*test.payload.PriceCents) {
						t.Fatalf("expected price_cents %d, got %v", *test.payload.PriceCents, updated.PriceCents)
					}
				} else {
					if updated.PriceCents != currentProduct.PriceCents {
						t.Fatalf("expected price_cents %d, got %v", currentProduct.PriceCents, updated.PriceCents)
					}
				}
			}
		})
	}
}

func TestDeleteProduct(t *testing.T) {
	tests := []struct {
		testName     string
		hasValidId   bool
		expectedCode int
	}{
		{
			testName:     "success",
			hasValidId:   true,
			expectedCode: http.StatusOK,
		},
		{
			testName:     "fail - invalid ID",
			hasValidId:   false,
			expectedCode: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			app, db := setupTestApp(t)

			product := domain.Product{Name: "test", PriceCents: 5}
			db.Create(&product)

			var path string
			if test.hasValidId {
				path = fmt.Sprintf("/cars/%d", product.ID)
			} else {
				path = fmt.Sprintf("/cars/%d", product.ID+1)
			}

			rec := executeRequest(t, app, http.MethodDelete, path, nil)

			if rec.Code != test.expectedCode {
				t.Fatalf("expected code %d, got %d", test.expectedCode, rec.Code)
			}

			if test.expectedCode == http.StatusOK {
				if err := db.First(&domain.Product{}, product.ID).Error; err == nil {
					t.Fatalf("product ID %d was not deleted", product.ID)
				}
			}
		})
	}
}
