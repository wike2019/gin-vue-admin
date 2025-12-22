package autocode

import (
	"fmt"
	"slices"
	"strings"
	"text/template"

	systemReq "github.com/flipped-aurora/gin-vue-admin/server/model/system/request"
)

// GetTemplateFuncMap 返回模板函数映射，用于在模板中使用
//
// 设计意义：
// 1. 将Go函数注册到text/template的FuncMap中，使得模板文件可以直接调用这些函数
// 2. 统一管理所有模板函数，便于维护和扩展
// 3. 将复杂的代码生成逻辑封装成函数，提高模板的可读性和可维护性
//
// 好处：
// - 模板文件只需要调用函数名，不需要关心具体实现细节
// - 函数可以在多个模板中复用，避免重复代码
// - 修改生成逻辑时只需修改函数实现，所有使用该函数的模板自动更新
func GetTemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		"title":                    strings.Title,            // 字符串首字母大写，用于生成结构体名称等
		"GenerateField":            GenerateField,            // 生成Go结构体字段定义
		"GenerateSearchField":      GenerateSearchField,      // 生成搜索结构体字段定义
		"GenerateSearchConditions": GenerateSearchConditions, // 生成数据库查询条件代码
		"GenerateSearchFormItem":   GenerateSearchFormItem,   // 生成前端搜索表单HTML
		"GenerateTableColumn":      GenerateTableColumn,      // 生成前端表格列HTML
		"GenerateFormItem":         GenerateFormItem,         // 生成前端表单输入项HTML
		"GenerateDescriptionItem":  GenerateDescriptionItem,  // 生成详情页描述项HTML
		"GenerateDefaultFormValue": GenerateDefaultFormValue, // 生成表单默认值
	}
}

// GenerateField 生成Go结构体字段定义，包含完整的标签和类型信息
//
// 设计意义：
// 1. 根据字段类型自动生成正确的Go类型（如int8/int16/int32/int64）
// 2. 自动构建GORM标签，包括索引、主键、默认值、注释等
// 3. 根据字段类型选择合适的数据类型（如JSON字段使用datatypes.JSON）
// 4. 处理特殊类型（enum、richtext、file等）的特殊标签需求
//
// 好处：
// - 统一字段生成规则，确保生成的代码符合GORM和Go的最佳实践
// - 自动处理类型映射，避免手动编写容易出错
// - 支持多种字段类型，扩展性强
//
// 示例：
//
// 示例1: 基础字符串字段
//
//	field := systemReq.AutoCodeField{
//		FieldName:    "UserName",
//		FieldJson:    "userName",
//		FieldType:    "string",
//		DataTypeLong: "100",
//		ColumnName:   "user_name",
//		FieldDesc:    "用户名",
//		Comment:      "用户名称",
//		Require:      true,
//	}
//	GenerateField(field)
//	// 输出: UserName  *string `json:"userName" form:"userName" gorm:"comment:用户名称;column:user_name;size:100;" binding:"required"`  //用户名
//
// 示例2: 整数类型字段（根据长度自动选择类型）
//
//	field := systemReq.AutoCodeField{
//		FieldName:    "Age",
//		FieldJson:    "age",
//		FieldType:    "int",
//		DataTypeLong: "3",  // 1-3位数字，生成int8
//		ColumnName:   "age",
//		FieldDesc:    "年龄",
//	}
//	GenerateField(field)
//	// 输出: Age  *int8 `json:"age" form:"age" gorm:"column:age;"`  //年龄
//
//	field.DataTypeLong = "10"  // 6-10位数字，生成int32
//	GenerateField(field)
//	// 输出: Age  *int32 `json:"age" form:"age" gorm:"column:age;"`  //年龄
//
// 示例3: 主键字段
//
//	field := systemReq.AutoCodeField{
//		FieldName:    "ID",
//		FieldJson:    "id",
//		FieldType:    "int",
//		DataTypeLong: "20",
//		ColumnName:   "id",
//		PrimaryKey:   true,
//		FieldDesc:    "主键ID",
//	}
//	GenerateField(field)
//	// 输出: ID  *int64 `json:"id" form:"id" gorm:"primarykey;column:id;"`  //主键ID
//
// 示例4: 枚举类型字段
//
//	field := systemReq.AutoCodeField{
//		FieldName:    "Status",
//		FieldJson:    "status",
//		FieldType:    "enum",
//		DataTypeLong: "active,inactive,pending",
//		ColumnName:   "status",
//		FieldDesc:    "状态",
//		Comment:      "用户状态",
//	}
//	GenerateField(field)
//	// 输出: Status  string `json:"status" form:"status" gorm:"comment:用户状态;column:status;type:enum(active,inactive,pending);"`  //状态
//
// 示例5: 带索引和默认值的字段
//
//	field := systemReq.AutoCodeField{
//		FieldName:     "Email",
//		FieldJson:     "email",
//		FieldType:     "string",
//		DataTypeLong:  "255",
//		ColumnName:    "email",
//		FieldIndexType: "unique",
//		DefaultValue:   "",
//		FieldDesc:     "邮箱",
//		Comment:       "用户邮箱地址",
//		Require:       true,
//	}
//	GenerateField(field)
//	// 输出: Email  *string `json:"email" form:"email" gorm:"unique;default:;comment:用户邮箱地址;column:email;size:255;" binding:"required"`  //邮箱
//
// 示例6: JSON类型字段
//
//	field := systemReq.AutoCodeField{
//		FieldName:    "ExtraInfo",
//		FieldJson:    "extraInfo",
//		FieldType:    "json",
//		ColumnName:   "extra_info",
//		FieldDesc:    "扩展信息",
//		Comment:      "JSON格式的扩展数据",
//	}
//	GenerateField(field)
//	// 输出: ExtraInfo  datatypes.JSON `json:"extraInfo" form:"extraInfo" gorm:"comment:JSON格式的扩展数据;column:extra_info;" swaggertype:"object"`  //扩展信息
//
// 示例7: 文件/图片数组类型字段
//
//	field := systemReq.AutoCodeField{
//		FieldName:    "Attachments",
//		FieldJson:    "attachments",
//		FieldType:    "pictures",
//		ColumnName:   "attachments",
//		FieldDesc:    "附件列表",
//		Comment:      "图片附件数组",
//	}
//	GenerateField(field)
//	// 输出: Attachments  datatypes.JSON `json:"attachments" form:"attachments" gorm:"comment:图片附件数组;column:attachments;" swaggertype:"array,object"`  //附件列表
//
// 示例8: 富文本字段
//
//	field := systemReq.AutoCodeField{
//		FieldName:    "Content",
//		FieldJson:    "content",
//		FieldType:    "richtext",
//		ColumnName:   "content",
//		FieldDesc:    "内容",
//		Comment:      "富文本内容",
//	}
//	GenerateField(field)
//	// 输出: Content  *string `json:"content" form:"content" gorm:"comment:富文本内容;column:content;type:text;"`  //内容
//
// 示例9: 图片/视频URL字段
//
//	field := systemReq.AutoCodeField{
//		FieldName:    "Avatar",
//		FieldJson:    "avatar",
//		FieldType:    "picture",
//		DataTypeLong: "500",
//		ColumnName:   "avatar",
//		FieldDesc:    "头像",
//		Comment:      "用户头像URL",
//	}
//	GenerateField(field)
//	// 输出: Avatar  string `json:"avatar" form:"avatar" gorm:"comment:用户头像URL;column:avatar;size:500;"`  //头像
//
// 示例10: 时间类型字段
//
//	field := systemReq.AutoCodeField{
//		FieldName:    "CreatedAt",
//		FieldJson:    "createdAt",
//		FieldType:    "time.Time",
//		ColumnName:   "created_at",
//		FieldDesc:    "创建时间",
//		Comment:      "记录创建时间",
//	}
//	GenerateField(field)
//	// 输出: CreatedAt  *time.Time `json:"createdAt" form:"createdAt" gorm:"comment:记录创建时间;column:created_at;"`  //创建时间
func GenerateField(field systemReq.AutoCodeField) string {
	// 构建gorm标签
	// 采用字符串拼接方式逐步构建，好处是逻辑清晰，易于理解和维护
	gormTag := ``

	// 添加索引类型（如index、unique等）
	// 先检查再添加，避免生成空的标签项
	if field.FieldIndexType != "" {
		gormTag += field.FieldIndexType + ";"
	}

	// 主键标识
	// 使用布尔值判断，简单直接
	if field.PrimaryKey {
		gormTag += "primarykey;"
	}

	// 默认值
	// 只在有默认值时添加，避免不必要的标签
	if field.DefaultValue != "" {
		gormTag += fmt.Sprintf("default:%s;", field.DefaultValue)
	}

	// 字段注释
	// 数据库层面的注释，有助于数据库文档生成
	if field.Comment != "" {
		gormTag += fmt.Sprintf("comment:%s;", field.Comment)
	}

	// 列名映射
	// 必须项，确保GORM知道数据库列名
	gormTag += "column:" + field.ColumnName + ";"

	// 对于int类型，根据DataTypeLong决定具体的Go类型，不使用size标签
	// 这是因为Go的int类型本身已经确定了大小，不需要在GORM标签中指定
	// 对于其他类型（如string），需要size标签来指定数据库字段长度
	if field.DataTypeLong != "" && field.FieldType != "enum" && field.FieldType != "int" {
		gormTag += fmt.Sprintf("size:%s;", field.DataTypeLong)
	}

	// 必填验证标签
	// 用于gin的binding验证，确保必填字段不为空
	requireTag := ` binding:"required"` + "`"

	// 根据字段类型构建不同的字段定义
	// 使用switch-case处理不同类型，逻辑清晰，易于扩展新类型
	var result string
	switch field.FieldType {
	case "enum":
		// 枚举类型：使用string存储，GORM标签中指定enum类型和可选值
		// 好处：数据库层面约束枚举值，前端可以选择，后端验证简单
		result = fmt.Sprintf(`%s  string `+"`"+`json:"%s" form:"%s" gorm:"%stype:enum(%s);"`+"`",
			field.FieldName, field.FieldJson, field.FieldJson, gormTag, field.DataTypeLong)
	case "picture", "video":
		// 图片/视频：使用string存储URL路径
		// 使用指针类型(*string)的好处：可以区分空字符串和未设置，便于判断是否需要更新
		tagContent := fmt.Sprintf(`json:"%s" form:"%s" gorm:"%s"`,
			field.FieldJson, field.FieldJson, gormTag)

		result = fmt.Sprintf(`%s  string `+"`"+`%s`+"`"+``, field.FieldName, tagContent)
	case "file", "pictures", "array":
		// 文件/图片数组/数组：使用datatypes.JSON存储
		// 好处：可以存储复杂结构（如文件信息包含name、url、size等），灵活性强
		// swaggertype用于Swagger文档生成，标识这是数组类型
		tagContent := fmt.Sprintf(`json:"%s" form:"%s" gorm:"%s"`,
			field.FieldJson, field.FieldJson, gormTag)

		result = fmt.Sprintf(`%s  datatypes.JSON `+"`"+`%s swaggertype:"array,object"`+"`"+``,
			field.FieldName, tagContent)
	case "richtext":
		// 富文本：使用*string指针类型，GORM使用text类型存储
		// 使用指针的好处：可以存储nil，表示未设置；text类型可以存储大文本
		tagContent := fmt.Sprintf(`json:"%s" form:"%s" gorm:"%s`,
			field.FieldJson, field.FieldJson, gormTag)

		result = fmt.Sprintf(`%s  *string `+"`"+`%stype:text;"`+"`"+``,
			field.FieldName, tagContent)
	case "json":
		// JSON类型：使用datatypes.JSON存储任意JSON结构
		// 好处：可以存储灵活的数据结构，不需要预定义所有字段
		tagContent := fmt.Sprintf(`json:"%s" form:"%s" gorm:"%s"`,
			field.FieldJson, field.FieldJson, gormTag)

		result = fmt.Sprintf(`%s  datatypes.JSON `+"`"+`%s swaggertype:"object"`+"`"+``,
			field.FieldName, tagContent)
	default:
		// 默认类型：处理string、int、float64、time.Time等基础类型
		tagContent := fmt.Sprintf(`json:"%s" form:"%s" gorm:"%s"`,
			field.FieldJson, field.FieldJson, gormTag)

		// 对于int类型，根据DataTypeLong决定具体的Go类型
		// 这样设计的好处：
		// 1. 根据数据库字段长度选择最合适的Go类型，节省内存
		// 2. 避免使用int64存储小数值造成的内存浪费
		// 3. 提高代码的类型安全性
		var fieldType string
		if field.FieldType == "int" {
			switch field.DataTypeLong {
			case "1", "2", "3":
				fieldType = "int8" // 1字节，范围-128到127
			case "4", "5":
				fieldType = "int16" // 2字节，范围-32768到32767
			case "6", "7", "8", "9", "10":
				fieldType = "int32" // 4字节，范围约-21亿到21亿
			case "11", "12", "13", "14", "15", "16", "17", "18", "19", "20":
				fieldType = "int64" // 8字节，大整数
			default:
				fieldType = "int64" // 默认使用最大类型，确保兼容性
			}
		} else {
			fieldType = field.FieldType // 其他类型直接使用原类型
		}

		// 使用指针类型(*fieldType)的好处：
		// 1. 可以区分零值和未设置（nil）
		// 2. 在更新时，nil字段不会被更新，只有非nil字段才会更新
		// 3. 符合Go的惯用法，特别是在处理可选字段时
		result = fmt.Sprintf(`%s  *%s `+"`"+`%s`+"`"+``,
			field.FieldName, fieldType, tagContent)
	}

	// 如果字段必填，添加binding验证标签
	// 字符串截取方式添加标签：去掉最后的反引号，添加binding标签，再加回反引号
	// 这样做的好处：不需要重新构建整个字符串，性能更好
	if field.Require {
		result = result[0:len(result)-1] + requireTag
	}

	// 添加字段描述作为注释
	// 好处：生成的代码可读性更好，开发者可以直接看到字段的用途
	if field.FieldDesc != "" {
		result += fmt.Sprintf("  //%s", field.FieldDesc)
	}

	return result
}

