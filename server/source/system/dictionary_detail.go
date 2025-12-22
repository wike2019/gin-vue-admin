package system

import (
	"context"
	"fmt"

	sysModel "github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/service/system"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// initOrderDictDetail 定义字典详情表的初始化顺序，在字典表之后执行
// 设置为 initOrderDict + 1 的好处：
// 1. 明确依赖关系：字典详情表依赖字典表，必须先有字典才能创建详情
// 2. 保证数据完整性：避免详情创建时关联的字典不存在，导致外键约束失败
// 3. 支持数据传递：通过 context 从字典表初始化器获取已创建的字典数据，建立关联关系
// 4. 灵活调整：当字典表顺序改变时，详情表顺序自动跟随，无需手动修改
const initOrderDictDetail = initOrderDict + 1

// initDictDetail 字典详情表初始化器结构体
// 采用空结构体的好处：
// 1. 零内存占用：空结构体不占用任何内存空间，仅作为方法接收者使用
// 2. 符合 Go 语言最佳实践：当只需要方法集合而不需要状态时，使用空结构体
// 3. 类型安全：通过类型实现接口，编译期检查，避免运行时错误
// 4. 可扩展性：未来如需添加配置或缓存，可以轻松扩展字段
type initDictDetail struct{}

// init 包初始化函数，在包被导入时自动执行
// 这种自动注册模式的好处：
// 1. 零配置：导入包即自动注册，无需手动调用注册函数，降低使用门槛
// 2. 解耦：初始化逻辑与注册逻辑分离，符合单一职责原则
// 3. 可扩展：新增初始化器只需实现接口并注册，框架会自动处理执行顺序
// 4. 依赖管理：通过 initOrder 自动处理初始化顺序，无需手动管理依赖链
// 5. 类型安全：编译期检查接口实现，避免运行时错误
func init() {
	system.RegisterInit(initOrderDictDetail, &initDictDetail{})
}

// MigrateTable 执行字典详情表的数据库迁移（创建或更新表结构）
// 参数 ctx: 上下文对象，用于传递数据库连接等初始化信息
// 返回值: 返回更新后的上下文和可能的错误
// 好处：
// 1. 使用 context 传递数据库连接，避免全局变量，提高代码的可测试性和并发安全性
// 2. 通过类型断言检查数据库连接是否存在，提前发现配置错误，避免运行时 panic
// 3. 使用 GORM 的 AutoMigrate 自动处理表结构变更，无需手动编写 SQL，降低维护成本
// 4. 支持表结构版本管理：当模型字段变更时，AutoMigrate 会自动更新表结构
func (i *initDictDetail) MigrateTable(ctx context.Context) (context.Context, error) {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return ctx, system.ErrMissingDBContext
	}
	return ctx, db.AutoMigrate(&sysModel.SysDictionaryDetail{})
}

// TableCreated 检查字典详情表是否已经创建
// 返回值: 如果表已存在返回 true，否则返回 false
// 好处：
// 1. 幂等性检查：在初始化前判断表是否已存在，避免重复创建，支持多次初始化
// 2. 支持增量初始化：如果表已存在可以跳过建表步骤，提高初始化效率
// 3. 错误容错：如果上下文缺少数据库连接，返回 false 而不是 panic，保证程序稳定性
// 4. 支持数据库迁移：在已有数据库上运行初始化时，可以跳过已存在的表
func (i *initDictDetail) TableCreated(ctx context.Context) bool {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return false
	}
	return db.Migrator().HasTable(&sysModel.SysDictionaryDetail{})
}

// InitializerName 返回初始化器的名称，用于日志记录和依赖管理
// 好处：
// 1. 使用模型自身的 TableName() 方法，保证名称一致性，避免硬编码字符串
// 2. 当表名变更时，只需修改模型定义，初始化器名称自动同步更新，降低维护成本
// 3. 便于在日志中追踪初始化过程，快速定位问题
// 4. 作为 context 的 key 存储初始化数据，供其他初始化器使用
func (i *initDictDetail) InitializerName() string {
	return sysModel.SysDictionaryDetail{}.TableName()
}

