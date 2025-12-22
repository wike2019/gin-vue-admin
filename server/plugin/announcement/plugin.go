package announcement

import (
	"context"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/announcement/initialize"
	interfaces "github.com/flipped-aurora/gin-vue-admin/server/utils/plugin/v2"
	"github.com/gin-gonic/gin"
)

// var _ interfaces.Plugin = (*plugin)(nil)
// 设计模式：接口实现检查（Interface Implementation Check）
// 好处：
// 1. 编译时检查 plugin 结构体是否实现了 interfaces.Plugin 接口
// 2. 如果接口方法签名发生变化，编译时会立即报错，避免运行时错误
// 3. 确保插件符合框架的插件规范，保证插件系统的稳定性
var _ interfaces.Plugin = (*plugin)(nil)

// Plugin 是插件的全局实例
// 设计模式：单例模式（Singleton Pattern） + 插件模式（Plugin Pattern）
// 好处：
// 1. 确保整个应用只有一个插件实例，避免重复创建
// 2. 通过 new() 创建，保证在包加载时初始化，线程安全
// 3. 主应用可以通过 announcement.Plugin 访问插件，实现插件注册
// 4. 符合 Go 的包级别导出规范，便于框架自动发现和加载插件
var Plugin = new(plugin)

// plugin 是公告插件的核心结构体
// 设计模式：插件模式（Plugin Pattern）
// 好处：
// 1. 通过空结构体实现，节省内存（只包含方法，不包含数据）
// 2. 实现 interfaces.Plugin 接口，符合框架的插件规范
// 3. 插件可以独立开发、测试和部署，实现模块化设计
// 4. 便于插件的热插拔，支持动态加载和卸载
type plugin struct{}

// Register 注册插件到主应用
// 设计模式：模板方法模式（Template Method Pattern） + 初始化模式（Initialization Pattern）
// 好处：
// 1. 统一的插件注册入口，框架可以统一管理所有插件
// 2. 按照固定顺序初始化各个组件，确保依赖关系正确
// 3. 插件初始化失败不会影响主应用，提高系统稳定性
// 4. 支持插件的热插拔，可以在运行时动态加载和卸载
// @param group *gin.Engine Gin 引擎实例，用于注册路由
func (p *plugin) Register(group *gin.Engine) {
	// 创建上下文，用于传递请求上下文信息（如超时、取消等）
	// 好处：支持请求级别的上下文管理，便于实现超时控制和请求取消
	ctx := context.Background()
	
	// 如果需要配置文件，请到 config.Config 中填充配置结构，且到下方方法中填入其在 config.yaml 中的 key
	// 设计模式：配置模式（Configuration Pattern）
	// 好处：
	// 1. 将配置与代码分离，便于不同环境使用不同配置
	// 2. 支持配置热更新，无需重启服务
	// 3. 统一管理配置，避免配置分散
	// initialize.Viper()
	
	// 注册 API 到系统
	// 设计模式：自动注册模式（Auto Registration Pattern）
	// 好处：
	// 1. 插件安装时自动注册 API 到权限系统，无需手动配置
	// 2. 统一管理 API 权限，便于权限控制和审计
	// 3. 支持 API 的自动发现和文档生成
	initialize.Api(ctx)
	
	// 注册菜单到系统
	// 设计模式：自动注册模式（Auto Registration Pattern）
	// 好处：
	// 1. 插件安装时自动注册菜单，用户可以直接使用
	// 2. 统一管理菜单结构，便于前端路由配置
	// 3. 支持菜单的权限控制和动态显示
	initialize.Menu(ctx)
	
	// 初始化数据库表结构
	// 设计模式：数据库迁移模式（Database Migration Pattern）
	// 好处：
	// 1. 自动创建和更新数据库表结构，无需手动执行 SQL
	// 2. 支持数据库版本管理，便于回滚和升级
	// 3. 确保数据库结构与代码模型一致，避免运行时错误
	initialize.Gorm(ctx)
	
	// 注册路由到 Gin 引擎
	// 设计模式：路由注册模式（Route Registration Pattern）
	// 好处：
	// 1. 将插件的路由挂载到主应用，实现路由的统一管理
	// 2. 支持路由前缀和中间件，便于实现权限控制和日志记录
	// 3. 插件路由与主应用路由隔离，避免路由冲突
	initialize.Router(group)
}
