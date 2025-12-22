// Package timer 提供了一个线程安全的定时任务管理器
// 设计思路：
// 1. 使用接口抽象，便于测试和扩展（符合依赖倒置原则）
// 2. 支持多个独立的 cron 实例，通过 cronName 区分，实现任务分组管理
// 3. 每个 cron 实例管理多个任务，提供灵活的任务生命周期管理
// 4. 使用互斥锁保证并发安全，所有操作都是线程安全的
// 5. 支持标准 cron 格式和秒级精度两种模式，满足不同场景需求
package timer

import (
	"sync"

	"github.com/robfig/cron/v3"
)

// Timer 定时任务管理器接口
// 为什么使用接口设计：
// 1. 依赖倒置：调用方依赖接口而非具体实现，降低耦合度
// 2. 易于测试：可以轻松创建 mock 实现进行单元测试
// 3. 便于扩展：未来可以替换不同的实现（如分布式定时任务）
// 4. 符合 Go 的接口设计哲学：接口应该小而专注
type Timer interface {
	// FindCronList 获取所有 cron 实例列表
	// 返回值：map 的 key 是 cronName，value 是对应的任务管理器
	// 用途：用于监控、调试或批量操作所有定时任务
	FindCronList() map[string]*taskManager

	// AddTaskByFuncWithSecond 通过函数方式添加支持秒级精度的定时任务
	// 为什么提供秒级精度版本：
	// 1. 标准 cron 格式最小单位是分钟，无法满足秒级任务需求
	// 2. 通过 WithSeconds() 选项启用秒级支持，格式：秒 分 时 日 月 周
	// 3. 适用于需要精确到秒的定时任务（如每 30 秒执行一次）
	AddTaskByFuncWithSecond(cronName string, spec string, fun func(), taskName string, option ...cron.Option) (cron.EntryID, error)

	// AddTaskByJobWithSeconds 通过接口方式添加支持秒级精度的定时任务
	// 为什么提供接口版本：
	// 1. 接口方式更适合复杂任务，可以封装状态和方法
	// 2. 便于实现任务的重用和组合
	// 3. 符合面向对象设计，任务可以是一个完整的对象
	AddTaskByJobWithSeconds(cronName string, spec string, job interface{ Run() }, taskName string, option ...cron.Option) (cron.EntryID, error)

	// AddTaskByFunc 通过函数方式添加标准 cron 格式的定时任务
	// 为什么提供函数版本：
	// 1. 简单直接，适合简单的任务逻辑
	// 2. 无需定义结构体，代码更简洁
	// 3. 标准 cron 格式：分 时 日 月 周（如 "0 0 * * *" 表示每天 0 点）
	AddTaskByFunc(cronName string, spec string, task func(), taskName string, option ...cron.Option) (cron.EntryID, error)

	// AddTaskByJob 通过接口方式添加标准 cron 格式的定时任务
	// 为什么同时提供函数和接口两种方式：
	// 1. 函数方式：简单快速，适合一次性任务
	// 2. 接口方式：更灵活，可以封装复杂逻辑和状态
	// 3. 接口方式便于实现任务的可测试性和可维护性
	// 参数 job 需要实现 Run() 方法
	AddTaskByJob(cronName string, spec string, job interface{ Run() }, taskName string, option ...cron.Option) (cron.EntryID, error)

	// FindCron 根据 cronName 查找对应的 cron 实例
	// 返回值：任务管理器和是否存在
	// 用途：用于检查 cron 是否存在，或获取 cron 实例进行操作
	FindCron(cronName string) (*taskManager, bool)

	// StartCron 启动指定名称的 cron 实例
	// 为什么需要手动启动：
	// 1. 允许延迟启动，可以先添加所有任务再统一启动
	// 2. 支持暂停后恢复执行
	// 3. 提供更灵活的任务控制
	StartCron(cronName string)

	// StopCron 停止指定名称的 cron 实例
	// 用途：暂停任务执行但不删除任务，可以后续通过 StartCron 恢复
	StopCron(cronName string)

	// FindTask 在指定 cron 实例中查找指定名称的任务
	// 为什么需要任务名称：
	// 1. EntryID 是内部标识，不便于记忆和使用
	// 2. 任务名称是业务层面的标识，更直观
	// 3. 便于通过业务名称管理任务
	FindTask(cronName string, taskName string) (*task, bool)

	// RemoveTask 根据 EntryID 删除指定任务
	// 为什么同时提供 ID 和名称两种删除方式：
	// 1. ID 方式：性能更好，直接定位
	// 2. 名称方式：更符合业务使用习惯
	RemoveTask(cronName string, id int)

	// RemoveTaskByName 根据任务名称删除指定任务
	// 实现方式：先通过名称查找任务，再调用 RemoveTask
	// 好处：提供更友好的 API，用户无需关心内部 ID
	RemoveTaskByName(cronName string, taskName string)

	// Clear 清除指定名称的整个 cron 实例及其所有任务
	// 与 RemoveTask 的区别：
	// 1. RemoveTask：删除单个任务
	// 2. Clear：删除整个 cron 实例（包括所有任务）
	// 用途：用于清理不再需要的任务组
	Clear(cronName string)

	// Close 关闭所有 cron 实例，释放资源
	// 为什么需要显式关闭：
	// 1. 优雅关闭：确保所有任务正常停止
	// 2. 资源清理：释放 goroutine 等资源
	// 3. 通常在应用退出时调用
	Close()

	// Reset 清空所有定时器并关闭所有定时任务
	// 与 Close 的区别：
	// 1. Close：只停止所有 cron 实例，但不删除，适用于应用退出场景
	// 2. Reset：停止所有 cron 实例并清空 cronList，将管理器重置为初始状态
	// 用途：
	// 1. 重置管理器：清空所有定时任务，重新开始
	// 2. 测试场景：测试前清理所有任务
	// 3. 动态重新配置：需要完全清除旧配置后重新添加任务
	// 注意：Reset 后所有任务都被清空，无法恢复
	Reset()
}