// GenerateSearchConditions 生成数据库查询条件代码（GORM链式调用）
//
// 设计意义：
// 1. 根据字段类型和搜索类型自动生成正确的SQL查询条件
// 2. 支持多种搜索类型：LIKE、BETWEEN、NOT BETWEEN、=、>、<等
// 3. 自动处理空值检查，避免无效查询
// 4. 使用GORM的链式调用，代码简洁且类型安全
//
// 好处：
// - 生成的查询代码统一规范，减少手动编写错误
// - 自动处理边界情况（空值、指针解引用等）
// - 支持范围查询、模糊查询等多种查询方式
// - 生成的代码可读性好，易于维护
//
// 示例：
//
// 示例1：字符串字段的LIKE查询
// 输入字段：
//
//	&AutoCodeField{
//	    FieldName: "UserName",
//	    ColumnName: "user_name",
//	    FieldType: "string",
//	    FieldSearchType: "LIKE",
//	}
//
// 生成代码：
//
//	if info.UserName != nil && *info.UserName != "" {
//	    db = db.Where("user_name LIKE ?", "%%"+ *info.UserName+"%%")
//	}
//
// 示例2：数值字段的范围查询（BETWEEN）
// 输入字段：
//
//	&AutoCodeField{
//	    FieldName: "Age",
//	    ColumnName: "age",
//	    FieldType: "int",
//	    FieldSearchType: "BETWEEN",
//	}
//
// 生成代码：
//
//	if info.StartAge != nil && info.EndAge != nil {
//	    db = db.Where("age BETWEEN ? AND ? ", *info.StartAge, *info.EndAge)
//	}
//
// 示例3：时间字段的范围查询
// 输入字段：
//
//	&AutoCodeField{
//	    FieldName: "CreatedAt",
//	    ColumnName: "created_at",
//	    FieldType: "time.Time",
//	    FieldSearchType: "BETWEEN",
//	}
//
// 生成代码：
//
//	if len(info.CreatedAtRange) == 2 {
//	    db = db.Where("created_at BETWEEN ? AND ? ", info.CreatedAtRange[0], info.CreatedAtRange[1])
//	}
//
// 示例4：枚举字段的精确匹配
// 输入字段：
//
//	&AutoCodeField{
//	    FieldName: "Status",
//	    ColumnName: "status",
//	    FieldType: "enum",
//	    FieldSearchType: "=",
//	}
//
// 生成代码：
//
//	if info.Status != "" {
//	    db = db.Where("status = ?", info.Status)
//	}
//
// 示例5：数值字段的大于等于查询
// 输入字段：
//
//	&AutoCodeField{
//	    FieldName: "Score",
//	    ColumnName: "score",
//	    FieldType: "float64",
//	    FieldSearchType: ">=",
//	}
//
// 生成代码：
//
//	if info.Score != nil {
//	    db = db.Where("score >= ?", *info.Score)
//	}
func GenerateSearchConditions(fields []*systemReq.AutoCodeField) string {
	var conditions []string

	// 遍历所有字段，为每个可搜索字段生成查询条件
	// 使用切片收集条件，最后拼接，好处是逻辑清晰，易于调试
	for _, field := range fields {
		// 跳过没有搜索类型的字段
		if field.FieldSearchType == "" {
			continue
		}

		var condition string

		// 处理复杂类型（JSON、数组、文件等）
		// 这些类型需要特殊处理，因为不能直接使用SQL操作符
		if slices.Contains([]string{"enum", "pictures", "picture", "video", "json", "richtext", "array"}, field.FieldType) {
			if field.FieldType == "enum" {
				// 枚举类型支持LIKE和其他操作符
				// LIKE用于模糊匹配枚举值，其他操作符用于精确匹配
				if field.FieldSearchType == "LIKE" {
					// 使用%%包裹实现模糊查询
					// 好处：可以搜索包含特定字符串的枚举值
					condition = fmt.Sprintf(`
    if info.%s != "" {
        db = db.Where("%s LIKE ?", "%%"+ info.%s+"%%")
    }`,
						field.FieldName, field.ColumnName, field.FieldName)
				} else {
					// 精确匹配或其他操作符
					condition = fmt.Sprintf(`
    if info.%s != "" {
        db = db.Where("%s %s ?", info.%s)
    }`,
						field.FieldName, field.ColumnName, field.FieldSearchType, field.FieldName)
				}
			} else {
				// 其他复杂类型暂不支持自动生成，需要手动实现
				// 这样设计的好处：避免生成错误的代码，提示开发者需要自定义实现
				condition = fmt.Sprintf(`
    if info.%s != "" {
        // TODO 数据类型为复杂类型，请根据业务需求自行实现复杂类型的查询业务
    }`, field.FieldName)
			}

		} else if field.FieldSearchType == "BETWEEN" || field.FieldSearchType == "NOT BETWEEN" {
			// 范围查询：支持时间范围和数值范围
			// 好处：可以查询某个范围内的数据，常用于日期筛选和价格筛选
			if field.FieldType == "time.Time" {
				// 时间范围：使用数组存储开始和结束时间
				// 使用数组的好处：前端可以一次性传递范围，后端统一处理
				condition = fmt.Sprintf(`
			if len(info.%sRange) == 2 {
				db = db.Where("%s %s ? AND ? ", info.%sRange[0], info.%sRange[1])
			}`,
					field.FieldName, field.ColumnName, field.FieldSearchType, field.FieldName, field.FieldName)
			} else {
				// 数值范围：使用Start和End两个字段
				// 使用指针的好处：可以判断是否设置了范围值
				condition = fmt.Sprintf(`
	if info.Start%s != nil && info.End%s != nil {
		db = db.Where("%s %s ? AND ? ", *info.Start%s, *info.End%s)
	}`,
					field.FieldName, field.FieldName, field.ColumnName,
					field.FieldSearchType, field.FieldName, field.FieldName)
			}
		} else {
			// 普通查询：支持LIKE、=、>、<等操作符
			// 先检查指针是否为nil，避免空指针解引用
			nullCheck := "info." + field.FieldName + " != nil"
			if field.FieldType == "string" {
				// 字符串类型：除了检查nil，还要检查是否为空字符串
				// 好处：避免对空字符串进行查询，提高查询效率
				condition = fmt.Sprintf(`
    if %s && *info.%s != "" {`, nullCheck, field.FieldName)
			} else {
				// 其他类型：只需检查nil即可
				condition = fmt.Sprintf(`
    if %s {`, nullCheck)
			}

			if field.FieldSearchType == "LIKE" {
				// LIKE查询：使用%%实现模糊匹配
				// 好处：可以搜索包含特定字符串的记录
				condition += fmt.Sprintf(`
        db = db.Where("%s LIKE ?", "%%"+ *info.%s+"%%")
    }`,
					field.ColumnName, field.FieldName)
			} else {
				// 精确匹配或其他操作符（=、>、<、>=、<=等）
				condition += fmt.Sprintf(`
        db = db.Where("%s %s ?", *info.%s)
    }`,
					field.ColumnName, field.FieldSearchType, field.FieldName)
			}
		}

		// 将生成的条件添加到切片中
		conditions = append(conditions, condition)
	}

	// 将所有条件拼接成完整的代码
	// 使用空字符串连接，因为每个条件本身已经包含了换行和缩进
	return strings.Join(conditions, "")
}

