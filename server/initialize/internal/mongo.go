// Package internal 提供 MongoDB 客户端内部配置，主要用于设置命令监控和日志记录
package internal

import (
	"context"
	"fmt"

	"github.com/qiniu/qmgo/options"
	"go.mongodb.org/mongo-driver/event"
	opt "go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// Mongo 是 mongo 结构体的全局单例实例
// 使用单例模式的好处：
// 1. 确保整个应用只有一个 MongoDB 配置实例，避免重复创建和资源浪费
// 2. 统一管理 MongoDB 客户端配置，便于维护和修改
// 3. 线程安全，可以在多个 goroutine 中安全使用
var Mongo = new(mongo)

// mongo 结构体封装了 MongoDB 客户端的配置逻辑
// 采用空结构体的原因：
// 1. 不占用内存空间（零字节），只作为方法接收器使用
// 2. 符合 Go 语言的最佳实践，当只需要方法集合而不需要状态时使用空结构体
type mongo struct{}

// GetClientOptions 返回 MongoDB 客户端的配置选项
// 返回值：[]options.ClientOptions - qmgo 库所需的客户端选项切片
//
// 设计目的：
// 1. 为 MongoDB 客户端添加命令监控功能，实现全链路追踪
// 2. 统一记录所有 MongoDB 操作的日志，便于问题排查和性能分析
// 3. 通过 RequestID 关联请求的完整生命周期（开始、成功、失败）
//
// 好处：
// 1. 可观测性：能够追踪每个数据库操作的完整执行过程
// 2. 性能监控：记录操作耗时，便于识别慢查询
// 3. 问题诊断：详细的日志信息帮助快速定位数据库相关问题
// 4. 审计追踪：记录所有数据库操作，满足审计和安全要求
func (m *mongo) GetClientOptions() []options.ClientOptions {
	// CommandMonitor 是 MongoDB 驱动提供的命令监控器
	// 它可以在命令执行的三个关键阶段进行拦截和记录：
	// - Started: 命令开始执行时触发
	// - Succeeded: 命令成功完成时触发
	// - Failed: 命令执行失败时触发
	//
	// 使用监控器的好处：
	// 1. 无需修改业务代码，自动记录所有数据库操作
	// 2. 在驱动层面拦截，确保不会遗漏任何操作
	// 3. 提供统一的日志格式，便于日志分析和监控系统集成
	cmdMonitor := &event.CommandMonitor{
		// Started 回调：在 MongoDB 命令开始执行时触发
		// 记录信息包括：
		// - RequestID: 请求唯一标识，用于关联同一次操作的开始、成功/失败日志
		// - DatabaseName: 目标数据库名称，便于区分不同数据库的操作
		// - Command: 执行的命令内容（如查询、插入、更新等），便于了解具体操作
		// - business: "mongo" 业务标识，便于在日志系统中过滤和分类 MongoDB 相关日志
		Started: func(ctx context.Context, event *event.CommandStartedEvent) {
			zap.L().Info(fmt.Sprintf("[MongoDB][RequestID:%d][database:%s] %s\n", event.RequestID, event.DatabaseName, event.Command), zap.String("business", "mongo"))
		},
		// Succeeded 回调：在 MongoDB 命令成功完成时触发
		// 记录信息包括：
		// - RequestID: 与 Started 事件中的 RequestID 相同，用于关联
		// - Duration: 命令执行耗时，用于性能分析和慢查询识别
		// - Reply: 命令返回的结果摘要，便于了解操作结果
		// 通过对比 Started 和 Succeeded 的时间戳，可以计算精确的执行时间
		Succeeded: func(ctx context.Context, event *event.CommandSucceededEvent) {
			zap.L().Info(fmt.Sprintf("[MongoDB][RequestID:%d] [%s] %s\n", event.RequestID, event.Duration.String(), event.Reply), zap.String("business", "mongo"))
		},
		// Failed 回调：在 MongoDB 命令执行失败时触发
		// 记录信息包括：
		// - RequestID: 与 Started 事件中的 RequestID 相同，用于关联失败的请求
		// - Duration: 失败前的执行时间，可能有助于分析超时问题
		// - Failure: 错误信息，包含详细的失败原因，便于快速定位问题
		// 使用 Error 级别记录，确保错误日志能够被监控系统及时捕获和告警
		Failed: func(ctx context.Context, event *event.CommandFailedEvent) {
			zap.L().Error(fmt.Sprintf("[MongoDB][RequestID:%d] [%s] %s\n", event.RequestID, event.Duration.String(), event.Failure), zap.String("business", "mongo"))
		},
	}
	// 返回客户端选项，将命令监控器注入到 MongoDB 客户端配置中
	// 这里使用了 qmgo 和 mongo-driver 的适配层：
	// - qmgo 是七牛云提供的 MongoDB Go 驱动封装，提供了更简洁的 API
	// - 但底层仍然使用官方的 mongo-driver，所以需要通过 ClientOptions 传递原生驱动选项
	// 这种设计的好处是既享受了 qmgo 的便利性，又能使用官方驱动的所有功能
	return []options.ClientOptions{{ClientOptions: &opt.ClientOptions{Monitor: cmdMonitor}}}
}
