package repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	goredislib "github.com/redis/go-redis/v9"
	"github.com/ricardojonathanromero/devsecops/internal/ports"
	"sync"
)

type ConcurrencyImplementation int

const (
	NoConcurrency ConcurrencyImplementation = iota
	AtomicOperator
	Transaction
	LUA
	Lock
)

var SelectedConcurrencyImplementation ConcurrencyImplementation

type repoImpl struct {
	client *goredislib.Client
	mutex  *redsync.Mutex
}

var _ ports.Repository = (*repoImpl)(nil)

func NewRepository(address string) ports.Repository {
	client := goredislib.NewClient(&goredislib.Options{
		Addr: address,
	})

	pool := goredis.NewPool(client)
	rs := redsync.New(pool)
	mutexname := "my-global-mutex"
	mutex := rs.NewMutex(mutexname)

	return &repoImpl{
		client: client,
		mutex:  mutex,
	}
}

func (r *repoImpl) BuyShares(ctx context.Context, userId, companyId string, numShares int, wg *sync.WaitGroup) error {

	defer wg.Done()

	switch SelectedConcurrencyImplementation {
	case NoConcurrency:
		return r.buySharesNoConcurrencyControl(ctx, userId, companyId, numShares)
	case AtomicOperator:
		return r.buySharesWithAtomicIncr(ctx, userId, companyId, numShares)
	case Transaction:
		return r.buySharesWithTransactions(ctx, userId, companyId, numShares)
	case LUA:
		return r.buySharesWithLUAScript(ctx, userId, companyId, numShares)
	case Lock:
		return r.buySharesWithRedisLock(ctx, userId, companyId, numShares)
	default:
		panic("invalid implementation method selected")
	}

}

func (r *repoImpl) buySharesWithTransactions(ctx context.Context, userId, companyId string, numShares int) error {
	companySharesKey := buildCompanySharesKey(companyId)

	err := r.client.Watch(ctx, func(tx *goredislib.Tx) error {
		// --- (1) ----
		// Get current number of shares
		currentShares, err := tx.Get(ctx, companySharesKey).Int()
		if err != nil {
			fmt.Print(fmt.Errorf("error getting value %v", err.Error()))
			return err
		}

		// --- (2) ----
		// Validate if the shares remaining are enough to be bought
		if currentShares < numShares {
			fmt.Print("error: company does not have enough shares \n")
			return errors.New("error: company does not have enough shares")
		}
		currentShares -= numShares

		// --- (3) ----
		// Update the current shares of the company and log who has bought shares
		_, err = tx.TxPipelined(ctx, func(pipe goredislib.Pipeliner) error {
			// pipe handles the error case
			pipe.Pipeline().Set(ctx, companySharesKey, currentShares, 0)
			return nil
		})
		if err != nil {
			fmt.Println(fmt.Errorf("error in pipeline %v", err.Error()))
			return err
		}
		return nil
	}, companySharesKey)
	return err
}

func (r *repoImpl) buySharesNoConcurrencyControl(ctx context.Context, userId, companyId string, numShares int) error {
	// --- (1) ----
	// Get current number of shares
	key := buildCompanySharesKey(companyId)
	fmt.Println("company_key [buy shares]", key)
	currentShares, err := r.client.Get(ctx, key).Int()
	if err != nil {
		fmt.Print(err.Error())
		return err
	}

	// --- (2) ----
	// Validate if the shares remaining are enough to be bought
	fmt.Println("current vs num shares [buy shares]", currentShares, numShares)
	if currentShares < numShares {
		fmt.Print("error: company does not have enough shares \n")
		return errors.New("error: company does not have enough shares")
	}
	currentShares -= numShares

	fmt.Println("current shares [buy shares]", currentShares)
	// --- (3) ----
	// Update the current shares of the company and log who has bought shares
	r.client.Set(ctx, key, currentShares, 0)
	return nil
}

func (r *repoImpl) buySharesWithAtomicIncr(ctx context.Context, userId, companyId string, numShares int) error {
	// --- (1) ----
	// Get current number of shares
	currentShares, err := r.client.Get(ctx, buildCompanySharesKey(companyId)).Int()
	if err != nil {
		fmt.Print(err.Error())
		return err
	}

	// --- (2) ----
	// Validate if the shares remaining are enough to be bought
	if currentShares < numShares {
		fmt.Print("error: company does not have enough shares \n")
		return errors.New("error: company does not have enough shares")
	}

	// --- (3) ----
	// Update the current shares of the company and log who has bought shares
	r.client.IncrBy(ctx, buildCompanySharesKey(companyId), -1*int64(numShares))
	return nil
}

var BuyShares = goredislib.NewScript(`
local sharesKey = KEYS[1]
local requestedShares = ARGV[1]

local currentShares = redis.call("GET", sharesKey)
if currentShares < requestedShares then
	return {err = "error: company does not have enough shares"}
end

currentShares = currentShares - requestedShares
redis.call("SET", sharesKey, currentShares)
`)

func (r *repoImpl) buySharesWithLUAScript(ctx context.Context, userId, companyId string, numShares int) error {
	keys := []string{buildCompanySharesKey(companyId)}
	err := BuyShares.Run(ctx, r.client, keys, numShares).Err()
	if err != nil {
		fmt.Println(err.Error())
	}
	return err
}

func (r *repoImpl) buySharesWithRedisLock(ctx context.Context, userId, companyId string, numShares int) error {

	// Obtain a lock for our given mutex. After this is successful, no one else
	// can obtain the same lock (the same mutex name) until we unlock it.
	if err := r.mutex.Lock(); err != nil {
		fmt.Printf("error during lock: %v \n", err)
	}

	defer func() {
		if ok, err := r.mutex.Unlock(); !ok || err != nil {
			fmt.Printf("error during unlock: %v \n", err)
		}
	}()

	// --- (1) ----
	// Get current number of shares
	currentShares, err := r.client.Get(ctx, buildCompanySharesKey(companyId)).Int()
	if err != nil {
		fmt.Print(err.Error())
		return err
	}

	// --- (2) ----
	// Validate if the shares remaining are enough to be bought
	if currentShares < numShares {
		fmt.Print("error: company does not have enough shares \n")
		return errors.New("error: company does not have enough shares")
	}
	currentShares -= numShares

	// --- (3) ----
	// Update the current shares of the company and log who has bought shares
	r.client.Set(ctx, buildCompanySharesKey(companyId), currentShares, 0)

	return nil
}

func (r *repoImpl) GetCompanyShares(ctx context.Context, companyId string) (int, error) {
	key := buildCompanySharesKey(companyId)
	fmt.Println("company key [get publish shares]", key)
	result := r.client.Get(ctx, key)
	currentShares, err := result.Int()
	if err != nil {
		return 0, err
	}
	return currentShares, nil
}

func (r *repoImpl) PublishShares(ctx context.Context, companyId string, numShares int) error {
	key := buildCompanySharesKey(companyId)
	fmt.Println("company key [publish shares]", key)
	status := r.client.Set(ctx, key, numShares, 0)
	return status.Err()
}
