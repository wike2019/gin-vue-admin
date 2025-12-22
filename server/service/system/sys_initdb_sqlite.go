// Package system 提供 SQLite 数据库初始化服务
// 设计思路：
// 1. 使用策略模式：SqliteInitHandler 实现 TypedDBInitHandler 接口，专门处理 SQLite 数据库的初始化逻辑
// 2. 使用 context 传递状态：通过 context 传递配置和数据库连接，避免全局变量污染
// 3. 职责分离：每个方法只负责一个特定功能，符合单一职责原则
// 4. 幂等性支持：通过检查数据是否已存在，支持重复初始化而不出错
package system

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/gookit/color"
	"gorm.io/gorm"

	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
)

// SqliteInitHandler SQLite 数据库初始化处理器
// 设计原因：使用结构体实现 TypedDBInitHandler 接口，封装 SQLite 特定的初始化逻辑
// 好处：
// 1. 类型安全：通过结构体方法实现接口，编译时检查，避免运行时错误
// 2. 可扩展：如果需要添加 SQLite 特定的配置或方法，可以在结构体中扩展
// 3. 可测试：结构体方法易于单元测试，可以 mock 依赖
// 4. 零值可用：空结构体即可使用，无需额外初始化（Go 语言特性）
type SqliteInitHandler struct{}

// NewSqliteInitHandler 创建 SQLite 初始化处理器实例
// 设计原因：使用构造函数模式，提供统一的创建入口
// 好处：
// 1. 一致性：所有 Handler 都通过 New 函数创建，保持代码风格一致
// 2. 可扩展：未来如果需要初始化参数，可以在构造函数中添加
// 3. 语义清晰：明确表示这是一个构造函数，而非直接使用结构体字面量
func NewSqliteInitHandler() *SqliteInitHandler {
	return &SqliteInitHandler{}
}

// WriteConfig 将 SQLite 配置回写到配置文件
// 设计原因：初始化完成后，需要将配置保存到配置文件，以便系统下次启动时直接使用
// 好处：
// 1. 持久化配置：避免每次启动都需要重新初始化数据库
// 2. 用户体验：初始化一次后，后续启动自动使用已配置的数据库
// 3. 配置统一：所有配置（包括数据库类型、连接信息、JWT密钥等）统一管理
//
// 实现细节说明：
// - 从 context 获取配置：使用类型断言确保配置类型正确，避免运行时错误
// - 生成 JWT 密钥：每次初始化时生成新的 UUID 作为 JWT 签名密钥，提高安全性
// - 结构体转 Map：将配置结构体转换为 Map，便于批量设置到配置管理器
// - 设置活动数据库名：记录当前使用的数据库名，用于多数据库场景
func (h SqliteInitHandler) WriteConfig(ctx context.Context) error {
	// 从 context 中获取 SQLite 配置
	// 设计原因：使用 context 传递配置，避免函数参数过多，符合 Go 语言最佳实践
	// 类型断言：确保配置类型正确，如果类型不匹配则返回错误
	// 好处：编译时无法检查 context 中的值类型，运行时检查可以避免类型错误导致的 panic
	c, ok := ctx.Value("config").(config.Sqlite)
	if !ok {
		return errors.New("sqlite config invalid")
	}

	// 设置数据库类型为 SQLite
	// 好处：系统知道当前使用的数据库类型，可以执行类型特定的操作
	global.GVA_CONFIG.System.DbType = "sqlite"

	// 保存 SQLite 配置到全局配置
	// 好处：后续代码可以从全局配置获取数据库连接信息
	global.GVA_CONFIG.Sqlite = c

	// 生成新的 JWT 签名密钥
	// 设计原因：每次初始化时生成新的 UUID 作为密钥，提高安全性
	// 好处：
	// 1. 安全性：使用随机 UUID 作为密钥，避免使用默认密钥
	// 2. 唯一性：每次初始化都有不同的密钥，即使配置相同也能保证密钥不同
	global.GVA_CONFIG.JWT.SigningKey = uuid.New().String()

	// 将配置结构体转换为 Map，便于批量设置
	// 设计原因：配置管理器使用 key-value 方式存储，需要将结构体扁平化
	// 好处：可以一次性设置所有配置项，代码简洁高效
	cs := utils.StructToMap(global.GVA_CONFIG)
	for k, v := range cs {
		global.GVA_VP.Set(k, v)
	}

	// 设置当前活动的数据库名
	// 设计原因：支持多数据库场景，记录当前使用的数据库
	// 好处：其他模块可以通过此变量获取当前数据库名，无需重复查询配置
	global.GVA_ACTIVE_DBNAME = &c.Dbname

	// 将配置写入配置文件
	// 好处：配置持久化到磁盘，下次启动时自动加载
	return global.GVA_VP.WriteConfig()
}