// GenerateSearchFormItem 生成前端搜索表单的HTML代码（Element Plus组件）
//
// 设计意义：
// 1. 根据字段类型自动选择最合适的输入组件（输入框、选择器、日期选择器等）
// 2. 支持范围查询（BETWEEN）的特殊UI处理
// 3. 自动处理数据源、字典等关联数据的选择器
// 4. 生成符合Element Plus规范的HTML代码
//
// 好处：
// - 统一的UI风格，提升用户体验
// - 根据数据类型自动选择组件，减少前端开发工作量
// - 支持复杂的搜索场景（范围查询、多选等）
//
// 示例：
//
// 示例1：布尔类型字段
//
//	field := systemReq.AutoCodeField{
//	    FieldType: "bool",
//	    FieldJson: "isActive",
//	    FieldDesc: "是否激活",
//	}
//	输出：
//	<el-form-item label="是否激活" prop="isActive">
//	  <el-select v-model="searchInfo.isActive" clearable placeholder="请选择">
//	    <el-option key="true" label="是" value="true"></el-option>
//	    <el-option key="false" label="否" value="false"></el-option>
//	  </el-select>
//	</el-form-item>
//
// 示例2：字典类型字段（单选）
//
//	field := systemReq.AutoCodeField{
//	    FieldType: "string",
//	    FieldJson: "status",
//	    FieldDesc: "状态",
//	    DictType: "sys_status",
//	    Clearable: true,
//	}
//	输出：
//	<el-form-item label="状态" prop="status">
//	  <el-tree-select v-model="searchInfo.status" placeholder="请选择状态" :data="sys_statusOptions" style="width:100%" filterable :clearable="true" check-strictly ></el-tree-select>
//	</el-form-item>
//
// 示例3：字典类型字段（多选，数组类型）
//
//	field := systemReq.AutoCodeField{
//	    FieldType: "array",
//	    FieldJson: "tags",
//	    FieldDesc: "标签",
//	    DictType: "article_tags",
//	    Clearable: true,
//	}
//	输出：
//	<el-form-item label="标签" prop="tags">
//	  <el-tree-select v-model="searchInfo.tags" placeholder="请选择标签" :data="article_tagsOptions" style="width:100%" filterable :clearable="true" check-strictly multiple ></el-tree-select>
//	</el-form-item>
//
// 示例4：数据源类型字段（关联表）
//
//	field := systemReq.AutoCodeField{
//	    FieldType: "uint",
//	    FieldJson: "userId",
//	    FieldDesc: "用户",
//	    CheckDataSource: true,
//	    DataSource: systemReq.DataSource{Association: 1},
//	    Clearable: true,
//	}
//	输出：
//	<el-form-item label="用户" prop="userId">
//	  <el-select v-model="searchInfo.userId" filterable placeholder="请选择用户" :clearable="true">
//	    <el-option v-for="(item,key) in dataSource.userId" :key="key" :label="item.label" :value="item.value" />
//	  </el-select>
//	</el-form-item>
//
// 示例5：数值类型字段（精确查询）
//
//	field := systemReq.AutoCodeField{
//	    FieldType: "int",
//	    FieldJson: "age",
//	    FieldDesc: "年龄",
//	    FieldSearchType: "=",
//	}
//	输出：
//	<el-form-item label="年龄" prop="age">
//	  <el-input v-model.number="searchInfo.age" placeholder="搜索条件" />
//	</el-form-item>
//
// 示例6：数值类型字段（范围查询）
//
//	field := systemReq.AutoCodeField{
//	    FieldType: "float64",
//	    FieldJson: "price",
//	    FieldName: "Price",
//	    FieldDesc: "价格",
//	    FieldSearchType: "BETWEEN",
//	}
//	输出：
//	<el-form-item label="价格" prop="price">
//	  <el-input class="!w-40" v-model.number="searchInfo.startPrice" placeholder="最小值" />
//	  —
//	  <el-input class="!w-40" v-model.number="searchInfo.endPrice" placeholder="最大值" />
//	</el-form-item>
//
// 示例7：时间类型字段（单个日期）
//
//	field := systemReq.AutoCodeField{
//	    FieldType: "time.Time",
//	    FieldJson: "createTime",
//	    FieldDesc: "创建时间",
//	    FieldSearchType: "=",
//	}
//	输出：
//	<el-form-item label="创建时间" prop="createTime">
//	  <el-date-picker v-model="searchInfo.createTime" type="datetime" placeholder="搜索条件"></el-date-picker>
//	</el-form-item>
//
// 示例8：时间类型字段（日期范围）
//
//	field := systemReq.AutoCodeField{
//	    FieldType: "time.Time",
//	    FieldJson: "createTime",
//	    FieldDesc: "创建时间",
//	    FieldSearchType: "BETWEEN",
//	}
//	输出：
//	<el-form-item label="创建时间" prop="createTime">
//	  <template #label>
//	    <span>
//	      创建时间
//	      <el-tooltip content="搜索范围是开始日期（包含）至结束日期（不包含）">
//	        <el-icon><QuestionFilled /></el-icon>
//	      </el-tooltip>
//	    </span>
//	  </template>
//	  <el-date-picker class="!w-380px" v-model="searchInfo.createTimeRange" type="datetimerange" range-separator="至"  start-placeholder="开始时间" end-placeholder="结束时间"></el-date-picker>
//	</el-form-item>
//
// 示例9：字符串类型字段（默认）
//
//	field := systemReq.AutoCodeField{
//	    FieldType: "string",
//	    FieldJson: "name",
//	    FieldDesc: "名称",
//	}
//	输出：
//	<el-form-item label="名称" prop="name">
//	  <el-input v-model="searchInfo.name" placeholder="搜索条件" />
//	</el-form-item>
func GenerateSearchFormItem(field systemReq.AutoCodeField) string {
	// 开始构建表单项
	// 使用el-form-item包裹，好处：统一的表单布局和验证机制
	result := fmt.Sprintf(`<el-form-item label="%s" prop="%s">
`, field.FieldDesc, field.FieldJson)

	// 根据字段属性生成不同的输入类型
	// 使用if-else链式判断，按优先级处理：特殊类型 > 数据源 > 字典 > 基础类型
	if field.FieldType == "bool" {
		// 布尔类型：使用下拉选择器，提供"是"/"否"两个选项
		// 好处：用户选择明确，避免输入错误
		result += fmt.Sprintf(`  <el-select v-model="searchInfo.%s" clearable placeholder="请选择">
`, field.FieldJson)
		result += `    <el-option key="true" label="是" value="true"></el-option>
`
		result += `    <el-option key="false" label="否" value="false"></el-option>
`
		result += `  </el-select>
`
	} else if field.DictType != "" {
		// 字典类型：使用树形选择器，支持层级选择
		// 好处：可以处理有层级关系的字典数据（如省市区）
		multipleAttr := ""
		if field.FieldType == "array" {
			// 数组类型支持多选
			multipleAttr = "multiple "
		}
		result += fmt.Sprintf(`    <el-tree-select v-model="searchInfo.%s" placeholder="请选择%s" :data="%sOptions" style="width:100%%" filterable :clearable="%v" check-strictly %s></el-tree-select>
`,
			field.FieldJson, field.FieldDesc, field.DictType, field.Clearable, multipleAttr)
	} else if field.CheckDataSource {
		// 数据源类型：使用下拉选择器，数据来自关联表
		// 好处：可以搜索关联数据，如通过用户ID搜索用户名
		multipleAttr := ""
		if field.DataSource.Association == 2 {
			// 多对多关联支持多选
			multipleAttr = "multiple "
		}
		result += fmt.Sprintf(`  <el-select %sv-model="searchInfo.%s" filterable placeholder="请选择%s" :clearable="%v">
`,
			multipleAttr, field.FieldJson, field.FieldDesc, field.Clearable)
		result += fmt.Sprintf(`    <el-option v-for="(item,key) in dataSource.%s" :key="key" :label="item.label" :value="item.value" />
`,
			field.FieldJson)
		result += `  </el-select>
`
	} else if field.FieldType == "float64" || field.FieldType == "int" {
		// 数值类型：支持范围查询和精确查询
		if field.FieldSearchType == "BETWEEN" || field.FieldSearchType == "NOT BETWEEN" {
			// 范围查询：提供最小值和最大值两个输入框
			// 好处：用户可以输入范围，查询更灵活
			result += fmt.Sprintf(`  <el-input class="!w-40" v-model.number="searchInfo.start%s" placeholder="最小值" />
`, field.FieldName)
			result += `  —
`
			result += fmt.Sprintf(`  <el-input class="!w-40" v-model.number="searchInfo.end%s" placeholder="最大值" />
`, field.FieldName)
		} else {
			// 精确查询：单个输入框
			// 使用v-model.number确保输入的是数字类型
			result += fmt.Sprintf(`  <el-input v-model.number="searchInfo.%s" placeholder="搜索条件" />
`, field.FieldJson)
		}
	} else if field.FieldType == "time.Time" {
		// 时间类型：支持日期范围选择和单个日期选择
		if field.FieldSearchType == "BETWEEN" || field.FieldSearchType == "NOT BETWEEN" {
			// 日期范围：使用日期范围选择器
			// 添加提示信息，说明范围查询的规则（包含开始，不包含结束）
			result += `  <template #label>
`
			result += `    <span>
`
			result += fmt.Sprintf(`      %s
`, field.FieldDesc)
			result += `      <el-tooltip content="搜索范围是开始日期（包含）至结束日期（不包含）">
`
			result += `        <el-icon><QuestionFilled /></el-icon>
`
			result += `      </el-tooltip>
`
			result += `    </span>
`
			result += `  </template>
`
			result += fmt.Sprintf(`<el-date-picker class="!w-380px" v-model="searchInfo.%sRange" type="datetimerange" range-separator="至"  start-placeholder="开始时间" end-placeholder="结束时间"></el-date-picker>`, field.FieldJson)
		} else {
			// 单个日期：使用日期时间选择器
			result += fmt.Sprintf(`<el-date-picker v-model="searchInfo.%s" type="datetime" placeholder="搜索条件"></el-date-picker>`, field.FieldJson)
		}
	} else {
		// 默认类型（主要是string）：使用普通输入框
		// 支持模糊搜索（LIKE查询）
		result += fmt.Sprintf(`  <el-input v-model="searchInfo.%s" placeholder="搜索条件" />
`, field.FieldJson)
	}

	// 关闭表单项
	result += `</el-form-item>`

	return result
}

