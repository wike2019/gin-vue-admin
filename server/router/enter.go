package router

import (
	"github.com/flipped-aurora/gin-vue-admin/server/router/example"
	"github.com/flipped-aurora/gin-vue-admin/server/router/system"
)

// RouterGroupApp 是路由组的全局统一入口
// 为什么使用全局变量？
// - 单例模式：整个应用只需要一个路由组实例，避免重复创建
// - 统一访问：所有模块通过同一个入口访问路由组，代码更清晰
// - 便于扩展：新增路由模块时只需在 RouterGroup 结构体中添加字段
// - 初始化简单：使用 new() 创建实例，自动初始化所有嵌套的路由组
var RouterGroupApp = new(RouterGroup)

// RouterGroup 是路由组的组合结构体，采用组合模式设计
// 为什么使用组合模式而不是继承？
// - Go 语言不支持继承，组合是 Go 推荐的代码复用方式
// - 职责分离：每个子路由组（System、Example）管理自己的路由逻辑
// - 模块化设计：不同功能模块的路由相互独立，便于维护和测试
// - 可扩展性：新增功能模块时只需添加新的字段，不影响现有代码
//
// 为什么将路由分组？
// - 代码组织：按功能模块组织路由，避免单个文件过大
// - 团队协作：不同团队可以负责不同的路由模块，减少代码冲突
// - 维护性：修改某个模块的路由时，只需关注对应的子路由组
// - 可读性：通过 RouterGroupApp.System 或 RouterGroupApp.Example 访问，语义清晰
//
// 设计优势：
//  1. 模块化：System 路由组管理系统核心功能（用户、权限、菜单等）
//     Example 路由组管理示例功能（文件上传、客户管理等）
//  2. 解耦：各路由组之间相互独立，修改一个不影响另一个
//  3. 统一管理：通过 RouterGroupApp 统一访问，避免分散的路由注册代码
//  4. 易于测试：可以单独测试每个路由组的功能
type RouterGroup struct {
	System  system.RouterGroup  // 系统核心功能路由组（用户管理、权限管理、菜单管理等）
	Example example.RouterGroup // 示例功能路由组（文件上传、客户管理等）
}
