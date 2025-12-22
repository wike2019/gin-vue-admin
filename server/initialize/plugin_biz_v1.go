package initialize

import (
	"fmt"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/email"
	"github.com/flipped-aurora/gin-vue-admin/server/utils/plugin"
	"github.com/gin-gonic/gin"
)

// PluginInit 插件初始化函数（v1版本）
// 设计原因：
// 1. 使用 RouterGroup 作为参数，而不是 Engine，因为 RouterGroup 可以应用中间件进行权限控制
// 2. 支持可变参数，可以一次性注册多个插件，提高灵活性
// 3. 为每个插件创建独立的子路由组，避免路由冲突
//
// 好处：
// 1. 权限控制：通过 RouterGroup 的中间件机制，可以轻松实现基于角色的权限控制（RBAC）
// 2. 路由隔离：每个插件拥有独立的路由前缀，避免路由冲突，提高代码组织性
// 3. 可扩展性：使用接口抽象，支持任何实现了 Plugin 接口的插件，符合开闭原则
// 4. 可观测性：输出日志信息，方便调试和监控插件注册过程
// 5. 解耦：插件注册逻辑与业务逻辑分离，符合单一职责原则
func PluginInit(group *gin.RouterGroup, Plugin ...plugin.Plugin) {
	for i := range Plugin {
		fmt.Println(Plugin[i].RouterPath(), "注册开始!")
		// 为每个插件创建独立的路由组，路由路径由插件的 RouterPath() 方法返回
		// 这样设计的好处：
		// - 每个插件可以有自己的路由前缀（如 /email、/upload 等）
		// - 可以为每个路由组单独配置中间件（如权限验证、限流等）
		PluginGroup := group.Group(Plugin[i].RouterPath())
		// 调用插件的注册方法，让插件在自己的路由组中注册路由
		Plugin[i].Register(PluginGroup)
		fmt.Println(Plugin[i].RouterPath(), "注册成功!")
	}
}

// bizPluginV1 业务插件初始化函数（v1版本）
// 设计原因：
// 1. 使用可变参数接收 RouterGroup，第一个是私有路由组（需要权限验证），第二个是公有路由组（无需权限验证）
// 2. 将插件初始化逻辑集中管理，便于维护和扩展
// 3. 调用 holder 函数确保相关包被正确导入（避免 Go 编译器的空变量检测报错）
//
// 好处：
// 1. 权限分离：通过 private 和 public 两个路由组，可以清晰地区分需要权限和不需要权限的插件
//   - private：需要登录和权限验证的插件（如邮件发送，需要管理员权限）
//   - public：公开访问的插件（如公告查询，所有人都可以访问）
//
// 2. 配置集中：所有插件的配置都在这里统一管理，方便查看和修改
// 3. 易于扩展：新增插件只需在此函数中添加一行 PluginInit 调用即可
// 4. 向后兼容：保留 v1 版本的设计，可以与 v2 版本共存，支持渐进式迁移
//
// 注意：本地示例模式与在线仓库模式的 import 可以自行切换，效果相同
func bizPluginV1(group ...*gin.RouterGroup) {
	private := group[0] // 私有路由组：需要权限验证的路由
	public := group[1]  // 公有路由组：公开访问的路由

	// 添加与角色权限挂钩的插件示例（邮件插件需要管理员权限）
	// 使用 global.GVA_CONFIG 获取配置，这样设计的好处：
	// - 配置统一管理，无需在多个地方硬编码
	// - 支持配置热更新（如果实现了配置监听机制）
	// - 配置与代码分离，提高可维护性
	PluginInit(private, email.CreateEmailPlug(
		global.GVA_CONFIG.Email.To,
		global.GVA_CONFIG.Email.From,
		global.GVA_CONFIG.Email.Host,
		global.GVA_CONFIG.Email.Secret,
		global.GVA_CONFIG.Email.Nickname,
		global.GVA_CONFIG.Email.Port,
		global.GVA_CONFIG.Email.IsSSL,
		global.GVA_CONFIG.Email.IsLoginAuth,
	))
	// holder 函数用于确保相关包被正确导入，避免 Go 编译器的空变量检测报错
	// 这是 Go 语言的一个技巧，当导入的包只用于初始化（如 init 函数）时，需要这种占位函数
	holder(public, private)
}
