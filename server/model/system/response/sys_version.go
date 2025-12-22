package response

import (
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
)

// ExportVersionResponse Export version response structure
type ExportVersionResponse struct {
	Version      request.VersionInfo    `json:"version"`      // 版本信息
	Menus        []system.SysBaseMenu   `json:"menus"`        // 菜单数据，直接复用SysBaseMenu
	Apis         []system.SysApi        `json:"apis"`         // API数据，直接复用SysApi
	Dictionaries []system.SysDictionary `json:"dictionaries"` // 字典数据，直接复用SysDictionary
}
