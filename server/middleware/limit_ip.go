package middleware

import (
	"context"
	"errors"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/gin-gonic/gin"
)

// LimitConfig IP限流配置结构体
// 采用策略模式设计，通过函数式接口实现高度可扩展的限流机制
// 这样设计的好处：
// 1. 解耦合：将限流逻辑与具体实现分离，便于测试和维护
// 2. 灵活性：允许用户自定义key生成规则和限流检查逻辑，适应不同业务场景
// 3. 可扩展性：可以轻松切换不同的限流算法（如令牌桶、漏桶、滑动窗口等）
type LimitConfig struct {
	// GenerationKey 根据业务生成限流key的函数
	// 为什么使用函数而不是字符串：
	// - 动态性：可以根据请求上下文（IP、用户ID、路径等）动态生成key
	// - 灵活性：不同接口可以使用不同的限流策略（如按IP、按用户、按接口）
	// - 可测试性：可以轻松mock和测试不同的key生成逻辑
	GenerationKey func(c *gin.Context) string
	// CheckOrMark 限流检查函数，用户可修改具体逻辑，更加灵活
	// 为什么使用函数接口：
	// - 策略模式：可以切换不同的限流实现（Redis、内存、数据库等）
	// - 单一职责：将限流算法与中间件逻辑分离
	// - 易于扩展：可以轻松添加新的限流算法而不修改现有代码
	CheckOrMark func(key string, expire int, limit int) error
	// Expire key 过期时间（单位：秒）
	// 为什么需要过期时间：
	// - 防止内存泄漏：自动清理过期的限流记录
	// - 滑动窗口：实现时间窗口内的限流控制
	Expire int
	// Limit 周期时间内允许的最大请求次数
	// 与Expire配合使用，实现"在Expire秒内最多允许Limit次请求"的限流策略
	Limit int
}

// LimitWithTime 返回一个Gin中间件函数，实现基于时间的IP限流
// 为什么返回gin.HandlerFunc：
// - 符合Gin框架的中间件规范，可以直接使用Use()方法注册
// - 延迟执行：返回函数而不是直接执行，让Gin框架控制执行时机
// - 闭包特性：可以访问LimitConfig的配置，实现配置的封装
func (l LimitConfig) LimitWithTime() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 生成限流key：根据配置的生成函数动态生成
		// 好处：支持按IP、用户、接口等不同维度限流
		key := l.GenerationKey(c)

		// 执行限流检查：调用配置的检查函数
		// 如果返回错误，说明超过限流阈值
		if err := l.CheckOrMark(key, l.Expire, l.Limit); err != nil {
			// 返回错误响应，告知客户端请求过于频繁
			// 使用统一的响应格式，保持API一致性
			c.JSON(http.StatusOK, gin.H{"code": response.ERROR, "msg": err.Error()})
			// 终止请求处理链，不再执行后续的handler
			// 为什么使用Abort()：防止后续中间件和handler继续执行，节省资源
			c.Abort()
			return
		} else {
			// 通过限流检查，继续执行下一个中间件或handler
			// Next()是Gin框架的核心方法，用于传递控制权
			c.Next()
		}
	}
}

// DefaultGenerationKey 默认的key生成函数，基于客户端IP地址
// 为什么使用IP作为限流维度：
// - 简单有效：IP是识别客户端的最直接方式
// - 防止滥用：可以有效防止单个IP的恶意请求
// - 性能考虑：IP提取速度快，不需要额外的数据库查询
// 为什么添加"GVA_Limit"前缀：
// - 命名空间隔离：避免与其他Redis key冲突
// - 便于管理：可以通过前缀批量查找和管理限流相关的key
func DefaultGenerationKey(c *gin.Context) string {
	return "GVA_Limit" + c.ClientIP()
}

// DefaultCheckOrMark 默认的限流检查函数，使用Redis实现分布式限流
// 为什么使用Redis：
// - 分布式支持：多服务器实例共享限流状态，避免单机限流的局限性
// - 高性能：Redis内存操作，响应速度快
// - 原子性：Redis的原子操作保证并发安全，避免竞态条件
// - 持久化：可配置持久化，重启后限流状态不丢失（可选）
func DefaultCheckOrMark(key string, expire int, limit int) (err error) {
	// 判断是否开启Redis
	// 为什么需要检查：如果Redis未配置，限流功能应该优雅降级
	// 好处：避免程序崩溃，提高系统的健壮性
	if global.GVA_REDIS == nil {
		// 如果Redis未配置，返回nil允许请求通过
		// 这样设计的好处：在开发环境或Redis故障时，系统仍可正常运行
		return err
	}
	// 调用核心限流逻辑，设置限流规则
	// expire转换为time.Duration：确保时间单位统一，避免单位混淆导致的bug
	if err = SetLimitWithTime(key, limit, time.Duration(expire)*time.Second); err != nil {
		// 记录错误日志，便于排查问题
		// 使用结构化日志（zap）：便于日志分析和监控告警
		global.GVA_LOG.Error("limit", zap.Error(err))
	}
	return err
}