// InitializeData 初始化字典详情表的初始数据
// 参数 ctx: 上下文对象，包含数据库连接和依赖的字典数据
// 返回值: 更新后的上下文和可能的错误
// 设计思路：
// 1. 从 context 获取依赖的字典数据：字典详情必须关联到已存在的字典，通过 context 传递依赖数据
// 2. 集中管理初始数据：将系统必需的字典详情数据集中定义，便于维护和版本控制
// 3. 使用 GORM Association 管理关联：通过 Association.Replace 方法建立字典与详情的关联关系
// 好处：
// 1. 依赖注入：通过 context 获取依赖数据，避免直接查询数据库，提高性能和可测试性
// 2. 数据完整性：确保字典存在后再创建详情，避免外键约束失败
// 3. 关联管理：使用 Association.Replace 的好处：
//   - 自动处理外键关系：GORM 会自动设置 SysDictionaryID 字段
//   - 幂等性：Replace 会先删除旧的关联，再创建新的，支持重复初始化
//   - 事务安全：在同一个事务中完成删除和创建，保证数据一致性
//
// 4. 错误包装：使用 errors.Wrap 包装错误，保留原始错误信息和堆栈，便于调试
// 5. 指针类型处理：Status 字段是指针类型（*bool），使用 &True 正确设置值
// 6. 扩展字段：使用 Extend 字段区分不同数据库类型（mysql/pgsql），支持多数据库场景
func (i *initDictDetail) InitializeData(ctx context.Context) (context.Context, error) {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return ctx, system.ErrMissingDBContext
	}
	// 从 context 获取字典表初始化器创建的字典数据
	// 使用 new(initDict).InitializerName() 作为 key 的好处：
	// 1. 类型安全：通过类型获取名称，避免硬编码字符串
	// 2. 解耦：不直接依赖 initDict 结构体，只依赖其 InitializerName() 方法
	// 3. 可维护：当字典表初始化器名称变更时，这里会自动同步
	dicts, ok := ctx.Value(new(initDict).InitializerName()).([]sysModel.SysDictionary)
	if !ok {
		return ctx, errors.Wrap(system.ErrMissingDependentContext,
			fmt.Sprintf("未找到 %s 表初始化数据", sysModel.SysDictionary{}.TableName()))
	}
	// 定义布尔值变量，用于设置 Status 字段
	// 使用变量而不是直接使用 true 的原因：Status 是指针类型（*bool），需要传递地址
	True := true
	// 为每个字典定义对应的详情数据
	// 设计说明：
	// 1. 性别字典（dicts[0]）：包含男、女两个选项，用于用户性别选择
	// 2. 数据库int类型字典（dicts[1]）：包含不同数据库的整数类型，支持代码生成功能
	// 3. 数据库时间日期类型字典（dicts[2]）：包含不同数据库的时间类型
	// 4. 数据库浮点型字典（dicts[3]）：包含不同数据库的浮点数类型
	// 5. 数据库字符串字典（dicts[4]）：包含不同数据库的字符串类型
	// 6. 数据库bool类型字典（dicts[5]）：包含不同数据库的布尔类型
	// 使用 Extend 字段区分数据库类型，支持多数据库场景下的代码生成
	dicts[0].SysDictionaryDetails = []sysModel.SysDictionaryDetail{
		{Label: "男", Value: "1", Status: &True, Sort: 1},
		{Label: "女", Value: "2", Status: &True, Sort: 2},
	}

	dicts[1].SysDictionaryDetails = []sysModel.SysDictionaryDetail{
		{Label: "smallint", Value: "1", Status: &True, Extend: "mysql", Sort: 1},
		{Label: "mediumint", Value: "2", Status: &True, Extend: "mysql", Sort: 2},
		{Label: "int", Value: "3", Status: &True, Extend: "mysql", Sort: 3},
		{Label: "bigint", Value: "4", Status: &True, Extend: "mysql", Sort: 4},
		{Label: "int2", Value: "5", Status: &True, Extend: "pgsql", Sort: 5},
		{Label: "int4", Value: "6", Status: &True, Extend: "pgsql", Sort: 6},
		{Label: "int6", Value: "7", Status: &True, Extend: "pgsql", Sort: 7},
		{Label: "int8", Value: "8", Status: &True, Extend: "pgsql", Sort: 8},
	}

	dicts[2].SysDictionaryDetails = []sysModel.SysDictionaryDetail{
		{Label: "date", Value: "0", Status: &True, Extend: "mysql", Sort: 0},
		{Label: "time", Value: "1", Status: &True, Extend: "mysql", Sort: 1},
		{Label: "year", Value: "2", Status: &True, Extend: "mysql", Sort: 2},
		{Label: "datetime", Value: "3", Status: &True, Extend: "mysql", Sort: 3},
		{Label: "timestamp", Value: "5", Status: &True, Extend: "mysql", Sort: 5},
		{Label: "timestamptz", Value: "6", Status: &True, Extend: "pgsql", Sort: 5},
	}
	dicts[3].SysDictionaryDetails = []sysModel.SysDictionaryDetail{
		{Label: "float", Value: "0", Status: &True, Extend: "mysql", Sort: 0},
		{Label: "double", Value: "1", Status: &True, Extend: "mysql", Sort: 1},
		{Label: "decimal", Value: "2", Status: &True, Extend: "mysql", Sort: 2},
		{Label: "numeric", Value: "3", Status: &True, Extend: "pgsql", Sort: 3},
		{Label: "smallserial", Value: "4", Status: &True, Extend: "pgsql", Sort: 4},
	}

	dicts[4].SysDictionaryDetails = []sysModel.SysDictionaryDetail{
		{Label: "char", Value: "0", Status: &True, Extend: "mysql", Sort: 0},
		{Label: "varchar", Value: "1", Status: &True, Extend: "mysql", Sort: 1},
		{Label: "tinyblob", Value: "2", Status: &True, Extend: "mysql", Sort: 2},
		{Label: "tinytext", Value: "3", Status: &True, Extend: "mysql", Sort: 3},
		{Label: "text", Value: "4", Status: &True, Extend: "mysql", Sort: 4},
		{Label: "blob", Value: "5", Status: &True, Extend: "mysql", Sort: 5},
		{Label: "mediumblob", Value: "6", Status: &True, Extend: "mysql", Sort: 6},
		{Label: "mediumtext", Value: "7", Status: &True, Extend: "mysql", Sort: 7},
		{Label: "longblob", Value: "8", Status: &True, Extend: "mysql", Sort: 8},
		{Label: "longtext", Value: "9", Status: &True, Extend: "mysql", Sort: 9},
	}

	dicts[5].SysDictionaryDetails = []sysModel.SysDictionaryDetail{
		{Label: "tinyint", Value: "1", Extend: "mysql", Status: &True},
		{Label: "bool", Value: "2", Extend: "pgsql", Status: &True},
	}
	// 遍历所有字典，为每个字典建立关联的详情数据
	// 使用 Association.Replace 的好处：
	// 1. 自动管理外键：GORM 会自动设置 SysDictionaryID 字段，无需手动设置
	// 2. 幂等性：Replace 会先删除旧的关联数据，再创建新的，支持重复初始化
	// 3. 事务安全：删除和创建在同一个事务中完成，保证数据一致性
	// 4. 性能优化：批量操作，比逐条插入更高效
	// 5. 数据一致性：确保每个字典的详情数据是最新的，避免旧数据残留
	for _, dict := range dicts {
		if err := db.Model(&dict).Association("SysDictionaryDetails").
			Replace(dict.SysDictionaryDetails); err != nil {
			return ctx, errors.Wrap(err, sysModel.SysDictionaryDetail{}.TableName()+"表数据初始化失败!")
		}
	}
	return ctx, nil
}

