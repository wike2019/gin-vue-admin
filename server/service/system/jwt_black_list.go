package system

import (
	"context"

	"go.uber.org/zap"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
)

// JwtService JWT 服务，负责 JWT 黑名单管理和 Redis JWT 查询
//
// 【设计目的】
// 1. JWT 黑名单管理：
//   - 当用户退出登录或 token 需要被拉黑时，将 JWT 加入黑名单
//   - 黑名单用于防止已失效的 token 继续被使用，提升系统安全性
//   - 支持双重存储：数据库（持久化）+ 本地缓存（快速查询）
//
// 2. Redis JWT 查询：
//   - 从 Redis 中获取用户的有效 JWT token
//   - 用于多点登录场景，管理用户在不同设备上的登录状态
//
// 【为什么使用双重存储（数据库+缓存）？】
// 1. 数据库存储（持久化）：
//   - 保证数据不丢失：即使服务重启，黑名单数据仍然存在
//   - 支持数据恢复：可以从数据库恢复缓存数据
//   - 支持数据查询：可以通过数据库查询历史黑名单记录
//   - 支持数据统计：可以统计黑名单数量、分析黑名单趋势
//
// 2. 本地缓存存储（快速查询）：
//   - 性能优势：内存查询速度极快（微秒级），比数据库查询快几个数量级
//   - 减少数据库压力：每次 JWT 验证都需要查询黑名单，如果都查数据库会带来巨大压力
//   - 降低延迟：JWT 验证是高频操作，低延迟对用户体验很重要
//   - 自动过期：缓存会自动过期，与 JWT 过期时间一致，无需手动清理
//
// 【使用场景】
// 1. 用户退出登录：将当前 token 加入黑名单，防止 token 继续使用
// 2. 管理员强制下线：将用户的 token 加入黑名单，立即失效
// 3. 安全事件：发现 token 泄露，立即拉黑，防止被恶意使用
// 4. JWT 验证：每次请求时快速检查 token 是否在黑名单中
type JwtService struct{}

// JwtServiceApp JWT 服务的全局实例
// 使用单例模式，确保整个应用只有一个 JWT 服务实例
// 好处：
// - 统一管理：所有 JWT 相关操作都通过这个实例
// - 避免重复创建：减少内存占用
// - 便于测试：可以轻松替换为 mock 对象
var JwtServiceApp = new(JwtService)

