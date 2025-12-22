package system

import (
	"context"

	adapter "github.com/casbin/gorm-adapter/v3"
	"github.com/flipped-aurora/gin-vue-admin/server/service/system"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// initOrderCasbin 定义 Casbin 权限表的初始化顺序
// 设置为 initOrderApiIgnore + 1 确保 API 忽略表先于 Casbin 表初始化
// 这样设计的好处：
// 1. 明确依赖关系：Casbin 权限规则可能依赖 API 列表，必须先有 API 数据才能建立权限映射
// 2. 保证初始化顺序：系统会按照 order 值排序执行，确保依赖表先创建
// 3. 易于维护：新增初始化器只需设置相对顺序，无需修改全局配置
// 4. 支持多依赖：如果依赖多个表，可以使用 initOrderA + initOrderB 的形式
const initOrderCasbin = initOrderApiIgnore + 1

// initCasbin Casbin 权限规则表初始化器
// 实现 SubInitializer 接口，遵循插件化初始化架构
// 使用结构体而不是函数的好处：
// 1. 可以保存状态（如果需要）：未来可以添加配置或缓存等字段
// 2. 可以实现接口方法：符合 Go 的接口设计模式，编译期检查接口实现
// 3. 便于扩展：可以添加辅助方法，如权限规则验证、批量导入等
// 4. 类型安全：编译期检查，避免方法签名错误
type initCasbin struct{}

// init 包初始化函数，在导入包时自动执行
// 这种自动注册模式的好处：
// 1. 零配置：导入包即自动注册，无需手动调用注册函数
// 2. 解耦：初始化逻辑与注册逻辑分离，符合单一职责原则
// 3. 可扩展：新增初始化器只需实现接口并注册，框架会自动处理
// 4. 依赖管理：通过 initOrder 自动处理初始化顺序，无需手动管理依赖链
// 5. 类型安全：编译期检查接口实现，避免运行时错误
func init() {
	system.RegisterInit(initOrderCasbin, &initCasbin{})
}

// MigrateTable 执行 Casbin 权限规则表的数据库表结构迁移
// 参数：ctx - 上下文，包含数据库连接等初始化所需信息
// 返回：next context - 可以用于传递数据给后续初始化步骤
//
//	error - 迁移过程中的错误
//
// 设计说明：
//  1. 使用 context 传递 db 连接，避免全局变量，提高可测试性和并发安全
//  2. 类型断言 + ok 模式确保安全获取数据库连接，避免 panic
//  3. AutoMigrate 自动根据模型结构创建/更新表结构，支持迭代开发
//     好处：无需手动编写 SQL，模型变更自动同步到数据库
//  4. 使用 adapter.CasbinRule 作为模型，这是 gorm-adapter 提供的标准 Casbin 规则表结构
//     好处：与 Casbin 库完美集成，支持策略持久化
func (i *initCasbin) MigrateTable(ctx context.Context) (context.Context, error) {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return ctx, system.ErrMissingDBContext
	}
	return ctx, db.AutoMigrate(&adapter.CasbinRule{})
}

// TableCreated 检查 Casbin 权限规则表是否已创建
// 参数：ctx - 上下文，包含数据库连接
// 返回：bool - 表是否存在
//
// 设计说明：
//  1. 幂等性检查：用于判断是否需要执行表创建，避免重复创建
//  2. 安全获取数据库连接：使用类型断言 + ok 模式，失败时返回 false 而不是 panic
//  3. 使用 GORM Migrator 的 HasTable 方法：标准化的表存在性检查
//     好处：跨数据库兼容，支持 MySQL、PostgreSQL、SQLite 等
//  4. 返回 false 而不是错误：简化调用方逻辑，只需判断布尔值
func (i *initCasbin) TableCreated(ctx context.Context) bool {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return false
	}
	return db.Migrator().HasTable(&adapter.CasbinRule{})
}

