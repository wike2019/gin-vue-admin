package config

// Disk 磁盘监控配置结构体
// 用于监控服务器磁盘的使用情况，防止磁盘空间耗尽导致服务异常
// 设计目的：
// 1. 磁盘监控：实时监控磁盘使用率，及时预警
// 2. 容量管理：帮助管理员了解磁盘使用情况，规划扩容
// 3. 故障预防：在磁盘空间不足时提前告警，避免服务中断
// 使用场景：
// - 日志文件监控：监控日志目录的磁盘使用
// - 文件存储监控：监控文件上传目录的磁盘使用
// - 系统盘监控：监控系统盘的剩余空间
type Disk struct {
	// MountPoint 挂载点：要监控的磁盘挂载点路径
	// 格式：Linux/Unix系统的挂载点路径，例如 "/"、"/var"、"/data"
	// 获取方式：使用 `df -h` 命令查看磁盘挂载点
	// 设计目的：指定要监控的磁盘分区
	// 示例：
	// - "/"：监控根分区
	// - "/var/log"：监控日志分区
	// - "/data/uploads"：监控文件存储分区
	MountPoint string `mapstructure:"mount-point" json:"mount-point" yaml:"mount-point"`
}

// DiskList 磁盘监控配置列表
// 设计目的：
// 1. 多磁盘监控：支持同时监控多个磁盘分区
// 2. 配置复用：通过嵌入Disk结构体，复用配置字段定义
// 3. 扁平化配置：使用 yaml:",inline" 和 mapstructure:",squash" 实现配置扁平化
// 使用场景：
// - 监控多个数据盘
// - 分别监控系统盘和数据盘
// - 监控不同用途的磁盘分区
type DiskList struct {
	Disk `yaml:",inline" mapstructure:",squash"` // 嵌入Disk配置，实现字段扁平化
}
