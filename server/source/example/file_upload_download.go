// Package example 提供示例模块的数据库初始化功能
// 本文件实现了文件上传下载表的自动迁移和初始数据插入
package example

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/model/example"
	"github.com/flipped-aurora/gin-vue-admin/server/service/system"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// initOrderExaFile 定义文件上传下载表的初始化顺序
// 设置为 system.InitOrderInternal + 1 确保在系统内部表初始化之后执行
// 这样设计的好处：
// 1. 明确依赖关系：示例表通常依赖系统基础表（如用户、权限等），通过相对顺序表达依赖
// 2. 保证初始化顺序：系统会按照 order 值排序执行，确保依赖表先创建
// 3. 易于维护：新增初始化器只需设置相对顺序，无需修改全局配置
// 4. 支持依赖注入：后续可以通过 context 获取系统表数据来建立关联
const initOrderExaFile = system.InitOrderInternal + 1

// initExaFileMysql 文件上传下载表初始化器
// 实现 SubInitializer 接口，遵循插件化初始化架构
// 使用结构体而不是函数的好处：
// 1. 可以保存状态（如果需要）：未来可以添加配置或缓存等字段
// 2. 可以实现接口方法：符合 Go 的接口设计模式，类型安全
// 3. 便于扩展：可以添加其他辅助方法，如数据校验、数据清理等
// 4. 编译期检查：确保实现了所有必需的接口方法，避免运行时错误
type initExaFileMysql struct{}

// init 包初始化函数，在导入包时自动执行
// 这种自动注册模式的好处：
// 1. 零配置：导入包即自动注册，无需手动调用注册函数，减少人为错误
// 2. 解耦：初始化逻辑与注册逻辑分离，符合单一职责原则
// 3. 可扩展：新增初始化器只需实现接口并注册，框架会自动处理执行顺序
// 4. 依赖管理：通过 initOrder 自动处理初始化顺序，无需手动管理依赖链
// 5. 类型安全：编译期检查接口实现，避免运行时错误
// 6. 统一管理：所有初始化器通过统一接口管理，便于维护和调试
func init() {
	system.RegisterInit(initOrderExaFile, &initExaFileMysql{})
}

// MigrateTable 执行数据库表结构迁移
// 参数：ctx - 上下文，包含数据库连接等初始化所需信息
// 返回：next context - 可以用于传递数据给后续初始化步骤
//
//	error - 迁移过程中的错误
//
// 设计说明：
//  1. 使用 context 传递 db 连接，避免全局变量，提高可测试性和并发安全性
//  2. 类型断言 + ok 模式确保安全获取数据库连接，避免 panic
//  3. AutoMigrate 自动根据模型结构创建/更新表结构，支持迭代开发
//     好处：模型变更时自动同步表结构，无需手动编写 SQL 迁移脚本
//  4. 返回新的 context 支持链式初始化，便于后续步骤获取表信息
func (i *initExaFileMysql) MigrateTable(ctx context.Context) (context.Context, error) {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return ctx, system.ErrMissingDBContext
	}
	return ctx, db.AutoMigrate(&example.ExaFileUploadAndDownload{})
}

// TableCreated 检查表是否已创建
// 用于判断是否需要执行表迁移，避免重复创建
//
// 设计说明：
// 1. 幂等性检查：确保初始化过程可以安全地重复执行
// 2. 性能优化：如果表已存在，可以跳过迁移步骤，提高初始化速度
// 3. 错误处理：如果 context 中没有 db，返回 false 而不是 panic，优雅降级
// 4. 使用 GORM 的 Migrator 接口，数据库无关，支持多种数据库类型
func (i *initExaFileMysql) TableCreated(ctx context.Context) bool {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return false
	}
	return db.Migrator().HasTable(&example.ExaFileUploadAndDownload{})
}

// InitializerName 返回初始化器的唯一标识名称
// 用于：
// 1. 日志输出：标识当前初始化的模块，便于调试和追踪问题
// 2. Context 存储：作为 key 存储初始化后的数据，供其他初始化器使用
// 3. 去重检查：防止同名初始化器重复注册，避免冲突
//
// 使用表名作为标识的好处：
// - 语义清晰：直接对应数据库表，一目了然
// - 自动获取：通过模型方法获取，避免硬编码字符串，减少拼写错误
// - 类型安全：编译期检查，如果模型不存在会编译失败
// - 易于重构：表名变更时只需修改模型，无需修改多处代码
func (i *initExaFileMysql) InitializerName() string {
	return example.ExaFileUploadAndDownload{}.TableName()
}

// InitializeData 初始化表的基础数据
// 插入示例文件记录，用于演示和测试
//
// 参数：ctx - 上下文，包含数据库连接
// 返回：next context - 可以用于传递初始化后的数据
//
//	error - 初始化过程中的错误
//
// 设计说明：
// 1. 批量插入：使用切片批量创建，提高性能，减少数据库交互次数
// 2. 错误包装：使用 errors.Wrap 包装错误，保留原始错误信息和堆栈，便于调试
// 3. 上下文传递：通过 context 传递数据，支持后续初始化器使用这些数据
// 4. 幂等性：配合 DataInserted 检查，确保数据不会重复插入
// 5. 示例数据：提供默认的示例文件，方便用户快速了解系统功能
func (i *initExaFileMysql) InitializeData(ctx context.Context) (context.Context, error) {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return ctx, system.ErrMissingDBContext
	}
	entities := []example.ExaFileUploadAndDownload{
		{Name: "10.png", Url: "https://qmplusimg.henrongyi.top/gvalogo.png", Tag: "png", Key: "158787308910.png"},
		{Name: "logo.png", Url: "https://qmplusimg.henrongyi.top/1576554439myAvatar.png", Tag: "png", Key: "1587973709logo.png"},
	}
	if err := db.Create(&entities).Error; err != nil {
		return ctx, errors.Wrap(err, example.ExaFileUploadAndDownload{}.TableName()+"表数据初始化失败!")
	}
	return ctx, nil
}

// DataInserted 检查初始数据是否已插入
// 用于判断是否需要执行数据初始化，避免重复插入
//
// 设计说明：
// 1. 幂等性检查：通过查询特定记录判断数据是否已存在，确保初始化可重复执行
// 2. 性能优化：只查询一条记录，使用唯一标识（Name + Key）快速定位
// 3. 错误处理：使用 errors.Is 检查 gorm.ErrRecordNotFound，精确判断记录不存在
// 4. 安全降级：如果 context 中没有 db，返回 false，让系统重新初始化而不是 panic
// 5. 查询优化：使用 First 方法只查询一条记录，比 Find 更高效
func (i *initExaFileMysql) DataInserted(ctx context.Context) bool {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return false
	}
	lookup := example.ExaFileUploadAndDownload{Name: "logo.png", Key: "1587973709logo.png"}
	if errors.Is(db.First(&lookup, &lookup).Error, gorm.ErrRecordNotFound) {
		return false
	}
	return true
}
