package email

import (
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/email/global"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/email/router"
	"github.com/gin-gonic/gin"
)

// emailPlugin 是邮件插件的核心结构体
// 设计模式：适配器模式（Adapter Pattern）+ 接口实现模式
// 
// 为什么使用空结构体？
// 1. 内存效率：空结构体不占用内存空间（0字节），只作为类型标识
// 2. 语义清晰：表明这是一个插件实例，不需要存储状态数据
// 3. 符合 Go 语言习惯：当只需要实现接口而不需要数据时，使用空结构体
// 
// 为什么实现 plugin.Plugin 接口？
// 1. 统一插件规范：所有插件都遵循相同的接口，便于统一管理和注册
// 2. 多态支持：主应用可以通过接口类型统一处理所有插件，无需关心具体实现
// 3. 解耦合：插件实现与主应用解耦，插件可以独立开发和测试
// 4. 可扩展性：新增插件只需实现接口，无需修改主应用代码
type emailPlugin struct{}

// CreateEmailPlug 是邮件插件的工厂函数，用于创建插件实例并初始化配置
// 
// 设计模式：工厂模式（Factory Pattern）+ 依赖注入（Dependency Injection）
// 
// 参数说明：
//   - To: 收件人邮箱地址（多个以英文逗号分隔）
//   - From: 发件人邮箱地址
//   - Host: SMTP 服务器地址（如 smtp.qq.com）
//   - Secret: SMTP 认证密钥（通常是邮箱授权码，而非登录密码）
//   - Nickname: 发件人显示昵称
//   - Port: SMTP 服务器端口（如 465）
//   - IsSSL: 是否启用 SSL/TLS 加密连接
//   - IsLoginAuth: 是否使用 LoginAuth 认证方式（适用于 IBM、微软邮箱等）
// 
// 返回值：*emailPlugin 插件实例指针
// 
// 为什么使用工厂函数而不是直接使用结构体字面量？
// 1. 配置初始化：在创建实例时同时完成配置初始化，确保配置在使用前已就绪
// 2. 参数验证：可以在工厂函数中添加参数验证逻辑，提前发现配置错误
// 3. 封装性：隐藏结构体的创建细节，外部只需调用工厂函数
// 4. 灵活性：未来可以添加默认值、配置合并等逻辑，而不影响调用方
// 
// 为什么将配置写入 global.GlobalConfig？
// 1. 全局访问：配置写入全局变量后，插件内的所有模块（service、utils等）都可以直接访问
// 2. 避免参数传递：不需要在每个函数调用链中传递配置参数，简化代码
// 3. 单例保证：确保整个插件使用同一份配置，避免配置不一致
// 4. 符合插件化设计：每个插件独立管理自己的全局配置，互不干扰
// 
// 使用场景：
//   // 在主应用的 initialize/plugin_biz_v1.go 中调用
//   email.CreateEmailPlug(
//       global.GVA_CONFIG.Email.To,
//       global.GVA_CONFIG.Email.From,
//       global.GVA_CONFIG.Email.Host,
//       global.GVA_CONFIG.Email.Secret,
//       global.GVA_CONFIG.Email.Nickname,
//       global.GVA_CONFIG.Email.Port,
//       global.GVA_CONFIG.Email.IsSSL,
//       global.GVA_CONFIG.Email.IsLoginAuth,
//   )
func CreateEmailPlug(To, From, Host, Secret, Nickname string, Port int, IsSSL bool, IsLoginAuth bool) *emailPlugin {
	// 将传入的配置参数写入全局配置实例
	// 为什么在创建时立即写入？
	// - 确保配置在使用前已初始化，避免运行时配置为空
	// - 配置写入是原子操作，保证配置的完整性
	global.GlobalConfig.To = To
	global.GlobalConfig.From = From
	global.GlobalConfig.Host = Host
	global.GlobalConfig.Secret = Secret
	global.GlobalConfig.Nickname = Nickname
	global.GlobalConfig.Port = Port
	global.GlobalConfig.IsSSL = IsSSL
	global.GlobalConfig.IsLoginAuth = IsLoginAuth
	
	// 返回插件实例
	// 为什么返回指针而不是值？
	// - 指针传递更高效，避免结构体拷贝（虽然这里是空结构体，但保持一致性）
	// - 符合 Go 语言接口实现的最佳实践
	// - 便于未来扩展，如果需要在结构体中添加字段，无需修改返回类型
	return &emailPlugin{}
}

