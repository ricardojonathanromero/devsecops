package repository

import "fmt"

func buildCompanySharesKey(companyId string) string {
	return fmt.Sprintf("shares:%s", companyId)
}
