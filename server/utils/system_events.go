package utils

import (
	"sync"
)

// SystemEvents 定义系统级事件处理
//
// 设计说明：
// 1. 采用观察者模式（Observer Pattern），实现系统配置/资源重载的统一管理
// 2. 各模块可以独立注册自己的重载逻辑，实现解耦和可扩展性
// 3. 当系统需要重载时（如配置变更、热更新等），统一触发所有注册的处理器
//
// 使用场景：
// - 配置文件热重载
// - 数据库连接池重连
// - 缓存策略更新
// - 路由规则刷新
type SystemEvents struct {
	// reloadHandlers 存储所有注册的重载处理函数
	// 使用函数切片而非接口，减少类型转换开销，提高性能
	// 函数签名 func() error 统一错误处理，便于错误传播和中断
	reloadHandlers []func() error

	// mu 读写互斥锁（RWMutex），而非普通互斥锁（Mutex）
	// 设计原因：
	// - RegisterReloadHandler 需要写锁（修改切片）
	// - TriggerReload 只需要读锁（读取切片），允许多个重载操作并发执行
	// - 读写锁在读多写少的场景下性能优于互斥锁
	// - 如果使用 Mutex，所有操作都会串行化，影响并发性能
	mu sync.RWMutex
}

// GlobalSystemEvents 全局事件管理器单例
//
// 为什么使用全局单例：
// 1. 系统级事件管理器应该是全局唯一的，避免多个实例导致状态不一致
// 2. 方便各个模块直接访问，无需传递依赖
// 3. 简化使用方式：utils.GlobalSystemEvents.RegisterReloadHandler(...)
// 4. 在 init() 函数或启动阶段注册，运行时触发，符合事件驱动架构
var GlobalSystemEvents = &SystemEvents{}

// RegisterReloadHandler 注册系统重载处理函数
//
// 参数说明：
//
//	handler: 重载处理函数，当系统触发重载时会调用此函数
//
// 设计要点：
// 1. 使用写锁（Lock）保护切片追加操作，确保并发安全
// 2. defer 确保锁一定会释放，避免死锁
// 3. append 操作是线程安全的（在锁保护下），但切片本身不是线程安全的
// 4. 支持动态注册，系统启动后仍可添加新的处理器
func (e *SystemEvents) RegisterReloadHandler(handler func() error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.reloadHandlers = append(e.reloadHandlers, handler)
}

// TriggerReload 触发所有注册的重载处理函数
//
// 返回值：
//
//	error: 如果任何处理器返回错误，立即返回该错误，中断后续处理
//
// 设计要点：
//  1. 使用读锁（RLock）保护读取操作，允许多个 TriggerReload 并发执行
//     这对于高并发场景下的配置热重载很有意义
//  2. 顺序执行所有处理器，保证重载的顺序性（如：先重载配置，再重载数据库连接）
//  3. 错误快速失败（fail-fast）：一旦某个处理器失败，立即返回，不继续执行后续处理器
//     这避免了部分重载成功、部分失败导致的不一致状态
//  4. 如果所有处理器都成功，返回 nil
//
// 性能考虑：
// - 读锁允许并发读取，提高并发性能
// - 如果使用写锁，多个重载请求会串行化，影响响应速度
func (e *SystemEvents) TriggerReload() error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, handler := range e.reloadHandlers {
		if err := handler(); err != nil {
			return err
		}
	}
	return nil
}
