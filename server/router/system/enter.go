package system

import api "github.com/flipped-aurora/gin-vue-admin/server/api/v1"

// RouterGroup 系统路由组，采用 Go 语言的结构体嵌入（Embedding）特性实现组合模式
//
// 设计说明：
// 1. 为什么使用结构体嵌入而不是继承？
//   - Go 语言不支持传统面向对象的继承机制，结构体嵌入是 Go 推荐的组合方式
//   - 嵌入允许 RouterGroup 直接访问所有嵌入结构体的方法和字段，实现代码复用
//   - 符合 Go 语言的"组合优于继承"设计哲学，更灵活、更易维护
//
// 2. 为什么将所有路由接口嵌入到一个结构体中？
//   - 统一管理：所有系统相关的路由功能集中在一个结构体中，便于统一初始化和调用
//   - 职责聚合：RouterGroup 作为系统路由的统一入口，聚合所有子路由模块的功能
//   - 简化调用：外部代码只需通过 RouterGroupApp.System 即可访问所有系统路由功能
//   - 模块化设计：每个嵌入的路由接口（如 ApiRouter、JwtRouter）负责特定功能模块的路由定义
//
// 3. 路由接口的职责划分：
//
//   - ApiRouter: API 接口管理路由（创建、删除、同步 API 等）
//
//   - JwtRouter: JWT 认证相关路由（token 黑名单等）
//
//   - SysRouter: 系统配置路由（系统信息查询、配置修改、服务重启等）
//
//   - BaseRouter: 基础功能路由（登录、验证码等公共接口）
//
//   - InitRouter: 数据库初始化路由（系统首次部署时的初始化接口）
//
//   - MenuRouter: 菜单管理路由（菜单的增删改查）
//
//   - UserRouter: 用户管理路由（用户的增删改查、权限分配等）
//
//   - CasbinRouter: 权限管理路由（基于 Casbin 的权限控制）
//
//   - AutoCodeRouter: 代码生成路由（自动生成 CRUD 代码）
//
//   - AuthorityRouter: 角色权限路由（角色的增删改查）
//
//   - DictionaryRouter: 字典管理路由（数据字典的增删改查）
//
//   - OperationRecordRouter: 操作记录路由（系统操作日志查询）
//
//   - DictionaryDetailRouter: 字典详情路由（字典项的增删改查）
//
//   - AuthorityBtnRouter: 按钮权限路由（按钮级别的权限控制）
//
//   - SysExportTemplateRouter: 导出模板路由（Excel 导出模板管理）
//
//   - SysParamsRouter: 系统参数路由（系统配置参数管理）
//
//   - SysVersionRouter: 版本管理路由（系统版本信息）
//
//   - SysErrorRouter: 错误管理路由（系统错误日志管理）
//
//     4. 设计优势：
//     a. 模块化：每个路由接口独立管理自己的路由逻辑，职责清晰
//     b. 可扩展性：新增功能模块时，只需定义新的路由接口并嵌入到 RouterGroup 中
//     c. 解耦：各路由模块相互独立，修改一个模块不影响其他模块
//     d. 可维护性：路由按功能分类，便于定位和修改特定功能的路由
//     e. 可测试性：可以单独测试每个路由接口的功能
//     f. 代码复用：通过嵌入复用各路由接口的方法，避免重复代码
//
//     5. 使用示例：
//     在 router/enter.go 中，通过 RouterGroupApp.System.InitApiRouter() 调用
//     由于嵌入特性，RouterGroup 可以直接调用所有嵌入结构体的方法
type RouterGroup struct {
	ApiRouter               // API 接口管理路由
	JwtRouter               // JWT 认证路由
	SysRouter               // 系统配置路由
	BaseRouter              // 基础功能路由（登录、验证码等）
	InitRouter              // 数据库初始化路由
	MenuRouter              // 菜单管理路由
	UserRouter              // 用户管理路由
	CasbinRouter            // Casbin 权限管理路由
	AutoCodeRouter          // 代码生成路由
	AuthorityRouter         // 角色权限路由
	DictionaryRouter        // 字典管理路由
	OperationRecordRouter   // 操作记录路由
	DictionaryDetailRouter  // 字典详情路由
	AuthorityBtnRouter      // 按钮权限路由
	SysExportTemplateRouter // 导出模板路由
	SysParamsRouter         // 系统参数路由
	SysVersionRouter        // 版本管理路由
	SysErrorRouter          // 错误管理路由
}