// task 表示一个定时任务的元数据
// 设计原因：
// 1. EntryID：cron 库返回的任务 ID，用于删除任务
// 2. Spec：cron 表达式，记录任务的执行规则（便于查询和调试）
// 3. TaskName：业务层面的任务名称，便于识别和管理
// 好处：将任务信息封装在一起，便于管理和查询
type task struct {
	EntryID  cron.EntryID // cron 库返回的任务唯一标识
	Spec     string       // cron 表达式，如 "0 0 * * *" 或 "*/30 * * * * *"（秒级）
	TaskName string       // 业务层面的任务名称，便于识别
}

// taskManager 管理一个 cron 实例及其所有任务
// 设计原因：
// 1. corn：底层的 cron 实例，负责实际的任务调度
// 2. tasks：任务映射表，通过 EntryID 快速查找任务元数据
// 好处：
// 1. 将 cron 实例和任务元数据绑定，便于统一管理
// 2. 支持通过 EntryID 快速查找任务信息
// 3. 实现任务的分组管理（每个 taskManager 是一个任务组）
type taskManager struct {
	corn  *cron.Cron             // 底层的 cron 调度器实例
	tasks map[cron.EntryID]*task // 任务映射表，key 是 EntryID，value 是任务元数据
}

// timer 定时任务管理器，实现了 Timer 接口
// 设计要点：
//  1. cronList：使用 map 存储多个 cron 实例，key 是 cronName
//     好处：支持多个独立的任务组，互不干扰，便于分类管理
//  2. sync.Mutex：嵌入互斥锁，保证并发安全
//     好处：所有操作都是线程安全的，可以在多 goroutine 环境下安全使用
//  3. 懒加载：cron 实例在首次添加任务时创建
//     好处：节省资源，只创建实际使用的 cron 实例
type timer struct {
	cronList   map[string]*taskManager // 多个 cron 实例的映射表，key 是 cronName
	sync.Mutex                         // 互斥锁，保证并发安全
}

