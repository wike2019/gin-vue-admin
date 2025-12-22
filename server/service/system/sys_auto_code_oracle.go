package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/response"
)

// AutoCodeOracle Oracle数据库自动代码生成服务的单例实例
// 使用单例模式的好处：
// 1. 避免重复创建对象，节省内存资源
// 2. 确保整个应用只有一个实例，保持行为一致性
// 3. 便于在接口中统一管理和调用（参考 sys_auto_code_interface.go 中的 Database 接口）
var AutoCodeOracle = new(autoCodeOracle)

// autoCodeOracle Oracle数据库元数据查询服务
// 该结构体实现了 Database 接口，专门用于 Oracle 数据库的元数据查询
// 设计为私有类型，外部只能通过 AutoCodeOracle 单例访问，符合封装原则
// 采用空结构体的设计，因为不需要存储任何状态信息
// 所有方法都是无状态的，只负责执行数据库查询操作
// 这种设计的好处：
// 1. 零内存占用（空结构体在 Go 中不占用内存）
// 2. 方法可以安全地并发调用，无需考虑状态同步问题
// 3. 代码简洁，职责单一，符合单一职责原则
type autoCodeOracle struct{}

// GetDB 获取Oracle数据库服务器的所有用户（Schema）名
// 设计说明：
//  1. 使用 all_users 系统视图：这是 Oracle 官方提供的系统视图，专门用于查询数据库用户信息
//     在 Oracle 中，用户（USER）和模式（SCHEMA）是紧密关联的概念，每个用户拥有一个同名的模式
//     好处：标准、可靠，能够获取所有用户，不受版本差异影响，性能稳定
//  2. 使用 lower(username) 转换为小写：
//     原因：Oracle 默认对象名是大写的，但转换为小写能够统一格式，与其他数据库（如 MySQL、PostgreSQL）保持一致
//     好处：前端展示更友好，代码生成时统一使用小写命名，符合常见编程规范
//  3. 使用双引号包裹 "database" 别名：
//     原因：Oracle 中如果别名不使用双引号，会被自动转换为大写；使用双引号可以保持大小写不变
//     好处：确保返回的 JSON 字段名为小写的 "database"，与 response.Db 结构体的字段名匹配
//  4. 直接使用 global.GVA_DBList[businessDB]：
//     注意：这里假设 businessDB 参数必须提供，与 MySQL/PostgreSQL 的实现略有不同
//     其他数据库实现中会判断 businessDB == "" 来选择默认数据库，但 Oracle 实现直接使用字典查找
//     好处：强制要求明确指定数据源，避免隐式使用默认数据库可能带来的混淆
//
// 参数说明：
//   - businessDB: 业务数据库标识符，必须指定，用于从 global.GVA_DBList 中选择对应的数据库连接
//
// Author [piexlmax](https://github.com/piexlmax)
// Author [SliverHorn](https://github.com/SliverHorn)
func (s *autoCodeOracle) GetDB(businessDB string) (data []response.Db, err error) {
	var entities []response.Db
	// 查询所有用户（在 Oracle 中，用户即 Schema）
	// all_users 是 Oracle 的系统视图，包含所有数据库用户的信息
	sql := `SELECT lower(username) AS "database" FROM all_users`
	err = global.GVA_DBList[businessDB].Raw(sql).Scan(&entities).Error
	return entities, err
}

