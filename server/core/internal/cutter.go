package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Cutter 实现 io.Writer 接口，用于日志切割功能
// 日志文件路径格式: strings.Join([]string{director, layout, formats..., level+".log"}, os.PathSeparator)
//
// 设计说明:
// 1. 实现 io.Writer 接口，可以无缝集成到 zap 等日志库中
// 2. 每次写入时动态计算文件路径，支持按时间自动切割日志文件
// 3. 使用读写锁保证并发安全，支持多 goroutine 同时写入
// 4. 每次写入后关闭文件句柄，避免长时间持有文件锁，便于日志轮转和清理
type Cutter struct {
	level        string        // 日志级别(debug, info, warn, error, dpanic, panic, fatal)
	layout       string        // 时间格式，如 "2006-01-02" 或 "2006-01-02 15:04:05"，用于按时间切割日志
	formats      []string      // 自定义路径参数，如 []string{"business"}，用于在日志路径中添加业务标识
	director     string        // 日志根目录
	retentionDay int           // 日志保留天数，超过此天数的日志目录会被自动清理
	file         *os.File      // 当前打开的文件句柄，每次写入后关闭，下次写入时重新打开
	mutex        *sync.RWMutex // 读写锁，保证并发写入时的线程安全
}

// CutterOption 选项函数类型，采用函数式选项模式（Functional Options Pattern）
//
// 设计优势:
// 1. 扩展性好：新增配置项时无需修改 NewCutter 函数签名，只需添加新的选项函数
// 2. 可读性强：调用时语义清晰，如 NewCutter(..., CutterWithLayout("2006-01-02"))
// 3. 向后兼容：新增可选参数不会破坏现有代码
// 4. 灵活配置：可以选择性地设置需要的配置项
type CutterOption func(*Cutter)

// CutterWithLayout 设置日志文件路径中的时间格式
//
// 为什么这样设计:
// - 通过选项模式，将时间格式作为可选参数，而不是必填参数
// - 如果不需要按时间切割，可以不设置 layout，日志将直接写入根目录
// - 支持灵活的时间格式，如 "2006-01-02"（按天）或 "2006-01-02 15"（按小时）
func CutterWithLayout(layout string) CutterOption {
	return func(c *Cutter) {
		c.layout = layout
	}
}

// CutterWithFormats 设置日志路径中的自定义格式化参数
//
// 为什么使用可变参数:
// - 支持多个自定义路径段，如 ["business", "module"] 可以生成 logs/2024-01-01/business/module/info.log
// - 如果不需要自定义路径，可以不传此选项
// - 检查长度避免空切片，提高代码健壮性
func CutterWithFormats(format ...string) CutterOption {
	return func(c *Cutter) {
		if len(format) > 0 {
			c.formats = format
		}
	}
}

// NewCutter 创建新的日志切割器实例
//
// 设计说明:
// 1. director、level、retentionDay 作为必填参数，因为这些是核心功能必需的
// 2. options 作为可变参数，使用选项模式处理可选配置
// 3. 初始化时创建读写锁，避免后续使用时再创建，保证线程安全
//
// 为什么使用选项模式而不是结构体参数:
// - 如果使用结构体参数，调用时需要创建结构体，代码冗长
// - 选项模式支持链式调用，代码更简洁优雅
// - 未来扩展时不会导致函数签名膨胀
func NewCutter(director string, level string, retentionDay int, options ...CutterOption) *Cutter {
	rotate := &Cutter{
		level:        level,
		director:     director,
		retentionDay: retentionDay,
		mutex:        new(sync.RWMutex),
	}
	// 应用所有选项函数，实现配置的灵活组合
	for i := 0; i < len(options); i++ {
		options[i](rotate)
	}
	return rotate
}

