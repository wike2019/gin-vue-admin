package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/utils"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// respPool 使用 sync.Pool 对象池来复用字节缓冲区
// 好处：减少内存分配和 GC 压力，提高性能
// 为什么这么写：在高并发场景下，频繁创建和销毁 []byte 会导致大量内存分配，
// 使用对象池可以复用已分配的内存，显著降低 GC 频率和内存占用
var respPool sync.Pool

// bufferSize 定义请求体和响应体的最大记录长度（字节）
// 为什么设置 1024：避免记录过大的请求/响应体导致数据库存储压力过大
// 好处：1. 控制数据库存储空间 2. 提高查询性能 3. 保护敏感大数据不被完整记录
// 意义：在可追溯性和存储效率之间取得平衡
var bufferSize = 1024

func init() {
	// 初始化对象池，定义如何创建新对象
	// 当池中没有可用对象时，会调用 New 函数创建新对象
	respPool.New = func() interface{} {
		return make([]byte, bufferSize)
	}
}

// OperationRecord 操作记录中间件
// 功能：记录所有 HTTP 请求的详细信息，包括请求参数、响应内容、执行时间等
// 意义：用于审计追踪、问题排查、性能分析、安全监控
// 好处：1. 完整的操作日志便于问题定位 2. 支持审计合规要求 3. 可用于性能优化分析
func OperationRecord() gin.HandlerFunc {
	return func(c *gin.Context) {
		var body []byte
		var userId int

		// 根据请求方法处理请求体
		// 为什么区分 GET 和其他方法：
		// - GET 请求的参数在 URL 查询字符串中，不在 Body 中
		// - POST/PUT/DELETE 等方法的参数在 Body 中
		if c.Request.Method != http.MethodGet {
			// 读取请求体内容
			// 为什么需要读取：Request.Body 是一个 io.ReadCloser，只能读取一次
			// 如果中间件不读取，后续的处理器就无法读取到请求体
			var err error
			body, err = io.ReadAll(c.Request.Body)
			if err != nil {
				global.GVA_LOG.Error("read body from request error:", zap.Error(err))
			} else {
				// 关键：将读取的内容重新放回 Request.Body
				// 为什么这么做：因为 Body 只能读取一次，读取后需要恢复给后续处理器使用
				// io.NopCloser 创建一个只读的 Closer，满足接口要求
				// 好处：既能在中间件中记录请求体，又不影响后续处理器的正常使用
				c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
			}
		} else {
			// GET 请求：从 URL 查询参数中提取数据
			// 为什么这么处理：GET 请求的参数在 URL 中，需要解析查询字符串
			query := c.Request.URL.RawQuery
			// URL 解码：将 %20 等编码字符还原为原始字符
			query, _ = url.QueryUnescape(query)
			// 按 & 分割查询参数：key1=value1&key2=value2
			split := strings.Split(query, "&")
			m := make(map[string]string)
			// 解析每个键值对
			for _, v := range split {
				kv := strings.Split(v, "=")
				// 确保是有效的键值对（避免空字符串或格式错误）
				if len(kv) == 2 {
					m[kv[0]] = kv[1]
				}
			}
			// 将查询参数转换为 JSON 格式，统一存储格式
			// 好处：GET 和 POST 的请求参数都以 JSON 格式存储，便于查询和分析
			body, _ = json.Marshal(&m)
		}
		// 获取用户ID，采用降级策略
		// 为什么使用降级策略：不同场景下用户身份信息可能存储在不同位置
		// 好处：提高兼容性和灵活性，支持多种认证方式
		claims, _ := utils.GetClaims(c)
		// 优先从 JWT Token 的 Claims 中获取用户ID（标准方式）
		// 为什么优先：JWT 是更安全、更标准的认证方式，信息经过签名验证
		if claims != nil && claims.BaseClaims.ID != 0 {
			userId = int(claims.BaseClaims.ID)
		} else {
			// 降级方案：从 HTTP Header 中获取用户ID
			// 使用场景：某些特殊场景下（如内部服务调用）可能通过 Header 传递用户ID
			// 为什么需要降级：确保即使 JWT 解析失败，也能记录操作（虽然用户ID可能为0）
			id, err := strconv.Atoi(c.Request.Header.Get("x-user-id"))
			if err != nil {
				userId = 0 // 解析失败时默认为 0，表示匿名用户或系统操作
			}
			userId = id
		}
		// 构建操作记录对象，收集请求的基本信息
		// 为什么记录这些信息：
		// - Ip: 用于安全审计，追踪请求来源
		// - Method: 记录操作类型（GET/POST/PUT/DELETE等）
		// - Path: 记录访问的接口路径
		// - Agent: 记录客户端信息，便于问题排查（浏览器版本、设备类型等）
		// - Body: 记录请求参数，便于问题复现和审计
		// - UserID: 记录操作用户，用于权限审计
		record := system.SysOperationRecord{
			Ip:     c.ClientIP(),
			Method: c.Request.Method,
			Path:   c.Request.URL.Path,
			Agent:  c.Request.UserAgent(),
			Body:   "",
			UserID: userId,
		}

		// 处理请求体的记录策略
		// 为什么需要特殊处理：不同类型的请求体需要不同的记录策略
		if strings.Contains(c.GetHeader("Content-Type"), "multipart/form-data") {
			// 文件上传请求：不记录实际内容，只标记为"[文件]"
			// 为什么这么做：
			// 1. 文件内容通常是二进制数据，无法以文本形式存储
			// 2. 文件可能很大，完整存储会占用大量数据库空间
			// 3. 文件内容通常不包含业务逻辑信息，只需要知道是文件上传即可
			// 好处：节省存储空间，提高性能，避免数据库被大文件内容拖慢
			record.Body = "[文件]"
		} else {
			// 普通请求：根据长度决定是否记录完整内容
			if len(body) > bufferSize {
				// 请求体过大：只标记，不记录完整内容
				// 为什么截断：避免大请求体导致数据库存储压力过大
				// 好处：在可追溯性和存储效率之间取得平衡
				record.Body = "[超出记录长度]"
			} else {
				// 正常大小的请求体：完整记录
				// 好处：便于问题排查和审计，可以完整复现请求参数
				record.Body = string(body)
			}
		}

		// 创建自定义的 ResponseWriter 包装器
		// 为什么需要包装：gin 的 ResponseWriter 不提供直接读取响应体的方法
		// 设计模式：使用装饰器模式，在不改变原有接口的情况下扩展功能
		// 好处：既能正常写入响应给客户端，又能同时捕获响应内容用于记录
		writer := responseBodyWriter{
			ResponseWriter: c.Writer,        // 保留原始的 ResponseWriter，确保功能正常
			body:           &bytes.Buffer{}, // 用于缓存响应体内容
		}
		// 替换 c.Writer，后续的响应写入都会经过我们的包装器
		c.Writer = writer
		// 记录请求开始时间，用于计算延迟
		now := time.Now()

		// 执行后续的中间件和处理器
		// 在 c.Next() 执行期间，响应会被写入到 writer.body 中
		c.Next()

		// 请求处理完成，收集响应相关信息
		latency := time.Since(now) // 计算请求处理耗时
		// 记录私有错误信息（服务器内部错误，不暴露给客户端）
		// 为什么记录：便于排查服务器内部问题
		record.ErrorMessage = c.Errors.ByType(gin.ErrorTypePrivate).String()
		record.Status = c.Writer.Status()  // HTTP 状态码（200, 404, 500等）
		record.Latency = latency           // 请求处理耗时
		record.Resp = writer.body.String() // 响应体内容

		// 检测是否为文件下载响应
		// 为什么需要检测：文件下载的响应体通常是二进制数据，且可能很大
		// 检测方式：通过检查响应头中的 Content-Type、Content-Disposition 等字段
		// 这些 HTTP 头是文件下载的标准标识：
		// - Pragma: public / Expires: 0 - 缓存控制，表示文件下载
		// - Cache-Control: must-revalidate - 强制重新验证，常用于下载
		// - Content-Type: application/force-download - 强制下载
		// - Content-Type: application/octet-stream - 二进制流（通用文件类型）
		// - Content-Type: application/vnd.ms-excel - Excel 文件
		// - Content-Disposition: attachment - 附件下载
		// - Content-Transfer-Encoding: binary - 二进制传输编码
		if strings.Contains(c.Writer.Header().Get("Pragma"), "public") ||
			strings.Contains(c.Writer.Header().Get("Expires"), "0") ||
			strings.Contains(c.Writer.Header().Get("Cache-Control"), "must-revalidate, post-check=0, pre-check=0") ||
			strings.Contains(c.Writer.Header().Get("Content-Type"), "application/force-download") ||
			strings.Contains(c.Writer.Header().Get("Content-Type"), "application/octet-stream") ||
			strings.Contains(c.Writer.Header().Get("Content-Type"), "application/vnd.ms-excel") ||
			strings.Contains(c.Writer.Header().Get("Content-Type"), "application/download") ||
			strings.Contains(c.Writer.Header().Get("Content-Disposition"), "attachment") ||
			strings.Contains(c.Writer.Header().Get("Content-Transfer-Encoding"), "binary") {
			// 文件下载响应：如果响应体过大，截断记录
			// 为什么截断：文件内容通常是二进制数据，且可能很大（几MB到几GB）
			// 完整存储会导致：1. 数据库存储压力大 2. 查询性能下降 3. 内存占用高
			// 注意：这里修改的是 record.Body（请求体），可能是代码中的一个小问题
			// 应该修改 record.Resp（响应体）更合理，但保持原逻辑不变
			if len(record.Resp) > bufferSize {
				record.Body = "超出记录长度"
			}
		}
		// 将操作记录保存到数据库
		// 为什么异步执行：避免阻塞请求响应，提高用户体验
		// 注意：这里使用的是同步 Create，在高并发场景下可能成为性能瓶颈
		// 建议：可以考虑使用异步队列（如消息队列）来保存日志，进一步提高性能
		if err := global.GVA_DB.Create(&record).Error; err != nil {
			global.GVA_LOG.Error("create operation record error:", zap.Error(err))
		}
	}
}

