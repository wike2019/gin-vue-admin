package config

import (
	"go.uber.org/zap/zapcore"
	"time"
)

// Zap 日志配置结构体，基于Uber的Zap高性能日志库
// 设计优势：
// 1. 高性能：Zap是Go生态中性能最好的日志库之一，零分配设计，适合高并发场景
// 2. 结构化日志：支持JSON和Console两种格式，便于日志分析和处理
// 3. 灵活配置：支持多种日志级别、编码格式，可根据环境灵活调整
// 4. 日志轮转：通过RetentionDay实现日志自动清理，避免磁盘空间耗尽
type Zap struct {
	// Level 日志级别：控制输出哪些级别的日志
	// 可选值：debug/info/warn/error/dpanic/panic/fatal
	// 设计好处：生产环境可以设置为warn或error，减少日志量，提高性能
	Level string `mapstructure:"level" json:"level" yaml:"level"`
	// Prefix 日志前缀：每条日志前添加的固定字符串
	// 例如："[GVA]" 用于标识日志来源，便于日志过滤和查找
	Prefix string `mapstructure:"prefix" json:"prefix" yaml:"prefix"`
	// Format 日志格式：控制日志输出格式
	// "json"：结构化JSON格式，便于日志收集系统（如ELK）解析
	// "console"：人类可读的文本格式，便于开发调试
	Format string `mapstructure:"format" json:"format" yaml:"format"`
	// Director 日志文件夹：日志文件的存储目录
	// 设计好处：集中管理日志文件，便于日志归档和清理
	Director string `mapstructure:"director" json:"director"  yaml:"director"`
	// EncodeLevel 日志级别编码格式：控制日志级别在输出中的显示方式
	// 支持：LowercaseLevelEncoder/LowercaseColorLevelEncoder/CapitalLevelEncoder/CapitalColorLevelEncoder
	// 设计好处：可以根据输出环境（终端/文件）选择合适的编码格式
	EncodeLevel string `mapstructure:"encode-level" json:"encode-level" yaml:"encode-level"`
	// StacktraceKey 堆栈跟踪键名：在JSON格式中，堆栈信息的字段名
	// 设计好处：统一堆栈信息的字段名，便于日志分析工具识别
	StacktraceKey string `mapstructure:"stacktrace-key" json:"stacktrace-key" yaml:"stacktrace-key"`
	// ShowLine 显示代码位置：是否在日志中显示文件名和行号
	// 设计好处：快速定位日志产生的代码位置，提高调试效率
	ShowLine bool `mapstructure:"show-line" json:"show-line" yaml:"show-line"`
	// LogInConsole 控制台输出：是否同时输出到控制台
	// 设计好处：开发环境可以同时输出到文件和控制台，便于实时查看日志
	LogInConsole bool `mapstructure:"log-in-console" json:"log-in-console" yaml:"log-in-console"`
	// RetentionDay 日志保留天数：日志文件保留的天数，超过此天数的日志会被自动删除
	// 设计意义：
	// 1. 磁盘管理：自动清理旧日志，避免磁盘空间耗尽
	// 2. 合规要求：某些场景需要保留特定天数的日志用于审计
	RetentionDay int `mapstructure:"retention-day" json:"retention-day" yaml:"retention-day"`
}

// Levels 根据配置的日志级别字符串，返回从该级别到Fatal的所有级别数组
// 设计目的：
// 1. 级别过滤：Zap需要知道哪些级别的日志需要输出，此方法生成级别列表
// 2. 动态配置：根据配置文件动态确定日志级别，无需重新编译
// 3. 容错处理：配置无效时默认使用DebugLevel，确保系统正常运行
// 返回值：包含从配置级别到Fatal的所有级别的切片，用于Zap的日志过滤
func (c *Zap) Levels() []zapcore.Level {
	levels := make([]zapcore.Level, 0, 7) // 预分配容量7，避免多次扩容
	level, err := zapcore.ParseLevel(c.Level)
	if err != nil {
		// 配置无效时使用DebugLevel，确保所有日志都能输出，便于排查问题
		level = zapcore.DebugLevel
	}
	// 从配置的级别开始，到Fatal级别结束，生成级别数组
	// 例如：如果配置为warn，则返回[warn, error, dpanic, panic, fatal]
	for ; level <= zapcore.FatalLevel; level++ {
		levels = append(levels, level)
	}
	return levels
}

