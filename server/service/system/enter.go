package system

// ServiceGroup 采用 Go 语言的嵌入结构体模式（Embedded Struct），将系统模块的所有服务聚合在一起
// 这种设计模式的意义和好处：
//
//  1. 【服务聚合统一管理】
//     将所有系统相关的服务（如 JWT、API、菜单、用户、权限等）集中在一个结构体中，
//     形成统一的系统服务入口，便于管理和维护
//
//  2. 【简化访问路径】
//     通过嵌入结构体，可以直接通过 ServiceGroup 访问各个服务的方法，无需通过中间字段
//     例如：service.ServiceGroupApp.SystemServiceGroup.ApiService.GetApis()
//     而不是：service.ServiceGroupApp.SystemServiceGroup.Services.ApiService.GetApis()
//     这样访问路径更短、更直观
//
//  3. 【代码组织清晰】
//     将同一业务模块（system）的所有服务集中管理，与 example 等其他业务模块的服务分离
//     每个模块都有独立的 ServiceGroup，保持了代码的模块化和层次化
//
//  4. 【类型安全和编译检查】
//     Go 语言的类型系统会在编译时检查所有嵌入的服务类型是否正确，
//     如果某个服务类型不存在或方法签名不匹配，编译时就会报错，避免了运行时错误
//
//  5. 【易于扩展和维护】
//     新增系统服务时，只需在此结构体中添加一个新的嵌入字段即可
//     例如：添加新的 NotificationService，只需添加一行：NotificationService
//     无需修改其他现有代码，符合开闭原则（对扩展开放，对修改关闭）
//
//  6. 【依赖注入友好】
//     可以通过统一的方式初始化和管理所有服务，便于后续集成依赖注入框架
//     所有服务的生命周期可以统一管理，降低了耦合度
//
//  7. 【便于测试】
//     可以轻松地为整个 ServiceGroup 创建 mock 对象，也可以单独测试某个服务
//     测试时可以通过替换整个 ServiceGroup 来实现服务层的隔离测试
//
//  8. 【清晰的依赖关系】
//     上层代码（API 层）通过 service.ServiceGroupApp.SystemServiceGroup 访问服务，
//     明确的访问路径使得代码的依赖关系一目了然，便于理解和维护
type ServiceGroup struct {
	JwtService                                // JWT 服务：处理令牌黑名单、过期验证等
	ApiService                                // API 服务：管理接口的增删改查
	MenuService                               // 菜单服务：处理菜单的 CRUD 操作
	UserService                               // 用户服务：用户管理相关业务逻辑
	CasbinService                             // Casbin 服务：基于 Casbin 的权限控制
	InitDBService                             // 数据库初始化服务：处理数据库的初始化和迁移
	AutoCodeService                           // 自动代码生成服务：根据数据库表自动生成 CRUD 代码
	BaseMenuService                           // 基础菜单服务：管理基础菜单配置
	AuthorityService                          // 权限服务：角色、权限管理
	DictionaryService                         // 字典服务：系统字典管理
	SystemConfigService                       // 系统配置服务：系统参数配置管理
	OperationRecordService                    // 操作记录服务：记录用户操作日志
	DictionaryDetailService                   // 字典详情服务：字典项明细管理
	AuthorityBtnService                       // 权限按钮服务：按钮级权限控制
	SysExportTemplateService                  // 导出模板服务：Excel 等导出模板管理
	SysParamsService                          // 系统参数服务：系统参数配置
	SysVersionService                         // 版本服务：系统版本管理
	AutoCodePlugin           autoCodePlugin   // 自动代码插件服务：插件相关代码生成
	AutoCodePackage          autoCodePackage  // 自动代码包服务：代码包管理
	AutoCodeHistory          autoCodeHistory  // 自动代码历史服务：代码生成历史记录
	AutoCodeTemplate         autoCodeTemplate // 自动代码模板服务：代码生成模板管理
	SysErrorService                           // 系统错误服务：系统错误记录和管理
}
