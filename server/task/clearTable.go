package task

import (
	"errors"
	"fmt"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common"

	"gorm.io/gorm"
)

//@author: [songzhibin97](https://github.com/songzhibin97)
//@function: ClearTable
//@description: 清理数据库表数据
//@param: db(数据库对象) *gorm.DB, tableName(表名) string, compareField(比较字段) string, interval(间隔) string
//@return: error

// ClearTable 定期清理数据库表中的历史数据，防止数据无限增长导致数据库性能下降
// 为什么需要这个功能：
// 1. 系统运行日志表（sys_operation_records）会持续记录用户操作，长期积累会占用大量存储空间
// 2. JWT黑名单表（jwt_blacklists）存储已失效的token，过期后无保留价值
// 3. 定期清理可以保持数据库性能，减少存储成本，提高查询效率
func ClearTable(db *gorm.DB) error {
	// 使用切片存储多个表的清理配置，好处：
	// 1. 集中管理所有需要清理的表配置，易于维护和扩展
	// 2. 使用统一的清理逻辑，避免代码重复（DRY原则）
	// 3. 后续可以通过循环统一处理，减少代码量
	var ClearTableDetail []common.ClearDB

	// 配置系统操作记录表的清理规则
	// 2160h = 90天（3个月），保留90天内的操作记录足够用于审计和问题排查
	// 为什么使用字符串格式（"2160h"）：
	// 1. 可读性强，直观表达时间间隔（2160小时）
	// 2. time.ParseDuration 支持标准格式（如 "168h", "7d", "2160h"），灵活易用
	// 3. 配置和代码分离，后续可以通过配置文件或数据库配置，无需修改代码
	ClearTableDetail = append(ClearTableDetail, common.ClearDB{
		TableName:    "sys_operation_records", // 系统操作记录表，记录用户的操作日志
		CompareField: "created_at",            // 使用创建时间字段进行比较，符合业务逻辑
		Interval:     "2160h",                 // 清理90天前的数据（2160小时 = 90天）
	})

	// 配置JWT黑名单表的清理规则
	// 168h = 7天，JWT token通常有效期较短，黑名单保留7天足够覆盖所有可能出现的过期token
	// 为什么JWT黑名单清理间隔更短：
	// 1. token有效期通常较短（几小时到几天），过期后立即失效
	// 2. 黑名单主要用于防止token在有效期内被重复使用，过期后无保留意义
	// 3. 更短的清理周期可以更快释放存储空间
	ClearTableDetail = append(ClearTableDetail, common.ClearDB{
		TableName:    "jwt_blacklists", // JWT黑名单表，存储已失效或被拉黑的token
		CompareField: "created_at",     // 使用创建时间字段进行比较
		Interval:     "168h",           // 清理7天前的数据（168小时 = 7天）
	})

	// 防御性编程：检查数据库连接对象是否为空
	// 为什么需要这个检查：
	// 1. 防止空指针异常（panic），如果db为nil，后续操作会导致程序崩溃
	// 2. 提前发现问题，返回明确的错误信息，便于调试
	// 3. 提高代码的健壮性和可维护性
	if db == nil {
		return errors.New("db Cannot be empty")
	}

	// 遍历所有需要清理的表配置，统一处理
	// 好处：使用循环统一处理，避免为每个表重复编写清理代码
	for _, detail := range ClearTableDetail {
		// 将字符串格式的时间间隔解析为time.Duration类型
		// 为什么需要解析：
		// 1. 字符串配置需要转换为程序可用的时间类型才能进行计算
		// 2. time.ParseDuration 会验证格式是否正确，起到配置校验的作用
		// 3. 如果格式错误（如"abc"），会立即返回错误，避免执行错误的清理操作
		duration, err := time.ParseDuration(detail.Interval)
		if err != nil {
			return err
		}

		// 检查解析后的时间间隔是否为负数
		// 为什么需要这个检查：
		// 1. 负数时间间隔会导致清理未来时间的数据，这是不符合逻辑的
		// 2. 防止配置错误导致意外删除新数据
		// 3. 虽然time.ParseDuration不会返回负数，但显式检查可以提高代码可读性和安全性
		if duration < 0 {
			return errors.New("parse duration < 0")
		}

		// 执行SQL删除操作
		// 使用GORM的Exec方法执行原生SQL的好处：
		// 1. 灵活性高，可以精确控制SQL语句，适合批量删除操作
		// 2. 使用参数化查询（?占位符），防止SQL注入攻击
		// 3. Debug()方法会打印实际执行的SQL语句，便于开发和调试
		//
		// SQL逻辑说明：
		// DELETE FROM 表名 WHERE 时间字段 < (当前时间 - 时间间隔)
		// 例如：DELETE FROM sys_operation_records WHERE created_at < (当前时间 - 90天)
		// 即删除所有90天前创建的记录
		//
		// time.Now().Add(-duration) 计算清理时间点：
		// - 如果duration是2160h（90天），Add(-2160h)得到90天前的时间点
		// - 所有早于这个时间点的记录都会被删除
		err = db.Debug().Exec(fmt.Sprintf("DELETE FROM %s WHERE %s < ?", detail.TableName, detail.CompareField), time.Now().Add(-duration)).Error
		if err != nil {
			// 如果执行失败，立即返回错误
			// 为什么不在循环中继续执行：
			// 1. 清理操作是关键的数据库维护任务，失败需要及时处理
			// 2. 如果某个表清理失败，应该停止后续清理，防止部分清理导致数据不一致
			// 3. 返回错误让调用方知道清理任务未完全成功，可以记录日志或告警
			return err
		}
	}
	return nil
}