// JsonInBlacklist 将 JWT token 加入黑名单
//
// 【功能说明】
// 将指定的 JWT token 同时保存到数据库和本地缓存中，使其立即失效
// 这是 JWT 黑名单的核心功能，用于实现安全的 token 撤销机制
//
// 【执行流程】
// 1. 先写入数据库：保证数据持久化，即使服务重启也不会丢失
// 2. 再写入缓存：保证立即生效，后续的 JWT 验证可以快速查询
//
// 【为什么先写数据库再写缓存？】
// 这是一个重要的设计决策，原因如下：
//
// 1. 数据一致性：
//   - 数据库是"源 of truth"（数据源），缓存是"副本"
//   - 如果先写缓存后写数据库，数据库写入失败会导致数据不一致
//   - 先写数据库可以确保数据持久化成功，即使缓存写入失败，下次启动时 LoadAll 会恢复
//
// 2. 容错性：
//   - 如果数据库写入失败，直接返回错误，不写入缓存
//   - 避免缓存中有数据但数据库没有，导致数据不一致
//   - 如果数据库写入成功但缓存写入失败，下次启动时 LoadAll 会恢复缓存
//
// 3. 数据恢复：
//   - 即使缓存丢失，也可以从数据库恢复
//   - 如果先写缓存，数据库写入失败，缓存数据无法恢复
//
// 【为什么使用 struct{}{} 作为缓存值？】
// 1. 内存优化：
//   - struct{}{} 是 Go 中占用内存最小的类型（0 字节）
//   - 我们只需要知道 JWT 是否在黑名单中，不需要存储额外信息
//   - 使用 map[string]struct{}{} 实现 Set 数据结构，比 map[string]bool 更节省内存
//
// 2. 语义清晰：
//   - 黑名单只需要"存在性"判断，不需要存储值
//   - struct{}{} 明确表示"只关心 key 是否存在，不关心 value"
//
// 3. 性能优势：
//   - 空结构体不占用内存，减少 GC 压力
//   - 查询速度与 map[string]bool 相同，但内存占用更小
//
// 【使用场景】
// 1. 用户退出登录：
//   - 用户点击退出，调用此方法将当前 token 拉黑
//   - 防止 token 被继续使用，提升安全性
//
// 2. 管理员强制下线：
//   - 管理员发现异常登录，强制用户下线
//   - 立即拉黑所有相关 token，防止继续访问
//
// 3. 安全事件处理：
//   - 发现 token 泄露，立即拉黑
//   - 防止恶意使用泄露的 token
//
// 【参数说明】
//   - jwtList: JWT 黑名单模型，包含要拉黑的 JWT token
//     模型结构：
//   - Jwt: JWT token 字符串（完整的 token）
//   - GVA_MODEL: 基础模型，包含 ID、创建时间等字段
//
// 【返回值】
//   - err: 如果数据库写入失败，返回错误；否则返回 nil
//     注意：缓存写入失败不会返回错误，因为缓存是可恢复的
//
// 【注意事项】
// - 此方法会同时写入数据库和缓存，确保数据一致性
// - 缓存会自动过期（过期时间与 JWT 过期时间一致），无需手动清理
// - 如果数据库写入失败，不会写入缓存，保证数据一致性
//
// @author: [piexlmax](https://github.com/piexlmax)
// @function: JsonInBlacklist
// @description: 拉黑jwt
// @param: jwtList model.JwtBlacklist
// @return: err error
func (jwtService *JwtService) JsonInBlacklist(jwtList system.JwtBlacklist) (err error) {
	// 第一步：将 JWT 黑名单记录写入数据库
	// 使用 GORM 的 Create 方法，自动处理 ID 生成、时间戳等
	// 为什么使用 .Error 获取错误？
	// - GORM 的 Create 方法返回 *gorm.DB，需要通过 .Error 获取实际错误
	// - 如果 err != nil，说明数据库写入失败，直接返回错误
	err = global.GVA_DB.Create(&jwtList).Error
	if err != nil {
		// 数据库写入失败，直接返回错误
		// 不写入缓存，保证数据一致性
		return
	}
	// 第二步：将 JWT token 加入本地缓存
	// 使用 SetDefault 方法，使用缓存的默认过期时间（与 JWT 过期时间一致）
	// 为什么使用 struct{}{} 作为值？
	// - 我们只需要知道 token 是否在黑名单中，不需要存储额外信息
	// - struct{}{} 是 0 字节，最节省内存
	// - 实现 Set 数据结构的效果：只关心 key 是否存在
	global.BlackCache.SetDefault(jwtList.Jwt, struct{}{})
	return
}

