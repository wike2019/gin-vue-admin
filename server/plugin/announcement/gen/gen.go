package main

import (
	"gorm.io/gen"
	"path/filepath" //go:generate go mod tidy
	//go:generate go mod download
	//go:generate go run gen.go
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/announcement/model"
)

// main 生成 GORM 代码生成器的代码
// 设计模式：代码生成模式（Code Generation Pattern） + GORM Gen 模式
// 好处：
// 1. 自动生成类型安全的数据库操作代码，减少手动编写 CRUD 代码
// 2. 生成的代码性能更好，避免了反射带来的性能开销
// 3. 支持代码提示和类型检查，提高开发效率和代码质量
// 4. 生成的代码易于维护，符合最佳实践
// 使用方法：
// 1. 在项目根目录执行：go generate ./server/plugin/announcement/gen
// 2. 或者直接运行：go run server/plugin/announcement/gen/gen.go
// 3. 生成的代码会输出到指定目录，包含 DAO 层代码
func main() {
	// 创建 GORM Gen 代码生成器
	// 设计模式：代码生成器模式（Code Generator Pattern）
	// 配置说明：
	// - OutPath: 输出目录，生成的代码会保存到此目录
	// - Mode: 生成模式
	//   - gen.WithoutContext: 不使用 context，简化代码
	//   - gen.WithDefaultQuery: 生成默认查询方法（如 First、Find 等）
	//   - gen.WithQueryInterface: 生成查询接口，便于扩展
	g := gen.NewGenerator(gen.Config{
		OutPath: filepath.Join("..", "..", "..", "announcement", "blender", "model", "dao"),
		Mode:    gen.WithoutContext | gen.WithDefaultQuery | gen.WithQueryInterface,
	})
	
	// 应用模型，生成对应的 DAO 代码
	// 设计模式：模型驱动模式（Model-Driven Pattern）
	// 好处：
	// 1. 根据模型定义自动生成 CRUD 操作代码
	// 2. 支持多个模型，可以一次性生成多个模型的代码
	// 3. 生成的代码包含类型安全的查询方法
	g.ApplyBasic(
		new(model.Info),
		// 可以继续添加其他模型，如：
		// new(model.Category),
		// new(model.Comment),
	)
	
	// 执行代码生成
	// 好处：生成所有配置的模型的 DAO 代码
	g.Execute()
}
