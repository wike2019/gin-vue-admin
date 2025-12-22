package system

import (
	"context"

	sysModel "github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/service/system"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// initApi API表初始化器
// 实现 SubInitializer 接口，遵循插件化初始化架构
// 使用结构体而不是函数的好处：
// 1. 可以实现接口方法，符合 Go 的接口设计模式
// 2. 类型安全：编译期检查接口实现，避免运行时错误
// 3. 便于扩展：未来可以添加配置或缓存等字段
// 4. 符合面向对象设计：方法可以接收者，更易于组织和维护
type initApi struct{}

// initOrderApi 定义API表的初始化顺序
// 设置为 InitOrderSystem + 1 确保系统基础表先于API表初始化
// 这样设计的好处：
// 1. 明确依赖关系：通过相对顺序表达初始化依赖，语义清晰
// 2. 保证初始化顺序：系统会按照 order 值排序执行，确保依赖表先创建
// 3. 易于维护：新增初始化器只需设置相对顺序，无需修改全局配置
// 4. 支持多依赖：如果依赖多个表，可以使用 initOrderA + initOrderB 的形式
const initOrderApi = system.InitOrderSystem + 1

// init 包初始化函数，在导入包时自动执行
// 这种自动注册模式的好处：
// 1. 零配置：导入包即自动注册，无需手动调用注册函数
// 2. 解耦：初始化逻辑与注册逻辑分离，符合单一职责原则
// 3. 可扩展：新增初始化器只需实现接口并注册，框架会自动处理
// 4. 依赖管理：通过 initOrder 自动处理初始化顺序，无需手动管理依赖链
// 5. 类型安全：编译期检查接口实现，避免运行时错误
// 6. 防止遗漏：通过包导入机制确保所有初始化器都会被注册
func init() {
	system.RegisterInit(initOrderApi, &initApi{})
}

// InitializerName 返回初始化器的唯一标识名称
// 用于：
// 1. 日志输出：标识当前初始化的模块，便于调试和追踪
// 2. Context 存储：作为 key 存储初始化后的数据，供其他初始化器使用
// 3. 去重检查：防止同名初始化器重复注册，在 RegisterInit 时会检查
//
// 使用表名作为标识的好处：
// - 语义清晰：直接对应数据库表，一目了然
// - 自动获取：通过模型方法获取，避免硬编码字符串，减少拼写错误
// - 类型安全：编译期检查，如果表名变更，编译时就能发现
func (i *initApi) InitializerName() string {
	return sysModel.SysApi{}.TableName()
}

// MigrateTable 执行数据库表结构迁移
// 参数：ctx - 上下文，包含数据库连接等初始化所需信息
// 返回：next context - 可以用于传递数据给后续初始化步骤
//
//	error - 迁移过程中的错误
//
// 设计说明：
// 1. 使用 context 传递 db 连接，避免全局变量，提高可测试性
//   - 好处：可以轻松模拟数据库连接进行单元测试
//   - 好处：减少全局状态，代码更清晰、更安全
//
// 2. 类型断言 + ok 模式确保安全获取数据库连接
//   - 好处：避免 panic，优雅处理错误情况
//   - 好处：如果 context 中没有 db，返回明确的错误信息
//
// 3. AutoMigrate 自动根据模型结构创建/更新表结构，支持迭代开发
//   - 好处：模型结构变更时，自动同步到数据库，无需手动写 SQL
//   - 好处：支持版本升级时的表结构迁移
//   - 好处：开发阶段可以快速迭代，无需关注 DDL 细节
func (i *initApi) MigrateTable(ctx context.Context) (context.Context, error) {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return ctx, system.ErrMissingDBContext
	}
	return ctx, db.AutoMigrate(&sysModel.SysApi{})
}

