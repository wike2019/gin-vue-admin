// Package system 提供数据库初始化服务
// 设计思路：
// 1. 使用接口抽象，支持多种数据库类型的初始化（策略模式）
// 2. 使用注册机制，解耦初始化器的注册和执行（注册模式）
// 3. 使用排序机制，确保初始化顺序满足依赖关系（拓扑排序思想）
// 4. 使用 context 传递状态，避免全局变量污染（依赖注入）
package system

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"gorm.io/gorm"
)

// 数据库类型常量定义
// 好处：使用常量而不是字符串字面量，避免拼写错误，提高代码可维护性
const (
	Mysql           = "mysql"
	Pgsql           = "pgsql"
	Sqlite          = "sqlite"
	Mssql           = "mssql"
	InitSuccess     = "\n[%v] --> 初始数据成功!\n"
	InitDataExist   = "\n[%v] --> %v 的初始数据已存在!\n"
	InitDataFailed  = "\n[%v] --> %v 初始数据失败! \nerr: %+v\n"
	InitDataSuccess = "\n[%v] --> %v 初始数据成功!\n"
)

// 初始化顺序常量
// 设计原因：使用数值区间划分，便于管理初始化顺序和依赖关系
// - InitOrderSystem: 系统核心初始化（如用户表、角色表等），必须最先初始化
// - InitOrderInternal: 内部模块初始化，依赖系统核心表
// - InitOrderExternal: 外部扩展初始化，可以依赖前面的所有模块
// 好处：通过数值大小控制依赖顺序，如 B=A+1 表示 B 依赖 A，C=A+B 表示 C 依赖 A 和 B
// 这种设计避免了复杂的依赖图解析，使用简单的数值比较即可确定顺序
const (
	InitOrderSystem   = 10     // 系统核心初始化顺序
	InitOrderInternal = 1000   // 内部模块初始化顺序
	InitOrderExternal = 100000 // 外部扩展初始化顺序
)

// 错误定义
// 好处：预定义错误，便于错误处理和统一管理，避免在代码中散布错误字符串
var (
	ErrMissingDBContext        = errors.New("missing db in context")
	ErrMissingDependentContext = errors.New("missing dependent value in context")
	ErrDBTypeMismatch          = errors.New("db type mismatch")
)

// SubInitializer 子初始化器接口，每个 initializer 完成一个初始化过程（如某个表的初始化）
// 设计原因：使用接口抽象，让每个初始化器独立实现自己的逻辑，符合开闭原则
// 好处：
// 1. 解耦：初始化器之间相互独立，易于扩展和维护
// 2. 可测试：每个初始化器可以独立测试
// 3. 可组合：通过组合不同的初始化器完成完整的初始化流程
// 4. 幂等性：通过 TableCreated 和 DataInserted 方法支持重复执行（幂等操作）
type SubInitializer interface {
	// InitializerName 返回初始化器的名称
	// 注意：名称不一定代表单独一个表，可能是多个表或一个功能模块，使用更宽泛的语义
	InitializerName() string
	// MigrateTable 执行表结构迁移（建表或修改表结构）
	// 返回新的 context，用于在初始化器之间传递状态（如创建的表对象、生成的ID等）
	MigrateTable(ctx context.Context) (next context.Context, err error)
	// InitializeData 执行数据初始化（插入初始数据）
	// 返回新的 context，用于传递初始化过程中产生的数据（如下一个初始化器需要的数据）
	InitializeData(ctx context.Context) (next context.Context, err error)
	// TableCreated 检查表是否已创建，用于支持幂等操作（重复初始化不会出错）
	TableCreated(ctx context.Context) bool
	// DataInserted 检查数据是否已插入，用于支持幂等操作
	DataInserted(ctx context.Context) bool
}