// GenerateTableColumn 生成前端表格列的HTML代码（Element Plus表格组件）
//
// 设计意义：
// 1. 根据字段类型自动选择最合适的展示方式（文本、图片、标签、格式化等）
// 2. 支持排序功能（根据字段配置）
// 3. 处理复杂类型的展示（图片预览、文件下载、富文本等）
// 4. 自动处理数据源和字典的显示转换
//
// 好处：
// - 统一的表格展示风格
// - 根据数据类型自动选择展示组件，提升用户体验
// - 支持图片预览、文件下载等交互功能
// - 自动处理数据转换（如字典值显示、布尔值格式化等）
//
// 示例：
//
// 示例1：基础文本字段（带排序）
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc: "用户名",
//	    FieldJson: "username",
//	    FieldType: "string",
//	    Sort:      true,
//	}
//	输出: <el-table-column sortable align="left" label="用户名" prop="username" width="120" />
//
// 示例2：布尔类型字段
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc: "是否启用",
//	    FieldJson: "enabled",
//	    FieldType: "bool",
//	}
//	输出: <el-table-column align="left" label="是否启用" prop="enabled" width="120">
//	     <template #default="scope">{{ formatBoolean(scope.row.enabled) }}</template>
//	     </el-table-column>
//
// 示例3：时间类型字段
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc: "创建时间",
//	    FieldJson: "createdAt",
//	    FieldType: "time.Time",
//	    Sort:      true,
//	}
//	输出: <el-table-column sortable align="left" label="创建时间" prop="createdAt" width="180">
//	     <template #default="scope">{{ formatDate(scope.row.createdAt) }}</template>
//	     </el-table-column>
//
// 示例4：字典类型字段（单个值）
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc: "状态",
//	    FieldJson: "status",
//	    FieldType: "string",
//	    DictType:  "status",
//	}
//	输出: <el-table-column align="left" label="状态" prop="status" width="120">
//	     <template #default="scope">
//	     {{ filterDict(scope.row.status,statusOptions) }}
//	     </template>
//	     </el-table-column>
//
// 示例5：字典类型字段（数组值）
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc: "标签",
//	    FieldJson: "tags",
//	    FieldType: "array",
//	    DictType:  "tag",
//	}
//	输出: <el-table-column align="left" label="标签" prop="tags" width="120">
//	     <template #default="scope">
//	     <el-tag class="mr-1" v-for="item in scope.row.tags" :key="item"> {{ filterDict(item,tagOptions) }}</el-tag>
//	     </template>
//	     </el-table-column>
//
// 示例6：数据源类型字段（一对一关联）
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc:       "部门",
//	    FieldJson:       "departmentId",
//	    FieldType:       "uint",
//	    CheckDataSource: true,
//	    DataSource: &systemReq.DataSource{
//	        Association: 1,
//	    },
//	}
//	输出: <el-table-column align="left" label="部门" prop="departmentId" width="120">
//	     <template #default="scope">
//	     <span>{{ filterDataSource(dataSource.departmentId,scope.row.departmentId) }}</span>
//	     </template>
//	     </el-table-column>
//
// 示例7：数据源类型字段（多对多关联）
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc:       "角色",
//	    FieldJson:       "roleIds",
//	    FieldType:       "array",
//	    CheckDataSource: true,
//	    DataSource: &systemReq.DataSource{
//	        Association: 2,
//	    },
//	}
//	输出: <el-table-column align="left" label="角色" prop="roleIds" width="120">
//	     <template #default="scope">
//	     <el-tag v-for="(item,key) in filterDataSource(dataSource.roleIds,scope.row.roleIds)" :key="key">
//	          {{ item }}
//	     </el-tag>
//	     </template>
//	     </el-table-column>
//
// 示例8：图片类型字段（单张）
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc: "头像",
//	    FieldJson: "avatar",
//	    FieldType: "picture",
//	}
//	输出: <el-table-column label="头像" prop="avatar" width="200">
//	     <template #default="scope">
//	     <el-image preview-teleported style="width: 100px; height: 100px" :src="getUrl(scope.row.avatar)" fit="cover"/>
//	     </template>
//	     </el-table-column>
//
// 示例9：图片类型字段（多张）
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc: "相册",
//	    FieldJson: "photos",
//	    FieldType: "pictures",
//	}
//	输出: <el-table-column label="相册" prop="photos" width="200">
//	     <template #default="scope">
//	     <div class="multiple-img-box">
//	     <el-image preview-teleported v-for="(item,index) in scope.row.photos" :key="index" style="width: 80px; height: 80px" :src="getUrl(item)" fit="cover"/>
//	     </div>
//	     </template>
//	     </el-table-column>
//
// 示例10：文件类型字段
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc: "附件",
//	    FieldJson: "attachments",
//	    FieldType: "file",
//	}
//	输出: <el-table-column label="附件" prop="attachments" width="200">
//	     <template #default="scope">
//	     <div class="file-list">
//	     <el-tag v-for="file in scope.row.attachments" :key="file.uid" @click="onDownloadFile(file.url)">{{ file.name }}</el-tag>
//	     </div>
//	     </template>
//	     </el-table-column>
//
// 示例11：视频类型字段
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc: "视频",
//	    FieldJson: "videoUrl",
//	    FieldType: "video",
//	}
//	输出: <el-table-column label="视频" prop="videoUrl" width="200">
//	     <template #default="scope">
//	     <video style="width: 100px; height: 100px" muted preload="metadata">
//	     <source :src="getUrl(scope.row.videoUrl) + '#t=1'">
//	     </video>
//	     </template>
//	     </el-table-column>
//
// 示例12：富文本类型字段
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc: "内容",
//	    FieldJson: "content",
//	    FieldType: "richtext",
//	}
//	输出: <el-table-column label="内容" prop="content" width="200">
//	     <template #default="scope">
//	     [富文本内容]
//	     </template>
//	     </el-table-column>
//
// 示例13：JSON类型字段
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc: "配置",
//	    FieldJson: "config",
//	    FieldType: "json",
//	}
//	输出: <el-table-column label="配置" prop="config" width="200">
//	     <template #default="scope">
//	     [JSON]
//	     </template>
//	     </el-table-column>
//
// 示例14：数组类型字段（非字典）
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc: "标签列表",
//	    FieldJson: "tagList",
//	    FieldType: "array",
//	}
//	输出: <el-table-column label="标签列表" prop="tagList" width="200">
//	     <template #default="scope">
//	     <ArrayCtrl v-model="scope.row.tagList"/>
//	     </template>
//	     </el-table-column>
func GenerateTableColumn(field systemReq.AutoCodeField) string {
	// 添加排序属性（如果需要）
	// 好处：用户可以点击列头进行排序，提升数据浏览体验
	sortAttr := ""
	if field.Sort {
		sortAttr = " sortable"
	}

	// 根据字段类型处理不同的展示方式
	// 优先级：数据源 > 字典 > 特殊类型 > 基础类型
	if field.CheckDataSource {
		// 数据源类型：通过filterDataSource函数将ID转换为显示名称
		// 好处：表格显示友好的名称而不是ID，提升可读性
		result := fmt.Sprintf(`<el-table-column%s align="left" label="%s" prop="%s" width="120">
`,
			sortAttr, field.FieldDesc, field.FieldJson)
		result += `    <template #default="scope">
`

		if field.DataSource.Association == 2 {
			// 多对多关联：使用标签展示多个值
			// 好处：多个关联值可以清晰展示，不会混淆
			result += fmt.Sprintf(`        <el-tag v-for="(item,key) in filterDataSource(dataSource.%s,scope.row.%s)" :key="key">
`,
				field.FieldJson, field.FieldJson)
			result += `             {{ item }}
`
			result += `        </el-tag>
`
		} else {
			// 一对一或一对多关联：直接显示名称
			result += fmt.Sprintf(`        <span>{{ filterDataSource(dataSource.%s,scope.row.%s) }}</span>
`,
				field.FieldJson, field.FieldJson)
		}

		result += `    </template>
`
		result += `</el-table-column>`
		return result
	} else if field.DictType != "" {
		// 字典类型：通过filterDict函数将字典值转换为显示文本
		// 好处：显示字典的label而不是value，更友好
		result := fmt.Sprintf(`<el-table-column%s align="left" label="%s" prop="%s" width="120">
`,
			sortAttr, field.FieldDesc, field.FieldJson)
		result += `    <template #default="scope">
`

		if field.FieldType == "array" {
			// 数组类型：使用多个标签展示
			result += fmt.Sprintf(`    <el-tag class="mr-1" v-for="item in scope.row.%s" :key="item"> {{ filterDict(item,%sOptions) }}</el-tag>
`,
				field.FieldJson, field.DictType)
		} else {
			// 单个值：直接显示
			result += fmt.Sprintf(`    {{ filterDict(scope.row.%s,%sOptions) }}
`,
				field.FieldJson, field.DictType)
		}

		result += `    </template>
`
		result += `</el-table-column>`
		return result
	} else if field.FieldType == "bool" {
		// 布尔类型：使用formatBoolean函数格式化显示
		// 好处：显示"是"/"否"而不是true/false，更符合中文习惯
		result := fmt.Sprintf(`<el-table-column%s align="left" label="%s" prop="%s" width="120">
`,
			sortAttr, field.FieldDesc, field.FieldJson)
		result += fmt.Sprintf(`    <template #default="scope">{{ formatBoolean(scope.row.%s) }}</template>
`, field.FieldJson)
		result += `</el-table-column>`
		return result
	} else if field.FieldType == "time.Time" {
		// 时间类型：使用formatDate函数格式化显示
		// 好处：统一的时间格式，易于阅读
		// 宽度设为180，因为时间字符串较长
		result := fmt.Sprintf(`<el-table-column%s align="left" label="%s" prop="%s" width="180">
`,
			sortAttr, field.FieldDesc, field.FieldJson)
		result += fmt.Sprintf(`   <template #default="scope">{{ formatDate(scope.row.%s) }}</template>
`, field.FieldJson)
		result += `</el-table-column>`
		return result
	} else if field.FieldType == "picture" {
		// 单张图片：使用el-image组件，支持预览功能
		// preview-teleported：预览时挂载到body，避免z-index问题
		// getUrl：处理相对路径和绝对路径
		result := fmt.Sprintf(`<el-table-column label="%s" prop="%s" width="200">
`, field.FieldDesc, field.FieldJson)
		result += `    <template #default="scope">
`
		result += fmt.Sprintf(`      <el-image preview-teleported style="width: 100px; height: 100px" :src="getUrl(scope.row.%s)" fit="cover"/>
`, field.FieldJson)
		result += `    </template>
`
		result += `</el-table-column>`
		return result
	} else if field.FieldType == "pictures" {
		// 多张图片：循环显示多张图片，每张都可以预览
		// 好处：可以在表格中直接看到所有图片，点击可预览大图
		result := fmt.Sprintf(`<el-table-column label="%s" prop="%s" width="200">
`, field.FieldDesc, field.FieldJson)
		result += `   <template #default="scope">
`
		result += `      <div class="multiple-img-box">
`
		result += fmt.Sprintf(`         <el-image preview-teleported v-for="(item,index) in scope.row.%s" :key="index" style="width: 80px; height: 80px" :src="getUrl(item)" fit="cover"/>
`, field.FieldJson)
		result += `     </div>
`
		result += `   </template>
`
		result += `</el-table-column>`
		return result
	} else if field.FieldType == "video" {
		// 视频：使用video标签显示视频缩略图
		// muted：静音，避免自动播放声音
		// preload="metadata"：只加载元数据，不加载完整视频，节省带宽
		// #t=1：跳转到第1秒，显示视频的第一帧作为缩略图
		result := fmt.Sprintf(`<el-table-column label="%s" prop="%s" width="200">
`, field.FieldDesc, field.FieldJson)
		result += `   <template #default="scope">
`
		result += `    <video
`
		result += `       style="width: 100px; height: 100px"
`
		result += `       muted
`
		result += `       preload="metadata"
`
		result += `       >
`
		result += fmt.Sprintf(`         <source :src="getUrl(scope.row.%s) + '#t=1'">
`, field.FieldJson)
		result += `       </video>
`
		result += `   </template>
`
		result += `</el-table-column>`
		return result
	} else if field.FieldType == "richtext" {
		// 富文本：表格中只显示占位符，因为富文本内容较长不适合在表格中显示
		// 好处：避免表格行高过大，影响浏览体验
		result := fmt.Sprintf(`<el-table-column label="%s" prop="%s" width="200">
`, field.FieldDesc, field.FieldJson)
		result += `   <template #default="scope">
`
		result += `      [富文本内容]
`
		result += `   </template>
`
		result += `</el-table-column>`
		return result
	} else if field.FieldType == "file" {
		// 文件：显示文件列表，点击可下载
		// 好处：用户可以直接在表格中看到文件，并快速下载
		result := fmt.Sprintf(`<el-table-column label="%s" prop="%s" width="200">
`, field.FieldDesc, field.FieldJson)
		result += `    <template #default="scope">
`
		result += `         <div class="file-list">
`
		result += fmt.Sprintf(`           <el-tag v-for="file in scope.row.%s" :key="file.uid" @click="onDownloadFile(file.url)">{{ file.name }}</el-tag>
`, field.FieldJson)
		result += `         </div>
`
		result += `    </template>
`
		result += `</el-table-column>`
		return result
	} else if field.FieldType == "json" {
		// JSON类型：表格中只显示占位符
		// 好处：JSON结构复杂，不适合在表格中直接显示，可以在详情页查看
		result := fmt.Sprintf(`<el-table-column label="%s" prop="%s" width="200">
`, field.FieldDesc, field.FieldJson)
		result += `    <template #default="scope">
`
		result += `        [JSON]
`
		result += `    </template>
`
		result += `</el-table-column>`
		return result
	} else if field.FieldType == "array" {
		// 数组类型：使用ArrayCtrl组件显示
		// 好处：可以以列表形式展示数组内容
		result := fmt.Sprintf(`<el-table-column label="%s" prop="%s" width="200">
`, field.FieldDesc, field.FieldJson)
		result += `    <template #default="scope">
`
		result += fmt.Sprintf(`       <ArrayCtrl v-model="scope.row.%s"/>
`, field.FieldJson)
		result += `    </template>
`
		result += `</el-table-column>`
		return result
	} else {
		// 默认类型（主要是string、int、float64等）：直接显示值
		// 使用自闭合标签，代码更简洁
		return fmt.Sprintf(`<el-table-column%s align="left" label="%s" prop="%s" width="120" />
`,
			sortAttr, field.FieldDesc, field.FieldJson)
	}
}

