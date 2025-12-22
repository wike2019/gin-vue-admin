// Package utils 提供系统监控相关的工具函数
// 本文件专门用于收集和格式化服务器系统信息，包括操作系统、CPU、内存和磁盘使用情况
// 设计目的：为系统监控面板提供统一的数据接口，便于前端展示服务器运行状态
package utils

import (
	"runtime"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"

	// 使用 gopsutil 库的原因：
	// 1. 跨平台支持：统一接口支持 Windows、Linux、macOS 等多个操作系统
	// 2. 封装底层差异：不同操作系统的系统调用差异被库封装，代码更简洁
	// 3. 性能优化：库内部已做性能优化，避免重复造轮子
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// 存储单位转换常量
// 设计原因：
// 1. 避免魔法数字：直接使用 1024 等数字会降低代码可读性
// 2. 统一单位转换：所有地方使用相同常量，保证一致性
// 3. 易于维护：如果需要修改单位换算（如改为1000进制），只需修改一处
// 4. 类型安全：通过常量计算，编译器可以检查类型错误
const (
	B  = 1         // 字节，基础单位
	KB = 1024 * B  // 千字节，1024字节
	MB = 1024 * KB // 兆字节，1024KB
	GB = 1024 * MB // 吉字节，1024MB
)

// Server 服务器系统信息聚合结构体
// 设计原因：
// 1. 统一数据结构：将所有系统信息聚合在一个结构体中，便于一次性返回
// 2. JSON序列化：使用 json tag 确保前端可以直接使用，字段名采用驼峰命名符合前端规范
// 3. 模块化设计：每个子系统（OS、CPU、RAM、Disk）独立结构体，职责清晰
type Server struct {
	Os   Os     `json:"os"`   // 操作系统信息
	Cpu  Cpu    `json:"cpu"`  // CPU信息
	Ram  Ram    `json:"ram"`  // 内存信息
	Disk []Disk `json:"disk"` // 磁盘信息（数组，因为可能有多个挂载点）
}

// Os 操作系统相关信息
// 设计考虑：
// 1. 使用 Go 标准库 runtime 包：轻量级，无需外部依赖，获取基础信息足够
// 2. 包含运行时信息：NumGoroutine 可以监控 goroutine 泄漏问题
// 3. 版本信息：便于排查环境问题，知道运行在哪个 Go 版本上
type Os struct {
	GOOS         string `json:"goos"`         // 操作系统类型（如：linux、darwin、windows）
	NumCPU       int    `json:"numCpu"`       // CPU核心数
	Compiler     string `json:"compiler"`     // 编译器类型（如：gc）
	GoVersion    string `json:"goVersion"`    // Go 版本号
	NumGoroutine int    `json:"numGoroutine"` // 当前 goroutine 数量（用于监控并发情况）
}

// Cpu CPU使用情况
// 设计原因：
// 1. Cpus 数组：支持多核CPU，每个核心的使用率单独记录，便于详细分析
// 2. float64 类型：CPU使用率是百分比，可能有小数，使用浮点数更精确
// 3. Cores 字段：快速获取核心数，无需计算数组长度
type Cpu struct {
	Cpus  []float64 `json:"cpus"`  // 每个CPU核心的使用率百分比数组（如：[45.2, 67.8, 23.1]）
	Cores int       `json:"cores"` // CPU核心总数
}

// Ram 内存使用情况
// 设计考虑：
// 1. 使用 MB 单位：内存通常以 MB 为单位展示更直观，避免字节数过大
// 2. UsedPercent：百分比更直观，前端可以直接用于进度条展示
// 3. int 类型：MB 级别的内存值使用 int 足够，避免浮点数精度问题
type Ram struct {
	UsedMB      int `json:"usedMb"`      // 已使用内存（MB）
	TotalMB     int `json:"totalMb"`     // 总内存（MB）
	UsedPercent int `json:"usedPercent"` // 使用率百分比（0-100）
}

// Disk 磁盘使用情况
// 设计原因：
// 1. 同时提供 MB 和 GB：前端可以根据需要选择合适的单位展示
// 2. MountPoint：记录挂载点，便于识别是哪个磁盘分区
// 3. 数组结构：系统可能有多个磁盘分区，需要分别监控
// 4. UsedPercent：快速判断磁盘是否快满了，便于告警
type Disk struct {
	MountPoint  string `json:"mountPoint"`  // 磁盘挂载点（如：/、/home、C:\）
	UsedMB      int    `json:"usedMb"`      // 已使用空间（MB）
	UsedGB      int    `json:"usedGb"`      // 已使用空间（GB）
	TotalMB     int    `json:"totalMb"`     // 总空间（MB）
	TotalGB     int    `json:"totalGb"`     // 总空间（GB）
	UsedPercent int    `json:"usedPercent"` // 使用率百分比（0-100）
}

// @author: [SliverHorn](https://github.com/SliverHorn)
// @function: InitOS
// @description: 初始化操作系统信息
// @return: o Os
// 设计说明：
// 1. 不返回错误：runtime 包的函数都是同步且不会失败，无需错误处理
// 2. 使用命名返回值：代码更简洁，直接赋值即可
// 3. 使用标准库：runtime 包是 Go 内置，无需外部依赖，性能好且稳定
func InitOS() (o Os) {
	o.GOOS = runtime.GOOS                   // 获取操作系统类型（跨平台常量）
	o.NumCPU = runtime.NumCPU()             // 获取逻辑CPU核心数（考虑超线程）
	o.Compiler = runtime.Compiler           // 获取编译器标识（通常是 "gc"）
	o.GoVersion = runtime.Version()         // 获取 Go 版本字符串（如：go1.21.0）
	o.NumGoroutine = runtime.NumGoroutine() // 获取当前 goroutine 数量（监控并发）
	return o
}

// @author: [SliverHorn](https://github.com/SliverHorn)
// @function: InitCPU
// @description: 初始化CPU使用情况信息
// @return: c Cpu, err error
// 设计说明：
// 1. 返回错误：gopsutil 库可能因为权限或系统问题失败，需要错误处理
// 2. cpu.Counts(false)：false 表示获取物理核心数（不包括超线程），更准确反映实际性能
// 3. 200ms 采样间隔：平衡准确性和性能
//   - 太短（如50ms）：可能不够稳定，波动大
//   - 太长（如1s）：响应慢，用户体验差
//   - 200ms：既能反映实时状态，又不会太慢
//     4. cpu.Percent 第二个参数 true：表示获取每个核心的使用率，而不是平均值
//     这样可以看到哪些核心负载高，便于性能分析
func InitCPU() (c Cpu, err error) {
	// 获取物理CPU核心数（不包括超线程虚拟核心）
	if cores, err := cpu.Counts(false); err != nil {
		return c, err
	} else {
		c.Cores = cores
	}
	// 获取每个CPU核心的使用率
	// 200ms 采样间隔：足够短以反映实时状态，又不会太频繁影响性能
	// true 参数：返回每个核心的使用率数组，而不是平均值
	if cpus, err := cpu.Percent(time.Duration(200)*time.Millisecond, true); err != nil {
		return c, err
	} else {
		c.Cpus = cpus
	}
	return c, nil
}

// @author: [SliverHorn](https://github.com/SliverHorn)
// @function: InitRAM
// @description: 初始化内存使用情况信息
// @return: r Ram, err error
// 设计说明：
// 1. 转换为 MB：原始数据是字节，转换为 MB 更易读，前端展示更方便
// 2. 使用常量除法：通过 MB 常量进行单位转换，代码清晰且避免硬编码
// 3. 保留百分比：gopsutil 已经计算好百分比，直接使用更准确（考虑了缓存等因素）
// 4. int 类型转换：MB 级别的值使用 int 足够，避免浮点数带来的精度问题和不必要的内存占用
func InitRAM() (r Ram, err error) {
	// 获取虚拟内存信息（包括物理内存和交换空间）
	if u, err := mem.VirtualMemory(); err != nil {
		return r, err
	} else {
		// 将字节转换为 MB，使用常量除法保证一致性
		r.UsedMB = int(u.Used) / MB
		r.TotalMB = int(u.Total) / MB
		// 直接使用库计算的百分比，更准确（考虑了系统缓存等因素）
		r.UsedPercent = int(u.UsedPercent)
	}
	return r, nil
}

// @author: [SliverHorn](https://github.com/SliverHorn)
// @function: InitDisk
// @description: 初始化磁盘使用情况信息
// @return: d []Disk, err error
// 设计说明：
//  1. 从配置读取挂载点：通过 global.GVA_CONFIG.DiskList 配置要监控的磁盘
//     好处：灵活配置，不需要监控的磁盘可以不配置，减少不必要的查询
//  2. 遍历所有配置的磁盘：系统可能有多个分区，需要分别监控
//  3. 同时提供 MB 和 GB：前端可以根据磁盘大小选择合适的单位展示
//     - 小磁盘（<100GB）：用 MB 更精确
//     - 大磁盘（>100GB）：用 GB 更直观
//  4. 错误处理策略：如果某个磁盘查询失败，立即返回错误
//     这样设计的好处：保证数据完整性，避免部分数据缺失导致前端展示错误
//     如果希望容错，可以改为记录错误但继续处理其他磁盘
//  5. 使用 append：动态构建数组，支持任意数量的磁盘分区
func InitDisk() (d []Disk, err error) {
	// 遍历配置文件中指定的所有磁盘挂载点
	for i := range global.GVA_CONFIG.DiskList {
		mp := global.GVA_CONFIG.DiskList[i].MountPoint
		// 查询指定挂载点的磁盘使用情况
		if u, err := disk.Usage(mp); err != nil {
			// 如果查询失败，立即返回错误（保证数据完整性）
			return d, err
		} else {
			// 将查询结果转换为我们的数据结构，同时提供 MB 和 GB 两种单位
			d = append(d, Disk{
				MountPoint:  mp,                 // 保留挂载点信息，便于识别
				UsedMB:      int(u.Used) / MB,   // 已使用空间（MB）
				UsedGB:      int(u.Used) / GB,   // 已使用空间（GB）
				TotalMB:     int(u.Total) / MB,  // 总空间（MB）
				TotalGB:     int(u.Total) / GB,  // 总空间（GB）
				UsedPercent: int(u.UsedPercent), // 使用率百分比
			})
		}
	}
	return d, nil
}
