package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/response"
)

// AutoCodePgsql PostgreSQL数据库自动代码生成服务的单例实例
// 使用单例模式的好处：
// 1. 避免重复创建对象，节省内存资源
// 2. 确保整个应用只有一个实例，保持行为一致性
// 3. 便于在接口中统一管理和调用（参考 sys_auto_code_interface.go）
var AutoCodePgsql = new(autoCodePgsql)

// autoCodePgsql PostgreSQL数据库元数据查询服务
// 该结构体实现了 Database 接口，专门用于 PostgreSQL 数据库的元数据查询
// 设计为私有类型，外部只能通过 AutoCodePgsql 单例访问，符合封装原则
type autoCodePgsql struct{}

// GetDB 获取PostgreSQL数据库服务器的所有数据库名
// 设计说明：
//  1. 使用 pg_database 系统视图：这是 PostgreSQL 官方提供的系统视图，专门用于查询数据库元信息
//     好处：标准、可靠，不受版本差异影响，性能稳定
//  2. WHERE datistemplate = false：过滤掉模板数据库（template0、template1等）
//     原因：模板数据库是 PostgreSQL 系统数据库，不应出现在业务数据库列表中
//     好处：返回的结果更干净，只包含用户实际使用的数据库，避免混淆
//  3. 使用 businessDB 参数支持多数据源：
//     - businessDB == ""：使用默认数据源 global.GVA_DB
//     - businessDB != ""：从 global.GVA_DBList 中选择指定的业务数据源
//     好处：支持多租户、多业务场景，可以同时管理多个数据库连接
//
// Author [piexlmax](https://github.com/piexlmax)
// Author [SliverHorn](https://github.com/SliverHorn)
func (a *autoCodePgsql) GetDB(businessDB string) (data []response.Db, err error) {
	var entities []response.Db
	// 使用标准系统视图查询，datistemplate = false 过滤系统模板数据库
	sql := `SELECT datname as database FROM pg_database WHERE datistemplate = false`

	// 根据 businessDB 参数选择数据源
	// 这种设计的好处：
	// 1. 代码简洁，避免重复的 if-else 逻辑
	// 2. 统一的错误处理方式
	// 3. 支持动态切换数据源，提高系统灵活性
	if businessDB == "" {
		err = global.GVA_DB.Raw(sql).Scan(&entities).Error
	} else {
		err = global.GVA_DBList[businessDB].Raw(sql).Scan(&entities).Error
	}

	return entities, err
}

// GetTables 获取指定数据库中的所有表名
// 设计说明：
// 1. 使用 information_schema.tables 标准视图：
//   - information_schema 是 SQL 标准定义的信息模式，所有符合 SQL 标准的数据库都支持
//   - 好处：跨数据库兼容性好，代码可维护性强，不依赖 PostgreSQL 特有的系统表
//
// 2. table_catalog = ?：指定数据库名（在 PostgreSQL 中，catalog 通常等于数据库名）
//   - 使用参数化查询（?占位符）而不是字符串拼接
//   - 好处：防止 SQL 注入攻击，提高安全性；GORM 会自动处理参数转义
//
// 3. table_schema = 'public'：只查询 public schema 中的表
//   - PostgreSQL 支持多 schema，public 是默认的 schema
//   - 原因：大多数业务表都在 public schema 中，过滤掉系统 schema（如 pg_catalog, information_schema）
//   - 好处：结果更聚焦业务表，避免返回大量系统表造成干扰
//
// 4. 先选择数据源再执行查询的设计：
//   - 将数据库连接对象赋值给局部变量 db，统一后续使用
//   - 好处：代码更清晰，避免重复的条件判断，便于后续扩展
//
// Author [piexlmax](https://github.com/piexlmax)
// Author [SliverHorn](https://github.com/SliverHorn)
func (a *autoCodePgsql) GetTables(businessDB string, dbName string) (data []response.Table, err error) {
	var entities []response.Table
	// 使用 SQL 标准的信息模式视图，table_catalog 对应数据库名，table_schema 对应 schema 名
	// 使用 ? 占位符进行参数化查询，防止 SQL 注入
	sql := `select table_name as table_name from information_schema.tables where table_catalog = ? and table_schema = ?`

	// 根据 businessDB 参数选择数据源，统一赋值给 db 变量
	// 这种写法的好处：代码简洁，避免在后续代码中重复判断 businessDB
	db := global.GVA_DB
	if businessDB != "" {
		db = global.GVA_DBList[businessDB]
	}

	// 执行查询，dbName 作为 table_catalog，'public' 作为 table_schema
	// 只查询 public schema 中的表，过滤系统表
	err = db.Raw(sql, dbName, "public").Scan(&entities).Error
	return entities, err
}

