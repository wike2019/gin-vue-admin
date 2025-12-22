package response

import "github.com/flipped-aurora/gin-vue-admin/server/model/system"

// SysAuthorityResponse System authority response structure
type SysAuthorityResponse struct {
	Authority system.SysAuthority `json:"authority"`
}

// SysAuthorityCopyResponse System authority copy response structure
type SysAuthorityCopyResponse struct {
	Authority      system.SysAuthority `json:"authority"`
	OldAuthorityId uint                `json:"oldAuthorityId"` // 旧角色ID
}