// TypedDBInitHandler 数据库类型特定的初始化处理器接口
// 设计原因：不同数据库类型（MySQL、PostgreSQL等）的初始化逻辑有差异，使用策略模式封装
// 好处：
// 1. 支持多数据库：通过实现不同 Handler 支持 MySQL、PostgreSQL、SQLite、MSSQL 等
// 2. 可扩展：新增数据库类型只需实现此接口，无需修改现有代码
// 3. 职责分离：数据库特定的逻辑（如建库SQL）封装在各自的 Handler 中
type TypedDBInitHandler interface {
	// EnsureDB 确保数据库存在，如果不存在则创建
	// 返回包含数据库连接的 context，后续操作从此 context 获取数据库连接
	// 注意：建库失败属于致命错误，通常应该 panic
	EnsureDB(ctx context.Context, conf *request.InitDB) (context.Context, error)
	// WriteConfig 将初始化配置回写到配置文件
	// 好处：初始化完成后，系统可以记住数据库配置，下次启动时直接使用
	WriteConfig(ctx context.Context) error
	// InitTables 执行所有初始化器的表创建操作
	// 好处：统一的表创建流程，可以在创建前后执行公共逻辑（如事务管理、日志记录）
	InitTables(ctx context.Context, inits initSlice) error
	// InitData 执行所有初始化器的数据初始化操作
	// 好处：统一的数据初始化流程，支持事务回滚、错误恢复等
	InitData(ctx context.Context, inits initSlice) error
}

// orderedInitializer 带顺序的初始化器包装器
// 设计原因：使用组合模式（组合 SubInitializer 接口），在不修改原接口的情况下添加顺序字段
// 好处：
// 1. 符合开闭原则：不需要修改 SubInitializer 接口，通过组合扩展功能
// 2. 单一职责：SubInitializer 只负责初始化逻辑，orderedInitializer 只负责顺序管理
// 3. 灵活性：可以在运行时动态设置顺序，支持依赖关系的调整
type orderedInitializer struct {
	order          int // 初始化顺序，数值越小越先执行
	SubInitializer     // 嵌入接口，通过组合实现功能扩展
}

// initSlice 初始化器切片类型
// 设计原因：定义为独立类型，可以为此类型实现 sort.Interface 接口，支持排序
// 好处：使用 Go 标准库的 sort.Sort 进行排序，代码简洁高效
type initSlice []*orderedInitializer

var (
	// initializers 全局初始化器列表，在程序启动时通过 RegisterInit 注册
	// 设计原因：使用全局变量收集所有初始化器，在 InitDB 时统一执行
	// 好处：解耦注册和执行，初始化器可以在任何地方注册，无需关心执行时机
	initializers initSlice
	// cache 初始化器名称到初始化器的映射缓存
	// 好处：
	// 1. 快速查找：O(1) 时间复杂度查找初始化器
	// 2. 防止重复注册：通过名称检查避免重复注册同一个初始化器
	cache map[string]*orderedInitializer
)

// RegisterInit 注册初始化器到全局列表
// 参数：
//   - order: 初始化顺序，数值越小越先执行（用于控制依赖关系）
//   - i: 实现 SubInitializer 接口的初始化器实例
//
// 设计原因：使用注册模式，将初始化器的注册和执行分离
// 好处：
// 1. 解耦：初始化器可以在模块初始化时注册（如 init 函数），无需关心何时执行
// 2. 灵活性：可以动态添加或移除初始化器，支持插件化扩展
// 3. 可测试：可以单独注册测试用的初始化器
// 4. 依赖控制：通过 order 参数控制初始化顺序，满足依赖关系
func RegisterInit(order int, i SubInitializer) {
	// 延迟初始化：只有在第一次调用时才初始化，避免全局变量初始化顺序问题
	if initializers == nil {
		initializers = initSlice{}
	}
	if cache == nil {
		cache = map[string]*orderedInitializer{}
	}
	name := i.InitializerName()
	// 检查名称冲突：同一名称只能注册一次，避免重复初始化
	if _, existed := cache[name]; existed {
		panic(fmt.Sprintf("Name conflict on %s", name))
	}
	// 创建包装器，添加顺序信息
	ni := orderedInitializer{order, i}
	initializers = append(initializers, &ni)
	cache[name] = &ni
}

/* ---- * service * ---- */

// InitDBService 数据库初始化服务
// 设计原因：使用服务结构体，符合服务层设计模式，便于依赖注入和测试
type InitDBService struct{}

