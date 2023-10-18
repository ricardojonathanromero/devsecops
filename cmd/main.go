package main

import (
	"context"
	"fmt"
	"github.com/ricardojonathanromero/devsecops/internal/repository"
	"os"
	"strconv"
	"sync"
)

const (
	totalClients = 30
)

func main() {
	const companyId = "TestCompanySL"

	// --- (0) ----
	// Recover implementation method
	argsWithoutProg := os.Args[1:]
	if len(argsWithoutProg) != 1 {
		panic("missing implementation method")
	}
	selectedImpl, err := strconv.Atoi(argsWithoutProg[0])
	if err != nil {
		panic(err)
	}
	repository.SelectedConcurrencyImplementation = repository.ConcurrencyImplementation(selectedImpl)
	switch repository.SelectedConcurrencyImplementation {
	case repository.NoConcurrency:
		fmt.Println(">> No Concurrency selected...")
	case repository.AtomicOperator:
		fmt.Println(">> Atomic Operator selected...")
	case repository.Transaction:
		fmt.Println(">> Transaction selected...")
	case repository.LUA:
		fmt.Println(">> LUA Script selected...")
	case repository.Lock:
		fmt.Println(">> Redis Locks selected...")
	default:
		panic("invalid implementation method selected")
	}

	// --- (1) ----
	// Get the redis config and init the repository
	repo := repository.NewRepository("0.0.0.0:58337")

	// --- (2) ----
	// Publish available shares
	err = repo.PublishShares(context.Background(), companyId, 100000)
	if err != nil {
		panic("error publishing shares => " + err.Error())
	}

	// --- (3) ----
	// Run concurrent clients that buy shares
	var wg sync.WaitGroup
	wg.Add(totalClients)

	for idx := 1; idx <= totalClients; idx++ {
		userId := fmt.Sprintf("user%d", idx)
		go func() {
			err := repo.BuyShares(context.Background(), userId, companyId, 100, &wg)
			if err != nil {
				fmt.Println("error but shares", err)
			}
		}()
	}
	wg.Wait()

	// --- (3) ----
	// Get the remaining company shares
	shares, err := repo.GetCompanyShares(context.Background(), companyId)
	if err != nil {
		panic(err)
	}
	fmt.Printf("the number of free shares the company %s has is: %d", companyId, shares)
}
