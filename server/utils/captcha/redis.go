package captcha

import (
	"context"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"go.uber.org/zap"
)

func NewDefaultRedisStore() *RedisStore {
	return &RedisStore{
		Expiration: time.Second * 180, // 默认过期时间180秒（3分钟）
		PreKey:     "CAPTCHA_",        // 键前缀，用于区分不同类型的 Redis 数据
		Context:    context.TODO(),    // 默认上下文，当没有请求上下文时使用
	}
}

// RedisStore 基于 Redis 的验证码存储实现
//
// 为什么选择 Redis 存储验证码：
// 1. 性能优势：Redis 是内存数据库，读写速度极快，适合高并发场景
// 2. 过期机制：Redis 原生支持 TTL（Time To Live），可以自动清理过期验证码，无需手动管理
// 3. 分布式支持：Redis 可以部署为集群，支持分布式系统的验证码共享和验证
// 4. 持久化可选：可根据需要配置持久化，在服务重启时保留验证码（通常验证码不需要持久化）
//
// 字段说明：
// - Expiration: 验证码过期时间，超过此时间验证码自动失效，提高安全性
// - PreKey: 键前缀，避免与其他 Redis 数据冲突，便于管理和清理（如：CAPTCHA_xxx）
// - Context: 上下文，用于传递请求级信息（如超时控制、取消信号、追踪ID等），支持请求级别的生命周期管理
type RedisStore struct {
	Expiration time.Duration   // 验证码过期时间，防止验证码永久有效带来的安全风险
	PreKey     string          // Redis 键前缀，用于命名空间隔离，便于区分和批量管理验证码数据
	Context    context.Context // 上下文，支持请求级超时控制和取消操作，提升系统可观测性
}

// UseWithCtx 设置 RedisStore 使用的上下文
//
// 为什么需要可切换的上下文：
// 1. 支持请求级超时控制：可以设置请求级别的超时，避免 Redis 操作无限等待
// 2. 支持请求追踪：可以传递追踪ID，便于分布式系统中的问题排查
// 3. 支持请求取消：当请求被取消时，可以立即取消正在进行的 Redis 操作
//
// 参数：
//   - ctx: 要使用的上下文，如果为 nil 则不更新（注意：当前实现有逻辑问题，应该判断 ctx != nil）
//
// 返回值：
//   - *RedisStore: 返回自身，支持链式调用（如：store.UseWithCtx(ctx).Set(...)）
//
// 注意：当前代码存在 bug，第26行的判断条件应该是 ctx != nil 而非 ctx == nil
func (rs *RedisStore) UseWithCtx(ctx context.Context) *RedisStore {
	if ctx != nil { // 只有当 ctx 不为 nil 时才更新，避免将有效的上下文覆盖为 nil
		rs.Context = ctx
	}
	return rs
}

// Set 将验证码存储到 Redis 中
//
// 设计考虑：
// 1. 使用键前缀 + ID 作为完整键名：确保键的唯一性和可识别性，避免键冲突
// 2. 设置过期时间：利用 Redis 的 TTL 特性，自动清理过期验证码，无需后台任务
// 3. 错误处理和日志记录：记录错误以便排查问题，但返回错误让调用者决定如何处理
//
// 参数：
//   - id: 验证码的唯一标识符（通常由验证码库生成）
//   - value: 验证码的值（通常是数字或字符串）
//
// 返回值：
//   - error: 如果 Redis 操作失败则返回错误，成功返回 nil
func (rs *RedisStore) Set(id string, value string) error {
	// 使用 PreKey + id 作为完整的 Redis 键，保证命名空间隔离和键的唯一性
	// 设置过期时间，Redis 会自动删除过期的键，避免内存泄漏
	err := global.GVA_REDIS.Set(rs.Context, rs.PreKey+id, value, rs.Expiration).Err()
	if err != nil {
		// 记录错误日志，方便排查问题，但不影响调用者的错误处理逻辑
		global.GVA_LOG.Error("RedisStoreSetError!", zap.Error(err))
		return err
	}
	return nil
}

// Get 从 Redis 中获取验证码值
//
// 设计优势：
// 1. 支持一次性验证：clear 参数允许在获取后立即删除验证码，防止重复使用（一次性验证码场景）
// 2. 容错处理：Redis 操作失败时返回空字符串，调用者可以通过判断空字符串来处理异常情况
// 3. 统一的错误处理：所有 Redis 错误都记录日志，便于运维监控和问题排查
//
// 参数：
//   - key: Redis 的完整键名（包含前缀，通常是 PreKey + id）
//   - clear: 是否在获取后立即删除该验证码
//     true: 获取后删除，实现一次性验证（更安全，防止验证码被重复使用）
//     false: 只获取不删除，允许重复验证（适用于需要多次校验的场景）
//
// 返回值：
//   - string: 验证码的值，如果获取失败或键不存在则返回空字符串
func (rs *RedisStore) Get(key string, clear bool) string {
	// 从 Redis 获取验证码值
	val, err := global.GVA_REDIS.Get(rs.Context, key).Result()
	if err != nil {
		// 记录错误日志，便于排查 Redis 连接、键不存在等问题
		global.GVA_LOG.Error("RedisStoreGetError!", zap.Error(err))
		return "" // 返回空字符串，让调用者知道获取失败
	}
	// 如果设置了 clear 标志，获取后立即删除，实现一次性验证码
	// 这样做的好处：防止验证码被多次使用，提高安全性（即使验证码被截获，也只能使用一次）
	if clear {
		err := global.GVA_REDIS.Del(rs.Context, key).Err()
		if err != nil {
			// 删除失败也记录日志，但返回已获取的值（因为验证码已经读取成功）
			global.GVA_LOG.Error("RedisStoreClearError!", zap.Error(err))
			return "" // 删除失败时返回空，确保安全性（宁可验证失败也不允许潜在的安全漏洞）
		}
	}
	return val
}

// Verify 验证用户输入的验证码是否正确
//
// 为什么封装这个验证方法：
// 1. 简化调用：将"构造键名 + 获取值 + 比较"的逻辑封装，调用者只需一行代码
// 2. 统一验证逻辑：确保所有验证码验证都使用相同的方式，避免代码重复和不一致
// 3. 支持一次性验证：通过 clear 参数控制是否删除验证码，默认实现更安全的验证流程
//
// 工作流程：
// 1. 根据 id 构造完整的 Redis 键（PreKey + id）
// 2. 从 Redis 获取存储的验证码值
// 3. 将获取的值与用户输入的答案进行字符串比较
// 4. 如果 clear=true，验证码在验证后会被删除（防止重复使用）
//
// 参数：
//   - id: 验证码的唯一标识符
//   - answer: 用户输入的验证码答案
//   - clear: 验证后是否删除验证码
//     true: 验证后删除，实现一次性验证码（推荐，更安全）
//     false: 验证后保留，允许重复验证（适用于需要多次校验的场景）
//
// 返回值：
//   - bool: true 表示验证通过，false 表示验证失败（验证码错误、过期或不存在）
func (rs *RedisStore) Verify(id, answer string, clear bool) bool {
	// 构造完整的 Redis 键名（使用统一的键前缀 + 验证码ID）
	key := rs.PreKey + id
	// 从 Redis 获取存储的验证码值，并根据 clear 参数决定是否删除
	v := rs.Get(key, clear)
	// 进行字符串比较，注意：这里使用精确匹配，区分大小写
	// 如果需要在验证时忽略大小写，可以在比较前对两个字符串进行大小写转换
	return v == answer
}