// GetColumn 获取指定数据库和指定数据表的所有字段的详细信息（字段名、类型、长度、注释、是否主键等）
// 设计说明：
// 1. 为什么使用复杂的 SQL 而不是简单的 information_schema.columns？
//   - information_schema.columns 只能提供基本信息（字段名、类型等）
//   - 字段注释（COMMENT）存储在 PostgreSQL 的系统表 pg_description 中，需要通过子查询关联
//   - 主键信息存储在 pg_constraint 中，需要查询约束表才能判断
//   - 因此需要关联多个系统表才能获取完整的字段元数据信息，用于代码自动生成
//
// 2. 为什么需要字段注释和主键信息？
//   - 字段注释：用于自动生成代码时的字段说明、API 文档、前端表单标签等
//   - 主键标识：用于自动生成代码时识别主键字段，决定是否自动填充、是否可编辑、是否必填等
//
// 3. CASE WHEN 处理不同数据类型的长度/精度（data_type_long 字段）：
//   - text/varchar：使用 CHARACTER_MAXIMUM_LENGTH（字符最大长度，如 varchar(255) 中的 255）
//   - numeric 类型（smallint/decimal等）：使用 NUMERIC_PRECISION 和 NUMERIC_SCALE（精度和小数位数，如 decimal(10,2)）
//   - integer 系列（integer/int4/int8/bigint）：使用 NUMERIC_PRECISION（显示精度信息）
//   - timestamp：使用 datetime_precision（时间精度，如秒、毫秒、微秒）
//   - 使用 concat_ws 拼接的好处：统一格式化为字符串，便于在代码生成时使用，避免类型转换问题
//
// 4. 子查询获取字段注释（column_comment）的设计：
//   - pg_description 存储对象描述，objoid 是对象 OID，objsubid 是子对象 ID（对于字段就是属性序号 attnum）
//   - 需要通过 pg_class 找到表的 OID（relname = table_name）
//   - 再通过 pg_attribute 找到字段的属性序号（attrelid = 表OID, attname = column_name）
//   - 这种关联查询的原因：PostgreSQL 的元数据系统是基于 OID（对象标识符）的，需要通过多表关联才能建立对应关系
//   - 好处：能够准确获取用户在创建表时添加的 COMMENT 注释
//
// 5. 子查询判断是否为主键（primary_key）的设计：
//   - pg_constraint 存储约束信息，contype = 'p' 表示主键约束（PRIMARY KEY）
//   - conrelid 是约束所属表的 OID，通过 pg_class 表的 relname 匹配找到
//   - conkey 是主键字段的属性序号数组（int[]类型），使用 PostgreSQL 的数组包含操作符 @> 检查当前字段是否在主键数组中
//   - COUNT(*) > 0 将结果转换为布尔值（在 PostgreSQL 中返回 1/0，表示 true/false）
//   - 好处：能够准确识别复合主键中的每个字段
//
// 6. ORDER BY ordinal_position：
//   - ordinal_position 是字段在表中的定义顺序位置（从 1 开始）
//   - 好处：返回的字段顺序与表结构定义顺序一致，便于生成代码时保持字段顺序，提高代码可读性
//
// 7. 为什么使用参数化查询（?占位符）而不是字符串拼接？
//   - tableName 和 dbName 来自用户输入，如果直接拼接存在 SQL 注入风险
//   - 使用 ? 占位符，GORM 会自动转义特殊字符，确保查询安全
//   - 代码中注释掉的字符串替换方式（ReplaceAll）是不安全的做法，已废弃，不应使用
//
// Author [piexlmax](https://github.com/piexlmax)
// Author [SliverHorn](https://github.com/SliverHorn)
func (a *autoCodePgsql) GetColumn(businessDB string, tableName string, dbName string) (data []response.Column, err error) {
	// todo 数据获取不全, 待完善sql
	// 注意：这个 SQL 查询比较复杂，但这是必要的，因为需要从多个系统表中关联获取完整信息
	// 包括：字段基本信息、数据类型长度/精度、字段注释、主键标识等
	sql := `
SELECT
    psc.COLUMN_NAME AS COLUMN_NAME,                    -- 字段名
    psc.udt_name AS data_type,                         -- 字段类型（PostgreSQL 的用户定义类型名，如 varchar, int4, timestamp 等）
    CASE
        psc.udt_name                                    -- 根据数据类型拼接长度/精度信息
        WHEN 'text' THEN
            concat_ws ( '', '', psc.CHARACTER_MAXIMUM_LENGTH )      -- 文本类型：获取字符长度
        WHEN 'varchar' THEN
            concat_ws ( '', '', psc.CHARACTER_MAXIMUM_LENGTH )      -- 可变字符串：获取字符长度
        WHEN 'smallint' THEN
            concat_ws ( ',', psc.NUMERIC_PRECISION, psc.NUMERIC_SCALE )  -- 小整数：精度,小数位
        WHEN 'decimal' THEN
            concat_ws ( ',', psc.NUMERIC_PRECISION, psc.NUMERIC_SCALE )  -- 小数：精度,小数位
        WHEN 'integer' THEN
            concat_ws ( '', '', psc.NUMERIC_PRECISION )             -- 整数：精度
        WHEN 'int4' THEN
            concat_ws ( '', '', psc.NUMERIC_PRECISION )             -- 4字节整数：精度
        WHEN 'int8' THEN
            concat_ws ( '', '', psc.NUMERIC_PRECISION )             -- 8字节整数：精度
        WHEN 'bigint' THEN
            concat_ws ( '', '', psc.NUMERIC_PRECISION )             -- 大整数：精度
        WHEN 'timestamp' THEN
            concat_ws ( '', '', psc.datetime_precision )            -- 时间戳：时间精度
        ELSE ''
        END AS data_type_long,                          -- 数据类型详细信息（长度/精度等）
    (
        -- 子查询：从 pg_description 系统表获取字段注释
        -- PostgreSQL 的字段注释存储在 pg_description 中，需要通过 OID 关联查询
        SELECT
            pd.description
        FROM
            pg_description pd
        WHERE
            (pd.objoid,pd.objsubid) in (
                -- 通过 pg_attribute 找到字段的属性序号（attnum）
                SELECT pa.attrelid,pa.attnum
                FROM
                    pg_attribute pa
                WHERE pa.attrelid = ( 
                    -- 通过 pg_class 找到表的 OID（对象标识符）
                    SELECT oid FROM pg_class pc WHERE
                    pc.relname = psc.table_name
                )
                  and attname = psc.column_name
            )
    ) AS column_comment,                                -- 字段注释
    (
        -- 子查询：判断当前字段是否为主键
        SELECT
            COUNT(*)
        FROM
            pg_constraint
        WHERE
            contype = 'p'                                -- 'p' 表示主键约束（PRIMARY KEY）
          AND conrelid = (
            -- 找到表的 OID
            SELECT
                oid
            FROM
                pg_class
            WHERE
                relname = psc.table_name
        )
          -- conkey 是主键字段的属性序号数组，使用 @> 操作符检查当前字段是否在数组中
          -- 这种方式可以处理复合主键的情况
          AND conkey::int[] @> ARRAY[(
            -- 获取当前字段的属性序号
            SELECT
                attnum::integer
            FROM
                pg_attribute
            WHERE
                attrelid = conrelid
              AND attname = psc.column_name
        )]
    ) > 0 AS primary_key,                               -- 是否为主键（1=是，0=否）
    psc.ordinal_position                                -- 字段在表中的顺序位置（用于保持字段顺序）
FROM
    INFORMATION_SCHEMA.COLUMNS psc                      -- SQL 标准的列信息视图
WHERE
  table_catalog = ?                                     -- 数据库名（参数化查询，防止 SQL 注入）
  AND table_schema = 'public'                           -- 只查询 public schema 中的字段
  AND TABLE_NAME = ?                                    -- 表名（参数化查询，防止 SQL 注入）
ORDER BY
    psc.ordinal_position;                               -- 按字段定义顺序排序
`
	var entities []response.Column
	// 废弃的字符串拼接方式（不安全，已注释）：
	//sql = strings.ReplaceAll(sql, "@table_catalog", dbName)
	//sql = strings.ReplaceAll(sql, "@table_name", tableName)
	// 应该使用参数化查询（下面的 db.Raw 方式），让 GORM 自动处理参数转义

	// 根据 businessDB 参数选择数据源，统一赋值给 db 变量
	// 这种写法的好处：代码简洁，避免在后续代码中重复判断 businessDB
	db := global.GVA_DB
	if businessDB != "" {
		db = global.GVA_DBList[businessDB]
	}

	// 执行参数化查询，dbName 和 tableName 作为参数传入，GORM 会自动转义特殊字符
	// 好处：防止 SQL 注入攻击，提高安全性
	err = db.Raw(sql, dbName, tableName).Scan(&entities).Error
	return entities, err
}
