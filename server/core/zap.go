package core

import (
	"fmt"
	"os"

	"github.com/flipped-aurora/gin-vue-admin/server/core/internal"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Zap 初始化并返回 zap.Logger 实例
//
// 设计思路：
// 1. 确保日志目录存在，不存在则创建
// 2. 根据配置创建多个日志级别核心（Core）
// 3. 使用 Tee 将多个核心组合，实现多级别日志输出
// 4. 配置堆栈跟踪和调用者信息
//
// 为什么使用 zap 而不是标准库 log？
// - 性能：zap 是高性能日志库，比标准库快 10-100 倍
// - 结构化日志：支持结构化字段，便于日志分析和查询
// - 日志级别：支持 Debug、Info、Warn、Error 等多个级别
// - 灵活输出：可以同时输出到文件、控制台、数据库等
//
// Author [SliverHorn](https://github.com/SliverHorn)
func Zap() (logger *zap.Logger) {
	// 第一步：检查并创建日志目录
	// 为什么需要检查目录？
	// - 日志文件需要写入到指定目录，目录不存在会导致日志写入失败
	// - 自动创建目录提高用户体验，无需手动创建
	// - 使用 os.ModePerm (0777) 确保目录有足够的权限
	if ok, _ := utils.PathExists(global.GVA_CONFIG.Zap.Director); !ok { // 判断是否有Director文件夹
		fmt.Printf("create %v directory\n", global.GVA_CONFIG.Zap.Director)
		/** 这里是不是应该使用 _ = os.MkdirAll(global.GVA_CONFIG.Zap.Director, os.ModePerm) 因为可能存在多级目录 */
		_ = os.MkdirAll(global.GVA_CONFIG.Zap.Director, os.ModePerm)
	}

	// 第二步：获取配置的日志级别列表
	// 为什么支持多个日志级别？
	// - 不同级别可以输出到不同文件（如 error.log、info.log）
	// - 可以分别控制不同级别的日志格式和输出位置
	// - 便于日志管理和分析
	levels := global.GVA_CONFIG.Zap.Levels()
	length := len(levels)

	// 第三步：为每个日志级别创建核心（Core）
	// 为什么使用 Core？
	// - Core 是 zap 的核心抽象，定义了日志的编码、输出位置等
	// - 每个 Core 可以有不同的配置（文件、格式、级别等）
	// - 通过组合多个 Core 实现复杂的日志输出策略
	cores := make([]zapcore.Core, 0, length) // 预分配容量，提高性能
	for i := 0; i < length; i++ {
		// 为每个级别创建对应的 Core
		// internal.NewZapCore 会创建文件输出、格式化等配置
		core := internal.NewZapCore(levels[i])
		cores = append(cores, core)
	}

	// 第四步：使用 Tee 组合多个 Core
	// 为什么使用 Tee？
	// - Tee 可以将日志同时输出到多个目标（文件、控制台等）
	// - 不同级别的日志可以输出到不同的文件
	// - 例如：Error 级别输出到 error.log，Info 级别输出到 info.log
	// 注意：错误级别的入库逻辑已在自定义 ZapCore 中处理
	logger = zap.New(zapcore.NewTee(cores...))

	// 第五步：配置日志选项
	opts := []zap.Option{}

	// 启用 Error 及以上级别的堆栈跟踪
	// 为什么只对 Error 级别启用堆栈跟踪？
	// - 错误日志需要堆栈信息来定位问题
	// - 堆栈跟踪有性能开销，只在必要时启用
	// - Info 和 Debug 级别通常不需要堆栈信息
	opts = append(opts, zap.AddStacktrace(zapcore.ErrorLevel))

	// 如果配置显示调用者信息，则添加调用者选项
	// 为什么可选显示调用者？
	// - 调用者信息有助于定位日志来源
	// - 但有性能开销，生产环境可能关闭以提高性能
	// - 开发环境通常启用，生产环境根据需求决定
	if global.GVA_CONFIG.Zap.ShowLine {
		opts = append(opts, zap.AddCaller())
	}

	// 应用所有选项
	logger = logger.WithOptions(opts...)
	return logger
}
