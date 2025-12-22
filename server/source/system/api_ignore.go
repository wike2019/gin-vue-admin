package system

import (
	"context"

	sysModel "github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/service/system"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// initApiIgnore API忽略表初始化器
// 实现 SubInitializer 接口，遵循插件化初始化架构
//
// 为什么需要这个初始化器：
// 1. 权限管理需求：系统使用 Casbin 进行 RBAC 权限控制，但某些 API 不应该走权限验证
//   - 例如：登录接口需要在登录前调用，不能要求权限；Swagger 文档接口是公开的
//   - 这些 API 需要在权限中间件中被跳过验证，避免权限检查逻辑的干扰
//
// 2. 集中管理：将需要忽略的 API 统一存储在数据库中，而不是硬编码在代码中
//   - 好处：可以通过管理界面动态添加/删除忽略的 API，无需修改代码
//   - 好处：配置化，便于在不同环境中灵活配置
//   - 好处：支持运行时调整，无需重启服务
//
// 3. API 同步优化：在 API 同步功能中，忽略的 API 不会被自动注册到权限系统
//   - 好处：避免系统自动同步时将这些公开接口误加入权限管理
//   - 好处：减少权限策略表的冗余数据，提高查询效率
//
// 使用结构体而不是函数的好处：
// 1. 可以实现接口方法，符合 Go 的接口设计模式
// 2. 类型安全：编译期检查接口实现，避免运行时错误
// 3. 便于扩展：未来可以添加配置或缓存等字段
// 4. 符合面向对象设计：方法可以接收者，更易于组织和维护
type initApiIgnore struct{}

// initOrderApiIgnore 定义API忽略表的初始化顺序
// 设置为 initOrderApi + 1 确保 API 表先于 API 忽略表初始化
//
// 为什么这样设计：
// 1. 明确的依赖关系：API 忽略表是对 API 表的补充，依赖 API 表的初始化
//   - 虽然数据库层面没有外键约束，但逻辑上 API 忽略表是对 API 表的功能扩展
//   - 通过相对顺序表达初始化依赖，语义清晰，易于理解
//
// 2. 保证初始化顺序：系统会按照 order 值排序执行，确保依赖表先创建
//   - 好处：如果未来需要在 API 忽略表的初始化中使用 API 表的数据，顺序是正确的
//   - 好处：符合直觉：先有 API，再有 API 的忽略规则
//
// 3. 易于维护：新增初始化器只需设置相对顺序，无需修改全局配置
//   - 好处：模块化设计，每个初始化器只关注自己的依赖关系
//   - 好处：支持多依赖：如果依赖多个表，可以使用 initOrderA + initOrderB 的形式
//
// 4. 防止循环依赖：通过明确的顺序定义，避免初始化器之间的循环依赖
const initOrderApiIgnore = initOrderApi + 1

// init 包初始化函数，在导入包时自动执行
// 这种自动注册模式的好处：
// 1. 零配置：导入包即自动注册，无需手动调用注册函数
//   - 好处：简化使用，开发者只需 import 即可，不需要记住额外的注册步骤
//   - 好处：降低出错概率，避免忘记注册导致的初始化遗漏
//
// 2. 解耦：初始化逻辑与注册逻辑分离，符合单一职责原则
//   - 好处：代码组织清晰，职责分明
//   - 好处：便于测试和维护
//
// 3. 可扩展：新增初始化器只需实现接口并注册，框架会自动处理
//   - 好处：开闭原则，对扩展开放，对修改关闭
//   - 好处：插件化架构，支持模块化开发
//
// 4. 依赖管理：通过 initOrder 自动处理初始化顺序，无需手动管理依赖链
//   - 好处：减少人工维护成本
//   - 好处：降低依赖管理的复杂度
//
// 5. 类型安全：编译期检查接口实现，避免运行时错误
//   - 好处：提前发现问题，而不是在生产环境运行时才发现
//
// 6. 防止遗漏：通过包导入机制确保所有初始化器都会被注册
//   - 好处：只要包被导入，初始化器就会被注册，不会遗漏
func init() {
	system.RegisterInit(initOrderApiIgnore, &initApiIgnore{})
}

// InitializerName 返回初始化器的唯一标识名称
// 用于：
// 1. 日志输出：标识当前初始化的模块，便于调试和追踪
//   - 好处：在初始化日志中可以清楚看到是哪个表在初始化
//   - 好处：出错时能快速定位是哪个初始化器的问题
//
// 2. Context 存储：作为 key 存储初始化后的数据，供其他初始化器使用
//   - 好处：实现初始化器之间的数据传递，避免重复查询数据库
//   - 好处：提高初始化性能，减少数据库访问
//
// 3. 去重检查：防止同名初始化器重复注册，在 RegisterInit 时会检查
//   - 好处：避免重复初始化导致的数据错误
//   - 好处：编译期就能发现配置错误
//
// 使用表名作为标识的好处：
// - 语义清晰：直接对应数据库表，一目了然
// - 自动获取：通过模型方法获取，避免硬编码字符串，减少拼写错误
// - 类型安全：编译期检查，如果表名变更，编译时就能发现
func (i *initApiIgnore) InitializerName() string {
	return sysModel.SysIgnoreApi{}.TableName()
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
//   - 好处：支持并发测试，每个测试用例可以使用独立的数据库连接
//
// 2. 类型断言 + ok 模式确保安全获取数据库连接
//   - 好处：避免 panic，优雅处理错误情况
//   - 好处：如果 context 中没有 db，返回明确的错误信息，便于调试
//   - 好处：符合 Go 语言的惯用法，代码更地道
//
// 3. AutoMigrate 自动根据模型结构创建/更新表结构，支持迭代开发
//   - 好处：模型结构变更时，自动同步到数据库，无需手动写 SQL
//   - 好处：支持版本升级时的表结构迁移
//   - 好处：开发阶段可以快速迭代，无需关注 DDL 细节
//   - 好处：减少人为错误，避免 SQL 语法错误或遗漏字段
func (i *initApiIgnore) MigrateTable(ctx context.Context) (context.Context, error) {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return ctx, system.ErrMissingDBContext
	}
	return ctx, db.AutoMigrate(&sysModel.SysIgnoreApi{})
}

// TableCreated 检查表是否已存在
// 用于判断是否需要执行表迁移和数据初始化
//
// 返回值：bool - true 表示表已存在，false 表示不存在
//
// 设计说明：
// 1. 幂等性检查：避免重复初始化导致的错误
//   - 好处：可以安全地多次执行初始化流程，不会因为重复执行而报错
//   - 好处：支持增量初始化，已存在的表可以跳过迁移步骤，提高效率
//   - 好处：支持系统升级场景，可以增量执行初始化
//
// 2. 使用 Migrator().HasTable() 方法，兼容不同数据库类型
//   - 好处：MySQL、PostgreSQL、SQLite、MSSQL 等都能正确检查
//   - 好处：通过 GORM 的抽象层，无需关心具体数据库差异
//   - 好处：代码可移植性强，切换数据库时无需修改代码
//
// 3. 类型断言失败时返回 false
//   - 好处：保守策略，如果获取不到数据库连接，认为表不存在，触发重新初始化
//   - 好处：避免因连接问题导致的误判，确保系统能够正确初始化
func (i *initApiIgnore) TableCreated(ctx context.Context) bool {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return false
	}
	return db.Migrator().HasTable(&sysModel.SysIgnoreApi{})
}

