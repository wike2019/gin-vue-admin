package system

import (
	"context"

	sysModel "github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/service/system"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// initOrderUser 定义用户表的初始化顺序
// 设置为 initOrderAuthority + 1 确保权限表先于用户表初始化
// 这样设计的好处：
// 1. 明确依赖关系：用户表依赖权限表，必须先有权限才能创建用户
// 2. 保证数据完整性：避免用户创建时关联的权限不存在
// 3. 支持依赖注入：后续可以通过 context 获取权限数据来建立关联
const initOrderUser = initOrderAuthority + 1

// initUser 用户表初始化器
// 实现 SubInitializer 接口，遵循插件化初始化架构
// 使用结构体而不是函数的好处：
// 1. 可以保存状态（如果需要）
// 2. 可以实现接口方法
// 3. 符合 Go 的面向对象设计模式
type initUser struct{}

// init 包初始化函数，在导入包时自动执行
// 这种自动注册模式的好处：
// 1. 零配置：导入包即自动注册，无需手动调用
// 2. 解耦：初始化逻辑与注册逻辑分离
// 3. 可扩展：新增初始化器只需实现接口并注册，框架会自动处理
func init() {
	system.RegisterInit(initOrderUser, &initUser{})
}

// MigrateTable 执行数据库表结构迁移
// 参数：ctx - 上下文，包含数据库连接等初始化所需信息
// 返回：next context - 可以用于传递数据给后续初始化步骤
//
//	error - 迁移过程中的错误
//
// 设计说明：
// 1. 使用 context 传递 db 连接，避免全局变量，提高可测试性
// 2. 类型断言 + ok 模式确保安全获取数据库连接
// 3. AutoMigrate 自动根据模型结构创建/更新表结构，支持迭代开发
func (i *initUser) MigrateTable(ctx context.Context) (context.Context, error) {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return ctx, system.ErrMissingDBContext
	}
	return ctx, db.AutoMigrate(&sysModel.SysUser{})
}

// TableCreated 检查表是否已存在
// 用于判断是否需要执行表迁移和数据初始化
//
// 返回值：bool - true 表示表已存在，false 表示不存在
//
// 设计说明：
// 1. 幂等性检查：避免重复初始化导致的错误
// 2. 支持增量初始化：已存在的表可以跳过迁移步骤
// 3. 使用 Migrator().HasTable() 方法，兼容不同数据库类型
func (i *initUser) TableCreated(ctx context.Context) bool {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return false
	}
	return db.Migrator().HasTable(&sysModel.SysUser{})
}

// InitializerName 返回初始化器的唯一标识名称
// 用于：
// 1. 日志输出：标识当前初始化的模块
// 2. Context 存储：作为 key 存储初始化后的数据，供其他初始化器使用
// 3. 去重检查：防止同名初始化器重复注册
//
// 使用表名作为标识的好处：
// - 语义清晰：直接对应数据库表
// - 自动获取：通过模型方法获取，避免硬编码
func (i *initUser) InitializerName() string {
	return sysModel.SysUser{}.TableName()
}