// GenerateFormItem 生成前端表单输入项的HTML代码（Element Plus表单组件）
//
// 设计意义：
// 1. 根据字段类型自动选择最合适的输入组件（输入框、选择器、日期选择器、文件上传等）
// 2. 支持数据源、字典等关联数据的选择器
// 3. 处理特殊类型（图片、文件、富文本等）的专用组件
// 4. 生成符合Element Plus规范的HTML代码
//
// 好处：
// - 统一的表单风格，提升用户体验
// - 根据数据类型自动选择组件，减少前端开发工作量
// - 支持复杂的输入场景（多选、文件上传、富文本编辑等）
//
// 示例：
//
// 示例1：数据源类型（关联表选择器）
//
//	field := systemReq.AutoCodeField{
//	    FieldJson: "userId",
//	    FieldDesc: "用户",
//	    FieldType: "uint",
//	    CheckDataSource: true,
//	    DataSource: systemReq.DataSource{Association: 1},
//	    Clearable: true,
//	}
//
// 输出：
// <el-form-item label="用户:" prop="userId">
//
//	<el-select v-model="formData.userId" placeholder="请选择用户" filterable style="width:100%" :clearable="true">
//	    <el-option v-for="(item,key) in dataSource.userId" :key="key" :label="item.label" :value="item.value" />
//	</el-select>
//
// </el-form-item>
//
// 示例2：布尔类型（开关组件）
//
//	field := systemReq.AutoCodeField{
//	    FieldJson: "isActive",
//	    FieldDesc: "是否启用",
//	    FieldType: "bool",
//	    Clearable: true,
//	}
//
// 输出：
// <el-form-item label="是否启用:" prop="isActive">
//
//	<el-switch v-model="formData.isActive" active-color="#13ce66" inactive-color="#ff4949" active-text="是" inactive-text="否" clearable ></el-switch>
//
// </el-form-item>
//
// 示例3：字符串类型（普通输入框）
//
//	field := systemReq.AutoCodeField{
//	    FieldJson: "userName",
//	    FieldDesc: "用户名",
//	    FieldType: "string",
//	    DictType: "",
//	    Clearable: true,
//	}
//
// 输出：
// <el-form-item label="用户名:" prop="userName">
//
//	<el-input v-model="formData.userName" :clearable="true" placeholder="请输入用户名" />
//
// </el-form-item>
//
// 示例4：字符串类型（带字典的树形选择器）
//
//	field := systemReq.AutoCodeField{
//	    FieldJson: "categoryId",
//	    FieldDesc: "分类",
//	    FieldType: "string",
//	    DictType: "category",
//	    Clearable: true,
//	}
//
// 输出：
// <el-form-item label="分类:" prop="categoryId">
//
//	<el-tree-select v-model="formData.categoryId" placeholder="请选择分类" :data="categoryOptions" style="width:100%" filterable :clearable="true" check-strictly></el-tree-select>
//
// </el-form-item>
//
// 示例5：整数类型
//
//	field := systemReq.AutoCodeField{
//	    FieldJson: "age",
//	    FieldDesc: "年龄",
//	    FieldType: "int",
//	    Clearable: true,
//	}
//
// 输出：
// <el-form-item label="年龄:" prop="age">
//
//	<el-input v-model.number="formData.age" :clearable="true" placeholder="请输入年龄" />
//
// </el-form-item>
//
// 示例6：浮点数类型
//
//	field := systemReq.AutoCodeField{
//	    FieldJson: "price",
//	    FieldDesc: "价格",
//	    FieldType: "float64",
//	    Clearable: true,
//	}
//
// 输出：
// <el-form-item label="价格:" prop="price">
//
//	<el-input-number v-model="formData.price" style="width:100%" :precision="2" :clearable="true" />
//
// </el-form-item>
//
// 示例7：时间类型
//
//	field := systemReq.AutoCodeField{
//	    FieldJson: "birthday",
//	    FieldDesc: "生日",
//	    FieldType: "time.Time",
//	    Clearable: true,
//	}
//
// 输出：
// <el-form-item label="生日:" prop="birthday">
//
//	<el-date-picker v-model="formData.birthday" type="date" style="width:100%" placeholder="选择日期" :clearable="true" />
//
// </el-form-item>
//
// 示例8：单张图片
//
//	field := systemReq.AutoCodeField{
//	    FieldJson: "avatar",
//	    FieldDesc: "头像",
//	    FieldType: "picture",
//	}
//
// 输出：
// <el-form-item label="头像:" prop="avatar">
//
//	 <SelectImage
//	 v-model="formData.avatar"
//	 file-type="image"
//	/>
//
// </el-form-item>
//
// 示例9：多张图片
//
//	field := systemReq.AutoCodeField{
//	    FieldJson: "gallery",
//	    FieldDesc: "图片集",
//	    FieldType: "pictures",
//	}
//
// 输出：
// <el-form-item label="图片集:" prop="gallery">
//
//	<SelectImage
//	multiple
//	v-model="formData.gallery"
//	file-type="image"
//	/>
//
// </el-form-item>
//
// 示例10：富文本类型
//
//	field := systemReq.AutoCodeField{
//	    FieldJson: "content",
//	    FieldDesc: "内容",
//	    FieldType: "richtext",
//	}
//
// 输出：
// <el-form-item label="内容:" prop="content">
//
//	<RichEdit v-model="formData.content"/>
//
// </el-form-item>
//
// 示例11：枚举类型
//
//	field := systemReq.AutoCodeField{
//	    FieldJson: "status",
//	    FieldDesc: "状态",
//	    FieldType: "enum",
//	    DataTypeLong: "'pending','active','inactive'",
//	    Clearable: true,
//	}
//
// 输出：
// <el-form-item label="状态:" prop="status">
//
//	<el-select v-model="formData.status" placeholder="请选择状态" style="width:100%" filterable :clearable="true">
//	   <el-option v-for="item in ['pending','active','inactive']" :key="item" :label="item" :value="item" />
//	</el-select>
//
// </el-form-item>
//
// 示例12：数组类型（带字典的多选下拉框）
//
//	field := systemReq.AutoCodeField{
//	    FieldJson: "tags",
//	    FieldDesc: "标签",
//	    FieldType: "array",
//	    DictType: "tag",
//	    Clearable: true,
//	}
//
// 输出：
// <el-form-item label="标签:" prop="tags">
//
//	<el-select multiple v-model="formData.tags" placeholder="请选择标签" filterable style="width:100%" :clearable="true">
//	    <el-option v-for="(item,key) in tagOptions" :key="key" :label="item.label" :value="item.value" />
//	</el-select>
//
// </el-form-item>
//
// 示例13：数组类型（无字典的数组控制器）
//
//	field := systemReq.AutoCodeField{
//	    FieldJson: "tags",
//	    FieldDesc: "标签",
//	    FieldType: "array",
//	    DictType: "",
//	}
//
// 输出：
// <el-form-item label="标签:" prop="tags">
//
//	<ArrayCtrl v-model="formData.tags" editable/>
//
// </el-form-item>
//
// 示例14：文件类型
//
//	field := systemReq.AutoCodeField{
//	    FieldJson: "attachment",
//	    FieldDesc: "附件",
//	    FieldType: "file",
//	}
//
// 输出：
// <el-form-item label="附件:" prop="attachment">
//
//	<SelectFile v-model="formData.attachment" />
//
// </el-form-item>
//
// 示例15：JSON类型
//
//	field := systemReq.AutoCodeField{
//	    FieldJson: "config",
//	    FieldDesc: "配置",
//	    FieldType: "json",
//	}
//
// 输出：
// <el-form-item label="配置:" prop="config">
//
//	// 此字段为json结构，可以前端自行控制展示和数据绑定模式 需绑定json的key为 formData.config 后端会按照json的类型进行存取
//	{{ formData.config }}
//
// </el-form-item>
func GenerateFormItem(field systemReq.AutoCodeField) string {
	// 开始构建表单项
	// 使用el-form-item包裹，好处：统一的表单布局、验证和错误提示
	result := fmt.Sprintf(`<el-form-item label="%s:" prop="%s">
`, field.FieldDesc, field.FieldJson)

	// 处理不同字段类型
	// 优先级：数据源 > 字典 > 特殊类型 > 基础类型
	if field.CheckDataSource {
		// 数据源类型：使用下拉选择器，数据来自关联表
		// 好处：可以从关联表中选择数据，避免手动输入ID
		multipleAttr := ""
		if field.DataSource.Association == 2 {
			// 多对多关联支持多选
			multipleAttr = " multiple"
		}
		result += fmt.Sprintf(`    <el-select%s v-model="formData.%s" placeholder="请选择%s" filterable style="width:100%%" :clearable="%v">
`,
			multipleAttr, field.FieldJson, field.FieldDesc, field.Clearable)
		result += fmt.Sprintf(`        <el-option v-for="(item,key) in dataSource.%s" :key="key" :label="item.label" :value="item.value" />
`,
			field.FieldJson)
		result += `    </el-select>
`
	} else {
		// 使用switch处理各种基础类型和特殊类型
		switch field.FieldType {
		case "bool":
			// 布尔类型：使用开关组件
			// 好处：视觉直观，操作简单
			result += fmt.Sprintf(`    <el-switch v-model="formData.%s" active-color="#13ce66" inactive-color="#ff4949" active-text="是" inactive-text="否" clearable ></el-switch>
`,
				field.FieldJson)

		case "string":
			// 字符串类型：根据是否有字典选择不同的组件
			if field.DictType != "" {
				// 有字典：使用树形选择器
				result += fmt.Sprintf(`    <el-tree-select v-model="formData.%s" placeholder="请选择%s" :data="%sOptions" style="width:100%%" filterable :clearable="%v" check-strictly></el-tree-select>
`,
					field.FieldJson, field.FieldDesc, field.DictType, field.Clearable)
			} else {
				// 无字典：使用普通输入框
				result += fmt.Sprintf(`    <el-input v-model="formData.%s" :clearable="%v" placeholder="请输入%s" />
`,
					field.FieldJson, field.Clearable, field.FieldDesc)
			}

		case "richtext":
			// 富文本：使用专用富文本编辑器组件
			// 好处：提供完整的富文本编辑功能（加粗、斜体、列表等）
			result += fmt.Sprintf(`    <RichEdit v-model="formData.%s"/>
`, field.FieldJson)

		case "json":
			// JSON类型：只生成注释和占位符，由开发者自行实现
			// 好处：JSON结构灵活，不同业务需求不同，不适合自动生成
			result += fmt.Sprintf(`    // 此字段为json结构，可以前端自行控制展示和数据绑定模式 需绑定json的key为 formData.%s 后端会按照json的类型进行存取
`, field.FieldJson)
			result += fmt.Sprintf(`    {{ formData.%s }}
`, field.FieldJson)

		case "array":
			// 数组类型：根据是否有字典选择不同的组件
			if field.DictType != "" {
				// 有字典：使用多选下拉框
				result += fmt.Sprintf(`    <el-select multiple v-model="formData.%s" placeholder="请选择%s" filterable style="width:100%%" :clearable="%v">
`,
					field.FieldJson, field.FieldDesc, field.Clearable)
				result += fmt.Sprintf(`        <el-option v-for="(item,key) in %sOptions" :key="key" :label="item.label" :value="item.value" />
`,
					field.DictType)
				result += `    </el-select>
`
			} else {
				// 无字典：使用数组控制器组件
				// editable：允许编辑数组内容
				result += fmt.Sprintf(`    <ArrayCtrl v-model="formData.%s" editable/>
`, field.FieldJson)
			}

		case "int":
			// 整数类型：使用数字输入框
			// v-model.number：确保输入的是数字类型
			result += fmt.Sprintf(`    <el-input v-model.number="formData.%s" :clearable="%v" placeholder="请输入%s" />
`,
				field.FieldJson, field.Clearable, field.FieldDesc)

		case "time.Time":
			// 时间类型：使用日期选择器
			// type="date"：只选择日期，不选择时间
			result += fmt.Sprintf(`    <el-date-picker v-model="formData.%s" type="date" style="width:100%%" placeholder="选择日期" :clearable="%v" />
`,
				field.FieldJson, field.Clearable)

		case "float64":
			// 浮点数类型：使用数字输入框
			// precision="2"：保留2位小数
			result += fmt.Sprintf(`    <el-input-number v-model="formData.%s" style="width:100%%" :precision="2" :clearable="%v" />
`,
				field.FieldJson, field.Clearable)

		case "enum":
			// 枚举类型：使用下拉选择器，选项来自DataTypeLong（枚举值列表）
			result += fmt.Sprintf(`    <el-select v-model="formData.%s" placeholder="请选择%s" style="width:100%%" filterable :clearable="%v">
`,
				field.FieldJson, field.FieldDesc, field.Clearable)
			result += fmt.Sprintf(`       <el-option v-for="item in [%s]" :key="item" :label="item" :value="item" />
`,
				field.DataTypeLong)
			result += `    </el-select>
`

		case "picture":
			// 单张图片：使用图片选择组件
			// 好处：提供图片上传、预览、裁剪等功能
			result += fmt.Sprintf(`    <SelectImage
     v-model="formData.%s"
     file-type="image"
    />
`, field.FieldJson)

		case "pictures":
			// 多张图片：使用图片选择组件，支持多选
			result += fmt.Sprintf(`    <SelectImage
     multiple
     v-model="formData.%s"
     file-type="image"
     />
`, field.FieldJson)

		case "video":
			// 视频：使用文件选择组件，指定文件类型为视频
			result += fmt.Sprintf(`    <SelectImage
    v-model="formData.%s"
    file-type="video"
    />
`, field.FieldJson)

		case "file":
			// 文件：使用文件选择组件
			// 好处：支持多种文件类型，提供上传进度、文件列表等功能
			result += fmt.Sprintf(`    <SelectFile v-model="formData.%s" />
`, field.FieldJson)
		}
	}

	// 关闭表单项
	result += `</el-form-item>`

	return result
}

