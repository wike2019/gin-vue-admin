package system

import (
	"errors"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
	"github.com/flipped-aurora/gin-vue-admin/server/model/system/response"
	"gorm.io/gorm"
)

// AuthorityBtnService 权限按钮服务结构体
// 采用服务层设计模式，将业务逻辑封装在服务层，实现关注点分离
// 好处：1. 便于业务逻辑复用 2. 便于单元测试 3. 便于维护和扩展
type AuthorityBtnService struct{}

// AuthorityBtnServiceApp 全局服务实例
// 使用单例模式，避免重复创建服务实例，节省内存资源
var AuthorityBtnServiceApp = new(AuthorityBtnService)

// GetAuthorityBtn 获取指定权限和菜单下已选中的按钮ID列表
// 参数：req - 包含权限ID和菜单ID的请求参数
// 返回：res - 包含已选中按钮ID列表的响应，err - 错误信息
//
// 实现思路：
// 1. 根据权限ID和菜单ID查询所有关联的权限按钮记录
// 2. 遍历查询结果，提取所有按钮ID到selected切片中
// 3. 返回按钮ID列表供前端使用
//
// 为什么这样写：
// - 使用Find方法配合条件查询，性能优于先查询所有再过滤
// - 使用切片收集ID，避免多次数据库查询，提高效率
// - 返回ID列表而非完整对象，减少数据传输量，提升接口性能
func (a *AuthorityBtnService) GetAuthorityBtn(req request.SysAuthorityBtnReq) (res response.SysAuthorityBtnRes, err error) {
	var authorityBtn []system.SysAuthorityBtn
	// 使用参数化查询，防止SQL注入，同时利用数据库索引提升查询性能
	err = global.GVA_DB.Find(&authorityBtn, "authority_id = ? and sys_menu_id = ?", req.AuthorityId, req.MenuID).Error
	if err != nil {
		return
	}
	var selected []uint
	// 遍历查询结果，提取按钮ID
	// 使用切片append，Go会自动扩容，无需预先分配大小（对于小数据量场景）
	for _, v := range authorityBtn {
		selected = append(selected, v.SysBaseMenuBtnID)
	}
	res.Selected = selected
	return res, err
}

// SetAuthorityBtn 设置权限按钮关联关系（先删除旧关联，再创建新关联）
// 参数：req - 包含权限ID、菜单ID和要选中的按钮ID列表
// 返回：err - 错误信息
//
// 实现思路：
// 1. 使用数据库事务确保操作的原子性
// 2. 先删除该权限和菜单下的所有旧关联关系
// 3. 再批量创建新的关联关系
//
// 为什么使用"先删后增"策略而不是"对比更新"：
// 1. 简化逻辑：无需对比新旧数据，计算差异（新增、删除、保留）
// 2. 性能优势：批量删除+批量创建，比逐条判断更新更高效
// 3. 数据一致性：事务保证要么全部成功要么全部回滚，避免部分更新导致的数据不一致
// 4. 代码简洁：逻辑清晰，易于理解和维护
//
// 为什么使用事务：
// - 保证原子性：删除和创建要么全部成功，要么全部失败
// - 避免数据不一致：如果创建失败，删除操作会回滚，保持原有数据
// - 并发安全：事务提供隔离性，避免并发操作导致的数据冲突
func (a *AuthorityBtnService) SetAuthorityBtn(req request.SysAuthorityBtnReq) (err error) {
	// 使用GORM事务，自动处理提交和回滚
	return global.GVA_DB.Transaction(func(tx *gorm.DB) error {
		var authorityBtn []system.SysAuthorityBtn
		// 先删除该权限和菜单下的所有旧关联关系
		// 使用Delete方法批量删除，性能优于逐条删除
		err = tx.Delete(&[]system.SysAuthorityBtn{}, "authority_id = ? and sys_menu_id = ?", req.AuthorityId, req.MenuID).Error
		if err != nil {
			return err
		}
		// 构建新的关联关系数据
		for _, v := range req.Selected {
			authorityBtn = append(authorityBtn, system.SysAuthorityBtn{
				AuthorityId:      req.AuthorityId,
				SysMenuID:        req.MenuID,
				SysBaseMenuBtnID: v,
			})
		}
		// 只有当有新的关联关系时才执行创建操作
		// 避免空切片导致的无效数据库操作
		if len(authorityBtn) > 0 {
			// 批量创建，使用Create方法一次性插入多条记录，性能优于循环插入
			err = tx.Create(&authorityBtn).Error
		}
		if err != nil {
			return err
		}
		return err
	})
}

// CanRemoveAuthorityBtn 检查指定的按钮是否可以删除
// 参数：ID - 按钮ID（字符串类型）
// 返回：err - 如果按钮正在被使用则返回错误，否则返回nil
//
// 实现思路：
// 1. 查询是否存在权限按钮关联记录使用了该按钮
// 2. 如果不存在（记录未找到），说明按钮未被使用，可以删除
// 3. 如果存在，说明按钮正在被使用，不能删除
//
// 为什么这样写：
// - 使用First方法查询单条记录，性能优于Find（查询到第一条就停止）
// - 使用errors.Is判断是否为记录未找到错误，这是Go 1.13+推荐的错误比较方式
// - 通过检查关联关系防止误删，保证数据完整性（外键约束的补充）
// - 返回明确的错误信息，便于前端提示用户
//
// 好处：
// 1. 数据完整性：防止删除正在使用的按钮，避免产生孤立数据
// 2. 用户体验：提前检查并提示，避免删除失败后的困惑
// 3. 性能优化：只查询一条记录即可判断，无需查询所有关联数据
func (a *AuthorityBtnService) CanRemoveAuthorityBtn(ID string) (err error) {
	// 查询是否存在使用该按钮的权限关联记录
	// 使用First方法，找到第一条记录即返回，性能优于Find
	fErr := global.GVA_DB.First(&system.SysAuthorityBtn{}, "sys_base_menu_btn_id = ?", ID).Error
	// 如果记录未找到，说明按钮未被使用，可以安全删除
	if errors.Is(fErr, gorm.ErrRecordNotFound) {
		return nil
	}
	// 如果找到记录，说明按钮正在被使用，返回错误阻止删除
	return errors.New("此按钮正在被使用无法删除")
}
