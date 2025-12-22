package config

import (
	"path/filepath"
	"strings"
)

// Autocode 代码自动生成配置结构体
// 用于配置代码生成器的路径和参数，支持快速生成CRUD代码
// 设计目的：
// 1. 提高效率：自动生成标准化的CRUD代码，减少重复劳动
// 2. 规范统一：生成的代码遵循项目规范，保证代码质量
// 3. 快速开发：从数据库表结构快速生成完整的业务代码
// 4. AI辅助：支持AI辅助代码生成，提高生成代码的质量
type Autocode struct {
	// Web 前端代码路径：前端项目相对于工作目录的路径
	// 格式：支持相对路径，如 "../web" 或 "web"
	// 设计目的：指定生成的前端代码存放位置
	Web string `mapstructure:"web" json:"web" yaml:"web"`
	// Root 根路径：代码生成的基础路径
	// 设计目的：统一管理生成的代码位置，便于组织和查找
	Root string `mapstructure:"root" json:"root" yaml:"root"`
	// Server 后端代码路径：后端代码相对于Root的路径
	// 设计目的：指定生成的后端代码（Go代码）存放位置
	Server string `mapstructure:"server" json:"server" yaml:"server"`
	// Module Go模块名：生成的Go代码使用的模块名
	// 格式：应符合Go模块命名规范，如 "github.com/user/project"
	// 设计目的：确保生成的代码import路径正确
	Module string `mapstructure:"module" json:"module" yaml:"module"`
	// AiPath AI代码生成路径：AI辅助代码生成的相关路径配置
	// 设计目的：支持AI辅助生成更智能、更符合业务需求的代码
	AiPath string `mapstructure:"ai-path" json:"ai-path" yaml:"ai-path"`
}

// WebRoot 获取前端代码的完整根路径
// 设计目的：
// 1. 路径规范化：将配置中的路径转换为标准化的文件系统路径
// 2. 跨平台兼容：自动处理不同操作系统的路径分隔符（Windows使用\，Unix使用/）
// 3. 路径拼接：将多个路径段正确拼接为完整路径
// 实现逻辑：
// - 首先尝试使用"/"分割路径（Unix风格）
// - 如果分割结果为空，尝试使用"\"分割（Windows风格）
// - 使用filepath.Join拼接路径，确保跨平台兼容
// 示例：
// - 输入："../web/src" -> 输出：标准化的完整路径
// - 输入："web\\src" -> 输出：自动转换为正确的路径格式
func (a *Autocode) WebRoot() string {
	// 尝试使用Unix路径分隔符分割
	webs := strings.Split(a.Web, "/")
	// 如果分割结果为空（可能是Windows路径），尝试使用Windows分隔符
	if len(webs) == 0 {
		webs = strings.Split(a.Web, "\\")
	}
	// 使用filepath.Join拼接路径，自动处理路径分隔符和规范化
	// 好处：跨平台兼容，自动处理".."和"."等路径符号
	return filepath.Join(webs...)
}
