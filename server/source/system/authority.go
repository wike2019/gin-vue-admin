package system

import (
	"context"

	sysModel "github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/service/system"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// initOrderAuthority 定义权限角色表的初始化顺序
// 设置为 initOrderCasbin + 1 确保 Casbin 权限表先于权限角色表初始化
// 这样设计的好处：
// 1. 明确依赖关系：权限角色可能需要在 Casbin 权限规则之后初始化，确保权限体系完整
// 2. 保证初始化顺序：系统会按照 order 值排序执行，确保依赖表先创建
// 3. 易于维护：使用相对顺序（+1）而非硬编码数字，当 Casbin 顺序改变时自动跟随调整
// 4. 支持依赖链：如果后续有表依赖权限角色表，可以使用 initOrderAuthority + 1 的形式
const initOrderAuthority = initOrderCasbin + 1

// initAuthority 权限角色表初始化器
// 实现 SubInitializer 接口，遵循插件化初始化架构
// 使用结构体而不是函数的好处：
// 1. 可以实现接口方法：符合 Go 的接口设计模式，编译期检查接口实现，确保方法签名正确
// 2. 便于扩展：可以添加辅助方法，如角色验证、权限检查等
// 3. 类型安全：编译期检查，避免方法签名错误导致的运行时问题
// 4. 状态管理：未来可以添加配置或缓存等字段，保持状态
type initAuthority struct{}

// init 包初始化函数，在导入包时自动执行
// 这种自动注册模式的好处：
// 1. 零配置：导入包即自动注册，无需手动调用注册函数，减少人为错误
// 2. 解耦：初始化逻辑与注册逻辑分离，符合单一职责原则
// 3. 可扩展：新增初始化器只需实现接口并注册，框架会自动处理执行顺序
// 4. 依赖管理：通过 initOrder 自动处理初始化顺序，无需手动管理依赖链
// 5. 类型安全：编译期检查接口实现，避免运行时错误
func init() {
	system.RegisterInit(initOrderAuthority, &initAuthority{})
}

// MigrateTable 执行权限角色表的数据库表结构迁移
// 参数：ctx - 上下文，包含数据库连接等初始化所需信息
// 返回：next context - 可以用于传递数据给后续初始化步骤
//
//	error - 迁移过程中的错误
//
// 设计说明：
//  1. 使用 context 传递 db 连接，避免全局变量，提高可测试性和并发安全性
//     好处：可以在测试中轻松替换数据库连接，支持并发测试
//  2. 类型断言 + ok 模式确保安全获取数据库连接，失败返回明确错误而非 panic
//     好处：错误处理更优雅，调用方可以决定如何处理错误
//  3. AutoMigrate 自动根据模型结构创建/更新表结构，支持迭代开发
//     好处：无需手动编写 SQL，模型变更自动同步到数据库，支持字段类型、索引等变更
func (i *initAuthority) MigrateTable(ctx context.Context) (context.Context, error) {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return ctx, system.ErrMissingDBContext
	}
	return ctx, db.AutoMigrate(&sysModel.SysAuthority{})
}

// TableCreated 检查权限角色表是否已创建
// 参数：ctx - 上下文，包含数据库连接
// 返回：bool - 表是否存在
//
// 设计说明：
//  1. 幂等性检查：用于判断是否需要执行表创建，避免重复创建导致的错误
//     好处：支持多次初始化，不会因为表已存在而失败
//  2. 安全获取数据库连接：使用类型断言 + ok 模式，失败时返回 false 而不是 panic
//     好处：优雅降级，调用方可以通过返回值判断表状态
//  3. 使用 GORM Migrator 的 HasTable 方法：标准化的表存在性检查
//     好处：跨数据库兼容，支持 MySQL、PostgreSQL、SQLite 等，无需编写数据库特定SQL
//  4. 返回 bool 而不是 error：简化调用方逻辑，只需判断布尔值
func (i *initAuthority) TableCreated(ctx context.Context) bool {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return false
	}
	return db.Migrator().HasTable(&sysModel.SysAuthority{})
}

// InitializerName 返回初始化器的唯一标识名称
// 返回：string - 权限角色表的表名
//
// 设计说明：
//  1. 使用表名作为标识：语义清晰，直接对应数据库表，便于日志追踪
//  2. 通过实体方法获取表名：避免硬编码，表名变更时自动同步
//     好处：如果表名规则改变（如添加前缀、修改命名规则），只需修改模型定义
//  3. 用于日志输出：标识当前初始化的模块，便于调试和问题追踪
//  4. 用于 Context 存储：作为 key 存储初始化后的数据，供其他初始化器使用
//     好处：后续初始化器（如用户表）可以通过表名获取已初始化的权限角色数据
func (i *initAuthority) InitializerName() string {
	return sysModel.SysAuthority{}.TableName()
}

