package response

import "github.com/flipped-aurora/gin-vue-admin/server/model/example"

// ExaCustomerResponse Customer response structure
type ExaCustomerResponse struct {
	Customer example.ExaCustomer `json:"customer"`
}
