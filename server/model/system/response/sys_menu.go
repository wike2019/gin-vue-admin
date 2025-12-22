package response

import "github.com/flipped-aurora/gin-vue-admin/server/model/system"

// SysMenusResponse System menus response structure
type SysMenusResponse struct {
	Menus []system.SysMenu `json:"menus"`
}

// SysBaseMenusResponse System base menus response structure
type SysBaseMenusResponse struct {
	Menus []system.SysBaseMenu `json:"menus"`
}

// SysBaseMenuResponse System base menu response structure
type SysBaseMenuResponse struct {
	Menu system.SysBaseMenu `json:"menu"`
}
