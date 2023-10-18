package ports

import (
	"context"
	"sync"
)

type Repository interface {
	BuyShares(ctx context.Context, buyerId, companyId string, numShares int, wg *sync.WaitGroup) error
	GetCompanyShares(ctx context.Context, companyId string) (int, error)
	PublishShares(ctx context.Context, companyId string, numShares int) error
}
