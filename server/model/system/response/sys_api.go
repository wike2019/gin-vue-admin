package response

import (
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
)

// SysAPIResponse System API response structure
type SysAPIResponse struct {
	Api system.SysApi `json:"api"`
}

// SysAPIListResponse System API list response structure
type SysAPIListResponse struct {
	Apis []system.SysApi `json:"apis"`
}

// SysSyncApis System sync APIs response structure
type SysSyncApis struct {
	NewApis    []system.SysApi `json:"newApis"`
	DeleteApis []system.SysApi `json:"deleteApis"`
}