// GetRedisJWT 从 Redis 中获取指定用户的有效 JWT token
//
// 【功能说明】
// 根据用户名从 Redis 中查询该用户当前有效的 JWT token
// 主要用于多点登录场景，管理用户在不同设备上的登录状态
//
// 【为什么使用 Redis 存储 JWT？】
// 1. 多点登录支持：
//   - 用户可以在多个设备上同时登录（手机、电脑、平板等）
//   - 每个设备可能有不同的 token，需要统一管理
//   - Redis 可以存储用户与 token 的映射关系
//
// 2. 分布式支持：
//   - 如果系统部署在多个服务器上，需要共享登录状态
//   - Redis 作为共享存储，所有服务器都可以访问
//   - 本地缓存无法在多个服务器间共享
//
// 3. 快速查询：
//   - Redis 是内存数据库，查询速度极快
//   - 比数据库查询快，适合高频的 token 验证场景
//
// 4. 自动过期：
//   - Redis 支持设置 key 的过期时间
//   - token 过期后自动删除，无需手动清理
//
// 【使用场景】
// 1. 多点登录验证：
//   - 用户登录时，将 token 存储到 Redis（key: 用户名, value: token）
//   - 验证 token 时，从 Redis 查询该用户的有效 token
//   - 如果 token 匹配，说明是有效的登录
//
// 2. 强制下线：
//   - 管理员强制用户下线时，删除 Redis 中的 token
//   - 用户再次请求时，token 验证失败，实现强制下线
//
// 3. 单点登录（SSO）：
//   - 如果配置为单点登录，新登录会覆盖旧 token
//   - 旧设备上的 token 失效，实现单设备登录
//
// 【为什么使用 context.Background()？】
// 1. 简单场景：
//   - 这是一个简单的查询操作，不需要复杂的上下文控制
//   - 不需要超时控制、取消操作等高级功能
//
// 2. 兼容性：
//   - Redis 客户端要求传入 context，使用 Background 是最简单的选择
//   - 如果需要超时控制，可以传入带超时的 context
//
// 【参数说明】
//   - userName: 用户名，作为 Redis 的 key
//     格式：通常直接使用用户名，如 "admin"、"user123"
//     注意：确保用户名唯一，避免不同用户的 token 冲突
//
// 【返回值】
//   - redisJWT: 从 Redis 中获取的 JWT token 字符串
//     如果 key 不存在，返回空字符串
//   - err: 查询过程中的错误
//     常见错误：
//   - redis.Nil: key 不存在（用户未登录或 token 已过期）
//   - 网络错误：Redis 连接失败
//   - 其他 Redis 错误
//
// 【注意事项】
// - 此方法只查询 Redis，不查询数据库
// - 如果 Redis 未启用，此方法会返回错误
// - 调用方需要处理 redis.Nil 错误（表示 key 不存在）
//
// @author: [piexlmax](https://github.com/piexlmax)
// @function: GetRedisJWT
// @description: 从redis取jwt
// @param: userName string
// @return: redisJWT string, err error
func (jwtService *JwtService) GetRedisJWT(userName string) (redisJWT string, err error) {
	// 从 Redis 中获取指定用户的 JWT token
	// 使用 context.Background() 作为上下文，适用于简单的查询操作
	// Get 方法返回 *StringCmd，需要调用 Result() 获取实际值
	// Result() 返回 (string, error)，如果 key 不存在，error 为 redis.Nil
	redisJWT, err = global.GVA_REDIS.Get(context.Background(), userName).Result()
	return redisJWT, err
}

