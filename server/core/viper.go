package core

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/flipped-aurora/gin-vue-admin/server/core/internal"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// Viper 初始化并返回 Viper 配置管理器实例
//
// 设计思路：
// 1. 使用 Viper 库统一管理配置文件，支持多种配置格式（YAML、JSON、TOML等）
// 2. 实现配置文件热更新机制，修改配置文件后无需重启服务即可生效
// 3. 自动将配置文件内容解析到全局配置结构体中，便于全局访问
//
// 好处：
// - 统一配置管理：所有配置集中在一个地方，便于维护和查找
// - 热更新支持：开发和生产环境都可以动态修改配置，提高灵活性
// - 类型安全：通过结构体映射，编译期就能发现配置项错误
// - 环境隔离：支持不同环境使用不同的配置文件（debug/release/test）
func Viper() *viper.Viper {
	// 获取配置文件路径，按照优先级顺序查找：
	// 1. 命令行参数 -c 指定的路径（最高优先级，适合临时测试）
	// 2. 环境变量 GVA_CONFIG（适合容器化部署，通过环境变量注入）
	// 3. 根据 Gin 运行模式自动选择（开发/生产/测试环境）
	// 4. 默认配置文件 config.yaml（兜底方案）
	config := getConfigPath()

	// 创建新的 Viper 实例
	// 使用 New() 而不是全局实例，避免多个服务实例之间的配置冲突
	v := viper.New()

	// 设置配置文件路径和类型
	// SetConfigFile 指定具体的配置文件路径，而不是让 Viper 自动搜索
	// 这样做的好处是：明确知道使用的是哪个配置文件，避免配置混乱
	v.SetConfigFile(config)
	v.SetConfigType("yaml") // 明确指定为 YAML 格式，提高解析效率

	// 读取配置文件内容
	// 如果配置文件不存在或格式错误，直接 panic
	// 这样设计的好处：配置错误在启动时就能发现，避免运行时出现不可预期的错误
	err := v.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %w", err))
	}

	// 启用配置文件监听功能
	// WatchConfig 会启动一个 goroutine 监控配置文件的变化
	// 好处：实现热更新，修改配置文件后自动重新加载，无需重启服务
	// 这在生产环境中特别有用：可以动态调整日志级别、数据库连接池大小等配置
	v.WatchConfig()

	// 注册配置文件变更回调函数
	// 当配置文件被修改时，自动触发此回调
	// 设计考虑：
	// 1. 只打印错误而不 panic，避免配置文件格式错误导致整个服务崩溃
	// 2. 使用全局变量 global.GVA_CONFIG 存储配置，确保所有模块都能访问最新配置
	// 3. 热更新机制让运维人员可以在不中断服务的情况下调整配置
	v.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("config file changed:", e.Name)
		// 重新解析配置文件到全局配置结构体
		// 注意：这里只打印错误，不 panic，保证服务的稳定性
		// wike补充  这里虽然重新加载了结构体 但是数据库 redis 这些三方组建是重启后才会重新连接的 所以这里只是重新加载了结构体 并没有重新连接数据库 redis 这些三方组建
		if err = v.Unmarshal(&global.GVA_CONFIG); err != nil {
			fmt.Println(err)
		}
	})

	// 首次解析配置文件到全局配置结构体
	// 如果解析失败则 panic，因为启动时配置错误必须立即发现
	// Unmarshal 会将 YAML 配置自动映射到 config.Server 结构体
	// 好处：类型安全，配置项错误在编译期或启动期就能发现
	if err = v.Unmarshal(&global.GVA_CONFIG); err != nil {
		panic(fmt.Errorf("fatal error unmarshal config: %w", err))
	}

	// root 路径适配性处理
	// 根据项目根目录位置动态计算 AutoCode.Root 路径
	// 使用 filepath.Abs("..") 获取上一级目录的绝对路径
	// 这样设计的好处：
	// 1. 无论从哪个目录启动程序，都能正确找到代码生成的目标目录
	// 2. 适配不同的部署环境（开发机、CI/CD、容器等）
	// 3. 忽略错误（使用 _）是因为即使路径计算失败，也不会影响主流程
	global.GVA_CONFIG.AutoCode.Root, _ = filepath.Abs("..")

	return v
}