// InitializeData 初始化API忽略表的初始数据
// 这是核心的数据初始化逻辑，用于创建系统需要忽略权限验证的API记录
//
// 为什么需要初始化这些数据：
// 1. 系统级接口：这些接口是系统运行的基础，不应该走权限验证
//   - 登录接口：用户未登录时调用，不能要求权限
//   - 验证码接口：登录前获取验证码，需要公开访问
//   - 初始化接口：系统初始化时使用，此时可能还没有权限数据
//
// 2. 公开资源：这些资源应该是公开访问的，不需要权限控制
//   - Swagger 文档：API 文档应该公开，方便开发者查看
//   - 文件上传：某些场景下需要支持匿名上传（根据业务需求）
//   - 健康检查：监控系统需要无权限访问
//
// 3. 系统管理接口：这些接口具有特殊权限要求，不应该走常规权限验证
//   - Casbin 刷新：权限策略刷新接口，需要特殊处理
//   - 系统重载：系统配置重载接口，需要高级权限但不在权限表管理
//
// 设计亮点：
// 1. 批量创建：使用 slice 定义所有需要忽略的 API 实体，一次性批量插入
//   - 好处：性能优异，一条 SQL 插入多条记录，比循环插入快得多
//   - 好处：事务性更好，要么全部成功，要么全部失败，保证数据一致性
//   - 好处：代码简洁，易于维护和扩展，新增忽略 API 只需在 slice 中添加
//
// 2. 完整的忽略清单：包含系统中所有应该跳过权限验证的API定义
//   - 好处：集中管理，所有忽略的API定义一目了然
//   - 好处：便于审查和审计，可以清楚知道哪些API不受权限控制
//   - 好处：统一的API规范，包含方法和路径，确保匹配准确性
//
// 3. Context 传递数据：将创建的实体存入 context，供后续初始化器使用
//   - 好处：避免其他初始化器重复查询数据库，提高效率
//   - 好处：实现初始化器之间的数据共享，无需额外查询
//   - 好处：减少数据库访问次数，提高初始化速度
//
// 4. 错误包装：使用 errors.Wrap 包装错误，保留错误上下文
//   - 好处：错误信息更丰富，便于定位问题
//   - 好处：可以追踪错误的调用链，便于调试
//   - 好处：错误信息包含表名，能够快速定位是哪个表初始化失败
//
// API 说明：
// - /swagger/*any (GET): Swagger API 文档接口，应该公开访问
// - /api/freshCasbin (GET): 刷新 Casbin 权限策略缓存，系统管理接口
// - /uploads/file/*filepath (GET/HEAD): 文件下载接口，某些场景需要公开访问
// - /health (GET): 健康检查接口，监控系统需要无权限访问
// - /autoCode/llmAuto (POST): LLM 自动代码生成接口，可能需要特殊处理
// - /system/reloadSystem (POST): 系统配置重载接口，需要特殊权限
// - /base/login (POST): 用户登录接口，登录前调用，必须跳过权限验证
// - /base/captcha (POST): 验证码获取接口，登录前调用，必须公开访问
// - /init/initdb (POST): 数据库初始化接口，初始化时使用，需要跳过权限
// - /init/checkdb (POST): 数据库检查接口，初始化时使用，需要跳过权限
// - /info/getInfoDataSource (GET): 数据源信息接口，可能需要公开访问
// - /info/getInfoPublic (GET): 公开信息接口，应该公开访问
func (i *initApiIgnore) InitializeData(ctx context.Context) (context.Context, error) {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return ctx, system.ErrMissingDBContext
	}
	// 定义所有需要忽略权限验证的API实体
	// 使用批量定义的方式，清晰、易维护
	// 注意：这些 API 在权限中间件中会被跳过验证，确保系统正常运行
	entities := []sysModel.SysIgnoreApi{
		{Method: "GET", Path: "/swagger/*any"},
		{Method: "GET", Path: "/api/freshCasbin"},
		{Method: "GET", Path: "/uploads/file/*filepath"},
		{Method: "GET", Path: "/health"},
		{Method: "HEAD", Path: "/uploads/file/*filepath"},
		{Method: "POST", Path: "/autoCode/llmAuto"},
		{Method: "POST", Path: "/system/reloadSystem"},
		{Method: "POST", Path: "/base/login"},
		{Method: "POST", Path: "/base/captcha"},
		{Method: "POST", Path: "/init/initdb"},
		{Method: "POST", Path: "/init/checkdb"},
		{Method: "GET", Path: "/info/getInfoDataSource"},
		{Method: "GET", Path: "/info/getInfoPublic"},
	}
	if err := db.Create(&entities).Error; err != nil {
		return ctx, errors.Wrap(err, sysModel.SysIgnoreApi{}.TableName()+"表数据初始化失败!")
	}
	next := context.WithValue(ctx, i.InitializerName(), entities)
	return next, nil
}

