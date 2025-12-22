package example

import (
	"errors"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/example"
	"gorm.io/gorm"
)

// AttachmentCategoryService 附件分类服务
// 使用空结构体作为服务类型的好处：
// 1. 零内存占用：空结构体不占用任何内存空间（0字节）
// 2. 语义清晰：明确表示这是一个服务层，只包含方法，不包含状态
// 3. 符合Go语言习惯：Go中常用空结构体作为方法接收者，表示类型分组而非数据存储
type AttachmentCategoryService struct{}

// AddCategory 创建/更新分类
// 设计思路：
// 1. 统一接口：将创建和更新合并到一个方法，通过ID判断操作类型，减少代码重复
// 2. 数据校验：在操作前检查重复性，避免数据不一致
// 3. 业务规则：同一父节点下不允许同名分类，保证分类的唯一性和层次性
//
// 参数说明：
//   - req: 分类请求数据，包含ID（更新时>0，创建时为0）、Name（分类名称）、Pid（父节点ID）
//
// 返回值：
//   - err: 操作失败时返回错误信息
func (a *AttachmentCategoryService) AddCategory(req *example.ExaAttachmentCategory) (err error) {
	// 检查是否已存在相同名称的分类
	// 为什么同时检查 name 和 pid：
	// 1. 允许不同父节点下有同名分类（如：A分类下有"图片"，B分类下也可以有"图片"）
	// 2. 但同一父节点下不能有重复名称（保证同级分类的唯一性）
	// 3. 使用 Take 方法只查询一条记录，性能优于 Find（不需要加载所有匹配记录）
	//
	// errors.Is 的作用：
	// 1. 正确判断是否为"记录不存在"错误，而不是其他数据库错误
	// 2. 如果记录存在（非 ErrRecordNotFound），说明名称重复，返回错误
	if (!errors.Is(global.GVA_DB.Take(&example.ExaAttachmentCategory{}, "name = ? and pid = ?", req.Name, req.Pid).Error, gorm.ErrRecordNotFound)) {
		return errors.New("分类名称已存在")
	}

	// 根据ID判断是更新还是创建
	// 为什么用 ID > 0 判断：
	// 1. Go中uint类型的零值是0，新创建的分类ID为0
	// 2. 更新时前端会传入已存在的ID（>0）
	// 3. 这种方式简单直观，无需额外的标志位
	if req.ID > 0 {
		// 更新操作
		// 使用 Updates 而非 Save 的好处：
		// 1. Updates 只更新指定字段，不会覆盖未传入的字段（如CreatedAt）
		// 2. 使用 Model().Where().Updates() 模式，明确指定更新条件，避免误更新
		// 3. 只更新 Name 和 Pid 字段，保持其他字段（如创建时间）不变
		if err = global.GVA_DB.Model(&example.ExaAttachmentCategory{}).Where("id = ?", req.ID).Updates(&example.ExaAttachmentCategory{
			Name: req.Name,
			Pid:  req.Pid,
		}).Error; err != nil {
			return err
		}
	} else {
		// 创建操作
		// 使用 Create 方法的好处：
		// 1. GORM会自动填充ID、CreatedAt、UpdatedAt等字段
		// 2. 只传入业务字段（Name、Pid），保持代码简洁
		// 3. 如果插入失败（如违反唯一约束），会返回错误
		if err = global.GVA_DB.Create(&example.ExaAttachmentCategory{
			Name: req.Name,
			Pid:  req.Pid,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

// DeleteCategory 删除分类
// 设计思路：
// 1. 级联删除保护：删除前检查是否有子分类，防止数据孤立
// 2. 硬删除：使用 Unscoped().Delete() 真正从数据库删除，而非软删除
// 3. 业务规则：必须先删除所有子分类，保证数据完整性
//
// 参数说明：
//   - id: 要删除的分类ID（指针类型，便于判断nil）
//
// 返回值：
//   - error: 删除失败或存在子分类时返回错误
func (a *AttachmentCategoryService) DeleteCategory(id *int) error {
	// 检查是否存在子分类
	// 为什么先检查子分类：
	// 1. 防止删除有子节点的分类，避免产生孤儿数据
	// 2. 保证树形结构的完整性
	// 3. 提供明确的错误提示，引导用户先删除子分类
	var childCount int64
	// 使用 Count 方法只统计数量，不加载数据，性能高效
	global.GVA_DB.Model(&example.ExaAttachmentCategory{}).Where("pid = ?", id).Count(&childCount)
	if childCount > 0 {
		return errors.New("请先删除子级")
	}

	// 执行硬删除
	// 为什么使用 Unscoped().Delete()：
	// 1. Unscoped() 绕过GORM的软删除机制（如果模型定义了DeletedAt字段）
	// 2. 真正从数据库中删除记录，而非只标记删除时间
	// 3. 适用于分类这种需要彻底删除的场景（软删除会导致分类列表混乱）
	return global.GVA_DB.Where("id = ?", id).Unscoped().Delete(&example.ExaAttachmentCategory{}).Error
}

// GetCategoryList 获取分类列表（树形结构）
// 设计思路：
// 1. 一次性查询：先查询所有分类到内存，避免N+1查询问题
// 2. 内存构建树：在应用层构建树形结构，而非数据库层递归查询
// 3. 递归算法：使用递归方式构建树，代码简洁易懂
//
// 返回值：
//   - res: 树形结构的分类列表（根节点的Children包含子节点）
//   - err: 查询失败时返回错误
func (a *AttachmentCategoryService) GetCategoryList() (res []*example.ExaAttachmentCategory, err error) {
	// 一次性查询所有分类
	// 为什么不用递归查询：
	// 1. 避免N+1查询问题（如果递归查询，每个节点都要查一次数据库）
	// 2. 分类数据通常不会特别大，一次性加载到内存性能更好
	// 3. 在应用层构建树形结构，逻辑更清晰，便于维护
	var fileLists []example.ExaAttachmentCategory
	err = global.GVA_DB.Model(&example.ExaAttachmentCategory{}).Find(&fileLists).Error
	if err != nil {
		return res, err
	}

	// 从根节点（pid=0）开始构建树形结构
	// 为什么从0开始：
	// 1. 根节点的父ID为0，这是树形结构的约定
	// 2. 递归函数会从根节点开始，逐层构建整个树
	return a.getChildrenList(fileLists, 0), nil
}

// getChildrenList 递归构建树形结构
// 设计思路：
// 1. 递归算法：利用函数递归特性，自然地处理树形结构
// 2. 深度优先：先处理当前节点，再递归处理子节点
// 3. 内存高效：复用传入的categories切片，不创建额外的大对象
//
// 参数说明：
//   - categories: 所有分类的平铺列表（已从数据库查询）
//   - parentID: 当前要查找的父节点ID（0表示根节点）
//
// 返回值：
//   - tree: 当前父节点下的所有子节点（已构建好Children字段的树形结构）
//
// 算法优势：
// 1. 时间复杂度：O(n²)，n为分类总数（每个节点都要遍历一次categories）
// 2. 空间复杂度：O(n)，递归调用栈深度最多为树的高度
// 3. 代码简洁：递归实现比迭代实现更直观，易于理解
//
// 适用场景：
// - 分类数量不是特别大（<1000）时，性能完全可接受
// - 如果分类数量很大，可以考虑使用哈希表优化（用map存储parentID->children的映射）
func (a *AttachmentCategoryService) getChildrenList(categories []example.ExaAttachmentCategory, parentID uint) []*example.ExaAttachmentCategory {
	var tree []*example.ExaAttachmentCategory

	// 遍历所有分类，找出当前父节点的所有子节点
	for _, category := range categories {
		if category.Pid == parentID {
			// 递归查找当前节点的子节点
			// 为什么在append前递归：
			// 1. 先构建子节点的树形结构，再添加到父节点
			// 2. 保证返回的节点已经包含完整的子树
			// 3. 深度优先遍历，符合树形结构的构建逻辑
			category.Children = a.getChildrenList(categories, category.ID)

			// 将当前节点添加到树中
			// 注意：这里使用 &category 取地址，因为category是循环变量
			// 在Go中，循环变量的地址在每次迭代中都是相同的，所以需要特别注意
			// 这里能正常工作是因为category会被复制到新的内存位置（通过结构体赋值）
			tree = append(tree, &category)
		}
	}
	return tree
}