// AddTaskByFunc 通过函数方式添加标准 cron 格式的定时任务
// 实现细节：
//  1. 使用互斥锁保证并发安全（所有操作都在锁保护下进行）
//  2. 懒加载：如果 cronName 对应的 cron 实例不存在，则创建新的实例
//     好处：按需创建，节省资源
//  3. 自动启动：添加任务后立即启动 cron 实例
//     好处：任务添加后立即可用，无需手动启动
//  4. 记录任务元数据：将任务信息保存到 tasks map 中
//     好处：便于后续查询、删除和管理任务
//
// 参数说明：
// - cronName: cron 实例名称，用于分组管理任务
// - spec: cron 表达式，标准格式为 "分 时 日 月 周"（如 "0 0 * * *" 表示每天 0 点）
// - fun: 要执行的任务函数
// - taskName: 任务名称，用于业务识别
// - option: cron 选项，可以自定义 cron 行为（如日志、时区等）
func (t *timer) AddTaskByFunc(cronName string, spec string, fun func(), taskName string, option ...cron.Option) (cron.EntryID, error) {
	t.Lock()         // 加锁，保证并发安全
	defer t.Unlock() // 确保函数返回时释放锁

	// 懒加载：如果 cron 实例不存在则创建
	// 好处：避免预先创建不需要的 cron 实例，节省资源
	if _, ok := t.cronList[cronName]; !ok {
		tasks := make(map[cron.EntryID]*task)
		t.cronList[cronName] = &taskManager{
			corn:  cron.New(option...), // 创建新的 cron 实例，传入自定义选项
			tasks: tasks,               // 初始化任务映射表
		}
	}

	// 添加任务到 cron 实例
	id, err := t.cronList[cronName].corn.AddFunc(spec, fun)
	if err != nil {
		return id, err
	}

	// 自动启动 cron 实例（如果还未启动）
	// 好处：任务添加后立即可用，无需手动调用 StartCron
	t.cronList[cronName].corn.Start()

	// 保存任务元数据到映射表
	// 好处：便于后续通过 EntryID 或 TaskName 查找和管理任务
	t.cronList[cronName].tasks[id] = &task{
		EntryID:  id,
		Spec:     spec,
		TaskName: taskName,
	}
	return id, err
}

// AddTaskByFuncWithSecond 通过函数方式添加支持秒级精度的定时任务
// 与 AddTaskByFunc 的区别：
//  1. 自动添加 cron.WithSeconds() 选项，启用秒级精度
//  2. cron 表达式格式变为：秒 分 时 日 月 周（6 个字段）
//     例如："*/30 * * * * *" 表示每 30 秒执行一次
//  3. 适用于需要精确到秒的定时任务场景
//
// 实现细节：
// - 在用户提供的 option 基础上追加 WithSeconds() 选项
// - 其他逻辑与 AddTaskByFunc 相同
func (t *timer) AddTaskByFuncWithSecond(cronName string, spec string, fun func(), taskName string, option ...cron.Option) (cron.EntryID, error) {
	t.Lock()
	defer t.Unlock()

	// 追加 WithSeconds() 选项，启用秒级精度支持
	// 好处：用户无需手动添加此选项，简化 API 使用
	option = append(option, cron.WithSeconds())

	// 懒加载 cron 实例
	if _, ok := t.cronList[cronName]; !ok {
		tasks := make(map[cron.EntryID]*task)
		t.cronList[cronName] = &taskManager{
			corn:  cron.New(option...), // 创建支持秒级精度的 cron 实例
			tasks: tasks,
		}
	}

	id, err := t.cronList[cronName].corn.AddFunc(spec, fun)
	if err != nil {
		return id, err
	}

	t.cronList[cronName].corn.Start()
	t.cronList[cronName].tasks[id] = &task{
		EntryID:  id,
		Spec:     spec,
		TaskName: taskName,
	}
	return id, err
}