// DefaultLimit 创建默认的IP限流中间件
// 为什么提供这个便捷函数：
// - 开箱即用：大多数场景使用默认配置即可，简化使用
// - 配置集中：从全局配置读取参数，便于统一管理
// - 代码复用：避免每次使用时重复创建LimitConfig
// 设计模式：工厂模式，封装了对象的创建过程
func DefaultLimit() gin.HandlerFunc {
	return LimitConfig{
		// 使用默认的IP key生成策略
		GenerationKey: DefaultGenerationKey,
		// 使用默认的Redis限流检查策略
		CheckOrMark: DefaultCheckOrMark,
		// 从配置文件读取限流时间窗口（秒）
		// 好处：可以在不修改代码的情况下调整限流参数
		Expire: global.GVA_CONFIG.System.LimitTimeIP,
		// 从配置文件读取限流次数阈值
		// 好处：可以根据实际业务需求动态调整限流强度
		Limit: global.GVA_CONFIG.System.LimitCountIP,
	}.LimitWithTime() // 链式调用，返回配置好的中间件函数
}

// SetLimitWithTime 核心限流逻辑：基于Redis实现固定时间窗口限流算法
// 算法原理：在expiration时间窗口内，最多允许limit次请求
// 为什么使用固定时间窗口：
// - 实现简单：逻辑清晰，易于理解和维护
// - 性能好：Redis操作少，响应速度快
// - 内存占用小：每个key只存储一个计数器
// 缺点：窗口边界可能出现请求突增（可通过滑动窗口优化，但会增加复杂度）
func SetLimitWithTime(key string, limit int, expiration time.Duration) error {
	// 检查key是否存在
	// 为什么先检查存在性：
	// - 区分首次请求和后续请求，采用不同的处理逻辑
	// - 首次请求需要初始化计数器和过期时间
	// - 后续请求只需要检查和递增计数器
	count, err := global.GVA_REDIS.Exists(context.Background(), key).Result()
	if err != nil {
		return err
	}

	// 如果key不存在，说明是时间窗口内的首次请求
	if count == 0 {
		// 使用Redis事务管道（TxPipeline）确保原子性
		// 为什么使用事务：
		// - 原子性：Incr和Expire必须同时成功，避免计数器存在但没有过期时间的情况
		// - 性能：管道批量执行，减少网络往返次数
		// - 一致性：防止并发请求时出现竞态条件
		pipe := global.GVA_REDIS.TxPipeline()
		// 初始化计数器为1（首次请求）
		pipe.Incr(context.Background(), key)
		// 设置key的过期时间，实现时间窗口
		// 好处：Redis自动清理过期key，无需手动管理
		pipe.Expire(context.Background(), key, expiration)
		// 执行事务，确保两个操作同时完成
		_, err = pipe.Exec(context.Background())
		return err
	} else {
		// key已存在，说明在时间窗口内已有请求记录
		// 获取当前请求次数
		if times, err := global.GVA_REDIS.Get(context.Background(), key).Int(); err != nil {
			return err
		} else {
			// 检查是否超过限流阈值
			if times >= limit {
				// 超过限制，返回友好的错误信息
				// 获取key的剩余过期时间（PTTL：毫秒级精度）
				// 为什么使用PTTL而不是TTL：
				// - 更精确：毫秒级精度，用户体验更好
				// - 一致性：与Expire设置的精度匹配
				if t, err := global.GVA_REDIS.PTTL(context.Background(), key).Result(); err != nil {
					// 如果获取剩余时间失败，返回通用错误信息
					// 容错处理：即使获取时间失败，也要阻止请求
					return errors.New("请求太过频繁，请稍后再试")
				} else {
					// 返回包含剩余等待时间的详细错误信息
					// 好处：用户知道需要等待多久，提升用户体验
					// 注意：t是time.Duration类型，String()方法会自动格式化为可读的时间字符串
					return errors.New("请求太过频繁, 请 " + t.String() + " 秒后尝试")
				}
			} else {
				// 未超过限制，递增计数器
				// 为什么使用Incr而不是先Get再Set：
				// - 原子性：Incr是原子操作，避免并发问题
				// - 性能：单次操作，比Get+Set更快
				// - 简洁：代码更简单，逻辑更清晰
				return global.GVA_REDIS.Incr(context.Background(), key).Err()
			}
		}
	}
}