// EnsureDB 确保 SQLite 数据库存在并建立连接
// 设计原因：SQLite 是文件数据库，不需要像 MySQL 那样先创建数据库，但需要确保文件存在并建立连接
// 好处：
// 1. 统一接口：与其他数据库类型的 EnsureDB 方法接口一致，便于多态调用
// 2. 连接管理：统一管理数据库连接，避免连接泄漏
// 3. 错误处理：统一的错误处理机制，任何步骤失败都会返回错误
//
// 实现细节说明：
// - 类型检查：验证 context 中的数据库类型，确保调用的是正确的 Handler
// - 配置转换：将请求参数转换为 SQLite 配置结构体
// - 空数据库名处理：如果没有数据库名，跳过初始化（可能是测试场景）
// - 禁用外键约束：SQLite 在迁移时禁用外键约束，避免依赖顺序问题
// - Context 传递：将配置和数据库连接存入 context，供后续步骤使用
func (h SqliteInitHandler) EnsureDB(ctx context.Context, conf *request.InitDB) (next context.Context, err error) {
	// 验证 context 中的数据库类型是否为 SQLite
	// 设计原因：确保调用的是正确的 Handler，避免类型不匹配导致的错误
	// 好处：在运行时检查类型，如果类型不匹配立即返回错误，避免后续操作出错
	if s, ok := ctx.Value("dbtype").(string); !ok || s != "sqlite" {
		return ctx, ErrDBTypeMismatch
	}

	// 将请求参数转换为 SQLite 配置结构体
	// 设计原因：请求参数和配置结构体分离，符合分层架构
	// 好处：配置结构体可以包含更多字段，而请求参数只包含用户输入的必要字段
	c := conf.ToSqliteConfig()

	// 将配置存入 context，供后续步骤使用
	// 设计原因：使用 context 传递状态，避免全局变量污染
	// 好处：函数签名简洁，不需要传递多个参数，符合 Go 语言最佳实践
	next = context.WithValue(ctx, "config", c)

	// 如果没有数据库名，跳过初始化
	// 设计原因：某些场景下可能不需要创建数据库（如测试场景）
	// 好处：提供灵活性，允许部分初始化流程
	if c.Dbname == "" {
		return ctx, nil
	}

	// 获取 SQLite 空数据库连接字符串（不指定数据库文件）
	// 设计原因：SQLite 是文件数据库，需要先连接到 SQLite 服务器（实际上是文件系统）
	// 注意：SQLite 不需要像 MySQL 那样先创建数据库，文件不存在时会自动创建
	dsn := conf.SqliteEmptyDsn()

	var db *gorm.DB
	// 打开 SQLite 数据库连接
	// 设计原因：使用 GORM 作为 ORM 框架，提供统一的数据库操作接口
	// 配置说明：
	//   - DisableForeignKeyConstraintWhenMigrating: true
	//     设计原因：在迁移时禁用外键约束检查
	//     好处：
	//     1. 避免依赖顺序问题：表可以按任意顺序创建，不需要考虑外键依赖
	//     2. 简化迁移逻辑：不需要复杂的依赖图解析
	//     3. SQLite 特性：SQLite 支持延迟外键检查，可以在迁移完成后启用
	if db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	}); err != nil {
		return ctx, err
	}

	// 设置自动代码生成的根目录
	// 设计原因：自动代码生成需要知道项目根目录，用于生成文件路径
	// 使用 filepath.Abs("..") 获取上一级目录的绝对路径
	// 好处：使用绝对路径，避免相对路径在不同工作目录下出错
	global.GVA_CONFIG.AutoCode.Root, _ = filepath.Abs("..")

	// 将数据库连接存入 context，供后续步骤使用
	// 设计原因：后续的表创建和数据初始化都需要数据库连接
	// 好处：通过 context 传递，避免全局变量，符合依赖注入原则
	next = context.WithValue(next, "db", db)
	return next, err
}

