package initialize

import (
	"context"
	"fmt"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/announcement/model"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// Gorm 初始化数据库表结构
// 设计模式：数据库迁移模式（Database Migration Pattern） + 自动迁移模式（Auto Migration Pattern）
// 好处：
// 1. 自动创建和更新数据库表结构，无需手动执行 SQL
// 2. 支持数据库版本管理，便于回滚和升级
// 3. 确保数据库结构与代码模型一致，避免运行时错误
// 4. 使用上下文传递，支持超时控制和请求取消
// @param ctx context.Context 上下文，用于传递请求上下文信息（如超时、取消等）
func Gorm(ctx context.Context) {
	// 使用 GORM 的 AutoMigrate 方法自动迁移数据库表结构
	// 设计模式：自动迁移模式（Auto Migration Pattern）
	// 好处：
	// 1. WithContext(ctx) 传递上下文，支持超时控制和请求取消
	// 2. AutoMigrate 会自动创建表（如果不存在）或更新表结构（如果字段有变化）
	// 3. 只更新新增的字段，不会删除已存在的字段，保证数据安全
	// 4. 支持多表迁移，可以一次性迁移多个模型
	err := global.GVA_DB.WithContext(ctx).AutoMigrate(
		// 注册公告表模型
		// 好处：GORM 会根据模型定义自动创建或更新表结构
		// 包括：表名、字段、索引、外键等
		new(model.Info),
		// 可以继续添加其他模型，如：
		// new(model.Category),
		// new(model.Comment),
	)
	
	// 错误处理和日志记录
	// 设计模式：错误包装模式（Error Wrapping Pattern） + 日志记录模式（Logging Pattern）
	// 好处：
	// 1. 使用 errors.Wrap 包装错误，保留原始错误信息和堆栈
	// 2. 使用 zap 记录错误日志，便于问题追踪和调试
	// 3. 使用 fmt.Sprintf("%+v", err) 输出完整的错误堆栈信息
	// 4. 即使迁移失败，也不会影响主应用启动（只记录日志）
	if err != nil {
		err = errors.Wrap(err, "注册表失败!")
		zap.L().Error(fmt.Sprintf("%+v", err))
	}
}