// InitDB 数据库初始化总入口，执行完整的初始化流程
// 流程：创建数据库 -> 创建表 -> 初始化数据 -> 回写配置
// 设计原因：统一的初始化入口，确保初始化流程的一致性和正确性
// 好处：
// 1. 流程清晰：所有初始化步骤在一个函数中，易于理解和维护
// 2. 错误处理：统一的错误处理机制，任何步骤失败都会中断并返回错误
// 3. 事务性：可以在此处添加事务管理，确保初始化的原子性
func (initDBService *InitDBService) InitDB(conf request.InitDB) (err error) {
	// 创建初始 context，用于在整个初始化流程中传递状态
	ctx := context.TODO()
	// 将管理员密码存入 context，后续初始化器可以从 context 获取（如创建默认管理员账户）
	ctx = context.WithValue(ctx, "adminPassword", conf.AdminPassword)

	// 检查是否有可用的初始化器
	if len(initializers) == 0 {
		return errors.New("无可用初始化过程，请检查初始化是否已执行完成")
	}

	// 对初始化器按顺序排序，确保有依赖关系的初始化器排在后面执行
	// 设计原因：通过数值比较实现拓扑排序的思想，简单高效
	// 好处：避免复杂的图算法，使用简单的整数比较即可确定执行顺序
	sort.Sort(&initializers)

	// 依赖顺序设计说明：
	// 1. 单一依赖：B=A+1, C=A+1 表示 B 和 C 都依赖 A
	//    - 由于 BC 之间没有依赖关系，执行顺序不影响结果
	// 2. 多重依赖：C=A+B, D=A+B+C, E=A+1
	//    - C 依赖 A 和 B，数值必然 > A 和 B，因此在 AB 之后执行
	//    - D 依赖 A、B、C，数值必然 > A、B、C，因此在 ABC 后执行
	//    - E 只依赖 A，顺序与 CD 无关，可以与 CD 并行或任意顺序执行

	// 根据数据库类型选择对应的初始化处理器（策略模式）
	// 设计原因：不同数据库类型的初始化逻辑有差异（如建库SQL、数据类型映射等）
	// 好处：通过接口抽象，使用统一的接口调用，但内部实现根据数据库类型不同而不同
	var initHandler TypedDBInitHandler
	switch conf.DBType {
	case "mysql":
		initHandler = NewMysqlInitHandler()
		ctx = context.WithValue(ctx, "dbtype", "mysql")
	case "pgsql":
		initHandler = NewPgsqlInitHandler()
		ctx = context.WithValue(ctx, "dbtype", "pgsql")
	case "sqlite":
		initHandler = NewSqliteInitHandler()
		ctx = context.WithValue(ctx, "dbtype", "sqlite")
	case "mssql":
		initHandler = NewMssqlInitHandler()
		ctx = context.WithValue(ctx, "dbtype", "mssql")
	default:
		// 默认使用 MySQL，向后兼容
		initHandler = NewMysqlInitHandler()
		ctx = context.WithValue(ctx, "dbtype", "mysql")
	}

	// 步骤1：确保数据库存在，如果不存在则创建
	// 返回的 context 包含数据库连接对象，后续步骤从此获取
	ctx, err = initHandler.EnsureDB(ctx, &conf)
	if err != nil {
		return err
	}

	// 从 context 获取数据库连接，设置为全局数据库对象
	// 设计原因：后续代码可能直接使用 global.GVA_DB，保持兼容性
	// 注意：这里假设 EnsureDB 已经将数据库连接存入 context
	db := ctx.Value("db").(*gorm.DB)
	global.GVA_DB = db

	// 步骤2：创建所有表结构
	// 好处：先创建表，再插入数据，符合数据库操作的逻辑顺序
	if err = initHandler.InitTables(ctx, initializers); err != nil {
		return err
	}

	// 步骤3：初始化数据（插入初始数据）
	// 好处：表创建完成后，才能插入数据，顺序不能颠倒
	if err = initHandler.InitData(ctx, initializers); err != nil {
		return err
	}

	// 步骤4：回写配置到配置文件
	// 好处：初始化完成后保存配置，下次启动时可以直接使用，无需重新初始化
	if err = initHandler.WriteConfig(ctx); err != nil {
		return err
	}

	// 清空全局变量，释放内存，避免内存泄漏
	// 设计原因：初始化是一次性操作，完成后不再需要这些数据
	initializers = initSlice{}
	cache = map[string]*orderedInitializer{}
	return nil
}

