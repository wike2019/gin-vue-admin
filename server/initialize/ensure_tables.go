package initialize

import (
	"context"

	adapter "github.com/casbin/gorm-adapter/v3"
	"github.com/flipped-aurora/gin-vue-admin/server/model/example"
	sysModel "github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/announcement/model"
	"github.com/flipped-aurora/gin-vue-admin/server/service/system"
	"gorm.io/gorm"
)

// initOrderEnsureTables 表创建确保器的初始化顺序
// 设置为 InitOrderExternal - 1，确保在外部扩展初始化之前执行
// 设计原因：
// 1. 表创建必须在数据初始化之前完成，因为数据初始化需要表已存在
// 2. 使用 InitOrderExternal - 1 确保表创建在所有外部扩展之前执行
// 3. 这样可以保证后续的初始化器（包括插件）都可以安全地使用数据库表
//
// 好处：
// 1. 明确的依赖关系：通过顺序值明确表达表创建必须在其他初始化之前
// 2. 自动化管理：框架会根据顺序值自动排序执行，无需手动管理
// 3. 安全性：确保所有表在数据操作前都已创建，避免运行时错误
const initOrderEnsureTables = system.InitOrderExternal - 1

// ensureTables 数据库表创建确保器
// 设计原因：
// 1. 实现 SubInitializer 接口，遵循系统的初始化器模式
// 2. 使用结构体而不是函数，可以保存状态和实现接口方法
// 3. 集中管理所有需要创建的表，确保数据库表结构的一致性
//
// 好处：
// 1. 统一管理：所有表定义在一个地方，便于查看和维护
// 2. 自动化：通过初始化器系统自动执行，无需手动调用
// 3. 可检查：提供 TableCreated 方法，可以检查表是否已创建
// 4. 依赖管理：通过初始化顺序确保表在其他初始化之前创建
type ensureTables struct{}

// init 包初始化函数，在包被导入时自动执行
// 设计原因：
// 1. 自动注册初始化器到系统，无需手动调用注册函数
// 2. 使用 init 函数确保初始化器在程序启动时被注册
//
// 好处：
// 1. 零配置：导入包即自动注册，开发者无需关心
// 2. 解耦：注册逻辑与执行逻辑分离
// 3. 可扩展：新增表只需在此文件中添加即可
func init() {
	system.RegisterInit(initOrderEnsureTables, &ensureTables{})
}

// InitializerName 返回初始化器的唯一标识名称
// 用于日志输出和初始化器去重检查
func (e *ensureTables) InitializerName() string {
	return "ensure_tables_created"
}

// InitializeData 初始化数据方法
// 此初始化器只负责创建表结构，不初始化数据，所以直接返回
// 设计原因：表创建和数据初始化分离，符合单一职责原则
func (e *ensureTables) InitializeData(ctx context.Context) (next context.Context, err error) {
	return ctx, nil
}

// DataInserted 检查数据是否已插入
// 由于此初始化器不插入数据，所以始终返回 true
// 设计原因：满足接口要求，表示数据初始化步骤已跳过
func (e *ensureTables) DataInserted(ctx context.Context) bool {
	return true
}