// GenerateDescriptionItem 生成详情页描述项的HTML代码（Element Plus描述列表组件）
//
// 设计意义：
// 1. 用于详情页展示数据，只读模式
// 2. 根据字段类型选择最合适的展示方式
// 3. 处理复杂类型的展示（图片预览、文件下载、富文本等）
// 4. 自动处理数据源和字典的显示转换
//
// 好处：
// - 统一的详情页展示风格
// - 根据数据类型自动选择展示组件
// - 支持图片预览、文件下载等交互功能
//
// 示例：
//
// 示例1：基础文本类型字段
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc: "用户名",
//	    FieldJson: "username",
//	    FieldType: "string",
//	    CheckDataSource: false,
//	}
//	输出：
//	<el-descriptions-item label="用户名">
//	    {{ detailForm.username }}
//	</el-descriptions-item>
//
// 示例2：数据源类型（一对一关联）
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc: "部门",
//	    FieldJson: "departmentId",
//	    FieldType: "int",
//	    CheckDataSource: true,
//	    DataSource: systemReq.DataSource{Association: 1},
//	}
//	输出：
//	<el-descriptions-item label="部门">
//	    <template #default="scope">
//	        <span>{{ filterDataSource(dataSource.departmentId,detailForm.departmentId) }}</span>
//	    </template>
//	</el-descriptions-item>
//
// 示例3：数据源类型（多对多关联）
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc: "标签",
//	    FieldJson: "tags",
//	    FieldType: "array",
//	    CheckDataSource: true,
//	    DataSource: systemReq.DataSource{Association: 2},
//	}
//	输出：
//	<el-descriptions-item label="标签">
//	    <template #default="scope">
//	        <el-tag v-for="(item,key) in filterDataSource(dataSource.tags,detailForm.tags)" :key="key">
//	             {{ item }}
//	        </el-tag>
//	    </template>
//	</el-descriptions-item>
//
// 示例4：单张图片类型
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc: "头像",
//	    FieldJson: "avatar",
//	    FieldType: "picture",
//	    CheckDataSource: false,
//	}
//	输出：
//	<el-descriptions-item label="头像">
//	    <el-image style="width: 50px; height: 50px" :preview-src-list="returnArrImg(detailForm.avatar)" :src="getUrl(detailForm.avatar)" fit="cover" />
//	</el-descriptions-item>
//
// 示例5：多张图片类型
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc: "产品图片",
//	    FieldJson: "productImages",
//	    FieldType: "pictures",
//	    CheckDataSource: false,
//	}
//	输出：
//	<el-descriptions-item label="产品图片">
//	    <el-image style="width: 50px; height: 50px; margin-right: 10px" :preview-src-list="returnArrImg(detailForm.productImages)" :initial-index="index" v-for="(item,index) in detailForm.productImages" :key="index" :src="getUrl(item)" fit="cover" />
//	</el-descriptions-item>
//
// 示例6：文件类型
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc: "附件",
//	    FieldJson: "attachments",
//	    FieldType: "file",
//	    CheckDataSource: false,
//	}
//	输出：
//	<el-descriptions-item label="附件">
//	    <div class="fileBtn" v-for="(item,index) in detailForm.attachments" :key="index">
//	        <el-button type="primary" text bg @click="onDownloadFile(item.url)">
//	          <el-icon style="margin-right: 5px"><Download /></el-icon>
//	          {{ item.name }}
//	        </el-button>
//	    </div>
//	</el-descriptions-item>
//
// 示例7：富文本类型
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc: "内容",
//	    FieldJson: "content",
//	    FieldType: "richtext",
//	    CheckDataSource: false,
//	}
//	输出：
//	<el-descriptions-item label="内容">
//	    <RichView v-model="detailForm.content" />
//	</el-descriptions-item>
//
// 示例8：数组类型
//
//	field := systemReq.AutoCodeField{
//	    FieldDesc: "技能列表",
//	    FieldJson: "skills",
//	    FieldType: "array",
//	    CheckDataSource: false,
//	}
//	输出：
//	<el-descriptions-item label="技能列表">
//	    <ArrayCtrl v-model="detailForm.skills"/>
//	</el-descriptions-item>
func GenerateDescriptionItem(field systemReq.AutoCodeField) string {
	// 开始构建描述项
	// 使用el-descriptions-item，好处：统一的详情页布局，左右对齐美观
	result := fmt.Sprintf(`<el-descriptions-item label="%s">
`, field.FieldDesc)

	if field.CheckDataSource {
		// 数据源类型：通过filterDataSource函数转换显示
		result += `    <template #default="scope">
`
		if field.DataSource.Association == 2 {
			// 多对多关联：使用标签展示多个值
			result += fmt.Sprintf(`        <el-tag v-for="(item,key) in filterDataSource(dataSource.%s,detailForm.%s)" :key="key">
`,
				field.FieldJson, field.FieldJson)
			result += `             {{ item }}
`
			result += `        </el-tag>
`
		} else {
			// 一对一或一对多关联：直接显示名称
			result += fmt.Sprintf(`        <span>{{ filterDataSource(dataSource.%s,detailForm.%s) }}</span>
`,
				field.FieldJson, field.FieldJson)
		}
		result += `    </template>
`
	} else if field.FieldType != "picture" && field.FieldType != "pictures" &&
		field.FieldType != "file" && field.FieldType != "array" &&
		field.FieldType != "richtext" {
		// 基础类型：直接显示值
		// 好处：简单直接，性能好
		result += fmt.Sprintf(`    {{ detailForm.%s }}
`, field.FieldJson)
	} else {
		// 特殊类型：使用专用组件展示
		switch field.FieldType {
		case "picture":
			// 单张图片：使用图片组件，支持预览
			// returnArrImg：将单张图片转换为数组，用于预览功能
			result += fmt.Sprintf(`    <el-image style="width: 50px; height: 50px" :preview-src-list="returnArrImg(detailForm.%s)" :src="getUrl(detailForm.%s)" fit="cover" />
`,
				field.FieldJson, field.FieldJson)
		case "array":
			// 数组类型：使用数组控制器组件展示
			result += fmt.Sprintf(`    <ArrayCtrl v-model="detailForm.%s"/>
`, field.FieldJson)
		case "pictures":
			// 多张图片：循环显示，每张都可以预览
			// initial-index：指定预览时的初始索引
			result += fmt.Sprintf(`    <el-image style="width: 50px; height: 50px; margin-right: 10px" :preview-src-list="returnArrImg(detailForm.%s)" :initial-index="index" v-for="(item,index) in detailForm.%s" :key="index" :src="getUrl(item)" fit="cover" />
`,
				field.FieldJson, field.FieldJson)
		case "richtext":
			// 富文本：使用富文本查看组件
			// 好处：可以正确渲染HTML内容，而不是显示HTML标签
			result += fmt.Sprintf(`    <RichView v-model="detailForm.%s" />
`, field.FieldJson)
		case "file":
			// 文件：显示文件列表，每个文件都可以下载
			// 好处：用户可以直接在详情页下载文件
			result += fmt.Sprintf(`    <div class="fileBtn" v-for="(item,index) in detailForm.%s" :key="index">
`, field.FieldJson)
			result += `        <el-button type="primary" text bg @click="onDownloadFile(item.url)">
`
			result += `          <el-icon style="margin-right: 5px"><Download /></el-icon>
`
			result += `          {{ item.name }}
`
			result += `        </el-button>
`
			result += `    </div>
`
		}
	}

	// 关闭描述项
	result += `</el-descriptions-item>`

	return result
}