// createDatabase 创建数据库的通用函数，供 EnsureDB() 方法调用
// 参数：
//   - dsn: 数据源名称（不含数据库名），用于连接到数据库服务器
//   - driver: 数据库驱动名称（如 "mysql", "postgres" 等）
//   - createSql: 创建数据库的 SQL 语句（如 "CREATE DATABASE IF NOT EXISTS dbname"）
//
// 设计原因：不同数据库类型的 EnsureDB 都需要创建数据库，提取公共逻辑
// 好处：
// 1. 代码复用：避免在每个 Handler 中重复实现相同的逻辑
// 2. 统一错误处理：所有数据库创建的错误处理逻辑集中在一处
// 3. 资源管理：统一的数据库连接管理和关闭逻辑
func createDatabase(dsn string, driver string, createSql string) error {
	// 打开数据库连接（连接到数据库服务器，而非具体数据库）
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return err
	}
	// 延迟关闭连接，确保资源释放（即使后续步骤出错也会执行）
	// 使用命名函数参数，便于错误处理
	defer func(db *sql.DB) {
		err = db.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(db)

	// 测试连接是否正常（Ping 检查连接是否可用）
	if err = db.Ping(); err != nil {
		return err
	}

	// 执行创建数据库的 SQL 语句
	_, err = db.Exec(createSql)
	return err
}

// createTables 创建所有表的通用函数，作为默认的 InitTables 实现
// 设计原因：大多数数据库类型的表创建逻辑相同，提取为公共函数
// 好处：
// 1. 代码复用：各 Handler 可以复用此函数，只需实现数据库特定的部分
// 2. 统一流程：确保所有数据库类型的表创建流程一致
// 3. 幂等性支持：通过 TableCreated 检查，支持重复执行（不会重复创建表）
func createTables(ctx context.Context, inits initSlice) error {
	// 创建可取消的 context，支持超时或取消机制
	// 设计原因：表创建可能需要较长时间，应该支持取消操作
	next, cancel := context.WithCancel(ctx)
	// 确保在函数退出时取消 context，释放资源
	defer func(c func()) { c() }(cancel)

	// 遍历所有初始化器，按顺序创建表
	for _, init := range inits {
		// 检查表是否已创建（幂等性检查）
		// 好处：如果表已存在，跳过创建，支持重复初始化而不出错
		if init.TableCreated(next) {
			continue
		}

		// 执行表迁移（创建表或修改表结构）
		// 返回新的 context，可能包含创建的表对象或其他状态信息
		if n, err := init.MigrateTable(next); err != nil {
			return err
		} else {
			// 更新 context，将新的状态传递给下一个初始化器
			// 设计原因：某些初始化器可能需要前面创建的表对象或ID
			next = n
		}
	}
	return nil
}

/* -- sortable interface -- */

// Len 返回初始化器切片的长度
// 实现 sort.Interface 接口，使 initSlice 支持排序
func (a initSlice) Len() int {
	return len(a)
}

// Less 比较两个初始化器的顺序，返回 true 表示 i 应该在 j 之前
// 设计原因：通过比较 order 字段实现排序，数值小的排在前面（先执行）
// 好处：
// 1. 简单高效：使用整数比较，O(1) 时间复杂度，比复杂的图算法简单
// 2. 易于理解：依赖关系通过数值大小直观表达
// 3. 稳定排序：相同 order 的元素保持原有顺序（稳定排序）
func (a initSlice) Less(i, j int) bool {
	return a[i].order < a[j].order
}

// Swap 交换两个初始化器的位置
// 实现 sort.Interface 接口，用于排序算法中的元素交换
func (a initSlice) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