// InitializeData 初始化权限角色表的默认数据
// 参数：ctx - 上下文，包含数据库连接
// 返回：next context - 包含初始化后的权限角色数据，供后续初始化器使用
//
//	error - 初始化过程中的错误
//
// 设计说明：
//
//  1. 初始化三个默认角色：
//     - 888（普通用户）：基础用户角色，ParentId=0 表示根角色
//     - 9528（测试角色）：用于测试的角色
//     - 8881（普通用户子角色）：ParentId=888 表示是普通用户的子角色，形成角色层级
//     好处：提供开箱即用的角色体系，支持角色继承和层级管理
//
//  2. 使用 utils.Pointer[uint](0) 创建指针：
//     好处：ParentId 是指针类型（*uint），可以区分 0 和 nil，nil 表示无父角色，0 需要显式指定
//
//  3. 初始化数据权限关联（DataAuthorityId）：
//     - DataAuthorityId 是 many2many 自引用关联，表示角色之间的数据权限关系
//     - 普通用户(888)关联所有三个角色：可以访问自己的数据、测试数据和子角色的数据
//     - 测试角色(9528)关联自己和子角色：可以访问自己的数据和子角色的数据
//     好处：实现细粒度的数据权限控制，角色可以控制能访问哪些其他角色的数据
//
//  4. 使用 Association("DataAuthorityId").Replace() 方法：
//     好处：先清空旧关联再设置新关联，确保关联关系准确，避免重复关联
//
//  5. 使用 errors.Wrapf 包装错误：
//     好处：保留原始错误信息和堆栈，同时添加上下文信息，便于问题定位
//
//  6. 将初始化后的实体存储到 context：
//     好处：后续初始化器（如用户表）可以直接使用这些角色数据，无需再次查询数据库
func (i *initAuthority) InitializeData(ctx context.Context) (context.Context, error) {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return ctx, system.ErrMissingDBContext
	}
	// 定义三个默认角色，形成基本的角色体系
	entities := []sysModel.SysAuthority{
		{AuthorityId: 888, AuthorityName: "普通用户", ParentId: utils.Pointer[uint](0), DefaultRouter: "dashboard"},
		{AuthorityId: 9528, AuthorityName: "测试角色", ParentId: utils.Pointer[uint](0), DefaultRouter: "dashboard"},
		{AuthorityId: 8881, AuthorityName: "普通用户子角色", ParentId: utils.Pointer[uint](888), DefaultRouter: "dashboard"},
	}

	// 批量创建角色记录，如果已存在会报错（由调用方检查 DataInserted 避免）
	if err := db.Create(&entities).Error; err != nil {
		return ctx, errors.Wrapf(err, "%s表数据初始化失败!", sysModel.SysAuthority{}.TableName())
	}

	// 初始化数据权限关联：普通用户(888)可以访问所有角色的数据
	// DataAuthorityId 是 many2many 自引用关联，通过中间表 sys_data_authority_id 存储
	// 这样设计的好处：实现细粒度的数据权限控制，角色可以控制能访问哪些其他角色的数据范围
	if err := db.Model(&entities[0]).Association("DataAuthorityId").Replace(
		[]*sysModel.SysAuthority{
			{AuthorityId: 888},
			{AuthorityId: 9528},
			{AuthorityId: 8881},
		}); err != nil {
		return ctx, errors.Wrapf(err, "%s表数据初始化失败!",
			db.Model(&entities[0]).Association("DataAuthorityId").Relationship.JoinTable.Name)
	}

	// 初始化数据权限关联：测试角色(9528)可以访问自己和子角色的数据
	if err := db.Model(&entities[1]).Association("DataAuthorityId").Replace(
		[]*sysModel.SysAuthority{
			{AuthorityId: 9528},
			{AuthorityId: 8881},
		}); err != nil {
		return ctx, errors.Wrapf(err, "%s表数据初始化失败!",
			db.Model(&entities[1]).Association("DataAuthorityId").Relationship.JoinTable.Name)
	}

	// 将初始化后的实体存储到 context，供后续初始化器使用
	// 好处：避免后续初始化器重复查询数据库，提高初始化效率
	next := context.WithValue(ctx, i.InitializerName(), entities)
	return next, nil
}

// DataInserted 检查权限角色表的默认数据是否已插入
// 参数：ctx - 上下文，包含数据库连接
// 返回：bool - 数据是否已存在
//
// 设计说明：
//  1. 幂等性检查：用于判断是否需要执行数据插入，避免重复插入导致的唯一约束错误
//     好处：支持多次初始化，不会因为数据已存在而失败
//  2. 使用特定的子角色 ID（8881）作为检查标志：
//     好处：子角色是最晚创建的，如果它存在说明所有默认数据都已初始化完成
//  3. 使用 errors.Is 和 gorm.ErrRecordNotFound 检查：
//     好处：精确判断是否因为记录不存在而返回错误，忽略其他类型的错误（如连接错误）
//  4. 返回 bool 而不是 error：简化调用方逻辑，只需判断布尔值
//  5. 安全获取数据库连接：失败时返回 false，表示未初始化
func (i *initAuthority) DataInserted(ctx context.Context) bool {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return false
	}
	// 检查子角色是否存在，如果存在说明所有默认数据都已初始化
	if errors.Is(db.Where("authority_id = ?", "8881").
		First(&sysModel.SysAuthority{}).Error, gorm.ErrRecordNotFound) { // 判断是否存在数据
		return false
	}
	return true
}