// Register 实现 plugin.Plugin 接口，用于注册插件的路由
// 
// 设计模式：模板方法模式（Template Method Pattern）
// 
// 参数：
//   - group: Gin 路由组，插件路由将挂载到此路由组下
// 
// 为什么需要 Register 方法？
// 1. 接口规范：实现 plugin.Plugin 接口的必需方法
// 2. 延迟注册：路由注册在插件初始化时进行，而非编译时，支持动态加载
// 3. 路由隔离：每个插件的路由挂载到独立的路由组，避免路由冲突
// 4. 统一管理：主应用通过统一的接口调用，无需关心各插件的路由注册细节
// 
// 为什么使用 router.RouterGroupApp？
// 1. 单例模式：确保路由注册逻辑只有一个入口，避免重复注册
// 2. 职责分离：路由注册逻辑集中在 router 包，main.go 只负责调用
// 3. 可维护性：路由变更只需修改 router 包，不影响插件入口文件
// 4. 符合 MVC 架构：main.go 作为控制器层，router 作为路由层，职责清晰
// 
// 执行流程：
//   1. 主应用调用 PluginInit 函数
//   2. PluginInit 遍历所有插件，为每个插件创建独立的路由组（通过 RouterPath() 获取路径）
//   3. 调用插件的 Register 方法，将路由组传入
//   4. Register 方法调用 router.RouterGroupApp.InitEmailRouter 注册具体路由
func (*emailPlugin) Register(group *gin.RouterGroup) {
	// 委托给路由组应用实例进行路由注册
	// 为什么使用委托模式？
	// - 将路由注册的具体实现委托给专门的 router 包，保持 main.go 的简洁
	// - 便于单元测试，可以 mock router.RouterGroupApp 进行测试
	// - 支持路由注册逻辑的复用和扩展
	router.RouterGroupApp.InitEmailRouter(group)
}

// RouterPath 实现 plugin.Plugin 接口，返回插件的路由路径前缀
// 
// 返回值：string 路由路径，如 "email"
// 
// 为什么需要 RouterPath 方法？
// 1. 路由隔离：每个插件使用独立的路由前缀，避免路由冲突
//   例如：email 插件的路由为 /email/xxx，announcement 插件的路由为 /announcement/xxx
// 2. 动态路由组创建：主应用根据此路径为插件创建独立的路由组
//   例如：group.Group("email") 创建 /email 路由组
// 3. 接口规范：实现 plugin.Plugin 接口的必需方法
// 4. 可配置性：未来可以支持从配置文件读取路由路径，实现动态配置
// 
// 路由路径的作用：
// - 主应用会使用此路径创建路由组：group.Group(RouterPath())
// - 最终的路由地址为：/api/v1/{RouterPath()}/具体路由
//   例如：/api/v1/email/sendEmail
// 
// 为什么返回固定字符串而不是动态配置？
// 1. 简洁性：对于大多数插件，路由路径是固定的，不需要动态配置
// 2. 可读性：代码中直接看到路由路径，便于理解和调试
// 3. 性能：避免每次调用都读取配置，提高性能
// 4. 如果未来需要动态配置，可以改为从 global.GlobalConfig 读取
func (*emailPlugin) RouterPath() string {
	// 返回插件的路由路径前缀
	// 此路径将作为插件所有路由的前缀，确保路由的唯一性和可识别性
	return "email"
}