// InitializerName 返回初始化器的唯一标识名称
// 返回：string - Casbin 规则表的表名
//
// 设计说明：
//  1. 使用表名作为标识：语义清晰，直接对应数据库表
//  2. 通过实体方法获取表名：避免硬编码，表名变更时自动同步
//     好处：如果表名规则改变（如添加前缀），只需修改模型定义
//  3. 用于日志输出：标识当前初始化的模块，便于调试和追踪
//  4. 用于 Context 存储：作为 key 存储初始化后的数据，供其他初始化器使用
//     好处：后续初始化器可以通过表名获取已初始化的权限规则数据
func (i *initCasbin) InitializerName() string {
	var entity adapter.CasbinRule
	return entity.TableName()
}

// InitializeData 初始化 Casbin 权限规则表的默认数据
// 参数：ctx - 上下文，包含数据库连接
// 返回：next context - 包含初始化后的权限规则数据，供后续初始化器使用
//
//	error - 初始化过程中的错误
//
// 设计说明：
// 1. 权限规则结构说明（CasbinRule）：
//   - Ptype: "p" 表示策略（Policy），"g" 表示角色继承（Group）
//   - V0: 主体（Subject），这里是角色ID（如 "888"、"8881"、"9528"）
//   - V1: 对象（Object），这里是 API 路径（如 "/user/getUserInfo"）
//   - V2: 动作（Action），这里是 HTTP 方法（如 "GET"、"POST"、"PUT"、"DELETE"）
//     规则含义：角色 V0 可以对资源 V1 执行动作 V2
//
// 2. 三个默认角色的权限设计：
//   - "888": 超级管理员，拥有所有权限（最完整的权限集合）
//   - "8881": 普通管理员，拥有大部分管理权限，但权限范围小于超级管理员
//   - "9528": 普通用户，拥有基础权限，主要用于日常操作
//     好处：开箱即用的权限体系，满足不同角色的需求
//
// 3. 批量创建权限规则：
//   - 使用切片一次性定义所有规则，代码集中管理，易于维护
//   - 使用 db.Create(&entities) 批量插入，性能优于逐条插入
//     好处：事务性操作，要么全部成功，要么全部失败，保证数据一致性
//
// 4. Context 传递初始化数据：
//   - 将初始化后的实体存储到 context 中，供后续初始化器使用
//     好处：其他模块（如用户初始化）可以获取默认权限，建立关联关系
//
// 5. 错误处理：
//   - 使用 errors.Wrap 包装错误，保留错误堆栈信息
//   - 错误信息包含表名，便于定位问题
func (i *initCasbin) InitializeData(ctx context.Context) (context.Context, error) {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return ctx, system.ErrMissingDBContext
	}
	// 定义默认权限规则：三个角色的完整权限配置
	// 规则格式：{Ptype: "p", V0: "角色ID", V1: "API路径", V2: "HTTP方法"}
	entities := []adapter.CasbinRule{
		// ========== 超级管理员（888）权限配置 ==========
		// 用户管理模块权限
		{Ptype: "p", V0: "888", V1: "/user/admin_register", V2: "POST"},

		// API 管理模块权限 - 完整的 CRUD 操作
		// 设计说明：API 管理是系统核心功能，需要完整的增删改查权限
		// 好处：管理员可以灵活配置系统 API，实现动态权限控制
		{Ptype: "p", V0: "888", V1: "/api/createApi", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/api/getApiList", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/api/getApiById", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/api/deleteApi", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/api/updateApi", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/api/getAllApis", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/api/deleteApisByIds", V2: "DELETE"},
		{Ptype: "p", V0: "888", V1: "/api/syncApi", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/api/getApiGroups", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/api/enterSyncApi", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/api/ignoreApi", V2: "POST"},

		// 权限管理模块权限 - 角色和权限的完整管理
		// 设计说明：权限管理是 RBAC 核心，需要完整的权限配置能力
		// 好处：支持角色复制、权限分配等高级功能，提高权限管理效率
		{Ptype: "p", V0: "888", V1: "/authority/copyAuthority", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/authority/updateAuthority", V2: "PUT"},
		{Ptype: "p", V0: "888", V1: "/authority/createAuthority", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/authority/deleteAuthority", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/authority/getAuthorityList", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/authority/setDataAuthority", V2: "POST"},

		// 菜单管理模块权限 - 系统菜单的完整管理
		// 设计说明：菜单是前端展示的基础，需要完整的菜单树管理能力
		// 好处：支持动态菜单配置，实现灵活的界面布局
		{Ptype: "p", V0: "888", V1: "/menu/getMenu", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/menu/getMenuList", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/menu/addBaseMenu", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/menu/getBaseMenuTree", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/menu/addMenuAuthority", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/menu/getMenuAuthority", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/menu/deleteBaseMenu", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/menu/updateBaseMenu", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/menu/getBaseMenuById", V2: "POST"},

		// 用户管理模块权限 - 用户信息的完整管理
		// 设计说明：用户管理是系统基础功能，需要完整的用户操作权限
		// 好处：支持用户信息查询、修改、密码重置等完整功能
		{Ptype: "p", V0: "888", V1: "/user/getUserInfo", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/user/setUserInfo", V2: "PUT"},
		{Ptype: "p", V0: "888", V1: "/user/setSelfInfo", V2: "PUT"},
		{Ptype: "p", V0: "888", V1: "/user/getUserList", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/user/deleteUser", V2: "DELETE"},
		{Ptype: "p", V0: "888", V1: "/user/changePassword", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/user/setUserAuthority", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/user/setUserAuthorities", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/user/resetPassword", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/user/setSelfSetting", V2: "PUT"},

		// 文件管理模块权限 - 文件上传下载的完整管理
		// 设计说明：文件管理支持普通上传和断点续传，需要完整的文件操作权限
		// 好处：支持大文件上传、文件管理、URL 导入等高级功能
		{Ptype: "p", V0: "888", V1: "/fileUploadAndDownload/findFile", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/fileUploadAndDownload/breakpointContinueFinish", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/fileUploadAndDownload/breakpointContinue", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/fileUploadAndDownload/removeChunk", V2: "POST"},

		{Ptype: "p", V0: "888", V1: "/fileUploadAndDownload/upload", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/fileUploadAndDownload/deleteFile", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/fileUploadAndDownload/editFileName", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/fileUploadAndDownload/getFileList", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/fileUploadAndDownload/importURL", V2: "POST"},

		// Casbin 权限管理模块权限 - 权限规则的动态管理
		// 设计说明：支持动态更新权限规则，无需重启系统
		// 好处：权限变更实时生效，提高系统灵活性
		{Ptype: "p", V0: "888", V1: "/casbin/updateCasbin", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/casbin/getPolicyPathByAuthorityId", V2: "POST"},

		// JWT 令牌管理权限 - 支持令牌黑名单功能
		// 设计说明：JWT 黑名单用于实现登出功能，使令牌立即失效
		// 好处：增强安全性，支持强制下线功能
		{Ptype: "p", V0: "888", V1: "/jwt/jsonInBlacklist", V2: "POST"},

		// 系统配置模块权限 - 系统参数和服务器信息管理
		// 设计说明：系统配置影响全局行为，需要管理员权限
		// 好处：支持动态配置系统参数，无需修改代码
		{Ptype: "p", V0: "888", V1: "/system/getSystemConfig", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/system/setSystemConfig", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/system/getServerInfo", V2: "POST"},

		// 客户管理模块权限 - 示例业务模块的完整 CRUD
		// 设计说明：这是示例模块，展示如何为业务模块配置权限
		// 好处：提供标准的权限配置模板，便于扩展新业务模块
		{Ptype: "p", V0: "888", V1: "/customer/customer", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/customer/customer", V2: "PUT"},
		{Ptype: "p", V0: "888", V1: "/customer/customer", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/customer/customer", V2: "DELETE"},
		{Ptype: "p", V0: "888", V1: "/customer/customerList", V2: "GET"},

		// 代码生成模块权限 - 自动化代码生成工具
		// 设计说明：代码生成是开发工具，需要完整的操作权限
		// 好处：支持从数据库表自动生成 CRUD 代码，提高开发效率
		// 包括：数据库连接、元数据获取、代码预览、模板管理、插件管理等
		{Ptype: "p", V0: "888", V1: "/autoCode/getDB", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/autoCode/getMeta", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/autoCode/preview", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/autoCode/getTables", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/autoCode/getColumn", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/autoCode/rollback", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/autoCode/createTemp", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/autoCode/delSysHistory", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/autoCode/getSysHistory", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/autoCode/createPackage", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/autoCode/getTemplates", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/autoCode/getPackage", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/autoCode/delPackage", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/autoCode/createPlug", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/autoCode/installPlugin", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/autoCode/pubPlug", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/autoCode/addFunc", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/autoCode/mcp", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/autoCode/mcpTest", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/autoCode/mcpList", V2: "POST"},

		// 字典管理模块权限 - 系统字典的完整管理
		// 设计说明：字典是系统配置数据，需要完整的增删改查权限
		// 好处：支持树形字典、类型查询、路径查询等高级功能
		{Ptype: "p", V0: "888", V1: "/sysDictionaryDetail/findSysDictionaryDetail", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/sysDictionaryDetail/updateSysDictionaryDetail", V2: "PUT"},
		{Ptype: "p", V0: "888", V1: "/sysDictionaryDetail/createSysDictionaryDetail", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/sysDictionaryDetail/getSysDictionaryDetailList", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/sysDictionaryDetail/deleteSysDictionaryDetail", V2: "DELETE"},
		{Ptype: "p", V0: "888", V1: "/sysDictionaryDetail/getDictionaryTreeList", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/sysDictionaryDetail/getDictionaryTreeListByType", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/sysDictionaryDetail/getDictionaryDetailsByParent", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/sysDictionaryDetail/getDictionaryPath", V2: "GET"},

		// 字典类型管理模块权限 - 字典分类的完整管理
		// 设计说明：字典类型是字典的分类，需要完整的类型管理权限
		// 好处：支持字典的导入导出，便于系统配置的迁移
		{Ptype: "p", V0: "888", V1: "/sysDictionary/findSysDictionary", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/sysDictionary/updateSysDictionary", V2: "PUT"},
		{Ptype: "p", V0: "888", V1: "/sysDictionary/getSysDictionaryList", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/sysDictionary/createSysDictionary", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/sysDictionary/deleteSysDictionary", V2: "DELETE"},
		{Ptype: "p", V0: "888", V1: "/sysDictionary/importSysDictionary", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/sysDictionary/exportSysDictionary", V2: "GET"},

		// 操作日志模块权限 - 系统操作记录的查询和管理
		// 设计说明：操作日志用于审计和问题追踪，需要完整的查询和清理权限
		// 好处：支持批量删除，便于日志管理和系统维护
		{Ptype: "p", V0: "888", V1: "/sysOperationRecord/findSysOperationRecord", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/sysOperationRecord/updateSysOperationRecord", V2: "PUT"},
		{Ptype: "p", V0: "888", V1: "/sysOperationRecord/createSysOperationRecord", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/sysOperationRecord/getSysOperationRecordList", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/sysOperationRecord/deleteSysOperationRecord", V2: "DELETE"},
		{Ptype: "p", V0: "888", V1: "/sysOperationRecord/deleteSysOperationRecordByIds", V2: "DELETE"},

		// 邮件管理模块权限 - 邮件发送和测试功能
		// 设计说明：邮件功能用于系统通知，需要发送和测试权限
		// 好处：支持邮件配置测试，确保邮件功能正常
		{Ptype: "p", V0: "888", V1: "/email/emailTest", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/email/sendEmail", V2: "POST"},

		// 简单上传模块权限 - 支持 MD5 校验的断点续传
		// 设计说明：简单上传器支持文件 MD5 校验和断点续传
		// 好处：提高大文件上传的可靠性和效率
		{Ptype: "p", V0: "888", V1: "/simpleUploader/upload", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/simpleUploader/checkFileMd5", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/simpleUploader/mergeFileMd5", V2: "GET"},

		// 按钮权限管理模块权限 - 细粒度的按钮级权限控制
		// 设计说明：按钮权限是页面级权限的细化，支持更精细的权限控制
		// 好处：可以实现同一页面不同按钮对不同角色的可见性控制
		{Ptype: "p", V0: "888", V1: "/authorityBtn/setAuthorityBtn", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/authorityBtn/getAuthorityBtn", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/authorityBtn/canRemoveAuthorityBtn", V2: "POST"},

		// Excel 导出模板模块权限 - 动态导出模板的完整管理
		// 设计说明：导出模板支持 SQL 查询和 Excel 导出，需要完整的模板管理权限
		// 好处：支持动态配置导出模板，无需修改代码即可调整导出格式
		{Ptype: "p", V0: "888", V1: "/sysExportTemplate/createSysExportTemplate", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/sysExportTemplate/deleteSysExportTemplate", V2: "DELETE"},
		{Ptype: "p", V0: "888", V1: "/sysExportTemplate/deleteSysExportTemplateByIds", V2: "DELETE"},
		{Ptype: "p", V0: "888", V1: "/sysExportTemplate/updateSysExportTemplate", V2: "PUT"},
		{Ptype: "p", V0: "888", V1: "/sysExportTemplate/findSysExportTemplate", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/sysExportTemplate/getSysExportTemplateList", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/sysExportTemplate/exportExcel", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/sysExportTemplate/exportTemplate", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/sysExportTemplate/previewSQL", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/sysExportTemplate/importExcel", V2: "POST"},

		// 系统错误管理模块权限 - 错误记录和解决方案管理
		// 设计说明：错误管理用于问题追踪和解决方案管理
		// 好处：支持错误解决方案的积累，提高问题处理效率
		{Ptype: "p", V0: "888", V1: "/sysError/createSysError", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/sysError/deleteSysError", V2: "DELETE"},
		{Ptype: "p", V0: "888", V1: "/sysError/deleteSysErrorByIds", V2: "DELETE"},
		{Ptype: "p", V0: "888", V1: "/sysError/updateSysError", V2: "PUT"},
		{Ptype: "p", V0: "888", V1: "/sysError/findSysError", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/sysError/getSysErrorList", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/sysError/getSysErrorSolution", V2: "GET"},

		// 信息管理模块权限 - 通用信息管理的完整 CRUD
		// 设计说明：这是通用信息管理模块，可用于各种业务场景
		// 好处：提供标准的信息管理模板，便于快速开发新功能
		{Ptype: "p", V0: "888", V1: "/info/createInfo", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/info/deleteInfo", V2: "DELETE"},
		{Ptype: "p", V0: "888", V1: "/info/deleteInfoByIds", V2: "DELETE"},
		{Ptype: "p", V0: "888", V1: "/info/updateInfo", V2: "PUT"},
		{Ptype: "p", V0: "888", V1: "/info/findInfo", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/info/getInfoList", V2: "GET"},

		// 系统参数管理模块权限 - 系统配置参数的完整管理
		// 设计说明：系统参数用于存储系统配置，需要完整的参数管理权限
		// 好处：支持动态配置系统参数，无需修改代码和重启服务
		{Ptype: "p", V0: "888", V1: "/sysParams/createSysParams", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/sysParams/deleteSysParams", V2: "DELETE"},
		{Ptype: "p", V0: "888", V1: "/sysParams/deleteSysParamsByIds", V2: "DELETE"},
		{Ptype: "p", V0: "888", V1: "/sysParams/updateSysParams", V2: "PUT"},
		{Ptype: "p", V0: "888", V1: "/sysParams/findSysParams", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/sysParams/getSysParamsList", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/sysParams/getSysParam", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/attachmentCategory/getCategoryList", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/attachmentCategory/addCategory", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/attachmentCategory/deleteCategory", V2: "POST"},

		// 系统版本管理模块权限 - 版本信息的完整管理
		// 设计说明：版本管理用于记录系统版本变更，支持导入导出
		// 好处：支持版本信息的迁移和备份，便于版本管理
		{Ptype: "p", V0: "888", V1: "/sysVersion/findSysVersion", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/sysVersion/getSysVersionList", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/sysVersion/downloadVersionJson", V2: "GET"},
		{Ptype: "p", V0: "888", V1: "/sysVersion/exportVersion", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/sysVersion/importVersion", V2: "POST"},
		{Ptype: "p", V0: "888", V1: "/sysVersion/deleteSysVersion", V2: "DELETE"},
		{Ptype: "p", V0: "888", V1: "/sysVersion/deleteSysVersionByIds", V2: "DELETE"},

		// ========== 普通管理员（8881）权限配置 ==========
		// 设计说明：普通管理员权限是超级管理员的子集，拥有大部分管理权限
		// 但权限范围小于超级管理员，适合分配给部门管理员或二级管理员
		// 好处：实现权限分级管理，避免权限过度集中
		{Ptype: "p", V0: "8881", V1: "/user/admin_register", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/api/createApi", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/api/getApiList", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/api/getApiById", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/api/deleteApi", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/api/updateApi", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/api/getAllApis", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/authority/createAuthority", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/authority/deleteAuthority", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/authority/getAuthorityList", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/authority/setDataAuthority", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/menu/getMenu", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/menu/getMenuList", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/menu/addBaseMenu", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/menu/getBaseMenuTree", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/menu/addMenuAuthority", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/menu/getMenuAuthority", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/menu/deleteBaseMenu", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/menu/updateBaseMenu", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/menu/getBaseMenuById", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/user/changePassword", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/user/getUserList", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/user/setUserAuthority", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/fileUploadAndDownload/upload", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/fileUploadAndDownload/getFileList", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/fileUploadAndDownload/deleteFile", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/fileUploadAndDownload/editFileName", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/fileUploadAndDownload/importURL", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/casbin/updateCasbin", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/casbin/getPolicyPathByAuthorityId", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/jwt/jsonInBlacklist", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/system/getSystemConfig", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/system/setSystemConfig", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/customer/customer", V2: "POST"},
		{Ptype: "p", V0: "8881", V1: "/customer/customer", V2: "PUT"},
		{Ptype: "p", V0: "8881", V1: "/customer/customer", V2: "DELETE"},
		{Ptype: "p", V0: "8881", V1: "/customer/customer", V2: "GET"},
		{Ptype: "p", V0: "8881", V1: "/customer/customerList", V2: "GET"},
		{Ptype: "p", V0: "8881", V1: "/user/getUserInfo", V2: "GET"},

		// ========== 普通用户（9528）权限配置 ==========
		// 设计说明：普通用户权限是最基础的权限集合，主要用于日常操作
		// 权限范围最小，适合分配给普通业务人员
		// 好处：最小权限原则，降低安全风险，只授予必要的操作权限
		{Ptype: "p", V0: "9528", V1: "/user/admin_register", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/api/createApi", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/api/getApiList", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/api/getApiById", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/api/deleteApi", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/api/updateApi", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/api/getAllApis", V2: "POST"},

		{Ptype: "p", V0: "9528", V1: "/authority/createAuthority", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/authority/deleteAuthority", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/authority/getAuthorityList", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/authority/setDataAuthority", V2: "POST"},

		{Ptype: "p", V0: "9528", V1: "/menu/getMenu", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/menu/getMenuList", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/menu/addBaseMenu", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/menu/getBaseMenuTree", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/menu/addMenuAuthority", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/menu/getMenuAuthority", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/menu/deleteBaseMenu", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/menu/updateBaseMenu", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/menu/getBaseMenuById", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/user/changePassword", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/user/getUserList", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/user/setUserAuthority", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/fileUploadAndDownload/upload", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/fileUploadAndDownload/getFileList", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/fileUploadAndDownload/deleteFile", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/fileUploadAndDownload/editFileName", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/fileUploadAndDownload/importURL", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/casbin/updateCasbin", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/casbin/getPolicyPathByAuthorityId", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/jwt/jsonInBlacklist", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/system/getSystemConfig", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/system/setSystemConfig", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/customer/customer", V2: "PUT"},
		{Ptype: "p", V0: "9528", V1: "/customer/customer", V2: "GET"},
		{Ptype: "p", V0: "9528", V1: "/customer/customer", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/customer/customer", V2: "DELETE"},
		{Ptype: "p", V0: "9528", V1: "/customer/customerList", V2: "GET"},
		{Ptype: "p", V0: "9528", V1: "/autoCode/createTemp", V2: "POST"},
		{Ptype: "p", V0: "9528", V1: "/user/getUserInfo", V2: "GET"},
	}

	// 批量插入权限规则到数据库
	// 设计说明：
	// 1. 使用 db.Create(&entities) 批量插入，性能优于逐条插入
	//    好处：减少数据库交互次数，提高初始化速度
	// 2. 事务性操作：GORM 的 Create 方法在事务中执行
	//    好处：要么全部成功，要么全部失败，保证数据一致性
	// 3. 错误包装：使用 errors.Wrap 保留错误堆栈信息
	//    好处：错误信息包含调用链，便于问题定位
	if err := db.Create(&entities).Error; err != nil {
		return ctx, errors.Wrap(err, "Casbin 表 ("+i.InitializerName()+") 数据初始化失败!")
	}

	// 将初始化后的权限规则存储到 context 中
	// 设计说明：
	// 1. 使用 context.WithValue 传递初始化数据
	//    好处：后续初始化器可以通过 InitializerName() 获取这些数据
	// 2. 使用表名作为 key，避免 key 冲突
	//    好处：语义清晰，便于其他模块获取数据
	// 3. 返回新的 context，不修改原 context
	//    好处：遵循 Go 的 context 使用规范，避免副作用
	next := context.WithValue(ctx, i.InitializerName(), entities)
	return next, nil
}

// DataInserted 检查 Casbin 权限规则表的默认数据是否已插入
// 参数：ctx - 上下文，包含数据库连接
// 返回：bool - 数据是否已存在
//
// 设计说明：
//  1. 幂等性检查：用于判断是否需要执行数据初始化，避免重复插入
//     好处：支持多次初始化，不会产生重复数据
//  2. 使用代表性数据检查：选择普通用户（9528）的一个权限规则作为检查点
//     设计原因：
//     - 9528 是最后一个初始化的角色，如果它存在，说明前面的数据都已插入
//     - 选择 "/user/getUserInfo" 是因为这是基础权限，必然存在
//     好处：只需检查一条记录，性能优于检查所有记录
//  3. 使用 errors.Is 判断错误类型：精确匹配 gorm.ErrRecordNotFound
//     好处：区分"记录不存在"和"其他错误"，避免误判
//  4. 返回 false 而不是错误：简化调用方逻辑，只需判断布尔值
//     好处：符合 Go 的布尔检查习惯，代码更简洁
func (i *initCasbin) DataInserted(ctx context.Context) bool {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return false
	}
	// 检查普通用户（9528）的基础权限是否存在
	// 如果这条记录存在，说明所有角色的权限规则都已初始化完成
	if errors.Is(db.Where(adapter.CasbinRule{Ptype: "p", V0: "9528", V1: "/user/getUserInfo", V2: "GET"}).
		First(&adapter.CasbinRule{}).Error, gorm.ErrRecordNotFound) {
		return false
	}
	return true
}
