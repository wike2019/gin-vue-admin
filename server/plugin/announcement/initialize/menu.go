package initialize

import (
	"context"
	model "github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/plugin-tool/utils"
)

// Menu 注册插件的菜单到系统
// 设计模式：自动注册模式（Auto Registration Pattern） + 批量注册模式（Batch Registration Pattern）
// 好处：
// 1. 插件安装时自动注册菜单，用户可以直接使用
// 2. 统一管理菜单结构，便于前端路由配置
// 3. 支持菜单的权限控制和动态显示
// 4. 批量注册提高效率，减少数据库交互次数
// @param ctx context.Context 上下文，用于传递请求上下文信息（如超时、取消等）
func Menu(ctx context.Context) {
	// 定义插件提供的所有菜单列表
	// 设计模式：配置即代码模式（Configuration as Code Pattern）
	// 好处：
	// 1. 菜单配置与代码在一起，便于版本控制和维护
	// 2. 支持代码自动生成，减少手动配置错误
	// 3. 每个菜单包含完整信息：父菜单、路径、组件、排序等
	entities := []model.SysBaseMenu{
		{
			// ParentId 父菜单ID，用于构建菜单树结构
			// 设计模式：树形结构模式（Tree Structure Pattern）
			// 好处：
			// 1. 通过 ParentId 构建菜单层级关系
			// 2. 支持多级菜单，便于组织复杂的菜单结构
			// 3. 便于菜单的权限控制和显示控制
			ParentId: 24,
			
			// Path 路由路径，用于前端路由配置
			// 好处：与前端路由配置对应，实现前后端路由统一
			Path: "anInfo",
			
			// Name 路由名称，用于前端路由导航
			// 好处：唯一标识路由，便于前端路由匹配和权限控制
			Name: "anInfo",
			
			// Hidden 是否隐藏菜单
			// 好处：支持菜单的动态显示和隐藏，便于权限控制
			Hidden: false,
			
			// Component 前端组件路径
			// 设计模式：组件路径模式（Component Path Pattern）
			// 好处：
			// 1. 指定前端组件路径，实现前后端分离
			// 2. 使用相对路径，便于组件管理和维护
			// 3. 支持动态加载组件，提高前端性能
			Component: "plugin/announcement/view/info.vue",
			
			// Sort 菜单排序，用于控制菜单显示顺序
			// 好处：支持自定义菜单顺序，便于用户体验优化
			Sort: 5,
			
			// Meta 菜单元信息，包含标题、图标等
			// 设计模式：元数据模式（Metadata Pattern）
			// 好处：
			// 1. 集中管理菜单的显示信息（标题、图标等）
			// 2. 便于前端统一渲染菜单
			// 3. 支持国际化，便于多语言支持
			Meta: model.Meta{
				Title: "公告管理", // 菜单标题，显示在界面上
				Icon:  "box",     // 菜单图标，使用图标库中的图标名称
			},
		},
	}
	
	// 批量注册菜单到系统
	// 设计模式：工具函数模式（Utility Function Pattern）
	// 好处：
	// 1. 使用工具函数统一处理菜单注册逻辑
	// 2. 自动处理重复注册、更新等场景
	// 3. 支持事务处理，保证数据一致性
	// 4. 便于后续扩展（如菜单权限管理）
	utils.RegisterMenus(entities...)
}