// GetTables 获取指定用户（Schema）的所有表名
// 设计说明：
//  1. 使用 all_tables 系统视图：这是 Oracle 提供的系统视图，包含当前用户有权限访问的所有表
//     与 user_tables（只包含当前用户自己的表）不同，all_tables 包含所有可见的表
//     好处：能够查询到其他用户授权给自己访问的表，提供更全面的表列表
//  2. 使用 owner 字段过滤：owner 表示表的拥有者（即 Schema 名）
//     原因：Oracle 是多用户（多 Schema）数据库，同一个表名可能在不同 Schema 中存在
//     好处：通过 owner 精确匹配，只返回指定 Schema 中的表，避免表名冲突
//  3. 使用 lower() 函数进行大小写转换：
//     原因：Oracle 默认对象名是大写的，但用户可能使用小写创建对象，使用 lower() 统一处理
//     WHERE lower(owner) = ? 和返回 lower(table_name) 确保查询和结果都使用小写，保持一致性
//     好处：不区分大小写的查询更加灵活，结果格式统一，便于后续处理
//  4. 使用双引号包裹 "table_name" 别名：
//     原因：Oracle 中别名不使用双引号会被转换为大写，使用双引号保持大小写不变
//     好处：确保返回的 JSON 字段名为小写的 "table_name"，与 response.Table 结构体的字段名匹配
//  5. 使用参数化查询（?占位符）：
//     原因：dbName 来自用户输入，如果直接拼接存在 SQL 注入风险
//     好处：GORM 会自动转义特殊字符，确保查询安全，防止 SQL 注入攻击
//
// 参数说明：
//   - businessDB: 业务数据库标识符，用于从 global.GVA_DBList 中选择对应的数据库连接
//   - dbName: Schema 名称（在 Oracle 中即用户名），用于过滤指定 Schema 下的表
//
// Author [piexlmax](https://github.com/piexlmax)
// Author [SliverHorn](https://github.com/SliverHorn)
func (s *autoCodeOracle) GetTables(businessDB string, dbName string) (data []response.Table, err error) {
	var entities []response.Table
	// 查询指定 Schema（owner）下的所有表
	// all_tables 包含当前用户有权限访问的所有表，owner 字段表示表的拥有者（Schema名）
	// 使用 lower() 确保大小写不敏感的匹配，返回结果统一为小写
	sql := `select lower(table_name) as "table_name" from all_tables where lower(owner) = ?`

	err = global.GVA_DBList[businessDB].Raw(sql, dbName).Scan(&entities).Error
	return entities, err
}