// GenerateDefaultFormValue 生成表单字段的默认值（JavaScript代码）
//
// 设计意义：
// 1. 为前端表单提供合理的默认值，避免undefined错误
// 2. 根据字段类型选择最合适的默认值
// 3. 确保表单初始状态正确
//
// 好处：
// - 避免前端访问undefined属性导致的错误
// - 提供合理的初始值，提升用户体验
// - 统一默认值规则，便于维护
//
// 示例：
//
//	// 示例1：布尔类型字段
//	field := systemReq.AutoCodeField{
//		FieldJson: "isActive",
//		FieldType: "bool",
//	}
//	result := GenerateDefaultFormValue(field)
//	// 输出: "isActive: false,"
//
//	// 示例2：字符串类型字段
//	field := systemReq.AutoCodeField{
//		FieldJson: "userName",
//		FieldType: "string",
//	}
//	result := GenerateDefaultFormValue(field)
//	// 输出: "userName: '',"
//
//	// 示例3：整数类型字段（无数据源）
//	field := systemReq.AutoCodeField{
//		FieldJson: "age",
//		FieldType: "int",
//		DataSource: nil,
//	}
//	result := GenerateDefaultFormValue(field)
//	// 输出: "age: 0,"
//
//	// 示例4：整数类型字段（有数据源，如下拉选择）
//	field := systemReq.AutoCodeField{
//		FieldJson: "status",
//		FieldType: "int",
//		DataSource: &systemReq.DataSource{},
//	}
//	result := GenerateDefaultFormValue(field)
//	// 输出: "status: undefined,"
//
//	// 示例5：时间类型字段
//	field := systemReq.AutoCodeField{
//		FieldJson: "createTime",
//		FieldType: "time.Time",
//	}
//	result := GenerateDefaultFormValue(field)
//	// 输出: "createTime: new Date(),"
//
//	// 示例6：数组类型字段
//	field := systemReq.AutoCodeField{
//		FieldJson: "tags",
//		FieldType: "array",
//	}
//	result := GenerateDefaultFormValue(field)
//	// 输出: "tags: [],"
//
//	// 示例7：JSON类型字段
//	field := systemReq.AutoCodeField{
//		FieldJson: "metadata",
//		FieldType: "json",
//	}
//	result := GenerateDefaultFormValue(field)
//	// 输出: "metadata: {},"
//
//	// 使用示例：生成完整的表单默认值对象
//	fields := []systemReq.AutoCodeField{
//		{FieldJson: "name", FieldType: "string"},
//		{FieldJson: "age", FieldType: "int"},
//		{FieldJson: "isActive", FieldType: "bool"},
//	}
//	var formData strings.Builder
//	formData.WriteString("const formData = {\n")
//	for _, field := range fields {
//		formData.WriteString("  " + GenerateDefaultFormValue(field) + "\n")
//	}
//	formData.WriteString("}")
//	// 输出:
//	// const formData = {
//	//   name: '',
//	//   age: 0,
//	//   isActive: false,
//	// }
func GenerateDefaultFormValue(field systemReq.AutoCodeField) string {
	// 根据字段类型确定默认值
	// 注意：这里返回的是JavaScript代码字符串，不是Go值
	var defaultValue string

	switch field.FieldType {
	case "bool":
		// 布尔类型：默认为false
		defaultValue = "false"
	case "string", "richtext":
		// 字符串类型：默认为空字符串
		// 使用''而不是null，好处：避免null检查，直接使用字符串方法
		defaultValue = "''"
	case "int":
		// 整数类型：根据是否有数据源决定默认值
		if field.DataSource != nil { // 检查数据源是否存在
			// 有数据源：使用undefined，表示未选择
			// 好处：可以区分"未选择"和"选择了0"
			defaultValue = "undefined"
		} else {
			// 无数据源：默认为0
			defaultValue = "0"
		}
	case "time.Time":
		// 时间类型：默认为当前时间
		// 好处：新建记录时自动填充当前时间，减少用户操作
		defaultValue = "new Date()"
	case "float64":
		// 浮点数类型：默认为0
		defaultValue = "0"
	case "picture", "video":
		// 图片/视频：默认为空字符串
		// 使用空字符串而不是null，好处：可以直接判断是否为空
		defaultValue = "\"\""
	case "pictures", "file", "array":
		// 数组类型：默认为空数组
		// 好处：可以直接使用数组方法（如push、map等），不需要先判断
		defaultValue = "[]"
	case "json":
		// JSON类型：默认为空对象
		// 好处：可以直接访问对象属性，不需要先判断
		defaultValue = "{}"
	default:
		// 其他类型：默认为null
		defaultValue = "null"
	}

	// 返回格式化后的默认值字符串
	// 格式：fieldName: defaultValue,
	// 可以直接用于JavaScript对象字面量
	return fmt.Sprintf(`%s: %s,`, field.FieldJson, defaultValue)
}

