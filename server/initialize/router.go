// Package initialize 负责初始化系统的各个组件
//
// 本包的核心职责：
// - 路由初始化：注册所有 HTTP 路由，包括公共路由和私有路由
// - 数据库初始化：连接数据库并初始化表结构
// - 中间件配置：配置全局中间件（认证、日志、跨域等）
// - 插件系统：支持动态加载和卸载插件
//
// 为什么集中管理初始化逻辑？
// - 统一管理：所有初始化代码集中在一处，便于维护和理解
// - 依赖顺序：确保各组件的初始化顺序正确（如数据库 -> 路由）
// - 可测试性：可以单独测试初始化逻辑，方便单元测试
package initialize

import (
	"net/http"
	"os"

	"github.com/flipped-aurora/gin-vue-admin/server/docs"       // Swagger 文档配置
	"github.com/flipped-aurora/gin-vue-admin/server/global"     // 全局配置和变量
	"github.com/flipped-aurora/gin-vue-admin/server/middleware" // 中间件（认证、日志等）
	"github.com/flipped-aurora/gin-vue-admin/server/router"     // 路由组定义
	"github.com/gin-gonic/gin"                                  // Gin Web 框架
	swaggerFiles "github.com/swaggo/files"                      // Swagger 静态文件服务
	ginSwagger "github.com/swaggo/gin-swagger"                  // Swagger UI 集成
)

// justFilesFilesystem 自定义文件系统包装器
//
// 设计目的：
// 防止目录遍历攻击，只允许访问文件，不允许列出目录内容
//
// 为什么需要这个包装器？
// - 安全考虑：直接使用 http.Dir 可能暴露目录结构，存在安全风险
// - 防止信息泄露：攻击者无法通过访问目录获取文件列表
// - 符合最小权限原则：只提供必要的文件访问功能
type justFilesFilesystem struct {
	fs http.FileSystem
}

// Open 打开文件，但拒绝打开目录
//
// 实现逻辑：
// 1. 尝试打开文件/目录
// 2. 检查是否为目录
// 3. 如果是目录，返回权限错误，拒绝访问
// 4. 如果是文件，正常返回
//
// 好处：
// - 安全性：防止目录遍历攻击
// - 简洁性：用户只能访问明确指定的文件
func (fs justFilesFilesystem) Open(name string) (http.File, error) {
	// 先尝试打开文件或目录
	// 为什么先打开再判断？
	// - 需要先打开才能获取文件信息（Stat）
	// - 如果文件不存在，这里会返回错误，提前退出
	f, err := fs.fs.Open(name)
	if err != nil {
		// 如果打开失败（文件不存在、权限不足等），直接返回错误
		// 好处：避免不必要的后续处理，提前返回
		return nil, err
	}

	// 获取文件信息，判断是否为目录
	// 为什么需要 Stat？
	// - 需要判断打开的是文件还是目录
	// - 只有通过 Stat() 才能获取文件的元信息
	stat, err := f.Stat()
	if err != nil {
		// 如果获取文件信息失败，关闭已打开的文件并返回错误
		// 为什么关闭文件？
		// - 避免文件描述符泄露
		// - 资源管理的最佳实践
		f.Close()
		return nil, err
	}

	if stat.IsDir() {
		// 如果是目录，关闭文件并返回权限错误，拒绝访问
		// 这样设计的好处：
		// - 防止攻击者通过访问目录获取文件列表（目录遍历攻击）
		// - 明确拒绝访问，而不是返回空响应
		// - 使用 os.ErrPermission 提供明确的错误信息
		f.Close()
		return nil, os.ErrPermission
	}

	// 如果是文件，正常返回
	// 后续调用者需要负责关闭文件（http.File 接口规范）
	return f, nil
}