// responseBodyWriter 自定义的 ResponseWriter 包装器
// 设计模式：装饰器模式（Decorator Pattern）
// 为什么需要这个包装器：
// 1. gin.ResponseWriter 接口不提供读取已写入响应的方法
// 2. 我们需要在响应写入时同时捕获内容用于日志记录
// 3. 必须保持原有的 ResponseWriter 功能不变（写入到客户端）
//
// 结构设计：
// - ResponseWriter: 嵌入原始的 ResponseWriter，实现接口的所有方法
// - body: 额外的缓冲区，用于缓存响应内容
//
// 好处：
// 1. 透明代理：对上层代码完全透明，不需要修改业务逻辑
// 2. 功能扩展：在不改变原有接口的情况下添加新功能
// 3. 性能友好：使用 bytes.Buffer 高效缓存响应内容
type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// Write 实现 io.Writer 接口
// 为什么重写这个方法：这是响应写入的关键方法，需要在这里拦截响应内容
// 工作原理：
// 1. 先将响应内容写入 body 缓冲区（用于日志记录）
// 2. 再将响应内容写入原始的 ResponseWriter（发送给客户端）
//
// 好处：
// 1. 双重写入：既满足日志记录需求，又保证客户端正常接收响应
// 2. 顺序保证：先写入缓冲区，再写入客户端，确保数据一致性
// 3. 错误传播：如果写入客户端失败，错误会正常返回给调用者
func (r responseBodyWriter) Write(b []byte) (int, error) {
	// 先写入缓冲区，用于后续日志记录
	r.body.Write(b)
	// 再写入原始 ResponseWriter，发送给客户端
	// 注意：这里返回的是原始写入的结果，保持接口语义
	return r.ResponseWriter.Write(b)
}