// API 接口变量定义，用于简化 API 访问路径
//
// 设计说明：
// 1. 为什么定义这些变量？
//   - 简化代码：避免在每个路由文件中重复写冗长的路径 api.ApiGroupApp.SystemApiGroup.XXX
//   - 提高可读性：使用简短的变量名（如 jwtApi、baseApi）比长路径更清晰易读
//   - 统一管理：所有 API 接口的引用集中在此处，便于统一管理和修改
//   - 性能优化：变量在包级别定义，只初始化一次，避免重复访问全局变量
//
// 2. 为什么使用包级别变量而不是函数？
//   - 包级别变量在包加载时初始化，后续访问无需函数调用开销
//   - 变量可以被多个路由文件共享，避免重复定义
//   - 符合 Go 语言的惯用法，简洁高效
//
// 3. 命名规范：
//
//   - 变量名采用驼峰命名，去掉 "Api" 后缀（如 DBApi -> dbApi）
//
//   - 保持与 API 结构体名称的对应关系，便于查找和维护
//
//     4. 好处：
//     a. 代码简洁：路由文件中直接使用 jwtApi.JsonInBlacklist，而不是 api.ApiGroupApp.SystemApiGroup.JwtApi.JsonInBlacklist
//     b. 易于维护：如果 API 路径结构发生变化，只需在此处修改一次
//     c. 类型安全：编译时检查，避免运行时错误
//     d. 性能优化：避免重复的链式访问，提高代码执行效率
var (
	dbApi               = api.ApiGroupApp.SystemApiGroup.DBApi                // 数据库操作 API
	jwtApi              = api.ApiGroupApp.SystemApiGroup.JwtApi               // JWT 认证 API
	baseApi             = api.ApiGroupApp.SystemApiGroup.BaseApi              // 基础功能 API（登录、验证码等）
	casbinApi           = api.ApiGroupApp.SystemApiGroup.CasbinApi            // Casbin 权限管理 API
	systemApi           = api.ApiGroupApp.SystemApiGroup.SystemApi            // 系统配置 API
	sysParamsApi        = api.ApiGroupApp.SystemApiGroup.SysParamsApi         // 系统参数 API
	autoCodeApi         = api.ApiGroupApp.SystemApiGroup.AutoCodeApi          // 代码生成 API
	authorityApi        = api.ApiGroupApp.SystemApiGroup.AuthorityApi         // 角色权限 API
	apiRouterApi        = api.ApiGroupApp.SystemApiGroup.SystemApiApi         // API 路由管理 API
	dictionaryApi       = api.ApiGroupApp.SystemApiGroup.DictionaryApi        // 字典管理 API
	authorityBtnApi     = api.ApiGroupApp.SystemApiGroup.AuthorityBtnApi      // 按钮权限 API
	authorityMenuApi    = api.ApiGroupApp.SystemApiGroup.AuthorityMenuApi     // 菜单权限 API
	autoCodePluginApi   = api.ApiGroupApp.SystemApiGroup.AutoCodePluginApi    // 代码生成插件 API
	autocodeHistoryApi  = api.ApiGroupApp.SystemApiGroup.AutoCodeHistoryApi   // 代码生成历史 API
	operationRecordApi  = api.ApiGroupApp.SystemApiGroup.OperationRecordApi   // 操作记录 API
	autoCodePackageApi  = api.ApiGroupApp.SystemApiGroup.AutoCodePackageApi   // 代码生成包 API
	dictionaryDetailApi = api.ApiGroupApp.SystemApiGroup.DictionaryDetailApi  // 字典详情 API
	autoCodeTemplateApi = api.ApiGroupApp.SystemApiGroup.AutoCodeTemplateApi  // 代码生成模板 API
	exportTemplateApi   = api.ApiGroupApp.SystemApiGroup.SysExportTemplateApi // 导出模板 API
	sysVersionApi       = api.ApiGroupApp.SystemApiGroup.SysVersionApi        // 版本管理 API
	sysErrorApi         = api.ApiGroupApp.SystemApiGroup.SysErrorApi          // 错误管理 API
)
