package system

import (
	"fmt"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/response"
)

// AutoCodeMssql 使用单例模式创建全局实例
// 好处：1. 避免重复创建对象，节省内存 2. 提供全局访问点，便于在系统各处使用 3. 符合Go语言常见的服务封装模式
var AutoCodeMssql = new(autoCodeMssql)

// autoCodeMssql 空结构体作为接收者类型
// 好处：1. 零内存占用（空结构体大小为0） 2. 仅用于组织方法，不需要存储状态 3. 清晰表明这是无状态的工具类
type autoCodeMssql struct{}

// GetDB 获取数据库的所有数据库名
// Author [piexlmax](https://github.com/piexlmax)
// Author [SliverHorn](https://github.com/SliverHorn)
//
// 参数说明：
//   - businessDB: 业务数据库连接名，为空字符串时使用默认连接，否则从连接池中获取指定连接
//
// 设计说明：
//  1. 使用 sys.databases 系统视图：这是 SQL Server 的标准系统视图，用于查询服务器上所有数据库的信息
//     这是 MSSQL 官方推荐的元数据查询方式，兼容性好，适用于 SQL Server 2005 及以上版本
//  2. 支持多数据库连接：通过 businessDB 参数实现多数据源支持，提高系统灵活性
//  3. 使用 Raw SQL + Scan：直接执行SQL并扫描到结构体，性能更好，避免了ORM的复杂转换
//
// 好处：
//   - 标准接口：使用 sys.databases 系统视图保证了跨版本兼容性（SQL Server 2005+）
//   - 灵活配置：支持默认连接和自定义连接，适应不同业务场景
//   - 性能优化：直接SQL查询，减少ORM层开销
//   - 权限友好：sys.databases 视图对所有有权限的用户可见，无需特殊权限
func (s *autoCodeMssql) GetDB(businessDB string) (data []response.Db, err error) {
	var entities []response.Db
	// 使用 sys.databases 系统视图查询所有数据库名称
	// sys.databases 是 SQL Server 的目录视图，返回服务器上的所有数据库信息
	// 使用 AS 'database' 别名与响应结构体的字段名匹配
	sql := "select name AS 'database' from sys.databases;"
	if businessDB == "" {
		// 使用默认数据库连接（全局连接）
		err = global.GVA_DB.Raw(sql).Scan(&entities).Error
	} else {
		// 使用指定的业务数据库连接（多数据源场景）
		// 从连接池中获取预配置的连接，实现数据源切换
		err = global.GVA_DBList[businessDB].Raw(sql).Scan(&entities).Error
	}
	return entities, err
}

// GetTables 获取数据库的所有表名
// Author [piexlmax](https://github.com/piexlmax)
// Author [SliverHorn](https://github.com/SliverHorn)
//
// 参数说明：
//   - businessDB: 业务数据库连接名，为空字符串时使用默认连接
//   - dbName: 目标数据库名称，用于查询指定数据库下的表
//
// 设计说明：
//  1. 使用 sysobjects 系统表：这是 SQL Server 的传统系统表，用于存储数据库对象信息
//     虽然在新版本中推荐使用 sys.objects，但 sysobjects 仍然被广泛使用且兼容性更好
//  2. 使用 %s.DBO.sysobjects 完整路径：通过数据库名称限定，明确指定要查询的数据库
//     DBO 是默认架构名，确保能正确访问目标数据库的表信息
//  3. 过滤条件 xtype='U'：xtype 表示对象类型，'U' 代表用户表（User Table）
//     其他常见类型：'S'=系统表，'V'=视图，'P'=存储过程等
//  4. 使用 fmt.Sprintf 动态拼接：由于需要指定数据库名，使用字符串格式化构建SQL
//     注意：虽然这种方式不如参数化查询安全，但 dbName 通常在系统内部使用，风险可控
//
// 好处：
//   - 兼容性强：sysobjects 系统表在 SQL Server 2000+ 都可用，兼容性好
//   - 查询精确：通过数据库名称限定，只返回指定数据库的表，避免数据混淆
//   - 类型过滤：通过 xtype='U' 只返回用户表，排除系统表和视图等对象
//   - 灵活性：支持跨数据库查询，可以从一个连接查询另一个数据库的表信息
func (s *autoCodeMssql) GetTables(businessDB string, dbName string) (data []response.Table, err error) {
	var entities []response.Table

	// 使用 sysobjects 系统表查询指定数据库的所有用户表
	// %s.DBO.sysobjects：完整的三部分命名（数据库名.架构名.对象名），确保查询正确的数据库
	// xtype='U'：只查询用户表（User Table），排除系统表、视图等对象
	sql := fmt.Sprintf(`select name as 'table_name' from %s.DBO.sysobjects where xtype='U'`, dbName)
	if businessDB == "" {
		// 使用默认连接，执行跨数据库查询
		err = global.GVA_DB.Raw(sql).Scan(&entities).Error
	} else {
		// 使用指定的业务数据库连接
		err = global.GVA_DBList[businessDB].Raw(sql).Scan(&entities).Error
	}

	return entities, err
}