// AddTaskByJob 通过接口方式添加标准 cron 格式的定时任务
// 与 AddTaskByFunc 的区别：
// 1. 使用 AddJob 而不是 AddFunc，接受实现了 Run() 方法的接口
// 2. 接口方式更适合复杂任务，可以封装状态、依赖注入等
// 3. 便于实现任务的可测试性（可以 mock 接口）
// 使用场景：
// - 任务需要访问外部依赖（如数据库、HTTP 客户端等）
// - 任务需要维护内部状态
// - 需要更好的代码组织和可测试性
// 参数 job 必须实现 Run() 方法，这是 cron 库的要求
func (t *timer) AddTaskByJob(cronName string, spec string, job interface{ Run() }, taskName string, option ...cron.Option) (cron.EntryID, error) {
	t.Lock()
	defer t.Unlock()

	// 懒加载 cron 实例
	if _, ok := t.cronList[cronName]; !ok {
		tasks := make(map[cron.EntryID]*task)
		t.cronList[cronName] = &taskManager{
			corn:  cron.New(option...),
			tasks: tasks,
		}
	}

	// 使用 AddJob 添加任务（接口方式）
	// 好处：job 可以是一个完整的对象，包含状态和方法
	id, err := t.cronList[cronName].corn.AddJob(spec, job)
	if err != nil {
		return id, err
	}

	t.cronList[cronName].corn.Start()
	t.cronList[cronName].tasks[id] = &task{
		EntryID:  id,
		Spec:     spec,
		TaskName: taskName,
	}
	return id, err
}

// AddTaskByJobWithSeconds 通过接口方式添加支持秒级精度的定时任务
// 结合了 AddTaskByJob 和 AddTaskByFuncWithSecond 的特点：
// 1. 使用接口方式（便于封装复杂逻辑）
// 2. 支持秒级精度（满足精确调度需求）
// 适用场景：需要秒级精度的复杂任务
func (t *timer) AddTaskByJobWithSeconds(cronName string, spec string, job interface{ Run() }, taskName string, option ...cron.Option) (cron.EntryID, error) {
	t.Lock()
	defer t.Unlock()

	// 启用秒级精度支持
	option = append(option, cron.WithSeconds())

	// 懒加载 cron 实例
	if _, ok := t.cronList[cronName]; !ok {
		tasks := make(map[cron.EntryID]*task)
		t.cronList[cronName] = &taskManager{
			corn:  cron.New(option...),
			tasks: tasks,
		}
	}

	id, err := t.cronList[cronName].corn.AddJob(spec, job)
	if err != nil {
		return id, err
	}

	t.cronList[cronName].corn.Start()
	t.cronList[cronName].tasks[id] = &task{
		EntryID:  id,
		Spec:     spec,
		TaskName: taskName,
	}
	return id, err
}

// FindCron 根据 cronName 查找对应的 cron 实例
// 返回值：
// - *taskManager: cron 实例的任务管理器，如果不存在则为 nil
// - bool: 是否存在该 cron 实例
// 用途：检查 cron 是否存在，或获取 cron 实例进行进一步操作
// 线程安全：使用互斥锁保护，保证并发安全
func (t *timer) FindCron(cronName string) (*taskManager, bool) {
	t.Lock()
	defer t.Unlock()
	v, ok := t.cronList[cronName]
	return v, ok
}

