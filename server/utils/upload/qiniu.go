// Package upload 提供了多种云存储服务的文件上传功能
// 本文件实现了七牛云存储的上传和删除功能
package upload

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
	"go.uber.org/zap"
)

// Qiniu 七牛云存储服务的实现结构体
// 使用空结构体的好处：
// 1. 零内存占用：空结构体不占用任何内存空间，仅作为类型标识
// 2. 语义清晰：明确表示这是一个七牛云的实现类型
// 3. 符合接口实现模式：通过方法接收者实现接口，避免不必要的状态存储
type Qiniu struct{}

//@author: [piexlmax](https://github.com/piexlmax)
//@author: [ccfish86](https://github.com/ccfish86)
//@author: [SliverHorn](https://github.com/SliverHorn)
//@object: *Qiniu
//@function: UploadFile
//@description: 上传文件到七牛云存储
//@param: file *multipart.FileHeader - HTTP multipart 请求中的文件头，包含文件名、大小等信息
//@return: string, string, error - 返回完整访问URL、文件在存储桶中的key、错误信息

// UploadFile 实现文件上传到七牛云的核心逻辑
// 设计思路：
//  1. 使用七牛云的 PutPolicy 机制：通过上传策略控制上传权限，Scope 指定目标存储桶
//     好处：安全性高，上传凭证（token）有时效性，防止权限滥用
//  2. 使用 MAC（Message Authentication Code）进行身份验证
//     好处：通过 AccessKey 和 SecretKey 生成签名，确保请求的完整性和真实性
//  3. 使用表单上传（FormUploader）方式
//     好处：适合中小文件上传，实现简单，兼容性好，支持自定义元数据
func (*Qiniu) UploadFile(file *multipart.FileHeader) (string, string, error) {
	// PutPolicy 定义了上传策略，Scope 指定目标存储桶
	// 这样做的好处：可以在策略中设置过期时间、文件大小限制等，增强安全性
	putPolicy := storage.PutPolicy{Scope: global.GVA_CONFIG.Qiniu.Bucket}

	// 使用 AccessKey 和 SecretKey 创建 MAC 对象，用于签名和身份验证
	// 为什么不直接传字符串？MAC 对象封装了签名算法，统一处理认证逻辑，降低出错概率
	mac := qbox.NewMac(global.GVA_CONFIG.Qiniu.AccessKey, global.GVA_CONFIG.Qiniu.SecretKey)

	// 生成上传凭证（UploadToken），这是上传文件的"门票"
	// 好处：token 可以设置过期时间，即使泄露也只在有效期内可用，提高安全性
	upToken := putPolicy.UploadToken(mac)

	// 获取七牛云配置（包括机房区域、HTTPS 设置等）
	// 使用独立函数的好处：配置逻辑集中管理，便于维护和复用
	cfg := qiniuConfig()

	// 创建表单上传器实例
	// 为什么用表单上传而不是分片上传？
	// - 表单上传适合文件大小在 100MB 以内的场景，实现简单
	// - 对于更大文件，可以考虑使用分片上传（ResumeUploader）以提高效率和稳定性
	formUploader := storage.NewFormUploader(cfg)

	// PutRet 用于接收上传成功后的返回信息（如文件key、hash等）
	ret := storage.PutRet{}

	// PutExtra 用于设置额外的上传参数，如自定义元数据
	// 使用 map[string]string 的好处：灵活扩展，可以添加任意自定义信息，便于后续检索和管理
	putExtra := storage.PutExtra{Params: map[string]string{"x:name": "github logo"}}

	// 打开文件流，准备读取文件内容
	// 为什么在这里打开？文件流需要在上传时传递给 SDK，提前打开便于错误处理
	f, openError := file.Open()
	if openError != nil {
		// 记录详细错误日志，便于问题排查
		// 使用 zap.Any 的好处：结构化日志，便于日志收集和分析系统处理
		global.GVA_LOG.Error("function file.Open() failed", zap.Any("err", openError.Error()))

		return "", "", errors.New("function file.Open() failed, err:" + openError.Error())
	}

	// 使用 defer 确保文件流一定会关闭，防止资源泄露
	// 为什么用 defer？无论函数是正常返回还是异常返回，defer 都会执行，保证资源释放
	defer f.Close()

	// 生成唯一的文件key，格式：时间戳+原文件名
	// 使用时间戳的好处：
	// 1. 保证文件名唯一性，避免同名文件覆盖
	// 2. 时间戳可以反映上传时间，便于按时间排序
	// 3. 简化命名规则，无需额外的UUID生成库
	// 注意：如果有更高要求，可以考虑使用 UUID 或 MD5 等更复杂的命名策略
	fileKey := fmt.Sprintf("%d%s", time.Now().Unix(), file.Filename)

	// 执行文件上传
	// 参数说明：
	// - context.Background(): 使用默认上下文，如果需要超时控制可以用 context.WithTimeout
	// - &ret: 上传成功后的返回结果会填充到这里
	// - upToken: 上传凭证
	// - fileKey: 文件在存储桶中的唯一标识
	// - f: 文件流
	// - file.Size: 文件大小，SDK 内部会校验，防止数据不完整
	// - &putExtra: 额外参数，如自定义元数据
	putErr := formUploader.Put(context.Background(), &ret, upToken, fileKey, f, file.Size, &putExtra)
	if putErr != nil {
		// 上传失败时记录错误日志
		global.GVA_LOG.Error("function formUploader.Put() failed", zap.Any("err", putErr.Error()))
		return "", "", errors.New("function formUploader.Put() failed, err:" + putErr.Error())
	}

	// 返回完整的访问URL和文件key
	// 返回两个值的好处：
	// 1. 完整URL可以直接用于前端显示或下载
	// 2. 文件key可用于后续的删除、更新等操作
	return global.GVA_CONFIG.Qiniu.ImgPath + "/" + ret.Key, ret.Key, nil
}