// GenerateSearchField 生成搜索结构体中的字段定义（Go结构体字段）
//
// 设计意义：
// 1. 为搜索功能生成专门的请求结构体字段
// 2. 支持范围查询（BETWEEN）的特殊字段处理
// 3. 根据字段类型选择合适的数据类型（指针类型用于可选字段）
// 4. 自动生成JSON和form标签，支持HTTP请求绑定
//
// 好处：
// - 搜索字段与普通字段分离，结构清晰
// - 使用指针类型可以区分"未设置"和"零值"
// - 支持范围查询，提供灵活的搜索方式
// - 自动处理复杂类型的搜索字段（统一使用string）
//
// 示例：
//
// 示例1：普通搜索字段（基础类型）
//
//	field := systemReq.AutoCodeField{
//	    FieldName: "UserName",
//	    FieldJson: "userName",
//	    FieldType: "string",
//	    FieldSearchType: "LIKE",
//	}
//	输出: UserName  *string `json:"userName" form:"userName"`
//
// 示例2：时间范围查询
//
//	field := systemReq.AutoCodeField{
//	    FieldName: "CreatedAt",
//	    FieldJson: "createdAt",
//	    FieldType: "time.Time",
//	    FieldSearchType: "BETWEEN",
//	}
//	输出: CreatedAtRange  []time.Time  `json:"createdAtRange" form:"createdAtRange[]"`
//
// 示例3：数值范围查询
//
//	field := systemReq.AutoCodeField{
//	    FieldName: "Age",
//	    FieldJson: "age",
//	    FieldType: "int",
//	    FieldSearchType: "BETWEEN",
//	}
//	输出: StartAge  *int  `json:"startAge" form:"startAge"`
//	     EndAge  *int  `json:"endAge" form:"endAge"`
//
// 示例4：复杂类型搜索字段（枚举、图片等）
//
//	field := systemReq.AutoCodeField{
//	    FieldName: "Status",
//	    FieldJson: "status",
//	    FieldType: "enum",
//	    FieldSearchType: "=",
//	}
//	输出: Status  string `json:"status" form:"status"`
//
// 示例5：无搜索类型（返回空字符串）
//
//	field := systemReq.AutoCodeField{
//	    FieldName: "Password",
//	    FieldJson: "password",
//	    FieldType: "string",
//	    FieldSearchType: "",
//	}
//	输出: ""（空字符串，不生成搜索字段）
func GenerateSearchField(field systemReq.AutoCodeField) string {
	var result string

	// 如果没有搜索类型，返回空字符串
	// 好处：只生成可搜索的字段，避免生成无用的字段
	if field.FieldSearchType == "" {
		return ""
	}

	if field.FieldSearchType == "BETWEEN" || field.FieldSearchType == "NOT BETWEEN" {
		// 范围查询：需要生成开始和结束两个字段
		// 好处：可以查询某个范围内的数据，常用于日期和数值筛选
		if field.FieldType == "time.Time" {
			// 时间范围：使用数组存储开始和结束时间
			// 好处：前端可以一次性传递范围，后端统一处理
			// form标签中的[]表示这是数组参数
			result = fmt.Sprintf("%sRange  []time.Time  `json:\"%sRange\" form:\"%sRange[]\"`",
				field.FieldName, field.FieldJson, field.FieldJson)
		} else {
			// 数值范围：使用Start和End两个指针字段
			// 使用指针的好处：可以判断是否设置了范围值
			// 如果Start和End都为nil，则不进行范围查询
			startField := fmt.Sprintf("Start%s  *%s  `json:\"start%s\" form:\"start%s\"`",
				field.FieldName, field.FieldType, field.FieldName, field.FieldName)
			endField := fmt.Sprintf("End%s  *%s  `json:\"end%s\" form:\"end%s\"`",
				field.FieldName, field.FieldType, field.FieldName, field.FieldName)
			result = startField + "\n" + endField
		}
	} else {
		// 普通搜索字段：根据字段类型选择数据类型
		// 复杂类型（JSON、数组、文件等）统一使用string类型
		// 好处：这些类型在搜索时通常以字符串形式传递（如JSON字符串、文件URL等）
		if field.FieldType == "enum" || field.FieldType == "picture" ||
			field.FieldType == "pictures" || field.FieldType == "video" ||
			field.FieldType == "json" || field.FieldType == "richtext" || field.FieldType == "array" || field.FieldType == "file" {
			// 复杂类型：使用string类型
			// 不使用指针，因为空字符串可以表示"不搜索"
			result = fmt.Sprintf("%s  string `json:\"%s\" form:\"%s\"` ",
				field.FieldName, field.FieldJson, field.FieldJson)
		} else {
			// 基础类型：使用原类型，但使用指针
			// 使用指针的好处：nil表示"不搜索"，非nil表示"搜索该值"
			// 这样可以区分"搜索空值"和"不搜索该字段"
			result = fmt.Sprintf("%s  *%s `json:\"%s\" form:\"%s\"` ",
				field.FieldName, field.FieldType, field.FieldJson, field.FieldJson)
		}
	}

	return result
}