// FindTask 在指定 cron 实例中查找指定名称的任务
// 实现方式：
// 1. 先查找 cron 实例是否存在
// 2. 如果存在，遍历该 cron 实例的所有任务，通过 TaskName 匹配
// 为什么需要遍历：
// - tasks map 的 key 是 EntryID，不是 TaskName
// - 需要通过 TaskName 查找，所以需要遍历
// 性能考虑：如果任务数量很大，可以考虑在 taskManager 中维护 TaskName 到 EntryID 的映射
// 返回值：
// - *task: 任务元数据，如果不存在则为 nil
// - bool: 是否找到任务
func (t *timer) FindTask(cronName string, taskName string) (*task, bool) {
	t.Lock()
	defer t.Unlock()

	// 先检查 cron 实例是否存在
	v, ok := t.cronList[cronName]
	if !ok {
		return nil, ok
	}

	// 遍历任务列表，通过 TaskName 查找
	// 注意：这是 O(n) 操作，如果任务很多可以考虑优化
	for _, t2 := range v.tasks {
		if t2.TaskName == taskName {
			return t2, true
		}
	}
	return nil, false
}

// FindCronList 获取所有 cron 实例的列表
// 返回值：返回整个 cronList map 的引用
// 注意：返回的是 map 的引用，调用方应该只读，不要修改
// 用途：用于监控、调试、批量操作等场景
// 线程安全：在锁保护下返回，但返回后对 map 的并发访问需要调用方自己保证
func (t *timer) FindCronList() map[string]*taskManager {
	t.Lock()
	defer t.Unlock()
	return t.cronList
}

// StartCron 启动指定名称的 cron 实例
// 用途：
// 1. 恢复之前停止的 cron 实例
// 2. 延迟启动：先添加所有任务，再统一启动
// 3. 动态控制：根据业务需求启动/停止不同的任务组
// 实现：如果 cron 实例存在则调用其 Start() 方法
// 注意：如果 cron 已经在运行，重复调用 Start() 是安全的（幂等操作）
func (t *timer) StartCron(cronName string) {
	t.Lock()
	defer t.Unlock()
	if v, ok := t.cronList[cronName]; ok {
		v.corn.Start() // cron 库的 Start() 方法是幂等的，可以安全重复调用
	}
}

// StopCron 停止指定名称的 cron 实例
// 与 RemoveTask 的区别：
// 1. StopCron：停止执行但不删除任务，可以后续恢复
// 2. RemoveTask：永久删除任务，无法恢复
// 用途：临时暂停任务组，如系统维护、资源紧张等场景
// 注意：停止后任务仍然存在，可以通过 StartCron 恢复执行
func (t *timer) StopCron(cronName string) {
	t.Lock()
	defer t.Unlock()
	if v, ok := t.cronList[cronName]; ok {
		v.corn.Stop() // 停止 cron 实例，但保留任务定义
	}
}

// RemoveTask 根据 EntryID 从指定 cron 实例中删除任务
// 实现步骤：
// 1. 从 cron 实例中移除任务（停止调度）
// 2. 从 tasks map 中删除任务元数据
// 为什么需要两步：
// - 第一步：停止 cron 库对该任务的调度
// - 第二步：清理任务元数据，释放内存
// 注意：删除后任务无法恢复，如需恢复请使用 StopCron
func (t *timer) RemoveTask(cronName string, id int) {
	t.Lock()
	defer t.Unlock()
	if v, ok := t.cronList[cronName]; ok {
		// 从 cron 实例中移除任务（停止调度）
		v.corn.Remove(cron.EntryID(id))
		// 从任务映射表中删除元数据
		delete(v.tasks, cron.EntryID(id))
	}
}

// RemoveTaskByName 根据任务名称删除指定任务
// 实现方式：
// 1. 先通过 FindTask 查找任务（通过名称）
// 2. 如果找到，获取其 EntryID
// 3. 调用 RemoveTask 删除任务
// 为什么需要这个封装：
// - 用户更习惯使用任务名称而不是 EntryID
// - 提供更友好的 API
// - 隐藏内部实现细节（EntryID）
// 注意：FindTask 内部会加锁，RemoveTask 也会加锁，但这是合理的（细粒度锁）
func (t *timer) RemoveTaskByName(cronName string, taskName string) {
	fTask, ok := t.FindTask(cronName, taskName)
	if !ok {
		return // 任务不存在，直接返回
	}
	// 通过 EntryID 删除任务
	t.RemoveTask(cronName, int(fTask.EntryID))
}