//@author: [piexlmax](https://github.com/piexlmax)
//@author: [ccfish86](https://github.com/ccfish86)
//@author: [SliverHorn](https://github.com/SliverHorn)
//@object: *Qiniu
//@function: DeleteFile
//@description: 从七牛云存储中删除指定文件
//@param: key string - 文件在存储桶中的唯一标识（通常是上传时返回的 key）
//@return: error - 删除操作失败时返回错误信息

// DeleteFile 实现从七牛云存储中删除文件的逻辑
// 设计思路：
//  1. 使用 BucketManager 进行文件管理操作
//     好处：BucketManager 封装了存储桶相关的所有操作（删除、复制、移动等），接口统一
//  2. 只需要文件 key 即可删除，无需完整的 URL
//     好处：简化参数，key 通常是上传时返回的，便于存储和传递
func (*Qiniu) DeleteFile(key string) error {
	// 创建 MAC 对象用于身份验证，与上传时使用相同的认证机制
	// 复用配置的好处：保持认证方式的一致性，降低维护成本
	mac := qbox.NewMac(global.GVA_CONFIG.Qiniu.AccessKey, global.GVA_CONFIG.Qiniu.SecretKey)

	// 获取七牛云配置（包括机房区域等）
	// 复用 qiniuConfig 函数的好处：配置逻辑统一，如果配置有变更只需修改一处
	cfg := qiniuConfig()

	// 创建存储桶管理器实例
	// BucketManager 的作用：提供对存储桶中文件的各种管理操作
	// 为什么不直接调用删除接口？SDK 封装了网络请求、错误处理等细节，使用更简单
	bucketManager := storage.NewBucketManager(mac, cfg)

	// 执行删除操作
	// 参数说明：
	// - global.GVA_CONFIG.Qiniu.Bucket: 目标存储桶名称
	// - key: 要删除的文件在存储桶中的唯一标识
	// 为什么需要同时指定 bucket 和 key？
	// - bucket 指定操作的目标存储桶（一个账号可能有多个存储桶）
	// - key 指定要删除的具体文件（存储桶内的文件唯一标识）
	if err := bucketManager.Delete(global.GVA_CONFIG.Qiniu.Bucket, key); err != nil {
		// 删除失败时记录错误日志，便于排查问题
		// 记录错误的好处：可以通过日志系统监控删除失败的情况，及时发现存储服务异常
		global.GVA_LOG.Error("function bucketManager.Delete() failed", zap.Any("err", err.Error()))
		return errors.New("function bucketManager.Delete() failed, err:" + err.Error())
	}

	// 删除成功返回 nil，符合 Go 的错误处理惯例
	return nil
}

