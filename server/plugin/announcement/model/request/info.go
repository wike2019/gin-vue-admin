package request

import (
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/request"
	"time"
)

// InfoSearch 公告查询参数结构体
// 设计模式：查询对象模式（Query Object Pattern） + 组合模式（Composition Pattern）
// 好处：
// 1. 将查询参数封装为结构体，便于参数验证和传递
// 2. 支持复杂的查询条件（如时间范围、关键词搜索等）
// 3. 嵌入 PageInfo 自动获得分页参数（Page、PageSize）
// 4. 使用指针类型支持可选参数，提高查询灵活性
// 5. 统一的查询参数格式，便于前端调用和参数验证
type InfoSearch struct {
	// StartCreatedAt 创建时间起始范围（可选）
	// 设计模式：时间范围查询模式（Time Range Query Pattern）
	// 好处：
	// 1. 使用指针类型（*time.Time），支持 NULL 值，表示可选参数
	// 2. 如果为空，则不限制起始时间
	// 3. 配合 EndCreatedAt 实现时间范围查询
	// 4. 便于实现"最近一周"、"最近一月"等时间筛选功能
	StartCreatedAt *time.Time `json:"startCreatedAt" form:"startCreatedAt"`
	
	// EndCreatedAt 创建时间结束范围（可选）
	// 设计模式：时间范围查询模式（Time Range Query Pattern）
	// 好处：
	// 1. 使用指针类型（*time.Time），支持 NULL 值，表示可选参数
	// 2. 如果为空，则不限制结束时间
	// 3. 配合 StartCreatedAt 实现时间范围查询
	// 4. 支持精确的时间范围筛选，提高查询准确性
	EndCreatedAt *time.Time `json:"endCreatedAt" form:"endCreatedAt"`
	
	// 嵌入 PageInfo，自动获得分页参数
	// 设计模式：组合模式（Composition Pattern）
	// 好处：
	// 1. 复用通用分页参数（Page、PageSize），避免重复定义
	// 2. 统一分页参数格式，便于前端统一处理
	// 3. 支持扩展更多查询条件（如关键词搜索、状态过滤等）
	// PageInfo 包含：
	// - Page: 当前页码（从1开始）
	// - PageSize: 每页记录数
	request.PageInfo
}
