package system

import "github.com/flipped-aurora/gin-vue-admin/server/service"

// ApiGroup 是系统管理模块的 API 组
// 这是整个系统的核心模块，包含用户、权限、菜单、字典等基础功能
// 设计模式：组合模式 + 单一职责原则
//
// 设计优势：
// 1. 模块化：每个 API 控制器负责一个业务领域，职责清晰
// 2. 可扩展：新增功能只需添加新的 API 控制器，无需修改现有代码
// 3. 统一管理：所有系统 API 通过一个结构体管理，便于路由注册
// 4. 依赖注入：通过全局变量注入 service 依赖，降低耦合
//
// 模块说明：
// - BaseApi: 基础功能（登录、注册、用户信息等）
// - UserApi: 用户管理（通过 BaseApi 实现）
// - AuthorityApi: 权限管理（角色、权限等）
// - AuthorityMenuApi: 菜单管理（动态路由、菜单权限等）
// - DictionaryApi: 字典管理（系统字典、字典项等）
// - SystemApi: 系统配置（系统参数、系统信息等）
// - AutoCodeApi: 代码生成（自动生成 CRUD 代码）
// - OperationRecordApi: 操作记录（审计日志）
// - CasbinApi: 权限控制（基于 Casbin 的 RBAC）
// - JwtApi: JWT 管理（token 黑名单等）
type ApiGroup struct {
	DBApi                // 数据库初始化
	JwtApi               // JWT token 管理
	BaseApi              // 基础功能（登录、注册、用户信息）
	SystemApi            // 系统配置管理
	CasbinApi            // Casbin 权限控制
	AutoCodeApi          // 代码自动生成
	SystemApiApi         // API 接口管理
	AuthorityApi         // 权限（角色）管理
	DictionaryApi        // 字典管理
	AuthorityMenuApi     // 菜单权限管理
	OperationRecordApi   // 操作记录（审计日志）
	DictionaryDetailApi  // 字典项管理
	AuthorityBtnApi      // 按钮权限管理
	SysExportTemplateApi // 导出模板管理
	AutoCodePluginApi    // 代码生成插件
	AutoCodePackageApi   // 代码生成包管理
	AutoCodeHistoryApi   // 代码生成历史
	AutoCodeTemplateApi  // 代码生成模板
	SysParamsApi         // 系统参数管理
	SysVersionApi        // 系统版本管理
	SysErrorApi          // 系统错误管理
}

// 服务层依赖注入
// 设计模式：依赖注入（Dependency Injection）+ 单例模式
//
// 为什么这么写：
// 1. 集中管理：所有 service 依赖集中在此处，便于查看和维护
// 2. 单例模式：所有 API 方法共享同一个 service 实例，节省内存
// 3. 解耦合：API 层不直接创建 service，降低耦合度
// 4. 易于测试：可以轻松替换为 mock service 进行单元测试
// 5. 延迟初始化：在应用启动时初始化，避免循环依赖
//
// 命名规范：
// - 变量名 = service 名 + "Service"
// - 例如：userService 对应 UserService
//
// 使用方式：
// - 在 API 方法中直接使用这些变量，无需再次获取
// - 例如：userService.GetUserInfo(id)
var (
	apiService              = service.ServiceGroupApp.SystemServiceGroup.ApiService
	jwtService              = service.ServiceGroupApp.SystemServiceGroup.JwtService
	menuService             = service.ServiceGroupApp.SystemServiceGroup.MenuService
	userService             = service.ServiceGroupApp.SystemServiceGroup.UserService
	initDBService           = service.ServiceGroupApp.SystemServiceGroup.InitDBService
	casbinService           = service.ServiceGroupApp.SystemServiceGroup.CasbinService
	baseMenuService         = service.ServiceGroupApp.SystemServiceGroup.BaseMenuService
	authorityService        = service.ServiceGroupApp.SystemServiceGroup.AuthorityService
	dictionaryService       = service.ServiceGroupApp.SystemServiceGroup.DictionaryService
	authorityBtnService     = service.ServiceGroupApp.SystemServiceGroup.AuthorityBtnService
	systemConfigService     = service.ServiceGroupApp.SystemServiceGroup.SystemConfigService
	sysParamsService        = service.ServiceGroupApp.SystemServiceGroup.SysParamsService
	operationRecordService  = service.ServiceGroupApp.SystemServiceGroup.OperationRecordService
	dictionaryDetailService = service.ServiceGroupApp.SystemServiceGroup.DictionaryDetailService
	autoCodeService         = service.ServiceGroupApp.SystemServiceGroup.AutoCodeService
	autoCodePluginService   = service.ServiceGroupApp.SystemServiceGroup.AutoCodePlugin
	autoCodePackageService  = service.ServiceGroupApp.SystemServiceGroup.AutoCodePackage
	autoCodeHistoryService  = service.ServiceGroupApp.SystemServiceGroup.AutoCodeHistory
	autoCodeTemplateService = service.ServiceGroupApp.SystemServiceGroup.AutoCodeTemplate
	sysVersionService       = service.ServiceGroupApp.SystemServiceGroup.SysVersionService
	sysErrorService         = service.ServiceGroupApp.SystemServiceGroup.SysErrorService
)