// Write 实现 io.Writer 接口，将日志内容写入文件
//
// ========== 函数详细说明 ==========
//
// 这个函数是整个日志切割器的核心，负责将日志内容写入到文件中。
// 它的设计非常巧妙：每次写入时都会重新计算文件路径并打开文件，写入后立即关闭。
//
// 【执行流程详解】
//
// 步骤1：加锁保护（第108行）
//   - 使用互斥锁（mutex.Lock()）确保同一时刻只有一个 goroutine 在执行写入
//   - 为什么需要锁？因为多个 goroutine 可能同时调用 Write，不加锁会导致：
//   - 文件句柄竞争：多个 goroutine 同时操作同一个文件句柄
//   - 数据竞争：同时修改 c.file 字段
//   - 路径计算错误：在构建路径时被其他 goroutine 打断
//
// 步骤2：延迟释放资源（第111-117行）
//   - 使用 defer 确保无论函数如何退出（正常返回或发生 panic），都会执行清理
//   - 关闭文件句柄：释放系统资源，避免文件描述符泄漏
//   - 清空 file 字段：避免悬空引用，防止后续误用已关闭的文件
//   - 释放锁：让其他等待的 goroutine 可以继续执行
//
// 步骤3：构建日志文件路径（第119-141行）
//
//	这是实现"自动日志切割"的关键步骤！
//
//	3.1 预分配切片（第124行）
//	    - 提前计算好需要的容量，避免多次内存重新分配
//	    - 容量 = 根目录(1) + 时间目录(0或1) + 自定义格式(长度) + 文件名(1)
//
//	3.2 组装路径组件（第125-136行）
//	    - 添加根目录：如 "logs"
//	    - 添加时间目录（如果设置了 layout）：如 "2024-01-01"
//	      * 这里每次都用 time.Now()，所以跨天时会自动切换到新目录！
//	    - 添加自定义格式：如 "business"、"module" 等业务标识
//	    - 添加文件名：日志级别 + ".log"，如 "info.log"
//
//	3.3 合并路径（第141行）
//	    - 使用 filepath.Join 而不是字符串拼接
//	    - 自动处理不同操作系统的路径分隔符（Windows 用 \，Linux/Mac 用 /）
//	    - 最终路径示例：logs/2024-01-01/business/info.log
//
// 步骤4：创建目录（第142-148行）
//   - 使用 filepath.Dir 获取文件所在目录
//   - 使用 os.MkdirAll 创建目录（如果不存在）
//   - MkdirAll 是幂等的：目录已存在不会报错
//   - os.ModePerm (0777) 表示所有用户都有读写执行权限
//
// 步骤5：清理过期日志（第149-158行）
//   - 在每次写入时清理超过保留天数的日志目录
//   - 为什么不单独启动定时任务？
//   - 简化架构：不需要额外的 goroutine 和定时器
//   - 按需清理：只有写入日志时才清理，不浪费资源
//   - 同步操作：清理和写入同步，保证一致性
//   - 注意：如果日志目录很大，这里可能有性能开销
//
// 步骤6：打开文件（第159-167行）
//   - 使用 os.OpenFile 打开文件，标志位说明：
//   - os.O_CREATE：文件不存在则创建
//   - os.O_APPEND：追加模式，写入内容会自动添加到文件末尾
//   - os.O_WRONLY：只写模式，提高性能（不需要读权限）
//   - 文件权限 0644：所有者可读写，其他人只读
//   - 将文件句柄保存到 c.file，供后续写入使用
//
// 步骤7：写入内容（第168-169行）
//   - 调用 file.Write 将日志内容写入文件
//   - 返回写入的字节数和可能的错误
//   - 写入完成后，defer 中的代码会自动关闭文件
//
// 【为什么这样设计？】
//
// 1. 为什么每次写入都重新打开文件？
//   - 支持自动切割：时间变化时（如跨天），路径会自动变化，自然切换到新文件
//   - 避免文件句柄泄漏：及时释放资源
//   - 便于日志轮转：外部工具可以安全地移动或删除旧日志
//   - 简化状态管理：不需要维护复杂的文件切换逻辑
//
// 2. 为什么使用互斥锁而不是读写锁的读锁？
//   - 因为写入操作会修改共享状态（c.file 字段）
//   - 读锁允许多个 goroutine 同时读取，但这里需要独占写入
//
// 3. 性能影响如何？
//   - 每次打开文件确实有开销，但日志写入频率通常不高（每秒几次到几十次）
//   - 使用 os.O_APPEND 标志，操作系统会优化追加写入
//   - 预分配切片容量，减少内存分配
//   - 对于大多数应用，这个性能开销是可以接受的
//
// 【使用示例】
//
//	cutter := NewCutter(
//	    "logs",           // 根目录
//	    "info",           // 日志级别
//	    7,                // 保留7天
//	    CutterWithLayout("2006-01-02"),           // 按天切割
//	    CutterWithFormats("business", "module"),  // 自定义路径
//	)
//	cutter.Write([]byte("这是一条日志\n"))
//	// 日志会写入到：logs/2024-01-01/business/module/info.log
//
// ==========================================
func (c *Cutter) Write(bytes []byte) (n int, err error) {
	// 获取写锁，保证同一时刻只有一个 goroutine 在执行写入操作
	// 使用写锁而非读锁，因为写入操作会修改共享状态（file 字段）
	c.mutex.Lock()
	// 使用 defer 确保锁一定会被释放，即使发生 panic 也能正确解锁
	// 同时关闭文件句柄，避免资源泄漏
	defer func() {
		if c.file != nil {
			_ = c.file.Close() // 忽略关闭错误，因为已经写入完成
			c.file = nil       // 清空文件句柄，避免悬空引用
		}
		c.mutex.Unlock()
	}()

	// 构建日志文件路径：director/layout/formats.../level.log
	// 例如: logs/2024-01-01/business/info.log
	length := len(c.formats)
	// 预分配切片容量：director(1) + layout(0或1) + formats(length) + level.log(1) = 最多 3+length
	// 预分配可以避免多次内存重新分配，提高性能
	values := make([]string, 0, 3+length)
	values = append(values, c.director)
	// 如果设置了时间格式，将当前时间格式化后加入路径
	// 这样每次写入时都会根据当前时间计算路径，实现自动切割
	if c.layout != "" {
		values = append(values, time.Now().Format(c.layout))
	}
	// 添加自定义格式参数，支持业务自定义路径结构
	for i := 0; i < length; i++ {
		values = append(values, c.formats[i])
	}
	// 添加日志级别和文件扩展名
	values = append(values, c.level+".log")
	// 使用 filepath.Join 而不是字符串拼接，好处：
	// 1. 自动处理不同操作系统的路径分隔符（Windows 用 \，Unix 用 /）
	// 2. 自动处理路径中的多余分隔符
	// 3. 代码更清晰，意图更明确
	filename := filepath.Join(values...)
	director := filepath.Dir(filename)
	// 确保目录存在，os.ModePerm (0777) 表示所有权限
	// 如果目录已存在，MkdirAll 不会报错，这是幂等操作
	err = os.MkdirAll(director, os.ModePerm)
	if err != nil {
		return 0, err
	}
	// 在每次写入时清理过期日志，而不是单独启动定时任务
	// 好处：
	// 1. 简化架构，不需要额外的定时器或 goroutine
	// 2. 清理操作与写入操作同步，保证一致性
	// 3. 如果应用长时间不写入日志，也不会浪费资源清理
	// 注意：这里可能会有性能开销，但日志写入频率通常不高，影响可接受
	defer func() {
		err = removeNDaysFolders(c.director, c.retentionDay)
		if err != nil {
			fmt.Println("清理过期日志失败", err)
		}
	}()

	// 打开文件，使用追加模式（O_APPEND）和只写模式（O_WRONLY）
	// O_CREATE: 如果文件不存在则创建
	// O_APPEND: 追加写入，文件指针自动定位到文件末尾
	// O_WRONLY: 只写模式，提高性能
	// 0644: 文件权限，所有者可读写，其他人只读
	c.file, err = os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return 0, err
	}
	// 写入日志内容，返回写入的字节数和可能的错误
	return c.file.Write(bytes)
}

