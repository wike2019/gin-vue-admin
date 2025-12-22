package middleware

import (
	"bytes"
	"io"
	"strconv"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/plugin/email/utils"
	utils2 "github.com/flipped-aurora/gin-vue-admin/server/utils"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ErrorToEmail 返回一个 Gin 中间件函数，用于在请求发生错误时自动发送邮件通知
//
// 设计目的：
// 1. 实时监控系统错误，当接口返回非200状态码时自动发送邮件告警
// 2. 记录完整的错误上下文信息（用户、IP、请求参数、错误信息等），便于快速定位问题
// 3. 提供错误追踪能力，帮助开发团队及时发现和解决生产环境问题
//
// 使用场景：
// - 生产环境错误监控和告警
// - 异常请求追踪和审计
// - 系统稳定性监控
func ErrorToEmail() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 第一步：获取用户名信息
		// 用户名用于标识是哪个用户触发了错误，便于问题追踪和责任定位
		var username string

		// 优先从 JWT token 中获取用户名（标准认证方式）
		// 这种方式性能最好，无需查询数据库，且信息最可靠
		claims, _ := utils2.GetClaims(c)
		if claims.Username != "" {
			username = claims.Username
		} else {
			// 如果 JWT 中没有用户名，尝试从请求头中获取用户ID
			// 这是为了兼容某些特殊场景（如内部服务调用、测试环境等）
			// 通过 x-user-id 头可以获取用户ID，然后查询数据库获取用户名
			id, _ := strconv.Atoi(c.Request.Header.Get("x-user-id"))
			var u system.SysUser
			err := global.GVA_DB.Where("id = ?", id).First(&u).Error
			if err != nil {
				// 如果查询失败，使用默认值，确保后续流程不会因用户名缺失而中断
				username = "Unknown"
			} else {
				username = u.Username
			}
		}
		// 第二步：读取并保存请求体内容
		// 为什么需要这样做？
		// 1. io.ReadAll 读取 Request.Body 后，流会被消费完，后续的处理器无法再读取
		// 2. 我们需要保存请求体内容用于错误日志，但也要保证后续处理器能正常读取
		// 3. 解决方案：读取后立即用 io.NopCloser 和 bytes.NewBuffer 重新构造一个可读的流
		// 这样做的好处：
		// - 既保存了请求体内容用于错误追踪
		// - 又不影响后续中间件和路由处理器的正常使用
		body, _ := io.ReadAll(c.Request.Body)
		// 重新写回请求体body中，确保后续处理器可以正常读取
		// io.NopCloser 返回一个实现了 io.ReadCloser 接口的对象，但 Close 方法为空操作
		// bytes.NewBuffer 将字节数组转换为可读的缓冲区
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
		// 第三步：构建操作记录对象，保存请求的基本信息
		// 这些信息对于错误排查非常重要：
		// - Ip: 定位请求来源，判断是否为异常IP或攻击
		// - Method: HTTP方法（GET/POST等），了解操作类型
		// - Path: 请求路径，定位具体出错的接口
		// - Agent: 用户代理信息，了解客户端环境
		// - Body: 请求参数，用于重现错误场景
		record := system.SysOperationRecord{
			Ip:     c.ClientIP(),          // 获取客户端真实IP（考虑了代理情况）
			Method: c.Request.Method,      // HTTP请求方法
			Path:   c.Request.URL.Path,    // 请求路径
			Agent:  c.Request.UserAgent(), // 用户代理字符串
			Body:   string(body),          // 请求体内容（已转换为字符串）
		}

		// 记录请求开始时间，用于计算请求处理耗时
		// 耗时信息可以帮助判断是否存在性能问题
		now := time.Now()

		// 第四步：执行后续的中间件和路由处理器
		// c.Next() 是关键：它会暂停当前中间件的执行，继续执行后续的中间件链
		// 当所有后续处理器执行完毕后，才会返回到这里继续执行
		// 这种设计的好处：
		// - 可以在请求处理前记录开始时间
		// - 可以在请求处理后获取响应状态码和错误信息
		// - 实现了"环绕式"的监控逻辑
		c.Next()

		// 第五步：请求处理完成后的处理逻辑
		// 计算请求处理耗时，用于性能监控
		latency := time.Since(now)

		// 获取HTTP响应状态码
		// 状态码是判断请求是否成功的关键指标
		status := c.Writer.Status()

		// 获取私有错误信息（通过 gin.ErrorTypePrivate 类型过滤）
		// gin.Errors 中可能包含多种类型的错误，我们只关心私有错误（业务逻辑错误）
		// 这样可以过滤掉一些公开的错误信息，只记录真正需要关注的错误
		record.ErrorMessage = c.Errors.ByType(gin.ErrorTypePrivate).String()

		// 构建详细的错误报告字符串
		// 包含所有关键信息：请求体、请求方法、错误信息、处理耗时
		// 这种格式化的错误报告便于运维人员快速理解问题
		str := "接收到的请求为" + record.Body + "\n" +
			"请求方式为" + record.Method + "\n" +
			"报错信息如下" + record.ErrorMessage + "\n" +
			"耗时" + latency.String() + "\n"

		// 第六步：判断是否需要发送错误邮件
		// 只有当状态码不是200时才发送邮件（包括4xx客户端错误和5xx服务器错误）
		// 这样设计的好处：
		// - 避免正常请求产生大量邮件通知
		// - 只关注异常情况，提高告警的有效性
		// - 减少邮件服务器负担和运维人员的工作量
		if status != 200 {
			// 构建邮件主题，包含关键信息：用户名、IP、请求路径
			// 主题要简洁明了，让收件人一眼就能看出问题概况
			subject := username + "" + record.Ip + "调用了" + record.Path + "报错了"

			// 发送错误邮件
			// 如果发送失败，记录日志但不影响主流程
			// 这样设计的好处：
			// - 邮件发送失败不会影响HTTP请求的正常返回
			// - 通过日志记录发送失败的情况，便于排查邮件服务问题
			if err := utils.ErrorToEmail(subject, str); err != nil {
				global.GVA_LOG.Error("ErrorToEmail Failed, err:", zap.Error(err))
			}
		}
	}
}