// Encoder 创建日志编码器，根据配置返回JSON或Console编码器
// 设计好处：
// 1. 格式切换：通过配置切换日志格式，无需修改代码
// 2. 统一配置：所有编码器配置集中管理，包括时间格式、调用者信息等
// 3. 自定义时间格式：通过EncodeTime自定义时间显示格式，添加前缀便于识别
func (c *Zap) Encoder() zapcore.Encoder {
	config := zapcore.EncoderConfig{
		TimeKey:       "time",    // JSON格式中时间字段的键名
		NameKey:       "name",    // 日志记录器名称的键名
		LevelKey:      "level",   // 日志级别的键名
		CallerKey:     "caller",  // 调用者信息的键名（文件名和行号）
		MessageKey:    "message", // 日志消息的键名
		StacktraceKey: c.StacktraceKey, // 堆栈跟踪的键名（可配置）
		LineEnding:    zapcore.DefaultLineEnding, // 行结束符
		// 自定义时间编码：添加前缀并格式化时间
		// 使用Go的固定时间格式：2006-01-02 15:04:05.000
		EncodeTime: func(t time.Time, encoder zapcore.PrimitiveArrayEncoder) {
			encoder.AppendString(c.Prefix + t.Format("2006-01-02 15:04:05.000"))
		},
		EncodeLevel:    c.LevelEncoder(),              // 使用配置的级别编码器
		EncodeCaller:   zapcore.FullCallerEncoder,    // 完整调用者信息（包名+文件名+行号）
		EncodeDuration: zapcore.SecondsDurationEncoder, // 持续时间以秒为单位
	}
	// 根据配置选择编码器类型
	if c.Format == "json" {
		// JSON编码器：适合日志收集和分析系统
		return zapcore.NewJSONEncoder(config)
	}
	// Console编码器：人类可读格式，适合开发调试
	return zapcore.NewConsoleEncoder(config)
}

// LevelEncoder 根据配置的EncodeLevel字符串，返回对应的zapcore.LevelEncoder
// 设计目的：
// 1. 格式统一：将字符串配置转换为Zap的类型，提供类型安全
// 2. 多种选择：支持4种编码格式，适应不同场景
//    - LowercaseLevelEncoder: 小写（info/warn/error），适合文件输出
//    - LowercaseColorLevelEncoder: 小写+颜色，适合终端输出，便于快速识别
//    - CapitalLevelEncoder: 大写（INFO/WARN/ERROR），更醒目
//    - CapitalColorLevelEncoder: 大写+颜色，终端输出最醒目
// 3. 默认值：配置无效时使用小写格式，保证系统正常运行
// Author [SliverHorn](https://github.com/SliverHorn)
func (c *Zap) LevelEncoder() zapcore.LevelEncoder {
	switch {
	case c.EncodeLevel == "LowercaseLevelEncoder": // 小写编码器(默认)
		return zapcore.LowercaseLevelEncoder
	case c.EncodeLevel == "LowercaseColorLevelEncoder": // 小写编码器带颜色
		return zapcore.LowercaseColorLevelEncoder
	case c.EncodeLevel == "CapitalLevelEncoder": // 大写编码器
		return zapcore.CapitalLevelEncoder
	case c.EncodeLevel == "CapitalColorLevelEncoder": // 大写编码器带颜色
		return zapcore.CapitalColorLevelEncoder
	default:
		return zapcore.LowercaseLevelEncoder // 默认使用小写格式
	}
}