// LoadAll 从数据库加载所有 JWT 黑名单到本地缓存
//
// 【功能说明】
// 在系统启动时，从数据库加载所有 JWT 黑名单记录到本地缓存
// 这是系统初始化的关键步骤，确保缓存与数据库数据一致
//
// 【为什么需要这个函数？】
// 1. 服务重启恢复：
//   - 服务重启后，本地缓存会清空，但数据库中的数据仍然存在
//   - 需要从数据库恢复缓存数据，确保黑名单功能正常工作
//   - 避免服务重启后，已拉黑的 token 重新生效
//
// 2. 数据一致性：
//   - 确保缓存与数据库数据一致
//   - 如果数据库中有黑名单记录，缓存中也应该有
//   - 避免因为缓存丢失导致的安全漏洞
//
// 3. 性能优化：
//   - 启动时一次性加载，避免运行时频繁查询数据库
//   - 后续的 JWT 验证可以直接查询缓存，无需访问数据库
//
// 【调用时机】
// 在系统启动时调用，通常在以下位置：
// - main.go 中的系统初始化流程
// - server/core/server.go 中的 RunServer 函数
// - 确保在服务启动前完成数据加载
//
// 【执行流程】
// 1. 从数据库查询所有 JWT 黑名单记录（只查询 jwt 字段）
// 2. 如果查询失败，记录错误日志并返回（不中断服务启动）
// 3. 遍历查询结果，将每个 JWT token 加入本地缓存
//
// 【为什么只查询 jwt 字段？】
// 1. 性能优化：
//   - 使用 Select("jwt") 只查询需要的字段，减少数据传输量
//   - 黑名单只需要 token 字符串，不需要其他字段（ID、时间戳等）
//   - 减少内存占用和网络传输时间
//
// 2. 简化处理：
//   - 只需要 token 字符串即可，不需要完整的模型对象
//   - 使用 []string 而不是 []JwtBlacklist，更简洁高效
//
// 【为什么查询失败不中断服务？】
// 1. 容错设计：
//   - 数据库可能暂时不可用（网络问题、数据库重启等）
//   - 不应该因为黑名单加载失败而阻止服务启动
//   - 服务可以先启动，黑名单功能暂时不可用，但不影响其他功能
//
// 2. 降级策略：
//   - 即使缓存加载失败，后续的 JsonInBlacklist 仍然可以正常工作
//   - 新的黑名单记录会同时写入数据库和缓存
//   - 只是已存在的黑名单记录暂时不在缓存中，但数据库查询仍然可以工作
//
// 3. 可恢复性：
//   - 数据库恢复后，可以手动调用此函数重新加载
//   - 或者等待服务重启，自动重新加载
//
// 【为什么使用 for 循环而不是 range？】
// 这是一个代码风格问题，两种方式都可以：
// 1. 使用 for i := 0; i < len(data); i++：
//   - 传统 C 风格，更明确地表达索引访问
//   - 适合需要索引的场景
//
// 2. 使用 for _, jwt := range data：
//   - Go 惯用法，更简洁
//   - 不需要索引时，这种方式更推荐
//
// 注意：当前代码使用第一种方式，可以改为 range 方式更符合 Go 习惯
//
// 【性能考虑】
// 1. 批量加载：
//   - 一次性加载所有记录，避免多次数据库查询
//   - 减少数据库连接开销
//
// 2. 内存占用：
//   - 黑名单数据量通常不大（只有主动拉黑的 token）
//   - 即使有大量记录，内存占用也相对较小（每个 token 只是一个字符串）
//
// 3. 启动时间：
//   - 如果黑名单记录很多，可能影响启动时间
//   - 可以考虑异步加载，不阻塞服务启动
//
// 【注意事项】
// - 此函数在系统启动时调用，确保缓存数据与数据库一致
// - 如果数据库查询失败，只记录错误，不中断服务启动
// - 缓存会自动过期，过期时间与 JWT 过期时间一致
// - 已过期的 token 会自动从缓存中清除，无需手动清理
func LoadAll() {
	// 定义字符串切片，用于存储从数据库查询的 JWT token 列表
	var data []string

	// 从数据库查询所有 JWT 黑名单记录
	// 使用 Model 指定模型，Select 只查询 jwt 字段，Find 查询所有记录
	// 为什么使用 Select("jwt")？
	// - 只查询需要的字段，减少数据传输量和内存占用
	// - 黑名单只需要 token 字符串，不需要其他字段
	err := global.GVA_DB.Model(&system.JwtBlacklist{}).Select("jwt").Find(&data).Error
	if err != nil {
		// 数据库查询失败，记录错误日志
		// 为什么不 panic 或返回错误？
		// - 这是初始化函数，不应该因为黑名单加载失败而阻止服务启动
		// - 记录错误日志，让运维人员知道问题，但服务可以继续启动
		// - 降级策略：即使加载失败，新的黑名单记录仍然可以正常工作
		global.GVA_LOG.Error("加载数据库jwt黑名单失败!", zap.Error(err))
		return
	}

	// 遍历查询结果，将每个 JWT token 加入本地缓存
	// 使用 SetDefault 方法，使用缓存的默认过期时间（与 JWT 过期时间一致）
	// 为什么使用 struct{}{} 作为值？
	// - 只需要知道 token 是否在黑名单中，不需要存储额外信息
	// - struct{}{} 是 0 字节，最节省内存
	// - 实现 Set 数据结构的效果：只关心 key 是否存在
	//
	// 注意：可以使用 range 方式更符合 Go 习惯：
	// for _, jwt := range data {
	//     global.BlackCache.SetDefault(jwt, struct{}{})
	// }
	for i := 0; i < len(data); i++ {
		global.BlackCache.SetDefault(data[i], struct{}{})
	} // jwt黑名单 加入 BlackCache 中
}
