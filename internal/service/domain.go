package service

import (
	"fmt"
	"math"

	"github.com/webitel/webitel-go-kit/pkg/errors"

	"github.com/webitel/im-gateway-service/infra/auth"
)

// domainIDInt32 narrows the caller's domain id to the int32 used by the im-* internal contracts.
// It fails instead of silently wrapping when the id does not fit.
func domainIDInt32(identity auth.Identifier) (int32, error) {
	domainID := identity.GetDomainID()
	if domainID < math.MinInt32 || domainID > math.MaxInt32 {
		return 0, errors.InvalidArgument(
			fmt.Sprintf("domain id %d is out of int32 range", domainID),
			errors.WithID("service.domain_id.out_of_range"),
		)
	}

	return int32(domainID), nil
}

// sizeInt32 converts a collection length to the int32 page size of the im-* contracts,
// clamping instead of wrapping when the length does not fit.
func sizeInt32(n int) int32 {
	if n < 0 {
		return 0
	}

	if n > math.MaxInt32 {
		return math.MaxInt32
	}

	return int32(n)
}
