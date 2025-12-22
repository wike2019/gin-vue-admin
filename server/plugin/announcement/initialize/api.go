package initialize

import (
	"context"
	model "github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/plugin-tool/utils"
)

// Api 注册插件的 API 到权限系统
// 设计模式：自动注册模式（Auto Registration Pattern） + 批量注册模式（Batch Registration Pattern）
// 好处：
// 1. 插件安装时自动注册 API 到权限系统，无需手动配置
// 2. 统一管理 API 权限，便于权限控制和审计
// 3. 支持 API 的自动发现和文档生成
// 4. 批量注册提高效率，减少数据库交互次数
// @param ctx context.Context 上下文，用于传递请求上下文信息（如超时、取消等）
func Api(ctx context.Context) {
	// 定义插件提供的所有 API 列表
	// 设计模式：配置即代码模式（Configuration as Code Pattern）
	// 好处：
	// 1. API 配置与代码在一起，便于版本控制和维护
	// 2. 支持代码自动生成，减少手动配置错误
	// 3. 每个 API 包含完整信息：路径、描述、分组、方法
	entities := []model.SysApi{
		{
			Path:        "/info/createInfo",
			Description: "新建公告",
			ApiGroup:    "公告",
			Method:      "POST",
		},
		{
			Path:        "/info/deleteInfo",
			Description: "删除公告",
			ApiGroup:    "公告",
			Method:      "DELETE",
		},
		{
			Path:        "/info/deleteInfoByIds",
			Description: "批量删除公告",
			ApiGroup:    "公告",
			Method:      "DELETE",
		},
		{
			Path:        "/info/updateInfo",
			Description: "更新公告",
			ApiGroup:    "公告",
			Method:      "PUT",
		},
		{
			Path:        "/info/findInfo",
			Description: "根据ID获取公告",
			ApiGroup:    "公告",
			Method:      "GET",
		},
		{
			Path:        "/info/getInfoList",
			Description: "获取公告列表",
			ApiGroup:    "公告",
			Method:      "GET",
		},
	}
	
	// 批量注册 API 到权限系统
	// 设计模式：工具函数模式（Utility Function Pattern）
	// 好处：
	// 1. 使用工具函数统一处理 API 注册逻辑
	// 2. 自动处理重复注册、更新等场景
	// 3. 支持事务处理，保证数据一致性
	// 4. 便于后续扩展（如 API 版本管理）
	utils.RegisterApis(entities...)
}
