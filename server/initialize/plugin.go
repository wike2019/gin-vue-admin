package initialize

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/gin-gonic/gin"
)

// InstallPlugin 插件安装的统一入口函数
// 设计原因：
// 1. 提供统一的插件安装入口，整合 v1 和 v2 两套插件系统
// 2. 在执行插件安装前检查数据库是否已初始化，确保插件可以正常使用数据库
// 3. 接收三个参数：私有路由组、公有路由组和 Engine，满足不同版本插件的需求
//
// 好处：
// 1. 统一管理：所有插件的安装都在一个函数中，便于维护和调试
// 2. 依赖检查：在安装插件前检查数据库是否初始化，避免插件因数据库未就绪而失败
// 3. 版本兼容：同时支持 v1 和 v2 插件系统，提供灵活的插件架构
// 4. 清晰调用：外部只需调用一个函数即可完成所有插件的安装，接口简洁
// 5. 错误友好：数据库未初始化时给出明确的提示信息，而不是抛出异常
//
// 执行流程：
// 1. 检查数据库是否已初始化（global.GVA_DB != nil）
// 2. 如果未初始化，记录日志并返回，提示用户先初始化数据库
// 3. 如果已初始化，依次调用 bizPluginV1 和 bizPluginV2 安装插件
//
// 注意：这个函数应该在数据库初始化完成后调用，通常在路由初始化阶段调用
func InstallPlugin(PrivateGroup *gin.RouterGroup, PublicRouter *gin.RouterGroup, engine *gin.Engine) {
	// 检查数据库是否已初始化
	// 这样设计的原因：
	// - 很多插件需要数据库支持（如存储配置、缓存数据等）
	// - 如果数据库未初始化就安装插件，可能导致插件无法正常工作
	// - 提前检查可以避免运行时错误，提供更好的用户体验
	if global.GVA_DB == nil {
		global.GVA_LOG.Info("项目暂未初始化，无法安装插件，初始化后重启项目即可完成插件安装")
		return
	}
	// 安装 v1 版本的插件（基于 RouterGroup，支持权限控制）
	bizPluginV1(PrivateGroup, PublicRouter)
	// 安装 v2 版本的插件（基于 Engine，提供全局控制）
	bizPluginV2(engine)
}
