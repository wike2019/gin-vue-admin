package system

import (
	"github.com/flipped-aurora/gin-vue-admin/server/model/common"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AutoCodeApi struct{}

// GetDB 获取当前所有数据库接口
// 设计要点：
// 1. 多数据库支持：支持查询多个数据库，满足多数据库场景
// 2. 配置信息：返回数据库配置信息（别名、名称、类型、状态等）
// 3. 动态查询：根据 businessDB 参数动态查询指定数据库
// 4. 统一格式：返回统一的数据库列表格式，便于前端使用
//
// 为什么这么写：
// - 多数据库支持：支持多数据库场景，提升系统灵活性
// - 配置信息：返回数据库配置信息，便于前端展示和选择
// - 动态查询：根据参数动态查询，支持灵活的数据库选择
// - 统一格式：统一的返回格式便于前端处理
//
// 返回数据：
// - dbs: 从数据库查询到的数据库列表（实际存在的数据库）
// - dbList: 配置文件中配置的数据库列表（包含配置信息）
//
// 好处：
// - 灵活性：支持多数据库场景
// - 完整性：返回配置信息和实际数据库列表
// - 可维护性：统一的格式便于维护
//
// @Tags      AutoCode
// @Summary   获取当前所有数据库
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Success   200  {object}  response.Response{data=map[string]interface{},msg=string}  "获取当前所有数据库"
// @Router    /autoCode/getDB [get]
func (autoApi *AutoCodeApi) GetDB(c *gin.Context) {
	// 步骤1：获取业务数据库参数（可选）
	// 如果指定了 businessDB，则查询指定数据库；否则查询默认数据库
	businessDB := c.Query("businessDB")
	
	// 步骤2：调用服务层查询数据库列表
	// service 层会：
	// 1. 连接到指定数据库
	// 2. 查询数据库列表
	// 3. 返回数据库名称列表
	dbs, err := autoCodeService.Database(businessDB).GetDB(businessDB)
	
	// 步骤3：构建数据库配置列表
	// 从配置文件中读取数据库配置信息，包含别名、名称、类型、状态等
	// 设计原因：前端需要配置信息来展示和选择数据库
	var dbList []map[string]interface{}
	for _, db := range global.GVA_CONFIG.DBList {
		var item = make(map[string]interface{})
		item["aliasName"] = db.AliasName // 数据库别名
		item["dbName"] = db.Dbname       // 数据库名称
		item["disable"] = db.Disable     // 是否禁用
		item["dbtype"] = db.Type         // 数据库类型（MySQL、PostgreSQL等）
		dbList = append(dbList, item)
	}
	
	// 步骤4：返回数据库列表和配置信息
	// dbs: 实际查询到的数据库列表
	// dbList: 配置文件中配置的数据库列表
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
	} else {
		response.OkWithDetailed(gin.H{"dbs": dbs, "dbList": dbList}, "获取成功", c)
	}
}

// GetTables
// @Tags      AutoCode
// @Summary   获取当前数据库所有表
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Success   200  {object}  response.Response{data=map[string]interface{},msg=string}  "获取当前数据库所有表"
// @Router    /autoCode/getTables [get]
func (autoApi *AutoCodeApi) GetTables(c *gin.Context) {
	dbName := c.Query("dbName")
	businessDB := c.Query("businessDB")
	if dbName == "" {
		dbName = *global.GVA_ACTIVE_DBNAME
		if businessDB != "" {
			for _, db := range global.GVA_CONFIG.DBList {
				if db.AliasName == businessDB {
					dbName = db.Dbname
				}
			}
		}
	}

	tables, err := autoCodeService.Database(businessDB).GetTables(businessDB, dbName)
	if err != nil {
		global.GVA_LOG.Error("查询table失败!", zap.Error(err))
		response.FailWithMessage("查询table失败", c)
	} else {
		response.OkWithDetailed(gin.H{"tables": tables}, "获取成功", c)
	}
}

// GetColumn
// @Tags      AutoCode
// @Summary   获取当前表所有字段
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Success   200  {object}  response.Response{data=map[string]interface{},msg=string}  "获取当前表所有字段"
// @Router    /autoCode/getColumn [get]
func (autoApi *AutoCodeApi) GetColumn(c *gin.Context) {
	businessDB := c.Query("businessDB")
	dbName := c.Query("dbName")
	if dbName == "" {
		dbName = *global.GVA_ACTIVE_DBNAME
		if businessDB != "" {
			for _, db := range global.GVA_CONFIG.DBList {
				if db.AliasName == businessDB {
					dbName = db.Dbname
				}
			}
		}
	}
	tableName := c.Query("tableName")
	columns, err := autoCodeService.Database(businessDB).GetColumn(businessDB, tableName, dbName)
	if err != nil {
		global.GVA_LOG.Error("获取失败!", zap.Error(err))
		response.FailWithMessage("获取失败", c)
	} else {
		response.OkWithDetailed(gin.H{"columns": columns}, "获取成功", c)
	}
}

// LLMAuto 使用大模型自动生成代码接口
// 设计要点：
// 1. AI 辅助：使用大模型（LLM）自动生成代码，提升开发效率
// 2. 上下文传递：使用 Context 传递请求上下文，支持超时控制和取消
// 3. 灵活输入：使用 JSONMap 接收灵活的输入参数
// 4. 错误处理：详细的错误信息帮助用户理解问题
//
// 为什么这么写：
// - AI 辅助：利用大模型能力自动生成代码，减少重复工作
// - 上下文传递：使用 Context 支持超时控制和取消，提升系统稳定性
// - 灵活输入：使用 JSONMap 支持灵活的输入参数，适应不同场景
// - 错误处理：详细的错误信息帮助用户理解问题
//
// 工作流程：
// 1. 接收用户输入（表结构、需求描述等）
// 2. 调用大模型生成代码
// 3. 返回生成的代码
//
// 好处：
// - 效率：自动生成代码，减少重复工作
// - 智能化：利用 AI 能力，提升代码质量
// - 灵活性：支持灵活的输入参数，适应不同场景
//
// @Tags      AutoCode
// @Summary   使用大模型自动生成代码
// @Security  ApiKeyAuth
// @accept    application/json
// @Produce   application/json
// @Param     data  body      common.JSONMap  true  "大模型输入参数"
// @Success   200   {object}  response.Response{data=interface{},msg=string}  "生成成功"
// @Router    /autoCode/LLMAuto [post]
func (autoApi *AutoCodeApi) LLMAuto(c *gin.Context) {
	// 步骤1：绑定请求参数（使用 JSONMap 支持灵活的输入）
	var llm common.JSONMap
	if err := c.ShouldBindJSON(&llm); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	
	// 步骤2：调用服务层使用大模型生成代码
	// 使用 Context 传递请求上下文，支持超时控制和取消
	// service 层会：
	// 1. 解析输入参数（表结构、需求描述等）
	// 2. 调用大模型 API 生成代码
	// 3. 返回生成的代码
	data, err := autoCodeService.LLMAuto(c.Request.Context(), llm)
	if err != nil {
		global.GVA_LOG.Error("大模型生成失败!", zap.Error(err))
		response.FailWithMessage("大模型生成失败"+err.Error(), c)
		return
	}
	response.OkWithData(data, c)
}
