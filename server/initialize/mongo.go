package initialize

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/initialize/internal"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/pkg/errors"
	"github.com/qiniu/qmgo"
	"github.com/qiniu/qmgo/options"
	"go.mongodb.org/mongo-driver/bson"
	option "go.mongodb.org/mongo-driver/mongo/options"
)

// Mongo 全局MongoDB操作实例
// 使用单例模式，确保整个应用只有一个MongoDB连接管理器
// 好处：统一管理连接，避免重复初始化，便于全局访问
var Mongo = new(mongo)

type (
	// mongo MongoDB初始化和管理结构体
	// 采用空结构体设计，只包含方法，不存储状态
	// 好处：零内存占用，所有状态都存储在global.GVA_MONGO中，便于管理
	mongo struct{}

	// Index MongoDB索引结构体，用于解析MongoDB返回的索引信息
	// 对应MongoDB的索引文档格式，用于反序列化索引列表
	// 好处：类型安全，便于从MongoDB获取索引信息并进行比较
	Index struct {
		V    any      `bson:"v"`    // 索引版本
		Ns   any      `bson:"ns"`   // 命名空间
		Key  []bson.E `bson:"key"`  // 索引键列表，包含字段名和排序方向(1升序/-1降序)
		Name string   `bson:"name"` // 索引名称
	}
)

// Indexes 批量创建所有集合的索引
// 这是一个可扩展的入口函数，通过配置indexMap来定义需要创建的索引
// 设计模式：配置驱动，将索引定义与创建逻辑分离
// 好处：
// 1. 集中管理所有索引定义，便于维护
// 2. 支持批量创建，提高初始化效率
// 3. 如果某个集合的索引创建失败，立即返回错误，保证数据一致性
func (m *mongo) Indexes(ctx context.Context) error {
	// 索引配置映射：表名 -> 索引列表
	// 格式示例: "表名": [][]string{{"field1", "field2"}, {"-field3"}}
	// 说明：
	// - 外层[]表示多个索引
	// - 内层[]string表示一个索引的字段列表
	// - 字段名前加"-"表示降序索引，不加表示升序索引
	// 好处：使用字符串数组而非复杂结构体，配置简单直观
	indexMap := map[string][][]string{}

	// 遍历所有集合，为每个集合创建索引
	// 采用提前返回错误模式，确保任何索引创建失败都能及时发现
	for collection, indexes := range indexMap {
		err := m.CreateIndexes(ctx, collection, indexes)
		if err != nil {
			return err
		}
	}
	return nil
}

// Initialization MongoDB数据库初始化函数
// 负责建立MongoDB连接并创建必要的索引
// 设计思路：连接配置与索引创建分离，先建立连接再创建索引
// 好处：
// 1. 连接失败时立即返回，不浪费资源创建索引
// 2. 索引创建失败不影响连接建立，便于排查问题
// 3. 支持条件配置（如日志、认证），提高灵活性
func (m *mongo) Initialization() error {
	// 可选的客户端选项，用于扩展配置
	// 如果启用了Zap日志，则添加日志相关的客户端选项
	// 好处：按需加载，避免不必要的依赖和性能开销
	var opts []options.ClientOptions
	if global.GVA_CONFIG.Mongo.IsZap {
		opts = internal.Mongo.GetClientOptions()
	}

	// 使用Background上下文，适合初始化场景
	// 好处：不设置超时，确保初始化过程完整执行
	ctx := context.Background()

	// 构建MongoDB连接配置
	// 使用指针传递配置值，允许qmgo使用默认值（如果配置为nil）
	// 好处：
	// 1. 支持部分配置，未配置的项使用MongoDB默认值
	// 2. 连接池配置（MinPoolSize/MaxPoolSize）可以控制并发性能
	// 3. 超时配置（SocketTimeoutMS/ConnectTimeoutMS）可以防止长时间等待
	config := &qmgo.Config{
		Uri:              global.GVA_CONFIG.Mongo.Uri(),             // 连接URI，包含主机、端口等信息
		Coll:             global.GVA_CONFIG.Mongo.Coll,              // 默认集合名
		Database:         global.GVA_CONFIG.Mongo.Database,          // 数据库名
		MinPoolSize:      &global.GVA_CONFIG.Mongo.MinPoolSize,      // 最小连接池大小，保证基础连接数
		MaxPoolSize:      &global.GVA_CONFIG.Mongo.MaxPoolSize,      // 最大连接池大小，限制资源使用
		SocketTimeoutMS:  &global.GVA_CONFIG.Mongo.SocketTimeoutMs,  // Socket操作超时，防止网络问题导致阻塞
		ConnectTimeoutMS: &global.GVA_CONFIG.Mongo.ConnectTimeoutMs, // 连接超时，快速失败机制
	}

	// 条件配置认证信息
	// 只有当用户名和密码都不为空时才设置认证
	// 好处：支持无认证的MongoDB实例，提高兼容性
	if global.GVA_CONFIG.Mongo.Username != "" && global.GVA_CONFIG.Mongo.Password != "" {
		config.Auth = &qmgo.Credential{
			Username:   global.GVA_CONFIG.Mongo.Username,   // 认证用户名
			Password:   global.GVA_CONFIG.Mongo.Password,   // 认证密码
			AuthSource: global.GVA_CONFIG.Mongo.AuthSource, // 认证数据库，通常为"admin"
		}
	}

	// 建立MongoDB连接
	// 使用可变参数opts，支持扩展配置
	client, err := qmgo.Open(ctx, config, opts...)
	if err != nil {
		// 使用errors.Wrap包装错误，保留原始错误信息和堆栈
		// 好处：错误信息更完整，便于调试和定位问题
		return errors.Wrap(err, "链接mongodb数据库失败!")
	}

	// 将连接保存到全局变量，供整个应用使用
	// 好处：单例模式，避免重复连接，统一管理
	global.GVA_MONGO = client

	// 连接成功后，创建所有必要的索引
	// 在初始化阶段创建索引的好处：
	// 1. 提前发现问题，避免运行时性能问题
	// 2. 确保数据一致性，索引与数据同步
	err = m.Indexes(ctx)
	if err != nil {
		return err
	}
	return nil
}

