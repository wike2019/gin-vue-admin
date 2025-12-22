package initialize

import (
	"fmt"

	"github.com/flipped-aurora/gin-vue-admin/server/task"

	"github.com/robfig/cron/v3"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// Timer 初始化定时任务管理器
// 该函数负责在应用启动时注册所有需要定时执行的任务
//
// 设计说明：
// 1. 使用 goroutine 异步执行的原因：
//   - 定时任务的注册过程不应该阻塞主程序的启动流程
//   - 即使定时任务注册失败，也不影响应用其他模块的正常初始化
//   - 符合 Go 语言"快速失败，并发执行"的设计理念
//
// 2. 使用 cron.WithSeconds() 选项的好处：
//   - 默认的 cron 表达式只支持到分钟级别（格式：分 时 日 月 周）
//   - 添加 WithSeconds() 后支持秒级精度（格式：秒 分 时 日 月 周）
//   - 提供更灵活的定时任务调度能力，可以精确到秒级执行
//   - 例如：可以设置 "0/30 * * * * *" 表示每30秒执行一次
//
// 3. 使用全局 GVA_Timer 的优势：
//   - 统一管理所有定时任务，便于后续的查询、停止、重启等操作
//   - 支持多个 cron 实例分组管理（通过 cronName 参数区分）
//   - 提供任务生命周期管理能力，可以在运行时动态添加/删除任务
//
// 4. 错误处理使用 fmt.Println 的原因：
//   - 定时任务初始化阶段，日志系统可能还未完全初始化
//   - 使用标准输出确保错误信息能够被捕获，便于调试
//   - 生产环境建议改为使用全局日志系统（global.GVA_LOG）
func Timer() {
	// 使用 goroutine 异步执行，避免阻塞主程序启动
	// 好处：即使定时任务注册失败，也不影响应用其他功能的正常启动
	go func() {
		// 定义 cron 选项配置
		// 使用可变参数设计，方便后续扩展其他选项（如时区、日志等）
		var option []cron.Option

		// 添加秒级精度支持选项
		// 好处：支持更精确的定时任务调度，例如可以设置每30秒执行一次的任务
		// 注意：添加此选项后，cron 表达式格式变为：秒 分 时 日 月 周
		option = append(option, cron.WithSeconds())

		// 注册清理数据库的定时任务
		// 参数说明：
		//   - "ClearDB": cron 实例名称，用于分组管理定时任务
		//   - "@daily": cron 表达式，表示每天执行一次（等同于 "0 0 0 * * *"）
		//   - func(): 定时执行的具体业务逻辑
		//   - "定时清理数据库【日志，黑名单】内容": 任务描述，用于日志和调试
		//   - option...: cron 配置选项，这里传入秒级精度支持
		//
		// 设计优势：
		//   1. 业务逻辑封装在 task 包中，保持代码结构清晰
		//   2. 使用匿名函数包装，可以在执行前后添加日志、监控等横切关注点
		//   3. 错误处理在任务内部，避免单个任务失败影响其他任务
		_, err := global.GVA_Timer.AddTaskByFunc("ClearDB", "@daily", func() {
			// 执行数据库清理任务
			// 清理内容：操作日志（90天前）、JWT黑名单（7天前）
			// 好处：自动清理过期数据，防止数据库无限增长，提升查询性能
			err := task.ClearTable(global.GVA_DB) // 定时任务方法定在task文件包中
			if err != nil {
				// 任务执行失败时的错误处理
				// 注意：这里使用 fmt.Println，因为定时任务可能在日志系统初始化前执行
				// 生产环境建议改为：global.GVA_LOG.Error("定时清理数据库失败", zap.Error(err))
				fmt.Println("timer error:", err)
			}
		}, "定时清理数据库【日志，黑名单】内容", option...)

		// 检查任务注册是否成功
		// 如果注册失败，记录错误但不中断程序运行
		if err != nil {
			fmt.Println("add timer error:", err)
		}

		// ========== 其他定时任务注册示例 ==========
		// 其他定时任务定在这里 参考上方使用方法
		//
		// 添加新定时任务的步骤：
		// 1. 在 task 包中实现具体的业务逻辑函数
		// 2. 使用 AddTaskByFunc 或 AddTaskByJob 注册任务
		// 3. 选择合适的 cron 表达式（支持标准格式和预定义格式如 @daily, @hourly）
		// 4. 添加适当的错误处理和日志记录
		//
		// 示例代码（已注释）：
		//_, err := global.GVA_Timer.AddTaskByFunc("定时任务标识", "corn表达式", func() {
		//	具体执行内容...
		//  ......
		//}, "任务描述", option...)
		//if err != nil {
		//	fmt.Println("add timer error:", err)
		//}
	}()
}
