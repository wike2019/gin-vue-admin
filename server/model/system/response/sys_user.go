package response

import (
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
)

// SysUserResponse System user response structure
type SysUserResponse struct {
	User system.SysUser `json:"user"`
}

// LoginResponse Login response structure
type LoginResponse struct {
	User      system.SysUser `json:"user"`
	Token     string         `json:"token"`
	ExpiresAt int64          `json:"expiresAt"`
}