// GetColumn 获取指定数据库和指定数据表的所有字段名,类型值等
// Author [piexlmax](https://github.com/piexlmax)
// Author [SliverHorn](https://github.com/SliverHorn)
//
// 参数说明：
//   - businessDB: 业务数据库连接名，为空字符串时使用默认连接
//   - tableName: 目标数据表名称
//   - dbName: 目标数据库名称
//
// 设计说明：
//
//  1. 复杂的系统视图 JOIN 查询设计：
//     - sys.columns：存储表中所有列的基本信息（列名、数据类型ID、最大长度等）
//     - sys.types：存储数据类型信息（类型名、是否为用户定义类型等）
//     - sys.objects：存储数据库对象信息（表、视图、存储过程等）
//     - sys.indexes：存储索引信息（包括主键索引）
//     - sys.index_columns：存储索引与列的关联信息
//     - sys.key_constraints：存储键约束信息（主键、唯一键等）
//     使用多个系统视图关联，一次性获取完整的列元数据信息
//
//  2. JOIN 策略说明：
//     - sys.columns JOIN sys.types：通过 user_type_id 关联，获取列的数据类型名称
//     使用 INNER JOIN 确保只返回有对应类型的列
//     - LEFT JOIN sys.objects：查找指定的表对象（type='U' 表示用户表）
//     使用 LEFT JOIN 是为了后续的 WHERE 过滤，如果表不存在则不返回任何结果
//     - LEFT JOIN sys.indexes：查找主键索引（is_primary_key = 1）
//     使用 LEFT JOIN 确保非主键列也能正常返回
//     - LEFT JOIN sys.index_columns：关联索引与列的对应关系
//     确定哪些列属于主键索引
//     - LEFT JOIN sys.key_constraints：最终确认主键约束
//     通过多重 LEFT JOIN 确保准确识别主键，同时不影响非主键列的返回
//
//  3. data_type_long 字段的处理：
//     - 使用 sc.max_length 直接获取列的最大长度（以字节为单位）
//     - 对于字符类型（varchar, nvarchar），max_length 表示字符/字节的最大长度
//     - 对于数值类型，max_length 可能不适用，但保留该字段便于后续扩展
//     注意：MSSQL 中不同类型的数据长度表示方式不同，这里统一使用 max_length
//
//  4. 主键判断逻辑：
//     - 通过多层 LEFT JOIN 关联 sys.indexes（is_primary_key=1）和 sys.key_constraints
//     - 使用 CASE 语句：如果 pk.object_id IS NOT NULL 说明该列属于主键，返回 1，否则返回 0
//     - 这种设计能准确识别复合主键中的每个列
//
//  5. 过滤条件：
//     - st.is_user_defined=0：只返回系统定义的数据类型，排除用户自定义类型
//     这对于代码生成很重要，因为需要映射到标准数据类型
//     - sc.object_id = so.object_id：确保只返回指定表的列
//
//  6. 排序：
//     - 使用 sc.column_id 排序，保证返回的列顺序与表定义顺序一致
//     - column_id 是列在表中的物理顺序，这对于代码生成很重要，可以保持字段的原始定义顺序
//
//  7. 数据库限定：
//     - 使用 %s.sys.xxx 格式（如 %s.sys.columns）明确指定数据库
//     - 支持跨数据库查询，可以从一个连接查询另一个数据库的表结构
//
// 好处：
//   - 一次性获取完整信息：列名、类型、长度、主键标识、顺序，减少多次查询的开销
//   - 主键识别准确：通过多表关联准确识别主键字段，包括复合主键
//   - 顺序保持：按 column_id 排序，保持表结构的原始定义顺序
//   - 类型标准化：通过过滤 is_user_defined=0 只返回系统标准类型，便于代码生成
//   - 兼容性好：使用 SQL Server 2005+ 的标准系统视图，兼容现代版本
//   - 跨数据库支持：通过数据库名称限定，支持跨数据库查询表结构
func (s *autoCodeMssql) GetColumn(businessDB string, tableName string, dbName string) (data []response.Column, err error) {
	var entities []response.Column
	// 复杂的系统视图关联查询，获取表的完整列信息
	sql := fmt.Sprintf(`
SELECT
    sc.name AS column_name,                                    -- 列名：从 sys.columns 获取
    st.name AS data_type,                                      -- 数据类型：从 sys.types 获取类型名称
    sc.max_length AS data_type_long,                           -- 数据类型长度：列的最大长度（字节数）
    CASE
        WHEN pk.object_id IS NOT NULL THEN 1                   -- 如果关联到主键约束，返回 1（是主键）
        ELSE 0                                                  -- 否则返回 0（非主键）
    END AS primary_key,                                        -- 主键标识
    sc.column_id                                               -- 列ID：用于排序，保持列在表中的原始顺序
FROM
    %s.sys.columns sc                                          -- 主表：指定数据库的列信息表
JOIN
    sys.types st ON sc.user_type_id=st.user_type_id            -- 内连接：获取列的数据类型名称（系统类型表）
LEFT JOIN
    %s.sys.objects so ON so.name='%s' AND so.type='U'          -- 左连接：查找指定的用户表对象（type='U'表示用户表）
LEFT JOIN
    %s.sys.indexes si ON si.object_id = so.object_id AND si.is_primary_key = 1  -- 左连接：查找该表的主键索引
LEFT JOIN
    %s.sys.index_columns sic ON sic.object_id = si.object_id AND sic.index_id = si.index_id AND sic.column_id = sc.column_id  -- 左连接：关联索引与列的关系
LEFT JOIN
    %s.sys.key_constraints pk ON pk.object_id = si.object_id   -- 左连接：确认主键约束
WHERE
    st.is_user_defined=0 AND sc.object_id = so.object_id       -- 过滤：只返回系统定义的类型，且只返回指定表的列
ORDER BY
    sc.column_id                                               -- 排序：按列在表中的原始顺序排序
`, dbName, dbName, tableName, dbName, dbName, dbName)

	if businessDB == "" {
		// 使用默认连接，执行跨数据库查询
		err = global.GVA_DB.Raw(sql).Scan(&entities).Error
	} else {
		// 使用指定的业务数据库连接
		err = global.GVA_DBList[businessDB].Raw(sql).Scan(&entities).Error
	}

	return entities, err
}