// TableCreated 检查表是否已存在
// 用于判断是否需要执行表迁移和数据初始化
//
// 返回值：bool - true 表示表已存在，false 表示不存在
//
// 设计说明：
// 1. 幂等性检查：避免重复初始化导致的错误
//   - 好处：可以安全地多次执行初始化流程
//   - 好处：支持增量初始化，已存在的表可以跳过迁移步骤
//
// 2. 使用 Migrator().HasTable() 方法，兼容不同数据库类型
//   - 好处：MySQL、PostgreSQL、SQLite 等都能正确检查
//   - 好处：通过 GORM 的抽象层，无需关心具体数据库差异
//
// 3. 类型断言失败时返回 false
//   - 好处：保守策略，如果获取不到数据库连接，认为表不存在，触发重新初始化
//   - 好处：避免因连接问题导致的误判
func (i *initApi) TableCreated(ctx context.Context) bool {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return false
	}
	return db.Migrator().HasTable(&sysModel.SysApi{})
}

// InitializeData 初始化API表的初始数据
// 这是核心的数据初始化逻辑，用于创建系统所需的基础API记录
//
// 设计亮点：
// 1. 批量创建：使用 slice 定义所有 API 实体，一次性批量插入
//   - 好处：性能优异，一条 SQL 插入多条记录，比循环插入快得多
//   - 好处：事务性更好，要么全部成功，要么全部失败，保证数据一致性
//   - 好处：代码简洁，易于维护和扩展
//
// 2. 完整的API清单：包含系统中所有功能模块的API定义
//   - 好处：集中管理，所有API定义一目了然
//   - 好处：便于权限管理和API文档生成
//   - 好处：统一的API规范，包含方法、路径、分组、描述等信息
//
// 3. Context 传递数据：将创建的实体存入 context，供后续初始化器使用
//   - 好处：避免其他初始化器重复查询数据库，提高效率
//   - 好处：实现初始化器之间的数据共享，无需额外查询
//
// 4. 错误包装：使用 errors.Wrap 包装错误，保留错误上下文
//   - 好处：错误信息更丰富，便于定位问题
//   - 好处：可以追踪错误的调用链，便于调试
//
// API 分组说明：
// - jwt: JWT认证相关API
// - 系统用户: 用户管理相关API（注册、登录、信息管理等）
// - api: API管理相关API（CRUD操作、同步等）
// - 角色: 角色/权限管理相关API
// - casbin: 权限策略管理相关API
// - 菜单: 菜单管理相关API
// - 文件上传与下载: 文件操作相关API
// - 系统服务: 系统配置相关API
// - 其他业务模块API...
func (i *initApi) InitializeData(ctx context.Context) (context.Context, error) {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return ctx, system.ErrMissingDBContext
	}
	// 定义所有需要初始化的API实体
	// 使用批量定义的方式，清晰、易维护
	entities := []sysModel.SysApi{
		{ApiGroup: "jwt", Method: "POST", Path: "/jwt/jsonInBlacklist", Description: "jwt加入黑名单(退出，必选)"},

		{ApiGroup: "系统用户", Method: "DELETE", Path: "/user/deleteUser", Description: "删除用户"},
		{ApiGroup: "系统用户", Method: "POST", Path: "/user/admin_register", Description: "用户注册"},
		{ApiGroup: "系统用户", Method: "POST", Path: "/user/getUserList", Description: "获取用户列表"},
		{ApiGroup: "系统用户", Method: "PUT", Path: "/user/setUserInfo", Description: "设置用户信息"},
		{ApiGroup: "系统用户", Method: "PUT", Path: "/user/setSelfInfo", Description: "设置自身信息(必选)"},
		{ApiGroup: "系统用户", Method: "GET", Path: "/user/getUserInfo", Description: "获取自身信息(必选)"},
		{ApiGroup: "系统用户", Method: "POST", Path: "/user/setUserAuthorities", Description: "设置权限组"},
		{ApiGroup: "系统用户", Method: "POST", Path: "/user/changePassword", Description: "修改密码（建议选择)"},
		{ApiGroup: "系统用户", Method: "POST", Path: "/user/setUserAuthority", Description: "修改用户角色(必选)"},
		{ApiGroup: "系统用户", Method: "POST", Path: "/user/resetPassword", Description: "重置用户密码"},
		{ApiGroup: "系统用户", Method: "PUT", Path: "/user/setSelfSetting", Description: "用户界面配置"},

		{ApiGroup: "api", Method: "POST", Path: "/api/createApi", Description: "创建api"},
		{ApiGroup: "api", Method: "POST", Path: "/api/deleteApi", Description: "删除Api"},
		{ApiGroup: "api", Method: "POST", Path: "/api/updateApi", Description: "更新Api"},
		{ApiGroup: "api", Method: "POST", Path: "/api/getApiList", Description: "获取api列表"},
		{ApiGroup: "api", Method: "POST", Path: "/api/getAllApis", Description: "获取所有api"},
		{ApiGroup: "api", Method: "POST", Path: "/api/getApiById", Description: "获取api详细信息"},
		{ApiGroup: "api", Method: "DELETE", Path: "/api/deleteApisByIds", Description: "批量删除api"},
		{ApiGroup: "api", Method: "GET", Path: "/api/syncApi", Description: "获取待同步API"},
		{ApiGroup: "api", Method: "GET", Path: "/api/getApiGroups", Description: "获取路由组"},
		{ApiGroup: "api", Method: "POST", Path: "/api/enterSyncApi", Description: "确认同步API"},
		{ApiGroup: "api", Method: "POST", Path: "/api/ignoreApi", Description: "忽略API"},

		{ApiGroup: "角色", Method: "POST", Path: "/authority/copyAuthority", Description: "拷贝角色"},
		{ApiGroup: "角色", Method: "POST", Path: "/authority/createAuthority", Description: "创建角色"},
		{ApiGroup: "角色", Method: "POST", Path: "/authority/deleteAuthority", Description: "删除角色"},
		{ApiGroup: "角色", Method: "PUT", Path: "/authority/updateAuthority", Description: "更新角色信息"},
		{ApiGroup: "角色", Method: "POST", Path: "/authority/getAuthorityList", Description: "获取角色列表"},
		{ApiGroup: "角色", Method: "POST", Path: "/authority/setDataAuthority", Description: "设置角色资源权限"},

		{ApiGroup: "casbin", Method: "POST", Path: "/casbin/updateCasbin", Description: "更改角色api权限"},
		{ApiGroup: "casbin", Method: "POST", Path: "/casbin/getPolicyPathByAuthorityId", Description: "获取权限列表"},

		{ApiGroup: "菜单", Method: "POST", Path: "/menu/addBaseMenu", Description: "新增菜单"},
		{ApiGroup: "菜单", Method: "POST", Path: "/menu/getMenu", Description: "获取菜单树(必选)"},
		{ApiGroup: "菜单", Method: "POST", Path: "/menu/deleteBaseMenu", Description: "删除菜单"},
		{ApiGroup: "菜单", Method: "POST", Path: "/menu/updateBaseMenu", Description: "更新菜单"},
		{ApiGroup: "菜单", Method: "POST", Path: "/menu/getBaseMenuById", Description: "根据id获取菜单"},
		{ApiGroup: "菜单", Method: "POST", Path: "/menu/getMenuList", Description: "分页获取基础menu列表"},
		{ApiGroup: "菜单", Method: "POST", Path: "/menu/getBaseMenuTree", Description: "获取用户动态路由"},
		{ApiGroup: "菜单", Method: "POST", Path: "/menu/getMenuAuthority", Description: "获取指定角色menu"},
		{ApiGroup: "菜单", Method: "POST", Path: "/menu/addMenuAuthority", Description: "增加menu和角色关联关系"},

		{ApiGroup: "分片上传", Method: "GET", Path: "/fileUploadAndDownload/findFile", Description: "寻找目标文件（秒传）"},
		{ApiGroup: "分片上传", Method: "POST", Path: "/fileUploadAndDownload/breakpointContinue", Description: "断点续传"},
		{ApiGroup: "分片上传", Method: "POST", Path: "/fileUploadAndDownload/breakpointContinueFinish", Description: "断点续传完成"},
		{ApiGroup: "分片上传", Method: "POST", Path: "/fileUploadAndDownload/removeChunk", Description: "上传完成移除文件"},

		{ApiGroup: "文件上传与下载", Method: "POST", Path: "/fileUploadAndDownload/upload", Description: "文件上传（建议选择）"},
		{ApiGroup: "文件上传与下载", Method: "POST", Path: "/fileUploadAndDownload/deleteFile", Description: "删除文件"},
		{ApiGroup: "文件上传与下载", Method: "POST", Path: "/fileUploadAndDownload/editFileName", Description: "文件名或者备注编辑"},
		{ApiGroup: "文件上传与下载", Method: "POST", Path: "/fileUploadAndDownload/getFileList", Description: "获取上传文件列表"},
		{ApiGroup: "文件上传与下载", Method: "POST", Path: "/fileUploadAndDownload/importURL", Description: "导入URL"},

		{ApiGroup: "系统服务", Method: "POST", Path: "/system/getServerInfo", Description: "获取服务器信息"},
		{ApiGroup: "系统服务", Method: "POST", Path: "/system/getSystemConfig", Description: "获取配置文件内容"},
		{ApiGroup: "系统服务", Method: "POST", Path: "/system/setSystemConfig", Description: "设置配置文件内容"},

		{ApiGroup: "客户", Method: "PUT", Path: "/customer/customer", Description: "更新客户"},
		{ApiGroup: "客户", Method: "POST", Path: "/customer/customer", Description: "创建客户"},
		{ApiGroup: "客户", Method: "DELETE", Path: "/customer/customer", Description: "删除客户"},
		{ApiGroup: "客户", Method: "GET", Path: "/customer/customer", Description: "获取单一客户"},
		{ApiGroup: "客户", Method: "GET", Path: "/customer/customerList", Description: "获取客户列表"},

		{ApiGroup: "代码生成器", Method: "GET", Path: "/autoCode/getDB", Description: "获取所有数据库"},
		{ApiGroup: "代码生成器", Method: "GET", Path: "/autoCode/getTables", Description: "获取数据库表"},
		{ApiGroup: "代码生成器", Method: "POST", Path: "/autoCode/createTemp", Description: "自动化代码"},
		{ApiGroup: "代码生成器", Method: "POST", Path: "/autoCode/preview", Description: "预览自动化代码"},
		{ApiGroup: "代码生成器", Method: "GET", Path: "/autoCode/getColumn", Description: "获取所选table的所有字段"},
		{ApiGroup: "代码生成器", Method: "POST", Path: "/autoCode/installPlugin", Description: "安装插件"},
		{ApiGroup: "代码生成器", Method: "POST", Path: "/autoCode/pubPlug", Description: "打包插件"},
		{ApiGroup: "代码生成器", Method: "POST", Path: "/autoCode/mcp", Description: "自动生成 MCP Tool 模板"},
		{ApiGroup: "代码生成器", Method: "POST", Path: "/autoCode/mcpTest", Description: "MCP Tool 测试"},
		{ApiGroup: "代码生成器", Method: "POST", Path: "/autoCode/mcpList", Description: "获取 MCP ToolList"},

		{ApiGroup: "模板配置", Method: "POST", Path: "/autoCode/createPackage", Description: "配置模板"},
		{ApiGroup: "模板配置", Method: "GET", Path: "/autoCode/getTemplates", Description: "获取模板文件"},
		{ApiGroup: "模板配置", Method: "POST", Path: "/autoCode/getPackage", Description: "获取所有模板"},
		{ApiGroup: "模板配置", Method: "POST", Path: "/autoCode/delPackage", Description: "删除模板"},

		{ApiGroup: "代码生成器历史", Method: "POST", Path: "/autoCode/getMeta", Description: "获取meta信息"},
		{ApiGroup: "代码生成器历史", Method: "POST", Path: "/autoCode/rollback", Description: "回滚自动生成代码"},
		{ApiGroup: "代码生成器历史", Method: "POST", Path: "/autoCode/getSysHistory", Description: "查询回滚记录"},
		{ApiGroup: "代码生成器历史", Method: "POST", Path: "/autoCode/delSysHistory", Description: "删除回滚记录"},
		{ApiGroup: "代码生成器历史", Method: "POST", Path: "/autoCode/addFunc", Description: "增加模板方法"},

		{ApiGroup: "系统字典详情", Method: "PUT", Path: "/sysDictionaryDetail/updateSysDictionaryDetail", Description: "更新字典内容"},
		{ApiGroup: "系统字典详情", Method: "POST", Path: "/sysDictionaryDetail/createSysDictionaryDetail", Description: "新增字典内容"},
		{ApiGroup: "系统字典详情", Method: "DELETE", Path: "/sysDictionaryDetail/deleteSysDictionaryDetail", Description: "删除字典内容"},
		{ApiGroup: "系统字典详情", Method: "GET", Path: "/sysDictionaryDetail/findSysDictionaryDetail", Description: "根据ID获取字典内容"},
		{ApiGroup: "系统字典详情", Method: "GET", Path: "/sysDictionaryDetail/getSysDictionaryDetailList", Description: "获取字典内容列表"},

		{ApiGroup: "系统字典详情", Method: "GET", Path: "/sysDictionaryDetail/getDictionaryTreeList", Description: "获取字典数列表"},
		{ApiGroup: "系统字典详情", Method: "GET", Path: "/sysDictionaryDetail/getDictionaryTreeListByType", Description: "根据分类获取字典数列表"},
		{ApiGroup: "系统字典详情", Method: "GET", Path: "/sysDictionaryDetail/getDictionaryDetailsByParent", Description: "根据父级ID获取字典详情"},
		{ApiGroup: "系统字典详情", Method: "GET", Path: "/sysDictionaryDetail/getDictionaryPath", Description: "获取字典详情的完整路径"},

		{ApiGroup: "系统字典", Method: "POST", Path: "/sysDictionary/createSysDictionary", Description: "新增字典"},
		{ApiGroup: "系统字典", Method: "DELETE", Path: "/sysDictionary/deleteSysDictionary", Description: "删除字典"},
		{ApiGroup: "系统字典", Method: "PUT", Path: "/sysDictionary/updateSysDictionary", Description: "更新字典"},
		{ApiGroup: "系统字典", Method: "GET", Path: "/sysDictionary/findSysDictionary", Description: "根据ID获取字典（建议选择）"},
		{ApiGroup: "系统字典", Method: "GET", Path: "/sysDictionary/getSysDictionaryList", Description: "获取字典列表"},
		{ApiGroup: "系统字典", Method: "POST", Path: "/sysDictionary/importSysDictionary", Description: "导入字典JSON"},
		{ApiGroup: "系统字典", Method: "GET", Path: "/sysDictionary/exportSysDictionary", Description: "导出字典JSON"},

		{ApiGroup: "操作记录", Method: "POST", Path: "/sysOperationRecord/createSysOperationRecord", Description: "新增操作记录"},
		{ApiGroup: "操作记录", Method: "GET", Path: "/sysOperationRecord/findSysOperationRecord", Description: "根据ID获取操作记录"},
		{ApiGroup: "操作记录", Method: "GET", Path: "/sysOperationRecord/getSysOperationRecordList", Description: "获取操作记录列表"},
		{ApiGroup: "操作记录", Method: "DELETE", Path: "/sysOperationRecord/deleteSysOperationRecord", Description: "删除操作记录"},
		{ApiGroup: "操作记录", Method: "DELETE", Path: "/sysOperationRecord/deleteSysOperationRecordByIds", Description: "批量删除操作历史"},

		{ApiGroup: "断点续传(插件版)", Method: "POST", Path: "/simpleUploader/upload", Description: "插件版分片上传"},
		{ApiGroup: "断点续传(插件版)", Method: "GET", Path: "/simpleUploader/checkFileMd5", Description: "文件完整度验证"},
		{ApiGroup: "断点续传(插件版)", Method: "GET", Path: "/simpleUploader/mergeFileMd5", Description: "上传完成合并文件"},

		{ApiGroup: "email", Method: "POST", Path: "/email/emailTest", Description: "发送测试邮件"},
		{ApiGroup: "email", Method: "POST", Path: "/email/sendEmail", Description: "发送邮件"},

		{ApiGroup: "按钮权限", Method: "POST", Path: "/authorityBtn/setAuthorityBtn", Description: "设置按钮权限"},
		{ApiGroup: "按钮权限", Method: "POST", Path: "/authorityBtn/getAuthorityBtn", Description: "获取已有按钮权限"},
		{ApiGroup: "按钮权限", Method: "POST", Path: "/authorityBtn/canRemoveAuthorityBtn", Description: "删除按钮"},

		{ApiGroup: "导出模板", Method: "POST", Path: "/sysExportTemplate/createSysExportTemplate", Description: "新增导出模板"},
		{ApiGroup: "导出模板", Method: "DELETE", Path: "/sysExportTemplate/deleteSysExportTemplate", Description: "删除导出模板"},
		{ApiGroup: "导出模板", Method: "DELETE", Path: "/sysExportTemplate/deleteSysExportTemplateByIds", Description: "批量删除导出模板"},
		{ApiGroup: "导出模板", Method: "PUT", Path: "/sysExportTemplate/updateSysExportTemplate", Description: "更新导出模板"},
		{ApiGroup: "导出模板", Method: "GET", Path: "/sysExportTemplate/findSysExportTemplate", Description: "根据ID获取导出模板"},
		{ApiGroup: "导出模板", Method: "GET", Path: "/sysExportTemplate/getSysExportTemplateList", Description: "获取导出模板列表"},
		{ApiGroup: "导出模板", Method: "GET", Path: "/sysExportTemplate/exportExcel", Description: "导出Excel"},
		{ApiGroup: "导出模板", Method: "GET", Path: "/sysExportTemplate/exportTemplate", Description: "下载模板"},
		{ApiGroup: "导出模板", Method: "GET", Path: "/sysExportTemplate/previewSQL", Description: "预览SQL"},
		{ApiGroup: "导出模板", Method: "POST", Path: "/sysExportTemplate/importExcel", Description: "导入Excel"},

		{ApiGroup: "错误日志", Method: "POST", Path: "/sysError/createSysError", Description: "新建错误日志"},
		{ApiGroup: "错误日志", Method: "DELETE", Path: "/sysError/deleteSysError", Description: "删除错误日志"},
		{ApiGroup: "错误日志", Method: "DELETE", Path: "/sysError/deleteSysErrorByIds", Description: "批量删除错误日志"},
		{ApiGroup: "错误日志", Method: "PUT", Path: "/sysError/updateSysError", Description: "更新错误日志"},
		{ApiGroup: "错误日志", Method: "GET", Path: "/sysError/findSysError", Description: "根据ID获取错误日志"},
		{ApiGroup: "错误日志", Method: "GET", Path: "/sysError/getSysErrorList", Description: "获取错误日志列表"},
		{ApiGroup: "错误日志", Method: "GET", Path: "/sysError/getSysErrorSolution", Description: "触发错误处理(异步)"},

		{ApiGroup: "公告", Method: "POST", Path: "/info/createInfo", Description: "新建公告"},
		{ApiGroup: "公告", Method: "DELETE", Path: "/info/deleteInfo", Description: "删除公告"},
		{ApiGroup: "公告", Method: "DELETE", Path: "/info/deleteInfoByIds", Description: "批量删除公告"},
		{ApiGroup: "公告", Method: "PUT", Path: "/info/updateInfo", Description: "更新公告"},
		{ApiGroup: "公告", Method: "GET", Path: "/info/findInfo", Description: "根据ID获取公告"},
		{ApiGroup: "公告", Method: "GET", Path: "/info/getInfoList", Description: "获取公告列表"},

		{ApiGroup: "参数管理", Method: "POST", Path: "/sysParams/createSysParams", Description: "新建参数"},
		{ApiGroup: "参数管理", Method: "DELETE", Path: "/sysParams/deleteSysParams", Description: "删除参数"},
		{ApiGroup: "参数管理", Method: "DELETE", Path: "/sysParams/deleteSysParamsByIds", Description: "批量删除参数"},
		{ApiGroup: "参数管理", Method: "PUT", Path: "/sysParams/updateSysParams", Description: "更新参数"},
		{ApiGroup: "参数管理", Method: "GET", Path: "/sysParams/findSysParams", Description: "根据ID获取参数"},
		{ApiGroup: "参数管理", Method: "GET", Path: "/sysParams/getSysParamsList", Description: "获取参数列表"},
		{ApiGroup: "参数管理", Method: "GET", Path: "/sysParams/getSysParam", Description: "获取参数列表"},
		{ApiGroup: "媒体库分类", Method: "GET", Path: "/attachmentCategory/getCategoryList", Description: "分类列表"},
		{ApiGroup: "媒体库分类", Method: "POST", Path: "/attachmentCategory/addCategory", Description: "添加/编辑分类"},
		{ApiGroup: "媒体库分类", Method: "POST", Path: "/attachmentCategory/deleteCategory", Description: "删除分类"},

		{ApiGroup: "版本控制", Method: "GET", Path: "/sysVersion/findSysVersion", Description: "获取单一版本"},
		{ApiGroup: "版本控制", Method: "GET", Path: "/sysVersion/getSysVersionList", Description: "获取版本列表"},
		{ApiGroup: "版本控制", Method: "GET", Path: "/sysVersion/downloadVersionJson", Description: "下载版本json"},
		{ApiGroup: "版本控制", Method: "POST", Path: "/sysVersion/exportVersion", Description: "创建版本"},
		{ApiGroup: "版本控制", Method: "POST", Path: "/sysVersion/importVersion", Description: "同步版本"},
		{ApiGroup: "版本控制", Method: "DELETE", Path: "/sysVersion/deleteSysVersion", Description: "删除版本"},
		{ApiGroup: "版本控制", Method: "DELETE", Path: "/sysVersion/deleteSysVersionByIds", Description: "批量删除版本"},
	}
	// 批量创建API记录
	// 使用 db.Create(&entities) 一次性插入所有记录
	// 好处：
	// 1. 性能优异：一条 SQL 批量插入，比循环插入快得多
	// 2. 事务性：要么全部成功，要么全部失败，保证数据完整性
	// 3. 原子性：在事务中执行，避免部分插入导致的脏数据
	if err := db.Create(&entities).Error; err != nil {
		// 使用 errors.Wrap 包装错误，保留错误上下文和调用链
		// 好处：错误信息包含表名，便于快速定位是哪个表初始化失败
		return ctx, errors.Wrap(err, sysModel.SysApi{}.TableName()+"表数据初始化失败!")
	}
	// 将创建的实体存入 context，供后续初始化器使用
	// 好处：避免其他初始化器重复查询数据库获取API列表，提高效率
	// 好处：实现初始化器之间的数据共享，无需额外查询
	next := context.WithValue(ctx, i.InitializerName(), entities)
	return next, nil
}