// Routers 初始化总路由
//
// 设计思路：
// 1. 创建 Gin 引擎并注册全局中间件（错误恢复、日志）
// 2. 注册 MCP 服务路由（如果启用）
// 3. 注册 Swagger 文档路由
// 4. 创建公共路由组和私有路由组（区分是否需要认证）
// 5. 注册各个业务模块的路由
//
// 为什么这样组织路由？
// - 中间件顺序：Recovery 必须在最外层，确保 panic 能被捕获
// - 路由分组：公共路由和私有路由分离，便于管理和维护
// - 统一前缀：通过 RouterPrefix 支持多服务器部署和 API 版本控制
func Routers() *gin.Engine {
	// 创建 Gin 引擎实例
	// 使用 gin.New() 而不是 gin.Default()
	// 原因：gin.Default() 会自动添加 Logger 和 Recovery 中间件
	// 但我们使用自定义的 Recovery 中间件（支持错误入库），所以用 gin.New()
	Router := gin.New()

	// 注册全局中间件 - Recovery（错误恢复）
	// 为什么 Recovery 必须在最外层？
	// - 确保所有 panic 都能被捕获，避免程序崩溃
	// - 自定义 Recovery 可以将错误信息记录到数据库，便于问题追踪
	// - 参数 true 表示启用错误入库功能
	Router.Use(middleware.GinRecovery(true))

	// 在开发模式下启用 Gin 默认日志中间件
	// 为什么只在 DebugMode 启用？
	// - 开发环境需要详细的请求日志，便于调试
	// - 生产环境日志量太大，影响性能，应该使用自定义日志中间件
	// - 减少生产环境的日志输出，提高性能
	if gin.Mode() == gin.DebugMode {
		Router.Use(gin.Logger())
	}

	// 注册 MCP（Model Context Protocol）服务路由
	// 为什么使用条件判断？
	// - MCP 服务是可选的，如果配置为分离模式（Separate），则不在这里注册
	// - 分离模式可能意味着 MCP 服务运行在独立的进程中
	// - 灵活性：支持不同的部署架构
	if !global.GVA_CONFIG.MCP.Separate {
		// 启动 MCP 服务器
		sseServer := McpRun()

		// 注册 SSE（Server-Sent Events）路由
		// SSE 用于服务器向客户端推送实时数据
		// 为什么使用 GET 方法？
		// - SSE 协议要求使用 GET 请求建立长连接
		// - 客户端通过 EventSource API 连接此端点
		// - GET 请求是单向的，服务器可以持续推送数据
		//
		// 为什么使用匿名函数包装？
		// - 将 Gin 的 Context 转换为标准 http.ResponseWriter 和 http.Request
		// - SSE 服务器使用标准库接口，需要适配 Gin 的接口
		// - 好处：保持 SSE 服务器的通用性，不依赖特定框架
		Router.GET(global.GVA_CONFIG.MCP.SSEPath, func(c *gin.Context) {
			sseServer.SSEHandler().ServeHTTP(c.Writer, c.Request)
		})

		// 注册 MCP 消息处理路由
		// 用于接收客户端发送的消息
		// 为什么使用 POST 方法？
		// - 消息可能包含请求体，POST 更适合传输数据
		// - POST 请求可以携带复杂的数据结构
		// - 语义明确：客户端向服务器发送数据
		//
		// 为什么使用匿名函数包装？
		// - 同 SSE 路由，适配 Gin 和标准库接口
		Router.POST(global.GVA_CONFIG.MCP.MessagePath, func(c *gin.Context) {
			sseServer.MessageHandler().ServeHTTP(c.Writer, c.Request)
		})
	}

	// 获取路由组实例
	// 为什么提前获取？
	// - 代码更清晰：后续注册路由时不需要重复写 router.RouterGroupApp
	// - 性能优化：避免重复访问结构体字段
	systemRouter := router.RouterGroupApp.System
	exampleRouter := router.RouterGroupApp.Example

	// 前端静态文件服务（可选）
	// 如果想要不使用nginx代理前端网页，可以修改 web/.env.production 下的
	// VUE_APP_BASE_API = /
	// VUE_APP_BASE_PATH = http://localhost
	// 然后执行打包命令 npm run build。在打开下面3行注释
	// Router.StaticFile("/favicon.ico", "./dist/favicon.ico")
	// Router.StaticFile("/", "./dist/index.html") // 前端网页入口页面
	// Router.Static("/assets", "./dist/assets")   // dist里面的静态资源
	// 为什么默认注释掉？
	// - 生产环境通常使用 Nginx 等专业 Web 服务器提供静态文件服务
	// - Nginx 性能更好，支持缓存、压缩等功能
	// - 前后端分离架构，静态文件应该由专门的服务器处理

	// 注册本地文件存储的静态文件服务
	// 使用自定义的 justFilesFilesystem 包装器，防止目录遍历攻击
	// 为什么使用 StaticFS 而不是 Static？
	// - StaticFS 支持自定义文件系统实现，更灵活
	// - 可以添加安全检查逻辑（如 justFilesFilesystem）
	Router.StaticFS(global.GVA_CONFIG.Local.StorePath, justFilesFilesystem{http.Dir(global.GVA_CONFIG.Local.StorePath)})

	// HTTPS 支持（可选）
	// Router.Use(middleware.LoadTls())  // 如果需要使用https 请打开此中间件 然后前往 core/server.go 将启动模式 更变为 Router.RunTLS("端口","你的cre/pem文件","你的key文件")
	// 为什么默认不启用？
	// - 生产环境通常使用 Nginx 或负载均衡器处理 HTTPS
	// - 应用层专注于业务逻辑，HTTPS 由基础设施层处理

	// 跨域处理（可选）
	// Router.Use(middleware.Cors()) // 直接放行全部跨域请求
	// Router.Use(middleware.CorsByRules()) // 按照配置的规则放行跨域请求
	// global.GVA_LOG.Info("use middleware cors")
	// 为什么默认注释掉？
	// - 如果前后端部署在同一域名下，不需要跨域
	// - 如果使用 Nginx 反向代理，可以在 Nginx 层处理跨域
	// - 需要时再启用，避免不必要的性能开销

	// 配置 Swagger 文档的基础路径
	// 为什么设置 BasePath？
	// - 支持路由前缀（RouterPrefix），多服务器部署时可能需要不同的前缀
	// - Swagger UI 需要知道 API 的基础路径才能正确生成请求 URL
	docs.SwaggerInfo.BasePath = global.GVA_CONFIG.System.RouterPrefix

	// 注册 Swagger 文档路由
	// 为什么使用通配符 /*any？
	// - Swagger UI 需要加载多个资源文件（HTML、CSS、JS等）
	// - 通配符可以匹配所有 Swagger 相关的路径
	// - 例如：/swagger/index.html, /swagger/doc.json 等
	Router.GET(global.GVA_CONFIG.System.RouterPrefix+"/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	global.GVA_LOG.Info("register swagger handler")

	// 创建路由组
	// 为什么创建两个路由组（PublicGroup 和 PrivateGroup）？
	// - 公共路由组：不需要认证的接口（登录、注册、健康检查等）
	// - 私有路由组：需要认证的接口（用户信息、数据管理等）
	// - 好处：
	//   1. 代码清晰：明确区分哪些接口需要认证
	//   2. 性能优化：公共接口不需要经过认证中间件，减少开销
	//   3. 安全性：统一管理认证逻辑，避免遗漏
	//   4. 维护性：修改认证逻辑时只需修改一处
	//
	// 为什么都使用 RouterPrefix？
	// - 方便统一添加路由组前缀，多服务器上线使用
	// - 支持 API 版本控制（如 /v1/api/...）
	// - 便于路由管理和迁移
	PublicGroup := Router.Group(global.GVA_CONFIG.System.RouterPrefix)
	PrivateGroup := Router.Group(global.GVA_CONFIG.System.RouterPrefix)

	// 为私有路由组添加认证和权限中间件
	// 中间件执行顺序：先 JWT 认证，再权限检查
	// 为什么先认证再权限？
	// - JWT 认证确定用户身份，权限检查需要知道用户是谁
	// - 如果认证失败，直接返回，不需要进行权限检查
	// - 性能优化：认证失败时避免不必要的权限查询
	//
	// 为什么使用链式调用（.Use().Use()）？
	// - 代码简洁：一行代码完成多个中间件的注册
	// - 执行顺序清晰：从上到下依次执行中间件
	// - 符合 Gin 框架的设计模式
	// 注意：中间件按照注册顺序执行，即 JWTAuth -> CasbinHandler
	PrivateGroup.Use(middleware.JWTAuth()).Use(middleware.CasbinHandler())

	// 注册公共路由（不需要认证）
	// 为什么使用代码块（{}）分组？
	// - 代码组织：将相关的路由注册逻辑分组，提高可读性
	// - 作用域管理：代码块可以限制变量的作用域，避免命名冲突
	// - 逻辑清晰：每个代码块代表一个功能模块，便于理解和维护
	{
		// 健康检查接口
		// 为什么需要健康检查？
		// - 容器编排工具（Kubernetes、Docker Swarm）需要健康检查来判断服务是否正常
		// - 负载均衡器可以通过健康检查决定是否将流量转发到此实例
		// - 监控系统可以通过健康检查接口收集服务状态
		//
		// 为什么返回简单的 "ok"？
		// - 简单高效：不需要复杂的业务逻辑，快速响应
		// - 标准化：许多健康检查工具期望简单的响应
		// - 性能考虑：减少处理时间，快速判断服务状态
		PublicGroup.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, "ok")
		})
	}
	{
		// 注册基础功能路由（不做鉴权）
		// 包括登录、注册、验证码等不需要认证的接口
		// 为什么放在公共路由组？
		// - 这些功能在用户登录前就需要使用
		// - 避免循环依赖：登录接口不应该需要认证
		// - 性能优化：不需要经过认证中间件，响应更快
		systemRouter.InitBaseRouter(PublicGroup)

		// 自动初始化相关路由
		// 用于系统首次部署时的自动初始化功能
		// 为什么放在公共路由组？
		// - 初始化时系统可能还没有管理员账户
		// - 允许在未认证状态下完成系统初始化
		// - 安全考虑：初始化接口应该在生产环境禁用或添加额外保护
		systemRouter.InitInitRouter(PublicGroup)
	}

	// 注册私有路由（需要认证和权限检查）
	// 为什么使用代码块分组？
	// - 逻辑分组：所有需要认证的路由集中管理
	// - 代码清晰：与公共路由形成对比，明确区分认证需求
	// - 便于维护：修改认证逻辑时，只需要关注这个代码块
	{
		// 为什么某些路由同时传入 PrivateGroup 和 PublicGroup？
		// - 某些接口可能同时提供需要认证和不需要认证的版本
		// - 例如：API 列表接口，管理员需要完整列表，普通用户只需要部分列表
		// - 灵活性：同一个功能可以有不同的访问级别
		// - 设计模式：策略模式，根据认证状态提供不同的行为

		// 注册功能api路由
		// 为什么同时传入两个路由组？
		// - API 列表接口可能在公共路由中提供基础查询功能
		// - 私有路由中提供完整的管理功能（增删改查）
		// - 好处：API 设计更灵活，支持不同场景的使用需求
		systemRouter.InitApiRouter(PrivateGroup, PublicGroup)
		// jwt相关路由（刷新token、退出登录等）
		// 为什么放在私有路由组？
		// - 刷新 token 需要有效的 refresh token，应该进行认证
		// - 退出登录需要知道当前用户身份，需要 JWT 认证
		// - 安全性：防止未授权的 token 操作
		systemRouter.InitJwtRouter(PrivateGroup)
		// 注册用户路由（用户信息管理）
		// 为什么需要认证？
		// - 用户信息是敏感数据，必须验证身份后才能访问
		// - 防止未授权访问其他用户的信息
		// - 符合数据保护法规要求（如 GDPR）
		systemRouter.InitUserRouter(PrivateGroup)

		// 注册menu路由（菜单管理）
		// 为什么需要认证？
		// - 菜单配置涉及系统功能权限，必须由管理员操作
		// - 防止未授权修改系统菜单结构
		systemRouter.InitMenuRouter(PrivateGroup)

		// system相关路由（系统配置）
		// 为什么需要认证和权限？
		// - 系统配置影响整个系统行为，必须严格权限控制
		// - 通常只有超级管理员才能修改系统配置
		systemRouter.InitSystemRouter(PrivateGroup)

		// 发版相关路由（版本管理）
		// 为什么需要认证？
		// - 版本信息可能包含敏感数据（如更新日志、漏洞修复信息）
		// - 版本管理操作需要管理员权限
		systemRouter.InitSysVersionRouter(PrivateGroup)

		// 权限相关路由（RBAC权限管理）
		// 为什么需要认证和权限？
		// - 权限管理是系统的核心安全功能
		// - 只有具有权限管理权限的用户才能操作
		// - 防止权限被未授权修改，导致安全漏洞
		systemRouter.InitCasbinRouter(PrivateGroup)
		// 创建自动化代码（代码生成功能）
		// 为什么同时传入两个路由组？
		// - 公共路由可能提供代码生成模板的查询功能
		// - 私有路由提供实际的代码生成和管理功能
		systemRouter.InitAutoCodeRouter(PrivateGroup, PublicGroup)

		// 注册角色路由（角色管理）
		// 为什么需要认证和权限？
		// - 角色管理涉及权限分配，必须严格控制
		// - 防止未授权创建或修改角色，导致权限混乱
		systemRouter.InitAuthorityRouter(PrivateGroup)

		// 字典管理（数据字典）
		// 为什么需要认证？
		// - 数据字典是系统的基础数据，修改影响业务逻辑
		// - 防止未授权修改导致业务异常
		systemRouter.InitSysDictionaryRouter(PrivateGroup)

		// 自动化代码历史（代码生成历史记录）
		// 为什么需要认证？
		// - 历史记录可能包含代码实现细节，属于敏感信息
		// - 只有生成过代码的用户才能查看自己的历史记录
		systemRouter.InitAutoCodeHistoryRouter(PrivateGroup)

		// 操作记录（操作日志）
		// 为什么需要认证和权限？
		// - 操作日志包含用户的敏感操作记录
		// - 通常只有管理员才能查看操作日志
		// - 防止日志泄露，保护用户隐私
		systemRouter.InitSysOperationRecordRouter(PrivateGroup)

		// 字典详情管理（字典项管理）
		// 为什么需要认证？
		// - 字典项是数据字典的子项，同样需要权限控制
		// - 字典项修改直接影响业务数据
		systemRouter.InitSysDictionaryDetailRouter(PrivateGroup)

		// 按钮权限管理（按钮级权限控制）
		// 为什么需要认证和权限？
		// - 按钮权限是细粒度的权限控制
		// - 必须严格控制，防止权限分配错误
		systemRouter.InitAuthorityBtnRouterRouter(PrivateGroup)

		// 导出模板（数据导出功能）
		// 为什么同时传入两个路由组？
		// - 公共路由可能提供模板下载功能
		// - 私有路由提供模板的创建和管理功能
		systemRouter.InitSysExportTemplateRouter(PrivateGroup, PublicGroup)

		// 参数管理（系统参数配置）
		// 为什么同时传入两个路由组？
		// - 某些系统参数可能允许公开查询（如系统公告）
		// - 参数的修改必须经过认证和权限检查
		systemRouter.InitSysParamsRouter(PrivateGroup, PublicGroup)

		// 错误日志（系统错误记录）
		// 为什么同时传入两个路由组？
		// - 某些错误信息可能需要公开（如 API 错误码说明）
		// - 详细的错误日志查看需要管理员权限
		systemRouter.InitSysErrorRouter(PrivateGroup, PublicGroup)

		// 示例模块路由
		// 为什么这些路由放在示例模块？
		// - 这些是示例代码，展示如何实现常见的业务功能
		// - 开发者可以参考这些实现来开发自己的业务模块
		// - 保持系统路由和示例路由的分离，便于管理

		// 客户路由（示例：客户管理）
		// 为什么需要认证？
		// - 客户信息是业务敏感数据，必须验证身份
		// - 防止未授权访问客户数据
		exampleRouter.InitCustomerRouter(PrivateGroup)

		// 文件上传下载功能路由
		// 为什么需要认证？
		// - 文件上传可能占用服务器资源，需要控制
		// - 文件下载可能包含敏感数据，需要权限验证
		// - 防止未授权的文件操作导致安全风险
		exampleRouter.InitFileUploadAndDownloadRouter(PrivateGroup)

		// 文件上传下载分类（附件分类管理）
		// 为什么需要认证？
		// - 附件分类管理涉及文件组织结构
		// - 防止未授权修改分类，导致文件管理混乱
		exampleRouter.InitAttachmentCategoryRouterRouter(PrivateGroup)
	}

	// 插件路由安装
	// 为什么插件路由单独处理？
	// - 插件是可选功能，可能动态加载和卸载
	// - 插件路由可能有特殊的注册逻辑
	// - 便于插件系统的扩展和管理
	InstallPlugin(PrivateGroup, PublicGroup, Router)

	// 注册业务路由
	// 业务路由是用户自定义的路由，通过代码生成或手动添加
	// 为什么单独注册？
	// - 业务路由与系统路由分离，便于管理和维护
	// - 支持业务路由的动态加载和重载
	initBizRouter(PrivateGroup, PublicGroup)

	// 保存所有路由信息到全局变量
	// 为什么保存路由信息？
	// - 路由重载功能需要知道当前有哪些路由，才能正确重新加载
	// - 路由权限管理需要遍历所有路由，动态分配权限
	// - 调试和监控需要路由信息，便于排查问题和性能分析
	// - API 文档生成可能需要路由信息
	//
	// 为什么使用全局变量？
	// - 路由信息在运行时不需要修改，适合使用全局变量
	// - 多个模块可能需要访问路由信息（如权限管理、监控等）
	// - 避免频繁传递路由信息，简化代码
	global.GVA_ROUTERS = Router.Routes()

	// 记录路由注册成功的日志
	// 为什么记录日志？
	// - 确认路由注册流程正常完成
	// - 便于排查路由注册问题
	// - 监控系统启动状态
	global.GVA_LOG.Info("router register success")

	// 返回配置好的 Gin 引擎实例
	// 为什么返回 Router？
	// - 调用者需要 Router 来启动 HTTP 服务器
	// - 保持函数的职责单一：初始化并返回，不负责启动服务
	return Router
}