// InitializeData 初始化用户表的初始数据
// 这是核心的数据初始化逻辑，包括：
// 1. 创建默认管理员账户和测试账户
// 2. 建立用户与权限的关联关系
//
// 设计亮点：
//  1. 支持自定义管理员密码：从 context 读取，提供默认值 "123456"
//     好处：可以在外部配置，提高安全性
//  2. 密码加密：使用 BcryptHash 加密，符合安全最佳实践
//  3. 使用 UUID：为每个用户生成唯一标识，而不是依赖自增ID
//     好处：分布式环境下避免ID冲突，提高安全性
//  4. Context 传递数据：将创建的实体存入 context，供后续初始化器使用
//     好处：实现初始化器之间的数据共享，无需查询数据库
//  5. 依赖检查：从 context 获取权限数据，确保依赖已初始化
//     好处：编译期无法检查的运行时依赖，通过显式检查避免错误
func (i *initUser) InitializeData(ctx context.Context) (next context.Context, err error) {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return ctx, system.ErrMissingDBContext
	}

	// 从 context 获取管理员密码，支持外部配置
	// 如果未配置则使用默认密码 "123456"
	// 这种设计的好处：可以在初始化时传入自定义密码，提高安全性
	ap := ctx.Value("adminPassword")
	apStr, ok := ap.(string)
	if !ok {
		apStr = "123456"
	}

	// 对密码进行 Bcrypt 加密
	// 好处：即使数据库泄露，密码也是安全的（不可逆加密）
	password := utils.BcryptHash(apStr)
	adminPassword := utils.BcryptHash(apStr)

	// 定义初始用户数据
	// 包含两个用户：
	// 1. admin - 管理员账户，拥有所有权限（AuthorityId: 888）
	// 2. a303176530 - 测试账户，拥有部分权限（AuthorityId: 9528）
	entities := []sysModel.SysUser{
		{
			UUID:        uuid.New(), // 使用 UUID 而非自增ID，支持分布式环境
			Username:    "admin",
			Password:    adminPassword,
			NickName:    "Mr.奇淼",
			HeaderImg:   "https://qmplusimg.henrongyi.top/gva_header.jpg",
			AuthorityId: 888, // 管理员权限ID
			Phone:       "17611111111",
			Email:       "333333333@qq.com",
		},
		{
			UUID:        uuid.New(),
			Username:    "a303176530",
			Password:    password,
			NickName:    "用户1",
			HeaderImg:   "https://qmplusimg.henrongyi.top/1572075907logo.png",
			AuthorityId: 9528, // 测试角色权限ID
			Phone:       "17611111111",
			Email:       "333333333@qq.com"},
	}

	// 批量创建用户记录
	// 使用 batch create 提高性能，一次性插入多条记录
	if err = db.Create(&entities).Error; err != nil {
		return ctx, errors.Wrap(err, sysModel.SysUser{}.TableName()+"表数据初始化失败!")
	}

	// 将创建的实体存入 context，供后续初始化器使用
	// 好处：避免其他初始化器重复查询数据库，提高效率
	next = context.WithValue(ctx, i.InitializerName(), entities)

	// 从 context 获取权限初始化数据（由 initAuthority 初始化器提供）
	// 这体现了初始化器之间的依赖关系和数据共享机制
	authorityEntities, ok := ctx.Value(new(initAuthority).InitializerName()).([]sysModel.SysAuthority)
	if !ok {
		return next, errors.Wrap(system.ErrMissingDependentContext, "创建 [用户-权限] 关联失败, 未找到权限表初始化数据")
	}

	// 建立用户与权限的多对多关联关系
	// entities[0] (admin) 关联所有权限（authorityEntities）
	// 使用 Association().Replace() 方法，会先清除旧关联再建立新关联
	// 好处：确保关联关系正确，避免脏数据
	if err = db.Model(&entities[0]).Association("Authorities").Replace(authorityEntities); err != nil {
		return next, err
	}

	// entities[1] (a303176530) 只关联第一个权限（authorityEntities[:1]）
	// 使用切片操作，简洁地选择部分权限
	if err = db.Model(&entities[1]).Association("Authorities").Replace(authorityEntities[:1]); err != nil {
		return next, err
	}
	return next, err
}

// DataInserted 检查初始化数据是否已存在
// 用于判断是否需要重新初始化数据
//
// 检查策略：
// 1. 检查特定用户（a303176530）是否存在
// 2. 同时检查该用户的权限关联是否正确（第一个权限是否为 888）
//
// 设计说明：
// 1. 使用 Preload 预加载关联数据，避免 N+1 查询问题
// 2. 使用 errors.Is 判断是否为记录不存在错误，兼容性好
// 3. 不仅检查用户存在，还检查权限关联，确保数据完整性
// 4. 这种双重检查的好处：避免数据不完整导致的运行时错误
func (i *initUser) DataInserted(ctx context.Context) bool {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return false
	}
	var record sysModel.SysUser
	// 查找特定用户并预加载权限关联
	// Preload("Authorities") 的好处：一次性加载关联数据，避免后续查询
	if errors.Is(db.Where("username = ?", "a303176530").
		Preload("Authorities").First(&record).Error, gorm.ErrRecordNotFound) { // 判断是否存在数据
		return false
	}
	// 这里是不是bug a303176530的权限 不应该是 888
	// 不仅检查用户存在，还验证权限关联是否正确
	// 确保数据完整性，避免只创建了用户但未关联权限的情况
	return len(record.Authorities) > 0 && record.Authorities[0].AuthorityId == 888
}
