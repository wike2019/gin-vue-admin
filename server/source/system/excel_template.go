package system

import (
	"context"

	sysModel "github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/service/system"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// initExcelTemplate Excel导出模板表初始化器
// 实现 SubInitializer 接口，遵循插件化初始化架构
// 使用结构体而不是函数的好处：
// 1. 可以保存状态（如果需要）
// 2. 可以实现接口方法，符合 Go 的接口设计模式
// 3. 便于扩展：未来可以添加配置或缓存等字段
// 4. 类型安全：编译期检查，避免方法签名错误
type initExcelTemplate struct{}

// initOrderExcelTemplate 定义 Excel 模板表的初始化顺序
// 设置为 initOrderDictDetail + 1 确保字典详情表先于 Excel 模板表初始化
// 这样设计的好处：
// 1. 明确依赖关系：通过相对顺序表达初始化依赖，语义清晰
// 2. 保证初始化顺序：系统会按照 order 值排序执行，确保依赖表先创建
// 3. 易于维护：新增初始化器只需设置相对顺序，无需修改全局配置
// 4. 支持多依赖：如果依赖多个表，可以使用 initOrderA + initOrderB 的形式
const initOrderExcelTemplate = initOrderDictDetail + 1

// init 包初始化函数，在导入包时自动执行
// 这种自动注册模式的好处：
// 1. 零配置：导入包即自动注册，无需手动调用注册函数
// 2. 解耦：初始化逻辑与注册逻辑分离，符合单一职责原则
// 3. 可扩展：新增初始化器只需实现接口并注册，框架会自动处理
// 4. 依赖管理：通过 initOrder 自动处理初始化顺序，无需手动管理依赖链
// 5. 类型安全：编译期检查接口实现，避免运行时错误
func init() {
	system.RegisterInit(initOrderExcelTemplate, &initExcelTemplate{})
}

// InitializerName 返回初始化器的唯一标识名称
// 用于：
// 1. 日志输出：标识当前初始化的模块，便于调试和追踪
// 2. Context 存储：作为 key 存储初始化后的数据，供其他初始化器使用
// 3. 去重检查：防止同名初始化器重复注册，避免冲突
// 4. 依赖检查：其他初始化器可以通过名称检查依赖是否已初始化
//
// 使用表名作为标识的好处：
// - 语义清晰：直接对应数据库表，一目了然
// - 避免冲突：表名在系统中唯一，保证标识唯一性
// - 便于查找：通过名称可以直接定位到对应的数据库表
func (i *initExcelTemplate) InitializerName() string {
	return "sys_export_templates"
}

// MigrateTable 执行数据库表结构迁移
// 使用 GORM 的 AutoMigrate 自动创建或更新表结构
// 这样设计的好处：
// 1. 自动化：无需手动编写 SQL，GORM 根据模型自动生成表结构
// 2. 版本管理：支持增量迁移，只更新变化的字段，不破坏已有数据
// 3. 类型安全：基于 Go 结构体定义，编译期检查类型错误
// 4. 跨数据库：GORM 支持多种数据库，代码无需修改即可适配
//
// 参数说明：
// - ctx: 上下文，包含数据库连接等初始化信息
// 返回值：
// - context.Context: 更新后的上下文，可以传递初始化状态
// - error: 迁移过程中的错误，如果失败会中断初始化流程
func (i *initExcelTemplate) MigrateTable(ctx context.Context) (context.Context, error) {
	// 从 context 中获取数据库连接
	// 使用类型断言确保类型安全，避免空指针异常
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		// 如果 context 中没有数据库连接，返回标准错误
		// 这样设计的好处：
		// 1. 错误信息明确：调用者知道具体缺少什么
		// 2. 统一错误处理：所有初始化器使用相同的错误类型
		// 3. 便于调试：错误信息包含上下文信息
		return ctx, system.ErrMissingDBContext
	}
	// 执行自动迁移，创建或更新表结构
	// AutoMigrate 会：
	// - 检查表是否存在，不存在则创建
	// - 检查字段是否存在，不存在则添加
	// - 检查索引是否存在，不存在则创建
	// - 不会删除字段或索引（保护已有数据）
	return ctx, db.AutoMigrate(&sysModel.SysExportTemplate{})
}

// TableCreated 检查表是否已经创建
// 用于判断是否需要执行表迁移操作
// 这样设计的好处：
// 1. 幂等性：多次执行初始化不会重复创建表
// 2. 性能优化：跳过已存在的表，减少不必要的操作
// 3. 容错性：支持中断后继续初始化，不会因为表已存在而失败
// 4. 状态检查：其他初始化器可以检查依赖表是否存在
//
// 返回值：
// - true: 表已存在，可以跳过迁移
// - false: 表不存在或检查失败，需要执行迁移
func (i *initExcelTemplate) TableCreated(ctx context.Context) bool {
	// 从 context 中获取数据库连接
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		// 如果没有数据库连接，保守返回 false
		// 这样会触发迁移操作，如果确实没有连接，迁移会失败并给出明确错误
		return false
	}
	// 使用 GORM Migrator 检查表是否存在
	// HasTable 是轻量级操作，只查询元数据，不扫描表内容
	return db.Migrator().HasTable(&sysModel.SysExportTemplate{})
}

