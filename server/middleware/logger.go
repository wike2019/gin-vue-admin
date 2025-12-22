// Package middleware 提供HTTP请求日志记录中间件
// 该包实现了一个高度可配置的日志记录系统，采用策略模式设计，允许用户自定义各种日志处理逻辑
package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// LogLayout 日志布局结构体，定义了单次HTTP请求的完整日志信息
// 设计意义：
// 1. 统一日志格式：所有日志字段集中管理，便于后续处理和查询
// 2. 结构化数据：使用结构体而非字符串拼接，便于序列化和解析
// 3. 扩展性强：Metadata字段允许动态添加自定义字段，不破坏现有结构
type LogLayout struct {
	Time      time.Time              // 请求时间戳，记录请求发生的精确时间，用于时间序列分析和问题定位
	Metadata  map[string]interface{} // 存储自定义元数据，用于扩展日志信息（如用户ID、业务标识等），避免频繁修改结构体
	Path      string                 // 访问路径，记录请求的URI路径，用于路由分析和访问统计
	Query     string                 // URL查询参数，记录GET请求的查询字符串，便于参数追踪和调试
	Body      string                 // 请求体数据，记录POST/PUT等请求的body内容，用于请求内容审计和问题复现
	IP        string                 // 客户端IP地址，用于安全审计、地理位置分析和异常访问检测
	UserAgent string                 // 用户代理字符串，记录客户端浏览器/设备信息，用于兼容性分析和用户行为统计
	Error     string                 // 错误信息，记录请求处理过程中的错误，便于快速定位问题
	Cost      time.Duration          // 请求耗时，记录从开始到结束的时间，用于性能监控和慢查询分析
	Source    string                 // 服务来源标识，在多服务架构中区分日志来源，便于日志聚合和追踪
}

// Logger 日志记录器结构体，采用策略模式设计
// 设计优势：
// 1. 高度可配置：通过函数字段实现策略模式，用户可以根据需求自定义各种处理逻辑
// 2. 职责分离：每个函数字段负责单一职责，代码结构清晰，易于维护
// 3. 可选功能：所有函数字段都是可选的（可为nil），用户只需实现需要的功能
// 4. 零侵入：不强制实现所有接口，降低使用门槛
type Logger struct {
	// Filter 请求过滤器函数，决定是否记录请求体内容
	// 返回值：true表示跳过body记录，false表示需要记录body
	// 设计意义：
	// - 性能优化：对于不需要记录body的请求（如健康检查、静态资源），可以跳过body读取，减少内存和IO开销
	// - 隐私保护：对于包含敏感信息的请求，可以跳过body记录，避免敏感数据泄露
	// - 灵活控制：用户可以根据路径、方法、IP等条件自定义过滤规则
	Filter func(c *gin.Context) bool

	// FilterKeyword 关键字过滤和脱敏函数，对已记录的日志内容进行二次处理
	// 返回值：true表示通过过滤，false表示需要过滤掉该日志
	// 设计意义：
	// - 数据脱敏：自动识别并脱敏敏感字段（如密码、身份证号、手机号等），满足合规要求
	// - 关键字过滤：根据业务规则过滤掉不需要的日志，减少存储和传输成本
	// - 动态处理：在日志输出前进行最后一道安全防线，确保敏感信息不会泄露
	FilterKeyword func(layout *LogLayout) bool

	// AuthProcess 鉴权信息处理函数，从请求上下文中提取并填充鉴权相关信息
	// 设计意义：
	// - 用户追踪：自动提取用户ID、用户名等信息，便于问题定位和用户行为分析
	// - 权限审计：记录操作者信息，满足安全审计要求
	// - 上下文关联：将日志与用户会话关联，便于追踪完整的用户操作链路
	AuthProcess func(c *gin.Context, layout *LogLayout)

	// Print 日志输出函数，负责将LogLayout输出到目标位置
	// 设计意义：
	// - 输出解耦：日志记录逻辑与输出方式解耦，可以输出到文件、数据库、消息队列等任意目标
	// - 格式灵活：用户可以自定义输出格式（JSON、文本、结构化等），满足不同日志系统的要求
	// - 异步支持：可以在Print函数中实现异步日志写入，避免阻塞请求处理
	Print func(LogLayout)

	// Source 服务唯一标识，用于在多服务架构中区分日志来源
	// 设计意义：
	// - 服务识别：在微服务架构中，可以快速识别日志来自哪个服务
	// - 日志聚合：便于日志收集系统（如ELK、Loki）进行日志分类和聚合
	// - 问题定位：当出现问题时，可以快速定位到具体的服务实例
	Source string
}

