package initialize

import (
	"os"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/example"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Gorm 根据配置初始化数据库连接
//
// 设计思路：
// 使用策略模式，根据配置的数据库类型选择对应的初始化函数
// 支持多种数据库：MySQL、PostgreSQL、Oracle、SQL Server、SQLite
//
// 为什么支持多种数据库？
// - 灵活性：不同项目可能使用不同的数据库
// - 可移植性：代码可以在不同数据库之间迁移
// - 开发友好：开发环境可以使用 SQLite，生产环境使用 MySQL/PostgreSQL
//
// 为什么使用 switch 而不是 if-else？
// - 代码清晰：每种数据库类型一目了然
// - 易于扩展：添加新数据库类型只需添加一个 case
// - 性能：switch 在某些情况下比 if-else 更高效
func Gorm() *gorm.DB {
	switch global.GVA_CONFIG.System.DbType {
	case "mysql":
		// MySQL 是最常用的关系型数据库
		// 为什么保存数据库名到全局变量？
		// - 某些功能需要知道当前使用的数据库名
		// - 多数据库切换时需要知道当前活跃的数据库
		global.GVA_ACTIVE_DBNAME = &global.GVA_CONFIG.Mysql.Dbname
		return GormMysql()
	case "pgsql":
		// PostgreSQL 是功能强大的开源数据库
		global.GVA_ACTIVE_DBNAME = &global.GVA_CONFIG.Pgsql.Dbname
		return GormPgSql()
	case "oracle":
		// Oracle 是企业级数据库
		global.GVA_ACTIVE_DBNAME = &global.GVA_CONFIG.Oracle.Dbname
		return GormOracle()
	case "mssql":
		// SQL Server 是微软的企业级数据库
		global.GVA_ACTIVE_DBNAME = &global.GVA_CONFIG.Mssql.Dbname
		return GormMssql()
	case "sqlite":
		// SQLite 是轻量级文件数据库，适合开发和小型项目
		global.GVA_ACTIVE_DBNAME = &global.GVA_CONFIG.Sqlite.Dbname
		return GormSqlite()
	default:
		// 默认使用 MySQL
		// 为什么默认使用 MySQL？
		// - MySQL 是最流行的开源数据库，兼容性好
		// - 大多数用户使用 MySQL，提供良好的默认体验
		// - 向后兼容：如果配置错误，至少能尝试连接 MySQL
		global.GVA_ACTIVE_DBNAME = &global.GVA_CONFIG.Mysql.Dbname
		return GormMysql()
	}
}

// RegisterTables 注册数据库表（自动迁移）
//
// 设计思路：
// 使用 GORM 的 AutoMigrate 功能自动创建或更新数据库表结构
//
// 为什么使用 AutoMigrate？
// - 自动化：无需手动编写 SQL 脚本创建表
// - 版本控制：代码即数据库结构，便于版本管理
// - 开发效率：修改模型后自动更新表结构
//
// 为什么需要 DisableAutoMigrate 选项？
// - 生产环境安全：某些生产环境可能禁止自动迁移，需要 DBA 手动执行
// - 权限控制：自动迁移需要 DDL 权限，生产环境可能没有此权限
// - 审计要求：某些企业要求所有数据库变更必须经过审批
func RegisterTables() {
	// 检查是否禁用自动迁移
	// 为什么提供禁用选项？
	// - 生产环境安全：防止意外修改数据库结构
	// - 权限控制：某些环境可能没有 DDL 权限
	// - 审计合规：企业可能要求手动执行数据库变更
	if global.GVA_CONFIG.System.DisableAutoMigrate {
		global.GVA_LOG.Info("auto-migrate is disabled, skipping table registration")
		return
	}

	db := global.GVA_DB

	// 自动迁移系统核心表
	// 为什么按模块分组？
	// - 代码清晰：系统表和示例表分开，便于理解和维护
	// - 错误处理：可以分别处理系统表和业务表的迁移错误
	err := db.AutoMigrate(
		// 系统核心表
		system.SysApi{},              // API 接口表：存储所有 API 接口信息，用于权限控制
		system.SysIgnoreApi{},        // 忽略的 API 表：不需要权限检查的接口
		system.SysUser{},             // 用户表：系统用户信息
		system.SysBaseMenu{},         // 基础菜单表：系统菜单结构
		system.JwtBlacklist{},        // JWT 黑名单表：存储已失效的 token
		system.SysAuthority{},        // 权限表：角色和权限信息
		system.SysDictionary{},       // 数据字典表：系统字典定义
		system.SysOperationRecord{},  // 操作记录表：用户操作日志
		system.SysAutoCodeHistory{},  // 自动代码历史表：代码生成历史记录
		system.SysDictionaryDetail{}, // 字典详情表：字典项的具体值
		system.SysBaseMenuParameter{}, // 菜单参数表：菜单的额外参数
		system.SysBaseMenuBtn{},      // 菜单按钮表：菜单关联的按钮
		system.SysAuthorityBtn{},     // 权限按钮表：角色拥有的按钮权限
		system.SysAutoCodePackage{},  // 自动代码包表：代码生成模板包
		system.SysExportTemplate{},   // 导出模板表：数据导出模板
		system.Condition{},           // 条件表：查询条件模板
		system.JoinTemplate{},        // 关联模板表：表关联查询模板
		system.SysParams{},           // 系统参数表：系统配置参数
		system.SysVersion{},          // 系统版本表：版本管理信息
		system.SysError{},            // 系统错误表：系统错误日志

		// 示例模块表（供开发者参考）
		example.ExaFile{},                      // 示例文件表
		example.ExaCustomer{},                  // 示例客户表
		example.ExaFileChunk{},                 // 文件分片表：大文件分片上传
		example.ExaFileUploadAndDownload{},    // 文件上传下载表
		example.ExaAttachmentCategory{},       // 附件分类表
	)

	// 为什么迁移失败要退出程序？
	// - 表结构错误会导致后续操作失败，继续运行没有意义
	// - 启动时发现错误比运行时发现错误更好，问题更明显
	// - 确保数据库结构正确，避免数据不一致
	if err != nil {
		global.GVA_LOG.Error("register table failed", zap.Error(err))
		os.Exit(0) // 退出程序，避免使用错误的数据库结构
	}

	// 注册业务表（用户自定义的表）
	// 为什么业务表单独处理？
	// - 业务表是动态的，可能通过代码生成工具创建
	// - 业务表可能有特殊的迁移逻辑
	// - 便于管理和维护业务相关的表
	err = bizModel()

	if err != nil {
		global.GVA_LOG.Error("register biz_table failed", zap.Error(err))
		os.Exit(0) // 业务表迁移失败也要退出，确保数据库结构完整
	}

	global.GVA_LOG.Info("register table success")
}