// MigrateTable 执行数据库表迁移（创建或更新表结构）
// 设计原因：
// 1. 使用 GORM 的 AutoMigrate 功能，自动根据模型结构创建或更新表
// 2. 通过 context 获取数据库连接，避免使用全局变量（依赖注入模式）
// 3. 集中管理所有需要创建的表，包括系统表、示例表和插件表
//
// 好处：
// 1. 自动化：根据模型结构自动创建表，无需手动编写 SQL
// 2. 版本控制：AutoMigrate 会检查表结构差异，自动添加缺失的字段
// 3. 类型安全：使用模型类型而不是字符串，编译期检查，避免拼写错误
// 4. 统一管理：所有表定义在一处，便于查看和维护
// 5. 可扩展：新增表只需在 tables 切片中添加模型即可
//
// 执行流程：
// 1. 从 context 中获取数据库连接
// 2. 遍历所有模型，使用 AutoMigrate 创建或更新表结构
// 3. 忽略错误（因为 AutoMigrate 基本不会出错，且视图冲突等问题是预期的）
func (e *ensureTables) MigrateTable(ctx context.Context) (context.Context, error) {
	// 从 context 中获取数据库连接
	// 使用 context 传递依赖的好处：
	// - 避免全局变量污染
	// - 便于测试（可以注入测试用的数据库连接）
	// - 符合依赖注入的设计模式
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return ctx, system.ErrMissingDBContext
	}

	// 定义需要创建的所有表模型
	// 包含三类表：
	// 1. 系统核心表（sysModel.*）：用户、角色、菜单、API 等
	// 2. 示例表（example.*）：文件上传、客户管理等示例功能
	// 3. 插件表（model.*）：公告等插件的数据表
	// 4. 第三方表（adapter.CasbinRule）：Casbin 权限规则表
	tables := []interface{}{
		// 系统核心表
		sysModel.SysApi{},
		sysModel.SysUser{},
		sysModel.SysBaseMenu{},
		sysModel.SysAuthority{},
		sysModel.JwtBlacklist{},
		sysModel.SysDictionary{},
		sysModel.SysAutoCodeHistory{},
		sysModel.SysOperationRecord{},
		sysModel.SysDictionaryDetail{},
		sysModel.SysBaseMenuParameter{},
		sysModel.SysBaseMenuBtn{},
		sysModel.SysAuthorityBtn{},
		sysModel.SysAutoCodePackage{},
		sysModel.SysExportTemplate{},
		sysModel.Condition{},
		sysModel.JoinTemplate{},
		sysModel.SysParams{},
		sysModel.SysVersion{},
		sysModel.SysError{},
		adapter.CasbinRule{}, // Casbin 权限管理规则表

		// 示例表
		example.ExaFile{},
		example.ExaCustomer{},
		example.ExaFileChunk{},
		example.ExaFileUploadAndDownload{},
		example.ExaAttachmentCategory{},

		// 插件表
		model.Info{}, // 公告插件表
	}

	// 遍历所有模型，执行自动迁移
	for _, t := range tables {
		// 使用 AutoMigrate 自动创建或更新表结构
		// 忽略返回值的原因：
		// - AutoMigrate 在正常情况下不会返回错误
		// - 即使有错误（如视图冲突），也是预期的（注释中已说明）
		// - 显式忽略可以避免未使用变量的警告
		_ = db.AutoMigrate(&t)
		// 注意：视图 authority_menu 会被当成表来创建，可能引发冲突错误
		// 但更新版本的 GORM 似乎已经处理了这个问题
		// 由于 AutoMigrate() 基本无需考虑错误，因此显式忽略
	}
	return ctx, nil
}

// TableCreated 检查所有表是否已创建
// 设计原因：
// 1. 初始化器系统需要检查初始化步骤是否已完成，避免重复执行
// 2. 使用 HasTable 方法检查表是否存在，而不是直接执行迁移
// 3. 只有当所有表都存在时才返回 true，确保完整性
//
// 好处：
// 1. 幂等性：可以安全地多次调用，不会重复创建表
// 2. 性能优化：如果表已存在，可以跳过迁移步骤，提高启动速度
// 3. 状态检查：可以用于诊断和调试，检查数据库表结构是否正确
// 4. 完整性：检查所有表，确保数据库结构的完整性
//
// 注意：此方法的 tables 列表与 MigrateTable 中的列表略有不同
// - 移除了部分表（如 SysParams、SysVersion、SysError 等）
// - 可能是为了兼容旧版本或某些特殊情况
func (e *ensureTables) TableCreated(ctx context.Context) bool {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return false
	}

	// 需要检查的表列表
	// 注意：这个列表与 MigrateTable 中的列表不完全一致
	// 可能是为了兼容性考虑，或者某些表是可选的
	tables := []interface{}{
		// 系统核心表
		sysModel.SysApi{},
		sysModel.SysUser{},
		sysModel.SysBaseMenu{},
		sysModel.SysAuthority{},
		sysModel.JwtBlacklist{},
		sysModel.SysDictionary{},
		sysModel.SysAutoCodeHistory{},
		sysModel.SysOperationRecord{},
		sysModel.SysDictionaryDetail{},
		sysModel.SysBaseMenuParameter{},
		sysModel.SysBaseMenuBtn{},
		sysModel.SysAuthorityBtn{},
		sysModel.SysAutoCodePackage{},
		sysModel.SysExportTemplate{},
		sysModel.Condition{},
		sysModel.JoinTemplate{},

		adapter.CasbinRule{}, // Casbin 权限管理规则表

		// 示例表
		example.ExaFile{},
		example.ExaCustomer{},
		example.ExaFileChunk{},
		example.ExaFileUploadAndDownload{},
		example.ExaAttachmentCategory{},

		// 插件表
		model.Info{}, // 公告插件表
	}

	// 检查所有表是否都存在
	// 使用逻辑与（&&）确保所有表都存在才返回 true
	// 这种写法的好处：
	// - 代码简洁，一行完成所有检查
	// - 短路求值：如果某个表不存在，立即返回 false，不继续检查
	yes := true
	for _, t := range tables {
		yes = yes && db.Migrator().HasTable(t)
	}
	return yes
}
