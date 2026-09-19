package cache

import (
	"fmt"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/repository"
)

func ProductCacheKey(id uint) string {
	return fmt.Sprintf("product:{%d}", id)
}

func ProductListCacheKey(inputs repository.GetProductsInput) string {
	return fmt.Sprintf(
		"products:o=%s:s=%s:n=%s:min=%d:max=%d:off=%d:lim=%d",
		inputs.OrderBy,
		inputs.SortIn,
		inputs.Name,
		inputs.MinPrice,
		inputs.MaxPrice,
		inputs.Offset,
		inputs.Limit,
	)
}