//@author: [SliverHorn](https://github.com/SliverHorn)
//@object: *Qiniu
//@function: qiniuConfig
//@description: 根据配置文件构建并返回七牛云的连接配置
//@return: *storage.Config - 七牛云存储的配置对象指针

// qiniuConfig 根据全局配置生成七牛云 SDK 所需的配置对象
// 设计思路：
//  1. 将配置逻辑封装为独立函数
//     好处：
//     - 配置集中管理，便于维护和复用（上传和删除都需要配置）
//     - 如果配置逻辑变复杂（如添加缓存、验证等），只需修改一处
//     - 提高代码可测试性，可以单独测试配置生成逻辑
//  2. 使用 switch 语句处理不同的机房区域
//     好处：
//     - 代码清晰，每种区域的处理一目了然
//     - 易于扩展新的区域配置
//     - 编译时检查，如果拼写错误会报编译错误
//  3. 返回指针而不是值
//     好处：
//     - 避免大对象拷贝，提高性能（虽然 Config 不大，但这是良好的实践）
//     - 语义明确：返回的是可以修改的配置对象
func qiniuConfig() *storage.Config {
	// 创建基础配置对象，设置 HTTPS 和 CDN 相关选项
	// UseHTTPS: 是否使用 HTTPS 协议
	//   好处：加密传输，防止数据被窃听或篡改，提高安全性
	// UseCdnDomains: 是否使用 CDN 加速域名
	//   好处：通过 CDN 分发，提高访问速度，减轻源站压力
	cfg := storage.Config{
		UseHTTPS:      global.GVA_CONFIG.Qiniu.UseHTTPS,
		UseCdnDomains: global.GVA_CONFIG.Qiniu.UseCdnDomains,
	}

	// 根据配置文件中指定的区域（Zone）设置对应的机房
	// 为什么需要指定 Zone？
	// 1. 不同区域的存储桶需要在对应的机房访问，否则会有网络延迟
	// 2. 七牛云在全球多个地区有机房，选择距离最近的机房可以提升性能
	// 3. 某些地区的存储桶只能通过对应的机房 API 访问
	// 使用字符串匹配的好处：
	// - 配置灵活，可以通过配置文件动态指定区域
	// - 如果配置错误（如拼写错误），使用默认配置（Zone 为 nil），SDK 会自动选择
	switch global.GVA_CONFIG.Qiniu.Zone {
	case "ZoneHuadong": // 华东区域（上海）
		cfg.Zone = &storage.ZoneHuadong
	case "ZoneHuabei": // 华北区域（北京）
		cfg.Zone = &storage.ZoneHuabei
	case "ZoneHuanan": // 华南区域（广州）
		cfg.Zone = &storage.ZoneHuanan
	case "ZoneBeimei": // 北美区域（美国）
		cfg.Zone = &storage.ZoneBeimei
	case "ZoneXinjiapo": // 亚太区域（新加坡）
		cfg.Zone = &storage.ZoneXinjiapo
		// 如果没有匹配的区域，Zone 保持为 nil
		// SDK 会自动根据存储桶的实际情况选择合适的区域，但性能可能不如明确指定
	}

	// 返回配置对象的指针
	// 返回指针的好处：避免值拷贝，提高效率，同时允许调用者复用配置对象
	return &cfg
}