// InitializeData 初始化表的基础数据
// 插入系统运行所需的基础配置数据
// 这样设计的好处：
// 1. 自动化：系统首次启动即可使用，无需手动导入数据
// 2. 标准化：确保所有环境的基础数据一致
// 3. 可扩展：可以添加更多模板配置，支持不同业务场景
// 4. 幂等性：通过 DataInserted 检查避免重复插入
//
// 当前初始化数据说明：
// - api 模板：用于导出 API 接口数据到 Excel
// - TemplateInfo 使用 JSON 格式定义导出字段映射
//   - path: API 路径
//   - method: HTTP 方法（大写）
//   - description: API 描述
//   - api_group: API 分组
//
// 参数说明：
// - ctx: 上下文，包含数据库连接
// 返回值：
// - context.Context: 更新后的上下文，包含初始化后的数据，供后续初始化器使用
// - error: 初始化过程中的错误
func (i *initExcelTemplate) InitializeData(ctx context.Context) (context.Context, error) {
	// 从 context 中获取数据库连接
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return ctx, system.ErrMissingDBContext
	}

	// 定义要初始化的基础数据
	// 使用切片批量创建，提高效率
	// 这样设计的好处：
	// 1. 批量操作：一次事务插入多条数据，性能更好
	// 2. 原子性：如果一条失败，整个批次回滚，保证数据一致性
	// 3. 易于维护：所有初始数据集中定义，便于查看和修改
	entities := []sysModel.SysExportTemplate{
		{
			Name:       "api",      // 模板名称，用于前端显示
			TableName:  "sys_apis", // 对应的数据库表名
			TemplateID: "api",      // 模板唯一标识，用于程序识别
			TemplateInfo: `{                      // JSON 格式的字段映射配置
"path":"路径",                                    // 定义导出时字段的中文名称
"method":"方法（大写）",                          // 支持字段转换和格式化说明
"description":"方法介绍",
"api_group":"方法分组"
}`,
		},
		// 可以在这里添加更多模板配置
		// 例如：用户导出模板、菜单导出模板等
	}

	// 批量创建数据
	// 使用 errors.Wrap 包装错误，添加上下文信息
	// 这样设计的好处：
	// 1. 错误追踪：保留原始错误和堆栈信息
	// 2. 错误定位：明确知道是哪个表初始化失败
	// 3. 便于调试：完整的错误链帮助快速定位问题
	if err := db.Create(&entities).Error; err != nil {
		return ctx, errors.Wrap(err, "sys_export_templates"+"表数据初始化失败!")
	}

	// 将初始化后的数据存储到 context 中
	// 这样设计的好处：
	// 1. 数据传递：后续初始化器可以通过 context 获取这些数据
	// 2. 依赖注入：实现初始化器之间的数据共享
	// 3. 状态管理：记录初始化状态，支持回滚和重试
	// 4. 解耦：初始化器之间通过 context 通信，不直接依赖
	next := context.WithValue(ctx, i.InitializerName(), entities)
	return next, nil
}

// DataInserted 检查基础数据是否已经插入
// 用于判断是否需要执行数据初始化操作
// 这样设计的好处：
// 1. 幂等性：多次执行初始化不会重复插入数据
// 2. 性能优化：跳过已有数据，减少不必要的数据库操作
// 3. 容错性：支持中断后继续初始化，不会因为数据已存在而失败
// 4. 数据保护：避免重复插入导致的数据冲突或重复
//
// 检查策略：
// - 使用 First 查询第一条记录
// - 如果查询到记录，说明数据已存在
// - 如果返回 ErrRecordNotFound，说明数据不存在
//
// 返回值：
// - true: 数据已存在，可以跳过初始化
// - false: 数据不存在或检查失败，需要执行初始化
func (i *initExcelTemplate) DataInserted(ctx context.Context) bool {
	// 从 context 中获取数据库连接
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		// 如果没有数据库连接，保守返回 false
		// 这样会触发初始化操作，如果确实没有连接，初始化会失败并给出明确错误
		return false
	}
	// 查询是否存在记录
	// 使用 errors.Is 检查是否为记录不存在的错误
	// 这样设计的好处：
	// 1. 精确判断：区分"记录不存在"和"查询出错"
	// 2. 错误处理：如果是其他错误（如连接失败），返回 false 会触发重试
	// 3. 类型安全：使用标准错误类型，避免字符串比较
	if errors.Is(db.First(&sysModel.SysExportTemplate{}).Error, gorm.ErrRecordNotFound) {
		return false
	}
	return true
}
