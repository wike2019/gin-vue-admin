// Package system 提供系统相关的路由定义
// 本文件专门负责自动代码生成功能的路由配置
package system

import (
	"github.com/gin-gonic/gin"
)

// AutoCodeRouter 自动代码生成路由结构体
// 使用结构体而非函数的设计模式，好处：
// 1. 更好的封装性：可以将路由相关的状态和方法组织在一起
// 2. 易于扩展：未来如果需要添加路由级别的中间件或配置，可以轻松扩展
// 3. 符合 Go 的面向对象编程习惯：通过方法接收者实现功能分组
// 4. 便于测试：可以针对结构体方法进行单元测试
type AutoCodeRouter struct{}

// InitAutoCodeRouter 初始化自动代码生成相关的路由
// 参数说明：
//   - Router: 需要认证的私有路由组，通常已绑定 JWT 认证中间件，用于需要登录才能访问的接口
//   - RouterPublic: 公开路由组，无需认证即可访问，用于公开的接口（如插件初始化、LLM自动生成等）
//
// 设计优势：
// 1. 路由分组管理：使用 Router.Group("autoCode") 创建路由组，所有自动代码相关接口统一前缀为 /autoCode
//    好处：代码组织清晰，URL 结构统一，便于 API 版本管理和路由中间件统一应用
//
// 2. 区分私有和公开路由：将需要认证和无需认证的接口分开管理
//    好处：
//    - 安全性：敏感操作（如创建代码、删除包）必须认证，防止未授权访问
//    - 灵活性：公开接口（如插件初始化）可以被外部系统调用，无需认证
//    - 清晰的权限边界：通过路由分组明确哪些接口需要权限，哪些不需要
//
// 3. 功能模块分组：使用代码块 {} 将相关功能的路由分组
//    好处：
//    - 可读性：代码结构清晰，一眼就能看出哪些路由属于同一功能模块
//    - 可维护性：新增或修改路由时，容易找到对应的功能模块
//    - 便于注释：可以为每个功能模块添加说明注释
//
// 4. RESTful 设计：根据操作类型选择对应的 HTTP 方法（GET/POST）
//    - GET: 用于查询操作（获取数据库、表、字段等），符合 RESTful 规范
//    - POST: 用于创建、修改、删除等有副作用的操作，符合 RESTful 规范
func (s *AutoCodeRouter) InitAutoCodeRouter(Router *gin.RouterGroup, RouterPublic *gin.RouterGroup) {
	// 创建私有路由组：需要 JWT 认证才能访问
	// 所有路由前缀为 /autoCode，例如：/api/v1/autoCode/getDB
	autoCodeRouter := Router.Group("autoCode")
	
	// 创建公开路由组：无需认证即可访问
	// 所有路由前缀为 /autoCode，例如：/api/v1/public/autoCode/llmAuto
	publicAutoCodeRouter := RouterPublic.Group("autoCode")
	
	// ========== 数据库信息查询模块 ==========
	// 功能：提供数据库、表、字段的查询接口
	// 设计：使用 GET 方法，因为这些是查询操作，不修改数据
	// 权限：需要认证，因为涉及数据库结构信息，属于敏感操作
	{
		autoCodeRouter.GET("getDB", autoCodeApi.GetDB)         // 获取数据库列表
		autoCodeRouter.GET("getTables", autoCodeApi.GetTables) // 获取指定数据库的所有表
		autoCodeRouter.GET("getColumn", autoCodeApi.GetColumn) // 获取指定表的所有字段信息
	}
	
	// ========== 代码模板生成模块 ==========
	// 功能：提供代码预览、创建、扩展等核心功能
	// 设计：使用 POST 方法，因为这些操作会生成代码文件，属于有副作用的操作
	// 权限：需要认证，代码生成是核心功能，必须确保用户身份
	{
		autoCodeRouter.POST("preview", autoCodeTemplateApi.Preview)   // 预览自动生成的代码（不实际创建文件）
		autoCodeRouter.POST("createTemp", autoCodeTemplateApi.Create) // 创建自动化代码文件
		autoCodeRouter.POST("addFunc", autoCodeTemplateApi.AddFunc)   // 为已有代码插入新方法
	}
	
	// ========== MCP 工具模块 ==========
	// 功能：提供 MCP (Model Context Protocol) 工具相关的操作
	// 设计：使用 POST 方法，因为涉及工具创建、列表获取、测试等操作
	// 权限：需要认证，MCP 工具是高级功能，需要权限控制
	{
		autoCodeRouter.POST("mcp", autoCodeTemplateApi.MCP)         // 自动创建 MCP Tool 模板
		autoCodeRouter.POST("mcpList", autoCodeTemplateApi.MCPList) // 获取 MCP Tool 列表
		autoCodeRouter.POST("mcpTest", autoCodeTemplateApi.MCPTest) // 测试 MCP 工具功能
	}
	
	// ========== Package 包管理模块 ==========
	// 功能：提供代码包（Package）的增删查操作
	// 设计：使用 POST 方法，因为涉及包的创建和删除，属于有副作用的操作
	// 权限：需要认证，包管理是系统级操作，必须确保用户权限
	{
		autoCodeRouter.POST("getPackage", autoCodePackageApi.All)       // 获取所有 package 包列表
		autoCodeRouter.POST("delPackage", autoCodePackageApi.Delete)    // 删除指定的 package 包
		autoCodeRouter.POST("createPackage", autoCodePackageApi.Create) // 创建新的 package 包
	}
	
	// ========== 模板查询模块 ==========
	// 功能：获取可用的代码模板列表
	// 设计：使用 GET 方法，这是纯查询操作
	// 权限：需要认证，模板信息属于系统资源
	{
		autoCodeRouter.GET("getTemplates", autoCodePackageApi.Templates) // 获取所有可用的代码模板
	}
	
	// ========== 插件管理模块 ==========
	// 功能：提供插件的打包和安装功能
	// 设计：使用 POST 方法，因为涉及文件操作和系统变更
	// 权限：需要认证，插件管理是系统级操作，需要管理员权限
	{
		autoCodeRouter.POST("pubPlug", autoCodePluginApi.Packaged)      // 打包插件为可分发的格式
		autoCodeRouter.POST("installPlugin", autoCodePluginApi.Install) // 自动安装插件到系统
	}
	
	// ========== 公开接口模块 ==========
	// 功能：提供无需认证即可访问的接口
	// 设计：使用 publicAutoCodeRouter，这些接口不需要 JWT 认证
	// 使用场景：
	//   - LLM 自动生成：可能被外部系统调用，需要公开访问
	//   - 插件初始化：插件安装后需要初始化菜单和 API，此时可能还没有完整的认证流程
	// 好处：提供灵活性，允许外部系统或自动化脚本调用这些接口
	{
		publicAutoCodeRouter.POST("llmAuto", autoCodeApi.LLMAuto)              // LLM 自动代码生成（公开接口）
		publicAutoCodeRouter.POST("initMenu", autoCodePluginApi.InitMenu)      // 同步插件菜单到系统（公开接口）
		publicAutoCodeRouter.POST("initAPI", autoCodePluginApi.InitAPI)        // 同步插件 API 到系统（公开接口）
	}
}