// GetColumn 获取指定数据库和指定数据表的所有字段信息（字段名、类型、注释、主键标识等）
// 设计说明：
//
//  1. 为什么需要字段注释和主键信息？
//     - 字段注释：用于自动生成代码时的字段说明、API 文档、前端表单标签等
//     - 主键标识：用于自动生成代码时识别主键字段，决定是否自动填充、是否可编辑、是否必填等
//
//  2. NUMBER 类型的特殊处理（data_type 字段）：
//     - Oracle 的 NUMBER 类型是一个通用数值类型，可以表示整数和小数
//     - 当 DATA_SCALE = 0 时，表示整数（没有小数位），转换为 'int' 类型
//     - 当 DATA_SCALE != 0 时，表示小数，保持为 'number' 类型
//     - 好处：代码生成时能够正确区分整数和浮点数，生成更准确的类型定义（如 Go 的 int vs float64）
//
//  3. 数据类型长度/精度的处理（data_type_long 字段）：
//     - NUMBER 类型：使用 DATA_PRECISION（精度，如 NUMBER(10,2) 中的 10）
//     - 其他类型（如 VARCHAR2）：使用 DATA_LENGTH（字符长度，如 VARCHAR2(255) 中的 255）
//     - 好处：统一返回类型详细信息，便于代码生成时正确设置字段长度限制
//
//  4. 多表关联获取完整信息的设计：
//     - all_tab_columns：存储表的列基本信息（字段名、类型、长度等）
//     - all_col_comments：存储列的注释信息（通过 COMMENT ON COLUMN 添加的注释）
//     - all_constraints + all_cons_columns：存储约束信息（主键、外键等）
//     - 使用 JOIN 关联这些系统视图，一次性获取所有需要的信息
//     - 好处：避免多次查询，提高性能；保证数据一致性（原子性查询）
//
//  5. 子查询获取主键信息的设计：
//     - all_constraints：存储约束定义，CONSTRAINT_TYPE = 'P' 表示主键约束（PRIMARY KEY）
//     - all_cons_columns：存储约束涉及的列信息
//     - 通过约束名（CONSTRAINT_NAME）关联，找到主键约束涉及的所有列
//     - 使用 LEFT JOIN：即使表没有主键，也能返回所有字段（主键字段标记为 0）
//     - 使用 CASE WHEN 将主键信息转换为 1/0（1=是主键，0=不是主键）
//     - 好处：能够准确识别复合主键中的每个字段，支持没有主键的表
//
//  6. 使用 lower() 函数进行大小写转换：
//     - WHERE 条件中使用 lower()：确保查询时不区分大小写，更加灵活
//     - SELECT 中使用 lower(COLUMN_NAME)：统一返回小写字段名，保持格式一致
//     - 好处：不区分大小写的查询更加用户友好，结果格式统一便于处理
//
//  7. 使用双引号包裹别名（"column_name", "data_type" 等）：
//     - 原因：Oracle 中别名不使用双引号会被自动转换为大写
//     - 好处：确保返回的 JSON 字段名与 response.Column 结构体的字段名完全匹配
//
//  8. ORDER BY COLUMN_ID：
//     - COLUMN_ID 是字段在表中的定义顺序（从 1 开始）
//     - 好处：返回的字段顺序与表结构定义顺序一致，便于生成代码时保持字段顺序，提高代码可读性
//
//  9. 使用参数化查询（?占位符）：
//     - tableName 和 dbName 来自用户输入，如果直接拼接存在 SQL 注入风险
//     - 使用 ? 占位符，GORM 会自动转义特殊字符，确保查询安全
//     - 好处：防止 SQL 注入攻击，提高安全性
//
// 参数说明：
//   - businessDB: 业务数据库标识符，用于从 global.GVA_DBList 中选择对应的数据库连接
//   - tableName: 表名，用于查询指定表的字段信息
//   - dbName: Schema 名称（在 Oracle 中即用户名），用于精确定位表（同一个表名可能在不同 Schema 中存在）
//
// Author [piexlmax](https://github.com/piexlmax)
// Author [SliverHorn](https://github.com/SliverHorn)
func (s *autoCodeOracle) GetColumn(businessDB string, tableName string, dbName string) (data []response.Column, err error) {
	var entities []response.Column
	// 复杂的 SQL 查询，从多个 Oracle 系统视图中关联获取完整的字段信息
	// 包括：字段基本信息、数据类型（特殊处理 NUMBER 类型）、长度/精度、字段注释、主键标识等
	sql := `
	SELECT
    lower(a.COLUMN_NAME) as "column_name",                                    -- 字段名（转换为小写）
    (CASE WHEN a.DATA_TYPE = 'NUMBER' AND a.DATA_SCALE=0 THEN 'int' else lower(a.DATA_TYPE) end)  as "data_type",  -- 字段类型（NUMBER且无小数位时转换为int）
    (CASE WHEN a.DATA_TYPE = 'NUMBER' THEN a.DATA_PRECISION else a.DATA_LENGTH end) as "data_type_long",  -- 类型长度/精度（NUMBER用精度，其他用长度）
    b.COMMENTS as "column_comment",                                           -- 字段注释
    (CASE WHEN pk.COLUMN_NAME IS NOT NULL THEN 1 ELSE 0 END) as "primary_key",  -- 是否为主键（1=是，0=否）
    a.COLUMN_ID                                                               -- 字段在表中的顺序位置
FROM
    all_tab_columns a                                                         -- 表的列基本信息
JOIN
    all_col_comments b ON a.OWNER = b.OWNER AND a.TABLE_NAME = b.TABLE_NAME AND a.COLUMN_NAME = b.COLUMN_NAME  -- 关联获取列注释
LEFT JOIN
    (
        -- 子查询：获取所有主键约束涉及的列
        SELECT
            acc.OWNER,
            acc.TABLE_NAME,
            acc.COLUMN_NAME
        FROM
            all_cons_columns acc                                              -- 约束涉及的列信息
        JOIN
            all_constraints ac ON acc.OWNER = ac.OWNER AND acc.CONSTRAINT_NAME = ac.CONSTRAINT_NAME  -- 关联约束定义
        WHERE
            ac.CONSTRAINT_TYPE = 'P'                                          -- 'P' 表示主键约束（PRIMARY KEY）
    ) pk ON a.OWNER = pk.OWNER AND a.TABLE_NAME = pk.TABLE_NAME AND a.COLUMN_NAME = pk.COLUMN_NAME  -- LEFT JOIN：即使没有主键也能返回所有字段
WHERE
    lower(a.table_name) = ?                                                   -- 表名（参数化查询，防止 SQL 注入）
    AND lower(a.OWNER) = ?                                                    -- Schema名（参数化查询，防止 SQL 注入）
ORDER BY
    a.COLUMN_ID                                                               -- 按字段定义顺序排序
`

	// 执行参数化查询，tableName 和 dbName 作为参数传入，GORM 会自动转义特殊字符
	// 好处：防止 SQL 注入攻击，提高安全性
	err = global.GVA_DBList[businessDB].Raw(sql, tableName, dbName).Scan(&entities).Error
	return entities, err
}
