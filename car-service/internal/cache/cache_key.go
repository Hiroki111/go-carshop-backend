package cache

import (
	"fmt"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/repository"
)

func CarCacheKey(id uint) string {
	return fmt.Sprintf("car:{%d}", id)
}

func CarListCacheKey(inputs repository.GetCarsInput) string {
	return fmt.Sprintf(
		"cars:o=%s:s=%s:n=%s:min=%d:max=%d:off=%d:lim=%d",
		inputs.OrderBy,
		inputs.SortIn,
		inputs.Name,
		inputs.MinPrice,
		inputs.MaxPrice,
		inputs.Offset,
		inputs.Limit,
	)
}