// Sync 将文件内容同步到磁盘，确保数据持久化
//
// 设计说明:
// 1. 实现类似 io.Writer 的 Sync 方法，供日志库在需要时调用
// 2. 使用互斥锁保护，防止在同步时文件被其他 goroutine 修改
// 3. 检查文件句柄是否存在，因为 Write 方法会在 defer 中关闭文件
//
// 使用场景:
// - 应用退出前需要确保所有日志都已写入磁盘
// - 重要日志需要立即持久化，不能依赖操作系统的缓冲
// - 日志库可能会定期调用 Sync 方法
func (c *Cutter) Sync() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 由于 Write 方法在 defer 中会关闭文件，这里需要检查文件是否还存在
	// 如果文件已关闭，说明当前没有打开的文件，无需同步
	if c.file != nil {
		// Sync 会将文件系统缓冲区的内容强制写入磁盘
		// 这是一个系统调用，可能会有性能开销，但能保证数据不丢失
		return c.file.Sync()
	}
	return nil
}

// removeNDaysFolders 清理指定天数之前的日志目录
//
// 设计思路:
// 1. 小于等于零的值表示不清理，直接返回，避免不必要的文件系统操作
// 2. 基于目录的修改时间判断是否过期，而不是文件内容
// 3. 只删除目录，不删除根目录本身，保护日志根目录结构
//
// 为什么在每次写入时调用而不是定时任务:
// - 简化架构：不需要额外的定时器和 goroutine 管理
// - 按需清理：只有在有日志写入时才清理，避免空转
// - 同步操作：清理与写入同步，保证一致性
//
// 性能考虑:
// - filepath.Walk 会遍历整个目录树，对于大量日志文件可能有性能开销
// - 但日志写入频率通常不高，且清理操作是必要的，这个开销可以接受
// - 如果性能成为瓶颈，可以考虑异步清理或优化清理策略
func removeNDaysFolders(dir string, days int) error {
	// 如果保留天数小于等于0，表示不清理，直接返回
	// 这样可以禁用自动清理功能，由外部工具（如 logrotate）管理日志
	if days <= 0 {
		return nil
	}
	// 计算截止时间：当前时间减去保留天数
	// 例如：保留7天，则删除7天前修改的目录
	cutoff := time.Now().AddDate(0, 0, -days)
	// 使用 filepath.Walk 递归遍历目录树
	// Walk 会访问目录中的每个文件和子目录
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		// 如果访问文件时出错（如权限不足），返回错误
		// 这样可以及时发现和处理文件系统问题
		if err != nil {
			return err
		}
		// 删除条件：
		// 1. 必须是目录（IsDir()）
		// 2. 修改时间早于截止时间（Before(cutoff)）
		// 3. 不是根目录本身（path != dir），保护日志根目录
		//
		// 为什么只删除目录：
		// - 日志文件通常按时间组织在目录中，删除整个目录更高效
		// - 避免逐个删除文件，减少系统调用次数
		// - 保持目录结构的整洁
		if info.IsDir() && info.ModTime().Before(cutoff) && path != dir {
			// RemoveAll 会递归删除目录及其所有内容
			// 这是一个危险操作，但在这里是预期的行为
			err = os.RemoveAll(path)
			if err != nil {
				return err
			}
		}
		return nil
	})
}