// DataInserted 检查初始化数据是否已存在
// 用于判断是否需要重新初始化数据
//
// 检查策略：
// 1. 选择具有代表性的API作为检查点（这里选择 "/authorityBtn/canRemoveAuthorityBtn"）
// 2. 使用 path 和 method 组合作为唯一标识（因为API是通过路径和方法唯一确定的）
//
// 为什么选择这个API作为检查点？
// - 这个API在列表的后面部分，如果它存在，说明大部分API都已初始化
// - 使用相对靠后的API可以减少检查点，提高检查效率
// - 如果系统更新添加了新的API，这个检查逻辑仍然有效
//
// 设计说明：
// 1. 使用 errors.Is 判断是否为记录不存在错误，兼容性好
//   - 好处：正确处理 GORM 的 ErrRecordNotFound 错误
//   - 好处：兼容不同版本的 GORM，避免因错误类型变更导致的问题
//
// 2. 使用组合条件查询（path + method），确保唯一性
//   - 好处：HTTP API 通过路径和方法唯一确定，符合 RESTful 规范
//   - 好处：避免误判，如果只有 path 可能不够准确
//
// 3. 类型断言失败时返回 false
//   - 好处：保守策略，如果获取不到数据库连接，认为数据未初始化
//   - 好处：触发重新初始化，确保数据完整性
func (i *initApi) DataInserted(ctx context.Context) bool {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return false
	}
	// 通过检查特定API是否存在来判断数据是否已初始化
	// 使用 path 和 method 组合查询，确保唯一性
	// 如果记录不存在（ErrRecordNotFound），说明数据未初始化
	if errors.Is(db.Where("path = ? AND method = ?", "/authorityBtn/canRemoveAuthorityBtn", "POST").
		First(&sysModel.SysApi{}).Error, gorm.ErrRecordNotFound) {
		return false
	}
	return true
}
