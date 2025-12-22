package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/response"
)

// AutoCodeMysql 使用单例模式创建全局实例
// 好处：1. 避免重复创建对象，节省内存 2. 提供全局访问点，便于在系统各处使用 3. 符合Go语言常见的服务封装模式
var AutoCodeMysql = new(autoCodeMysql)

// autoCodeMysql 空结构体作为接收者类型
// 好处：1. 零内存占用（空结构体大小为0） 2. 仅用于组织方法，不需要存储状态 3. 清晰表明这是无状态的工具类
type autoCodeMysql struct{}

// GetDB 获取数据库的所有数据库名
// Author [piexlmax](https://github.com/piexlmax)
// Author [SliverHorn](https://github.com/SliverHorn)
//
// 参数说明：
//   - businessDB: 业务数据库连接名，为空字符串时使用默认连接，否则从连接池中获取指定连接
//
// 设计说明：
//  1. 使用 INFORMATION_SCHEMA.SCHEMATA 标准系统表：这是MySQL的标准元数据表，兼容性好，不依赖具体数据库版本
//  2. 支持多数据库连接：通过 businessDB 参数实现多数据源支持，提高系统灵活性
//  3. 使用 Raw SQL + Scan：直接执行SQL并扫描到结构体，性能更好，避免了ORM的复杂转换
//
// 好处：
//   - 标准接口：使用 INFORMATION_SCHEMA 保证了跨版本兼容性
//   - 灵活配置：支持默认连接和自定义连接，适应不同业务场景
//   - 性能优化：直接SQL查询，减少ORM层开销
func (s *autoCodeMysql) GetDB(businessDB string) (data []response.Db, err error) {
	var entities []response.Db
	// 查询所有数据库名称，使用 INFORMATION_SCHEMA 标准表获取元数据
	// 使用反引号包裹 database 关键字，避免与MySQL保留字冲突
	sql := "SELECT SCHEMA_NAME AS `database` FROM INFORMATION_SCHEMA.SCHEMATA;"
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
//   - dbName: 目标数据库名称，用于过滤指定数据库下的表
//
// 设计说明：
//  1. 使用参数化查询（? 占位符）：防止SQL注入攻击，提高安全性
//  2. 查询 information_schema.tables：标准系统表，获取数据库表元数据
//  3. 使用 table_schema 过滤：精确查询指定数据库的表，避免返回其他数据库的表信息
//
// 好处：
//   - 安全性：参数化查询自动转义用户输入，防止SQL注入
//   - 准确性：只返回指定数据库的表，避免数据混淆
//   - 标准化：使用 INFORMATION_SCHEMA 标准接口，稳定可靠
func (s *autoCodeMysql) GetTables(businessDB string, dbName string) (data []response.Table, err error) {
	var entities []response.Table
	// 使用参数化查询，? 占位符会被 GORM 自动处理，防止SQL注入
	// table_schema 字段用于指定数据库名称，实现精确查询
	sql := `select table_name as table_name from information_schema.tables where table_schema = ?`
	if businessDB == "" {
		// 使用默认连接，dbName 作为参数传递给SQL查询
		err = global.GVA_DB.Raw(sql, dbName).Scan(&entities).Error
	} else {
		// 使用指定的业务数据库连接
		err = global.GVA_DBList[businessDB].Raw(sql, dbName).Scan(&entities).Error
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
//  1. 复杂的SQL查询设计：
//     - 使用 INFORMATION_SCHEMA.COLUMNS 获取列的基本信息（名称、类型、注释等）
//     - 使用 LEFT JOIN + KEY_COLUMN_USAGE 关联查询主键信息，LEFT JOIN 确保即使没有主键也能返回所有列
//     - 使用 CASE 语句根据不同的数据类型提取相应的精度/长度信息，统一数据格式
//
//  2. data_type_long 字段的处理逻辑：
//     - 字符串类型（longtext, varchar）：返回 CHARACTER_MAXIMUM_LENGTH（字符最大长度）
//     - 浮点类型（double, decimal）：返回精度和小数位数（格式：精度,小数位数）
//     - 整数类型（int, bigint）：返回 NUMERIC_PRECISION（数值精度）
//     - 其他类型：返回空字符串
//     这样设计的好处是前端可以根据类型和 data_type_long 完整还原数据库字段定义
//
//  3. 主键判断：
//     - 通过关联 KEY_COLUMN_USAGE 表，查询 CONSTRAINT_NAME = 'PRIMARY' 的记录
//     - 如果关联到记录则 primary_key = 1，否则为 0
//     - 使用 LEFT JOIN 而非 INNER JOIN，确保非主键字段也能正常返回
//
//  4. 排序：
//     - 使用 ORDINAL_POSITION 排序，保证返回的列顺序与表定义顺序一致
//     - 这对于代码生成很重要，可以保持字段的原始定义顺序
//
// 好处：
//   - 一次性获取完整信息：列名、类型、长度、注释、主键标识、顺序，减少多次查询
//   - 数据类型统一处理：CASE 语句将不同类型的数据统一格式，便于后续处理
//   - 主键识别准确：通过标准系统表关联，准确识别主键字段
//   - 顺序保持：按 ORDINAL_POSITION 排序，保持表结构的原始顺序
//   - 安全可靠：参数化查询防止SQL注入，使用标准系统表保证兼容性
func (s *autoCodeMysql) GetColumn(businessDB string, tableName string, dbName string) (data []response.Column, err error) {
	var entities []response.Column
	sql := `
	SELECT 
    c.COLUMN_NAME column_name,                              -- 列名
    c.DATA_TYPE data_type,                                  -- 数据类型（如：varchar, int, decimal）
    CASE c.DATA_TYPE
        WHEN 'longtext' THEN c.CHARACTER_MAXIMUM_LENGTH     -- 长文本类型：返回字符长度
        WHEN 'varchar' THEN c.CHARACTER_MAXIMUM_LENGTH      -- 变长字符串：返回字符长度
        WHEN 'double' THEN CONCAT_WS(',', c.NUMERIC_PRECISION, c.NUMERIC_SCALE)  -- 双精度浮点：返回"精度,小数位数"
        WHEN 'decimal' THEN CONCAT_WS(',', c.NUMERIC_PRECISION, c.NUMERIC_SCALE) -- 定点数：返回"精度,小数位数"
        WHEN 'int' THEN c.NUMERIC_PRECISION                 -- 整数：返回精度
        WHEN 'bigint' THEN c.NUMERIC_PRECISION              -- 大整数：返回精度
        ELSE ''                                             -- 其他类型：返回空字符串
    END AS data_type_long,                                  -- 类型详细说明（长度、精度等信息）
    c.COLUMN_COMMENT column_comment,                        -- 列注释
    CASE WHEN kcu.COLUMN_NAME IS NOT NULL THEN 1 ELSE 0 END AS primary_key,  -- 是否主键（1=是，0=否）
    c.ORDINAL_POSITION                                      -- 列在表中的位置序号
FROM 
    INFORMATION_SCHEMA.COLUMNS c                            -- 主表：列信息表
LEFT JOIN 
    INFORMATION_SCHEMA.KEY_COLUMN_USAGE kcu                 -- 左关联：键列使用表（用于查找主键）
ON 
    c.TABLE_SCHEMA = kcu.TABLE_SCHEMA                       -- 匹配数据库名
    AND c.TABLE_NAME = kcu.TABLE_NAME                       -- 匹配表名
    AND c.COLUMN_NAME = kcu.COLUMN_NAME                     -- 匹配列名
    AND kcu.CONSTRAINT_NAME = 'PRIMARY'                     -- 仅匹配主键约束
WHERE 
    c.TABLE_NAME = ?                                        -- 参数1：表名（防止SQL注入）
    AND c.TABLE_SCHEMA = ?                                  -- 参数2：数据库名（防止SQL注入）
ORDER BY 
    c.ORDINAL_POSITION;` // 按列的位置顺序排序，保证返回顺序与表定义一致
	if businessDB == "" {
		// 使用默认连接，tableName 和 dbName 作为参数传递给SQL查询
		err = global.GVA_DB.Raw(sql, tableName, dbName).Scan(&entities).Error
	} else {
		// 使用指定的业务数据库连接
		err = global.GVA_DBList[businessDB].Raw(sql, tableName, dbName).Scan(&entities).Error
	}

	return entities, err
}