// Clear 清除指定名称的整个 cron 实例及其所有任务
// 与 RemoveTask 的区别：
// 1. RemoveTask：删除单个任务
// 2. Clear：删除整个 cron 实例（包括所有任务）
// 实现步骤：
// 1. 停止 cron 实例（停止所有任务的调度）
// 2. 从 cronList 中删除该 cron 实例
// 用途：
// - 清理不再需要的任务组
// - 应用重启前的资源清理
// - 动态管理任务组（创建和销毁）
// 注意：删除后无法恢复，如需恢复请使用 StopCron
func (t *timer) Clear(cronName string) {
	t.Lock()
	defer t.Unlock()
	if v, ok := t.cronList[cronName]; ok {
		v.corn.Stop()                // 先停止 cron 实例
		delete(t.cronList, cronName) // 从列表中删除，GC 会自动回收内存
	}
}

// Close 关闭所有 cron 实例，释放资源
// 用途：
// 1. 应用优雅关闭：确保所有任务正常停止
// 2. 资源清理：释放 cron 实例占用的 goroutine 等资源
// 3. 通常在应用退出时调用（如收到 SIGTERM 信号）
// 实现：
// - 遍历所有 cron 实例并停止它们
// - 注意：这里只停止不删除，因为应用即将退出
// 为什么需要显式关闭：
// - cron 实例内部会启动 goroutine，需要显式停止
// - 确保任务不会在应用退出时仍在执行
// - 避免资源泄漏
func (t *timer) Close() {
	t.Lock()
	defer t.Unlock()
	// 遍历所有 cron 实例并停止
	for _, v := range t.cronList {
		v.corn.Stop() // 停止每个 cron 实例
	}
	// 注意：这里不清空 cronList，因为应用即将退出，GC 会自动回收
}

// Reset 清空所有定时器并关闭所有定时任务
// 与 Close 的区别：
// 1. Close：只停止所有 cron 实例，但不删除，适用于应用退出场景
// 2. Reset：停止所有 cron 实例并清空 cronList，将管理器重置为初始状态
// 实现步骤：
// 1. 遍历所有 cron 实例并停止它们（关闭所有定时任务）
// 2. 清空 cronList map（清空所有定时器）
// 用途：
// 1. 重置管理器：清空所有定时任务，重新开始
// 2. 测试场景：测试前清理所有任务
// 3. 动态重新配置：需要完全清除旧配置后重新添加任务
// 注意：
// - Reset 后所有任务都被清空，无法恢复
// - 如果需要保留任务定义只暂停执行，应该使用 StopCron
// - Reset 会将管理器重置为初始状态，之后可以重新添加任务
// 线程安全：使用互斥锁保护，保证并发安全
func (t *timer) Reset() {
	t.Lock()
	defer t.Unlock()
	// 遍历所有 cron 实例并停止（关闭所有定时任务）
	for _, v := range t.cronList {
		v.corn.Stop() // 停止每个 cron 实例
	}
	// 清空 cronList，清空所有定时器
	// 使用 make 重新创建 map，GC 会自动回收旧的 map 和 taskManager
	t.cronList = make(map[string]*taskManager)
}

// NewTimerTask 创建并返回一个新的定时任务管理器实例
// 设计模式：工厂函数
// 好处：
// 1. 返回接口类型而非具体类型，符合依赖倒置原则
// 2. 隐藏具体实现，调用方只需关心接口
// 3. 便于后续替换实现（如分布式定时任务）
// 初始化：
// - 创建空的 cronList map，使用懒加载策略创建 cron 实例
// - 互斥锁使用零值初始化即可（sync.Mutex 的零值是有效的）
func NewTimerTask() Timer {
	return &timer{cronList: make(map[string]*taskManager)}
}
