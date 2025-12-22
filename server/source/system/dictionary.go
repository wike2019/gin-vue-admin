package system

import (
	"context"

	sysModel "github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/service/system"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// initOrderDict 定义字典表的初始化顺序，在 Casbin 之后执行
// 好处：通过顺序控制确保依赖关系，Casbin 初始化完成后才初始化字典表，避免依赖冲突
// 使用 initOrderCasbin + 1 的方式可以灵活调整顺序，当 Casbin 顺序改变时，字典表顺序自动跟随
const initOrderDict = initOrderCasbin + 1

// initDict 字典表初始化器结构体
// 采用空结构体的好处：零内存占用，仅作为方法接收者使用，符合 Go 语言最佳实践
type initDict struct{}

// init 包初始化函数，在包被导入时自动执行
// 好处：利用 Go 的包初始化机制，无需手动调用，系统启动时自动注册初始化器
// 这样设计使得每个初始化器都是自包含的，降低了耦合度，提高了可维护性
func init() {
	system.RegisterInit(initOrderDict, &initDict{})
}

// MigrateTable 执行字典表的数据库迁移（创建或更新表结构）
// 参数 ctx: 上下文对象，用于传递数据库连接等初始化信息
// 返回值: 返回更新后的上下文和可能的错误
// 好处：
// 1. 使用 context 传递数据库连接，避免全局变量，提高代码的可测试性和并发安全性
// 2. 通过类型断言检查数据库连接是否存在，提前发现配置错误
// 3. 使用 GORM 的 AutoMigrate 自动处理表结构变更，无需手动编写 SQL，降低维护成本
func (i *initDict) MigrateTable(ctx context.Context) (context.Context, error) {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return ctx, system.ErrMissingDBContext
	}
	return ctx, db.AutoMigrate(&sysModel.SysDictionary{})
}

// TableCreated 检查字典表是否已经创建
// 返回值: 如果表已存在返回 true，否则返回 false
// 好处：
// 1. 幂等性检查：在初始化前判断表是否已存在，避免重复创建
// 2. 支持增量初始化：如果表已存在可以跳过建表步骤，提高初始化效率
// 3. 错误容错：如果上下文缺少数据库连接，返回 false 而不是 panic，保证程序稳定性
func (i *initDict) TableCreated(ctx context.Context) bool {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return false
	}
	return db.Migrator().HasTable(&sysModel.SysDictionary{})
}

// InitializerName 返回初始化器的名称，用于日志记录和依赖管理
// 好处：
// 1. 使用模型自身的 TableName() 方法，保证名称一致性，避免硬编码字符串
// 2. 当表名变更时，只需修改模型定义，初始化器名称自动同步更新
// 3. 便于在日志中追踪初始化过程，快速定位问题
func (i *initDict) InitializerName() string {
	return sysModel.SysDictionary{}.TableName()
}

// InitializeData 初始化字典表的初始数据
// 参数 ctx: 上下文对象，包含数据库连接
// 返回值: 更新后的上下文（包含已插入的数据）和可能的错误
// 好处：
// 1. 集中管理初始数据：将系统必需的字典数据集中定义，便于维护和版本控制
// 2. 使用指针类型 Status: &True，因为 Status 字段可能是指针类型（*bool），这样可以正确设置值
// 3. 批量插入：使用 db.Create(&entities) 批量插入，提高性能
// 4. 错误包装：使用 errors.Wrap 包装错误，保留原始错误信息和堆栈，便于调试
// 5. 上下文传递：将插入的数据存入上下文，供后续初始化器使用，实现数据依赖传递
func (i *initDict) InitializeData(ctx context.Context) (next context.Context, err error) {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return ctx, system.ErrMissingDBContext
	}
	// 定义布尔值变量，用于设置 Status 字段
	// 使用变量而不是直接使用 true 的好处：当 Status 是指针类型时，需要传递地址
	True := true
	// 定义系统必需的字典数据
	// 包含：性别字典、数据库类型字典等基础数据
	// 这些数据是系统运行的基础，必须在初始化时创建
	entities := []sysModel.SysDictionary{
		{Name: "性别", Type: "gender", Status: &True, Desc: "性别字典"},
		{Name: "数据库int类型", Type: "int", Status: &True, Desc: "int类型对应的数据库类型"},
		{Name: "数据库时间日期类型", Type: "time.Time", Status: &True, Desc: "数据库时间日期类型"},
		{Name: "数据库浮点型", Type: "float64", Status: &True, Desc: "数据库浮点型"},
		{Name: "数据库字符串", Type: "string", Status: &True, Desc: "数据库字符串"},
		{Name: "数据库bool类型", Type: "bool", Status: &True, Desc: "数据库bool类型"},
	}

	// 批量插入字典数据
	// 使用 GORM 的 Create 方法，自动处理批量插入和错误处理
	if err = db.Create(&entities).Error; err != nil {
		// 错误包装：保留原始错误信息，同时添加业务语义，便于问题定位
		return ctx, errors.Wrap(err, sysModel.SysDictionary{}.TableName()+"表数据初始化失败!")
	}
	// 将插入的数据存入上下文，供后续初始化器使用
	// 好处：实现初始化器之间的数据依赖传递，例如其他表可能需要引用这些字典数据
	next = context.WithValue(ctx, i.InitializerName(), entities)
	return next, nil
}

// DataInserted 检查字典表的初始数据是否已经插入
// 返回值: 如果数据已存在返回 true，否则返回 false
// 好处：
// 1. 幂等性保证：通过检查特定数据（type="bool"）是否存在来判断初始化是否完成
// 2. 避免重复插入：如果数据已存在，跳过初始化步骤，提高效率
// 3. 使用 errors.Is 和 gorm.ErrRecordNotFound 进行精确的错误判断，避免误判
// 4. 选择 "bool" 类型作为检查标志的原因：它是初始数据中的一条，且类型唯一，适合作为判断依据
func (i *initDict) DataInserted(ctx context.Context) bool {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return false
	}
	// 通过查询特定类型的字典数据来判断数据是否已插入
	// 使用 errors.Is 检查是否为记录未找到错误，如果是则说明数据未初始化
	if errors.Is(db.Where("type = ?", "bool").First(&sysModel.SysDictionary{}).Error, gorm.ErrRecordNotFound) { // 判断是否存在数据
		return false
	}
	return true
}