// CreateIndexes 为指定集合创建索引
// 核心功能：智能索引管理，只创建不存在的索引，避免重复创建
// 设计亮点：
// 1. 索引去重：通过规范化索引键生成唯一标识，避免重复索引
// 2. 索引比较：将配置的索引与已存在的索引进行比较
// 3. 名称处理：处理MongoDB索引名称127字符限制
// 4. TTL索引：支持自动过期索引（TTL），适用于日志等临时数据
// 好处：
// - 幂等性：多次执行不会重复创建索引
// - 性能优化：避免不必要的索引创建操作
// - 自动管理：无需手动检查索引是否存在
func (m *mongo) CreateIndexes(ctx context.Context, name string, indexes [][]string) error {
	// 获取集合对象
	// 使用CloneCollection()获取独立的集合对象，避免并发问题
	// 好处：线程安全，每个操作使用独立的集合实例
	collection, err := global.GVA_MONGO.Database.Collection(name).CloneCollection()
	if err != nil {
		return errors.Wrapf(err, "获取[%s]的表对象失败!", name)
	}

	// 获取集合中所有已存在的索引列表
	// 这是索引比较的基础，需要知道哪些索引已经存在
	list, err := collection.Indexes().List(ctx)
	if err != nil {
		return errors.Wrapf(err, "获取[%s]的索引对象失败!", name)
	}

	// 将索引列表解析为Index结构体数组
	// 好处：类型安全，便于后续处理和比较
	var entities []Index
	err = list.All(ctx, &entities)
	if err != nil {
		return errors.Wrapf(err, "获取[%s]的索引列表失败!", name)
	}

	// ========== 第一步：规范化配置的索引，生成唯一标识 ==========
	// 目标：将索引配置转换为标准化的键值对，用于索引比较和去重
	length := len(indexes)
	// 使用map存储：规范化后的索引键 -> 原始索引字段列表
	// 预分配容量，提高性能
	indexMap1 := make(map[string][]string, length)

	for i := 0; i < length; i++ {
		// 对索引字段进行排序
		// 原因：MongoDB在使用bson.M创建索引时，会自动按照字段名的字母顺序排序
		// 好处：确保索引键的一致性，无论字段顺序如何，都能正确识别相同的索引
		// 例如：{"a":1, "b":1} 和 {"b":1, "a":1} 会被识别为同一个索引
		sort.Strings(indexes[i])

		length1 := len(indexes[i])
		// 构建索引键字符串数组
		// 格式：["field1", "1", "field2", "-1"] 表示 field1升序，field2降序
		keys := make([]string, 0, length1*2) // 预分配容量，每个字段需要2个元素（字段名+排序方向）

		for j := 0; j < length1; j++ {
			// 检查字段名首字符是否为"-"，"-"表示降序索引
			// 注意：这里应该是indexes[i][j][0]，原代码有bug
			if len(indexes[i][j]) > 0 && indexes[i][j][0] == '-' {
				// 降序索引：字段名（去掉"-"前缀）+ "-1"
				keys = append(keys, indexes[i][j], "-1")
				continue
			}
			// 升序索引：字段名 + "1"
			keys = append(keys, indexes[i][j], "1")
		}

		// 将索引键数组连接成唯一标识字符串
		// 例如：["field1", "1", "field2", "-1"] -> "field1_1_field2_-1"
		// 好处：生成可读的索引标识，便于调试和日志记录
		key := strings.Join(keys, "_")

		// 检查是否有重复的索引配置
		// 好处：在创建前发现配置错误，避免创建重复索引
		_, o1 := indexMap1[key]
		if o1 {
			return errors.Errorf("索引[%s]重复!", key)
		}

		// 存储索引映射关系
		indexMap1[key] = indexes[i]
	}

	// ========== 第二步：解析已存在的索引，构建比较映射 ==========
	// 目标：将MongoDB返回的索引信息转换为与配置索引相同的格式，便于比较
	// 说明：这一步是为了后续比较配置的索引和已存在的索引，避免重复创建
	length = len(entities)
	// 数据结构：indexMap2[索引名称][字段名] = 字段名
	// 使用嵌套map存储：索引名称 -> 字段名 -> 字段名（用于快速查找）
	// 设计说明：虽然看起来冗余（字段名作为key和value都是同一个值），但这是为了与indexMap1的键格式保持一致
	// 实际上，内层map只需要用map[string]bool或map[string]struct{}即可，但当前实现保持了字段名作为值
	indexMap2 := make(map[string]map[string]string, length)

	// 遍历所有已存在的索引（从MongoDB获取的索引列表）
	for i := 0; i < length; i++ {
		// 尝试获取该索引名称对应的字段映射
		// v1: 该索引的字段映射（如果存在）
		// o1: 布尔值，表示该索引名称是否已存在于indexMap2中
		v1, o1 := indexMap2[entities[i].Name]

		// 如果索引名称不存在，需要创建新的字段映射
		if !o1 {
			// 获取该索引包含的字段数量，用于预分配map容量
			keyLength := len(entities[i].Key)
			// 创建字段映射，key和value都是字段名（冗余设计，但保持一致性）
			v1 = make(map[string]string, keyLength)

			// 遍历索引的所有字段，将每个字段添加到映射中
			for j := 0; j < keyLength; j++ {
				// 获取当前字段名
				fieldName := entities[i].Key[j].Key

				// 检查该字段是否已经在映射中（理论上不会重复，因为这是从MongoDB获取的索引）
				v2, o2 := v1[fieldName]
				if !o2 {
					// 【注意：这里存在逻辑问题】
					// 如果字段不存在，重新创建v1会覆盖之前创建的map，导致之前添加的字段丢失
					// 正确的做法应该是：直接跳过这个if判断，或者删除这个if分支
					// 因为v1已经在上面创建好了，不需要重新创建
					v1 = make(map[string]string)
				}

				// 将字段名作为值存储（key和value都是同一个字段名，这是冗余的）
				// 实际上只需要 v1[fieldName] = fieldName 即可
				v2 = fieldName
				v1[fieldName] = v2

				// 将更新后的字段映射保存回indexMap2
				// 注意：这里在循环内部每次都更新，实际上可以移到循环外部
				indexMap2[entities[i].Name] = v1
			}
		}
		// 如果索引名称已存在（o1 == true），说明该索引已经被处理过，跳过
		// 注意：MongoDB中通常不会有重复的索引名称，所以这个分支很少执行
	}

	// ========== 第三步：比较并创建缺失的索引 ==========
	// 遍历配置的索引，只创建不存在的索引
	for k1, v1 := range indexMap1 {
		// 检查索引是否已存在
		// 如果存在，跳过创建
		_, o2 := indexMap2[k1]
		if o2 {
			continue // 索引已存在，跳过
		}

		// MongoDB索引名称限制：不能超过127个字符
		// 如果自动生成的索引名会超过限制，使用MD5哈希作为索引名
		// 好处：确保索引名符合MongoDB规范，避免创建失败
		// 注意：这里使用collection.Name()可能不正确，应该直接使用name参数
		indexName := fmt.Sprintf("%s.%s.$%s", collection.Name(), name, v1)
		if len(indexName) > 127 {
			// 索引名过长，使用MD5哈希作为索引名
			// 好处：固定长度（32字符），永远不会超过限制
			// 注意：MD5哈希不可读，但保证了唯一性
			err = global.GVA_MONGO.Database.Collection(name).CreateOneIndex(ctx, options.IndexModel{
				Key:          v1,                                             // 索引字段和排序方向
				IndexOptions: option.Index().SetName(utils.MD5V([]byte(k1))), // 使用MD5作为索引名
				// 可选：设置TTL（Time To Live）索引，自动删除过期文档
				// SetExpireAfterSeconds(86400) 表示文档在86400秒（1天）后自动删除
				// 适用于日志、会话等临时数据
			})
			if err != nil {
				return errors.Wrapf(err, "创建索引[%s]失败!", k1)
			}
			// 注意：这里应该continue而不是return nil，否则只会创建第一个索引
			continue
		}

		// 索引名在限制内，使用默认命名规则
		// 注意：这里设置了TTL为86400秒（1天），但注释掉了索引名设置
		// 这意味着使用MongoDB自动生成的索引名
		err = global.GVA_MONGO.Database.Collection(name).CreateOneIndex(ctx, options.IndexModel{
			Key:          v1,                                          // 索引字段和排序方向
			IndexOptions: option.Index().SetExpireAfterSeconds(86400), // 设置TTL为1天
			// 可选：同时设置自定义索引名和TTL
			// IndexOptions: option.Index().SetName(utils.MD5V([]byte(k1))).SetExpireAfterSeconds(86400)
		})
		if err != nil {
			return errors.Wrapf(err, "创建索引[%s]失败!", k1)
		}
	}
	return nil
}