// InitTables 执行所有初始化器的表创建操作
// 设计原因：SQLite 的表创建逻辑与其他数据库相同，直接复用通用函数
// 好处：
// 1. 代码复用：避免重复实现相同的逻辑
// 2. 统一流程：所有数据库类型的表创建流程一致
// 3. 易于维护：表创建逻辑的修改只需要在一个地方进行
//
// 实现说明：
// - 调用 createTables 通用函数，该函数会遍历所有初始化器并按顺序创建表
// - 支持幂等性：如果表已存在，跳过创建，不会报错
// - 使用 context 传递数据库连接和状态信息
func (h SqliteInitHandler) InitTables(ctx context.Context, inits initSlice) error {
	return createTables(ctx, inits)
}

// InitData 执行所有初始化器的数据初始化操作
// 设计原因：SQLite 的数据初始化逻辑需要按顺序执行，并支持幂等性检查
// 好处：
// 1. 幂等性：可以重复执行而不会出错，支持重新初始化
// 2. 顺序执行：按照初始化器的顺序执行，满足依赖关系
// 3. 错误处理：任何初始化器失败都会中断流程并返回错误
// 4. 日志输出：使用彩色输出显示初始化进度，提升用户体验
//
// 实现细节说明：
// - 可取消 Context：支持超时或取消机制，避免长时间阻塞
// - 幂等性检查：通过 DataInserted 方法检查数据是否已存在
// - 状态传递：每个初始化器返回新的 context，可能包含新创建的数据或状态
// - 错误处理：任何错误都会中断流程，确保数据一致性
// - 日志输出：使用 color.Info 输出彩色日志，区分不同状态（已存在/成功/失败）
func (h SqliteInitHandler) InitData(ctx context.Context, inits initSlice) error {
	// 创建可取消的 context，支持超时或取消机制
	// 设计原因：数据初始化可能需要较长时间，应该支持取消操作
	// 好处：
	// 1. 超时控制：可以设置超时时间，避免无限等待
	// 2. 优雅退出：可以取消正在进行的初始化操作
	// 3. 资源清理：取消后可以清理相关资源
	next, cancel := context.WithCancel(ctx)

	// 确保在函数退出时取消 context，释放资源
	// 设计原因：使用 defer 确保资源清理，即使函数提前返回也会执行
	// 使用命名函数参数：将 cancel 函数作为参数传递给 defer，避免闭包捕获问题
	// 好处：代码更清晰，明确表示 defer 函数的作用
	defer func(c func()) { c() }(cancel)

	// 遍历所有初始化器，按顺序执行数据初始化
	// 设计原因：初始化器已经按照依赖关系排序，按顺序执行即可满足依赖
	// 好处：简单的循环即可处理复杂的依赖关系，无需复杂的图算法
	for _, init := range inits {
		// 检查数据是否已插入（幂等性检查）
		// 设计原因：支持重复初始化，如果数据已存在则跳过
		// 好处：
		// 1. 安全性：重复执行不会导致数据重复或错误
		// 2. 可恢复性：初始化失败后可以重新执行，不会因为部分数据已存在而失败
		// 3. 用户体验：显示友好的提示信息，告知用户数据已存在
		if init.DataInserted(next) {
			color.Info.Printf(InitDataExist, Sqlite, init.InitializerName())
			continue
		}

		// 执行数据初始化
		// 返回新的 context，可能包含新创建的数据或状态信息
		// 设计原因：某些初始化器可能需要前面创建的数据（如 ID、关联关系等）
		// 好处：通过 context 传递状态，避免全局变量，符合依赖注入原则
		if n, err := init.InitializeData(next); err != nil {
			// 初始化失败，输出错误信息并返回错误
			// 设计原因：详细的错误信息有助于调试和问题定位
			// 好处：用户可以看到是哪个初始化器失败，以及失败的原因
			color.Info.Printf(InitDataFailed, Sqlite, init.InitializerName(), err)
			return err
		} else {
			// 更新 context，将新的状态传递给下一个初始化器
			// 设计原因：某些初始化器可能需要前面创建的数据
			// 好处：支持初始化器之间的数据传递，满足依赖关系
			next = n
			// 输出成功信息，提升用户体验
			color.Info.Printf(InitDataSuccess, Sqlite, init.InitializerName())
		}
	}

	// 所有初始化器执行完成，输出成功信息
	// 好处：给用户明确的反馈，表示初始化已完成
	color.Info.Printf(InitSuccess, Sqlite)
	return nil
}