// getConfigPath 获取配置文件路径
//
// 优先级策略（从高到低）：
// 1. 命令行参数 -c：适合临时测试、调试特定配置
// 2. 环境变量 GVA_CONFIG：适合容器化部署、CI/CD 环境
// 3. Gin 运行模式自动选择：适合标准开发流程
// 4. 默认配置文件：兜底方案，确保程序总能找到配置
//
// 为什么这样设计：
// - 灵活性：支持多种配置方式，适应不同的使用场景
// - 可移植性：通过环境变量可以在不同环境使用相同代码
// - 开发友好：开发/测试/生产环境自动选择对应配置，减少手动操作
// - 容错性：即使指定配置不存在，也能回退到默认配置
func getConfigPath() (config string) {
	// 第一步：检查命令行参数
	// 使用 flag 包解析命令行参数，支持 -c 或 --c 指定配置文件路径
	// 好处：启动时临时指定配置文件，方便测试和调试
	// 例如：./server -c /path/to/custom-config.yaml
	flag.StringVar(&config, "c", "", "choose config file.")
	flag.Parse()
	if config != "" { // 命令行参数不为空 将值赋值于config
		fmt.Printf("您正在使用命令行的 '-c' 参数传递的值, config 的路径为 %s\n", config)
		return
	}

	// 第二步：检查环境变量
	// 环境变量方式特别适合容器化部署（Docker、Kubernetes）
	// 好处：
	// 1. 不修改代码和配置文件，通过环境变量注入不同环境的配置
	// 2. 符合 12-Factor App 原则，配置与代码分离
	// 3. 在 K8s 中可以通过 ConfigMap 和 Secret 管理配置
	if env := os.Getenv(internal.ConfigEnv); env != "" { // 判断环境变量 GVA_CONFIG
		config = env
		fmt.Printf("您正在使用 %s 环境变量, config 的路径为 %s\n", internal.ConfigEnv, config)
		return
	}

	// 第三步：根据 Gin 运行模式自动选择配置文件
	// Gin 有三种运行模式：
	// - DebugMode：开发模式，使用 config.debug.yaml（通常包含详细日志、调试信息）
	// - ReleaseMode：生产模式，使用 config.release.yaml（通常优化性能、关闭调试）
	// - TestMode：测试模式，使用 config.test.yaml（通常使用测试数据库、模拟服务）
	//
	// 这样设计的好处：
	// 1. 环境隔离：不同环境使用不同配置，避免配置冲突
	// 2. 自动化：根据 GIN_MODE 环境变量自动选择，无需手动指定
	// 3. 安全性：生产环境默认使用 release 配置，减少误操作风险
	switch gin.Mode() { // 根据 gin 模式文件名
	case gin.DebugMode:
		config = internal.ConfigDebugFile
	case gin.ReleaseMode:
		config = internal.ConfigReleaseFile
	case gin.TestMode:
		config = internal.ConfigTestFile
	}
	fmt.Printf("您正在使用 gin 的 %s 模式运行, config 的路径为 %s\n", gin.Mode(), config)

	// 第四步：容错处理 - 如果指定配置文件不存在，回退到默认配置
	// 使用 os.Stat 检查文件是否存在
	// 好处：
	// 1. 提高容错性：即使环境配置不存在，也能使用默认配置启动
	// 2. 简化部署：新环境不需要准备所有配置文件，使用默认配置即可
	// 3. 向后兼容：老项目可能只有 config.yaml，仍能正常运行
	_, err := os.Stat(config)
	if err != nil || os.IsNotExist(err) {
		config = internal.ConfigDefaultFile
		fmt.Printf("配置文件路径不存在, 使用默认配置文件路径: %s\n", config)
	}

	return
}
