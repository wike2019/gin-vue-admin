package initialize

import (
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/announcement"
	"github.com/flipped-aurora/gin-vue-admin/server/utils/plugin/v2"
	"github.com/gin-gonic/gin"
)

// PluginInitV2 插件初始化函数（v2版本）
// 设计原因：
// 1. 使用 Engine 而不是 RouterGroup，提供更底层的路由控制能力
// 2. v2 版本的插件接口更简洁，只需要实现 Register 方法，不需要 RouterPath 方法
// 3. 使用传统 for 循环而不是 range，保持代码风格一致性（虽然功能相同）
//
// 与 v1 版本的区别和优势：
// 1. 更灵活：Engine 提供了更底层的路由控制，可以注册全局中间件、404处理等
// 2. 更简洁：v2 接口不需要 RouterPath 方法，插件内部自行管理路由路径
// 3. 更强大：可以访问 Engine 的所有功能，如静态文件服务、模板渲染等
// 4. 适用场景：适合需要全局控制的插件（如公告系统，可能需要全局拦截器）
//
// 缺点：
// 1. 权限控制需要插件内部实现，无法直接利用 RouterGroup 的中间件机制
// 2. 路由管理更加分散，需要插件内部自行管理路由前缀
func PluginInitV2(group *gin.Engine, plugins ...plugin.Plugin) {
	for i := 0; i < len(plugins); i++ {
		// 直接将 Engine 传递给插件，让插件自行注册路由
		// 这种设计的好处是插件有更大的灵活性，可以注册任何类型的路由和中间件
		plugins[i].Register(group)
	}
}

// bizPluginV2 业务插件初始化函数（v2版本）
// 设计原因：
// 1. 接收 Engine 实例，用于注册需要全局控制的插件
// 2. 与 v1 版本分离，支持两套插件系统共存
//
// 好处：
// 1. 版本隔离：v1 和 v2 插件系统独立，不会相互影响，支持渐进式迁移
// 2. 灵活选择：可以根据插件需求选择合适的版本（需要权限控制用 v1，需要全局控制用 v2）
// 3. 向后兼容：保留旧版本插件系统，避免破坏性变更
//
// 当前示例：公告插件（announcement.Plugin）
// - 公告系统通常需要全局访问，适合使用 v2 版本
// - 可以直接在 Engine 层面注册路由和中间件
func bizPluginV2(engine *gin.Engine) {
	PluginInitV2(engine, announcement.Plugin)
}