// DataInserted 检查字典详情表的初始数据是否已经插入
// 返回值: 如果数据已存在返回 true，否则返回 false
// 检查策略：
// 1. 查询特定的字典（"数据库bool类型"）及其关联的详情数据
// 2. 检查详情数据是否存在且第一条记录的 Label 是否为 "tinyint"
// 好处：
// 1. 幂等性保证：通过检查特定数据是否存在来判断初始化是否完成，支持重复初始化
// 2. 避免重复插入：如果数据已存在，跳过初始化步骤，提高效率
// 3. 精确判断：不仅检查数据是否存在，还检查具体内容，确保数据正确性
// 4. 使用 Preload 预加载关联数据：避免 N+1 查询问题，提高查询效率
// 5. 选择 "数据库bool类型" 作为检查标志的原因：
//   - 它是初始数据中的一条，且名称唯一，适合作为判断依据
//   - 它的详情数据较少（只有2条），查询效率高
//   - 它的第一条详情是 "tinyint"，可以作为数据正确性的验证
func (i *initDictDetail) DataInserted(ctx context.Context) bool {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return false
	}
	var dict sysModel.SysDictionary
	// 使用 Preload 预加载关联的详情数据，避免 N+1 查询问题
	// 好处：一次查询获取字典及其所有详情，而不是先查字典再查详情
	if err := db.Preload("SysDictionaryDetails").
		First(&dict, &sysModel.SysDictionary{Name: "数据库bool类型"}).Error; err != nil {
		return false
	}
	// 检查详情数据是否存在且第一条记录的 Label 是否为 "tinyint"
	// 双重检查的好处：
	// 1. len(dict.SysDictionaryDetails) > 0：确保详情数据已插入
	// 2. dict.SysDictionaryDetails[0].Label == "tinyint"：确保数据内容正确，不是空数据或错误数据
	return len(dict.SysDictionaryDetails) > 0 && dict.SysDictionaryDetails[0].Label == "tinyint"
}
