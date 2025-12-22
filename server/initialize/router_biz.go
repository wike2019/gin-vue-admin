package initialize

import (
	"github.com/flipped-aurora/gin-vue-admin/server/router"
	"github.com/gin-gonic/gin"
)

// holder 占位方法，保证文件可以正确加载，避免 Go 空变量检测报错
// 设计原因：
// 1. Go 编译器会对未使用的导入和变量报错或警告
// 2. 但有些包只需要执行其 init 函数（如注册路由、初始化配置等），不需要直接使用
// 3. 通过 holder 函数"使用"这些变量，可以绕过编译器的检查
//
// 为什么需要这个函数：
// - router 包可能只用于其 init 函数自动注册路由，而不需要在当前文件中直接使用
// - RouterGroupApp 可能是 router 包中导出的变量，需要确保 router 包被导入
// - Go 的 init 函数机制：导入包时会自动执行其 init 函数，但编译器不知道这一点
//
// 好处：
// 1. 保持代码整洁：不需要使用 //nolint 或其他注释来抑制警告
// 2. 明确意图：通过函数名 holder 明确表示这是占位用途
// 3. 类型安全：使用 _ = variable 的方式确保变量确实存在且类型正确
// 4. 文档化：注释说明这是占位函数，提醒其他开发者不要删除
//
// 注意：请勿删除此函数，否则可能导致相关包的 init 函数无法执行
func holder(routers ...*gin.RouterGroup) {
	_ = routers                    // 使用 routers 参数，避免"未使用的参数"警告
	_ = router.RouterGroupApp      // 使用 router 包的导出变量，确保 router 包被导入
}

// initBizRouter 业务路由初始化函数
// 设计原因：
// 1. 提供业务路由的初始化入口，与系统路由分离
// 2. 接收可变参数，支持传入私有路由组和公有路由组
// 3. 当前实现为空，调用 holder 确保相关包被正确导入
//
// 好处：
// 1. 预留扩展：虽然当前为空实现，但提供了统一的业务路由初始化入口
// 2. 结构清晰：业务路由与系统路由分离，便于维护
// 3. 依赖管理：通过 holder 确保 router 包被导入，其 init 函数会被执行
//
// 未来扩展：
// - 可以在此函数中添加自定义业务路由的注册逻辑
// - 可以注册业务相关的中间件
// - 可以配置业务相关的路由组选项
func initBizRouter(routers ...*gin.RouterGroup) {
	privateGroup := routers[0] // 私有路由组：需要权限验证
	publicGroup := routers[1]  // 公有路由组：公开访问

	// 调用 holder 确保 router 包被导入，其 init 函数会执行路由注册
	holder(publicGroup, privateGroup)
}