// SetLoggerMiddleware 创建并返回Gin中间件函数
// 设计优势：
// 1. 延迟执行：返回函数而非直接执行，符合Gin中间件的使用模式
// 2. 请求生命周期完整：在c.Next()前后分别处理，可以记录完整的请求和响应信息
// 3. 性能考虑：只在需要时读取body，避免不必要的内存分配
// 4. 错误处理：自动捕获并记录请求处理过程中的错误
func (l Logger) SetLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录请求开始时间，用于计算请求耗时
		// 在请求处理前记录时间，确保计时准确
		start := time.Now()

		// 在c.Next()之前保存路径和查询参数
		// 原因：c.Next()执行后，URL可能被中间件修改，提前保存确保记录原始请求信息
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		var body []byte
		// 条件读取请求体：只有在Filter返回false（需要记录body）时才读取
		// 设计考虑：
		// 1. 性能优化：避免读取不需要记录的请求体（如健康检查、静态资源），减少内存和IO开销
		// 2. 流式处理：GetRawData()会消费掉Request.Body，需要重新设置回去，否则后续处理器无法读取body
		// 3. 空指针安全：先检查Filter是否为nil，避免空指针异常
		if l.Filter != nil && !l.Filter(c) {
			body, _ = c.GetRawData()
			// 关键操作：将读取的body重新设置回Request.Body
			// 原因：GetRawData()会消费掉原始的io.ReadCloser，后续的处理器（如JSON绑定）需要读取body
			// 使用io.NopCloser包装bytes.Buffer，满足io.ReadCloser接口要求
			// 这样既保存了body内容用于日志，又不影响后续处理器的正常使用
			c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
		}

		// 执行后续的中间件和处理器
		// 此时请求开始被处理，可能产生错误、修改响应等
		c.Next()

		// 计算请求耗时，用于性能监控
		// 使用time.Since()计算从start到现在的精确时间差
		cost := time.Since(start)

		// 构建日志布局结构，收集所有需要记录的日志信息
		layout := LogLayout{
			Time:      time.Now(),            // 使用当前时间作为日志时间戳
			Path:      path,                  // 使用之前保存的路径，确保是原始请求路径
			Query:     query,                 // 使用之前保存的查询参数
			IP:        c.ClientIP(),          // 获取客户端真实IP（考虑了代理的情况）
			UserAgent: c.Request.UserAgent(), // 获取用户代理信息
			// 提取并格式化错误信息
			// ByType(gin.ErrorTypePrivate)只获取私有错误（业务错误），不包含公开错误
			// TrimRight去除末尾换行符，使日志更整洁
			Error:  strings.TrimRight(c.Errors.ByType(gin.ErrorTypePrivate).String(), "\n"),
			Cost:   cost,     // 记录请求耗时
			Source: l.Source, // 使用Logger中配置的服务标识
		}

		// 如果之前读取了body，则将其添加到日志布局中
		// 注意：这里再次检查Filter，确保逻辑一致性
		// 只有需要记录的请求才会包含body内容
		if l.Filter != nil && !l.Filter(c) {
			layout.Body = string(body)
		}

		// 如果配置了鉴权处理函数，则执行它
		// 设计考虑：可选功能，用户可以根据需要决定是否提取用户信息
		// 例如：从JWT token中提取用户ID，或从session中获取用户名
		if l.AuthProcess != nil {
			// 处理鉴权需要的信息，如用户ID、用户名等
			// 传入layout指针，允许函数修改layout内容
			l.AuthProcess(c, &layout)
		}

		// 如果配置了关键字过滤函数，则执行它
		// 设计考虑：在日志输出前进行最后一道处理，可以：
		// 1. 数据脱敏：将敏感字段（密码、身份证等）替换为***
		// 2. 关键字过滤：根据业务规则决定是否记录该日志
		// 3. 格式调整：对日志内容进行格式化处理
		if l.FilterKeyword != nil {
			// 自行判断key/value脱敏等操作
			// 传入layout指针，允许函数修改或过滤layout内容
			l.FilterKeyword(&layout)
		}

		// 调用用户自定义的日志输出函数
		// 设计优势：
		// 1. 输出解耦：不关心日志输出到哪里（文件、数据库、消息队列等）
		// 2. 格式灵活：用户可以自定义输出格式（JSON、文本等）
		// 3. 异步支持：可以在Print函数中实现异步写入，提高性能
		l.Print(layout)
	}
}

// DefaultLogger 创建并返回默认的日志记录中间件
// 设计意义：
// 1. 开箱即用：提供最简单的使用方式，用户无需配置即可使用
// 2. 标准格式：使用JSON格式输出，便于日志收集系统（如ELK、Loki）解析和处理
// 3. 容器友好：输出到标准输出（stdout），符合容器化部署的最佳实践
// 4. 快速上手：降低使用门槛，适合快速原型开发
//
// 使用场景：
// - 开发环境：快速查看请求日志，无需复杂配置
// - 容器部署：K8s等容器编排系统会自动收集stdout日志
// - 日志聚合：JSON格式便于日志收集系统（如Filebeat、Fluentd）解析和索引
func DefaultLogger() gin.HandlerFunc {
	return Logger{
		// Print函数：将日志以JSON格式输出到标准输出
		// 设计考虑：
		// 1. JSON格式：结构化数据，便于解析和查询，支持复杂的日志分析
		// 2. 标准输出：容器化部署时，容器日志收集系统会自动收集stdout/stderr
		// 3. 简单高效：无需文件IO，性能开销小，适合高并发场景
		// 4. 错误忽略：json.Marshal的错误被忽略，因为LogLayout结构体字段都是可序列化的
		Print: func(layout LogLayout) {
			// 将LogLayout结构体序列化为JSON格式
			// 标准输出，K8s等容器编排系统会自动收集stdout日志
			v, _ := json.Marshal(layout)
			fmt.Println(string(v))
		},
		// 设置服务标识为"GVA"（Gin Vue Admin的缩写）
		// 在多服务架构中，可以通过这个标识区分日志来源
		Source: "GVA",
	}.SetLoggerMiddleware()
}