// DataInserted 检查初始化数据是否已经插入
// 用于判断是否需要执行数据初始化，避免重复插入
//
// 返回值：bool - true 表示数据已存在，false 表示不存在
//
// 设计说明：
// 1. 幂等性检查：通过检查特定的数据是否存在来判断是否已初始化
//   - 好处：可以安全地多次执行初始化流程，不会重复插入数据
//   - 好处：支持增量初始化，已存在的数据可以跳过，提高效率
//   - 好处：避免重复插入导致的唯一键冲突错误
//
// 2. 使用代表性数据检查：检查 "/swagger/*any" + "GET" 这条记录
//   - 好处：这条记录是系统必需的基础数据，如果它存在，说明初始化已完成
//   - 好处：只检查一条记录，性能好，查询快速
//   - 好处：如果这条记录存在，其他记录应该也已经存在（因为是一次性批量插入）
//
// 3. 使用 errors.Is 检查 gorm.ErrRecordNotFound 错误
//   - 好处：这是 Go 1.13+ 推荐的错误检查方式，可以正确处理错误包装
//   - 好处：即使错误被包装，也能正确识别记录不存在的错误
//   - 好处：代码更健壮，不会因为错误包装而导致误判
//
// 4. 类型断言失败时返回 false
//   - 好处：保守策略，如果获取不到数据库连接，认为数据未初始化，触发重新初始化
//   - 好处：确保系统能够正确初始化，不会因为连接问题而跳过数据初始化
func (i *initApiIgnore) DataInserted(ctx context.Context) bool {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return false
	}
	// 检查代表性数据是否存在，如果存在说明初始化已完成
	if errors.Is(db.Where("path = ? AND method = ?", "/swagger/*any", "GET").
		First(&sysModel.SysIgnoreApi{}).Error, gorm.ErrRecordNotFound) {
		return false
	}
	return true
}
