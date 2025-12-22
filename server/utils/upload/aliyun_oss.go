package upload

import (
	"errors"
	"mime/multipart"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"go.uber.org/zap"
)

// AliyunOSS 实现了统一的「上传接口」，专门用于对接阿里云 OSS。
// 使用结构体而不是简单函数，有两个好处：
// 1. 便于和本项目中其他云厂商（七牛、腾讯云、MinIO 等）形成「策略模式」，统一通过接口调用，随时可替换实现；
// 2. 以后如果需要在该结构体中挂载更多状态或配置（例如统计上传次数、缓存 client 等），扩展起来更自然。
type AliyunOSS struct{}

// UploadFile 负责把 HTTP 上传上来的文件上传到阿里云 OSS。
// 返回值依次为：文件完整访问 URL、对象在 OSS 中的 key、错误信息。
// 把这些逻辑集中在一个方法里，可以：
// 1. 统一错误处理与日志格式，排查线上问题时更快定位；
// 2. 统一路径规则与返回格式，业务层只关心「我能拿到一个可访问的 URL」和「后续删除时的 key」即可。
func (*AliyunOSS) UploadFile(file *multipart.FileHeader) (string, string, error) {
	// 通过 NewBucket 统一创建 Bucket 对象，而不是在每个方法里重复创建：
	// - 所有 OSS 连接配置集中在一处，修改配置或接入新能力时只需要改一个地方；
	// - 可读性更好，调用处只关注「我要一个 bucket」而不是「如何创建 bucket」。
	bucket, err := NewBucket()
	if err != nil {
		global.GVA_LOG.Error("function AliyunOSS.NewBucket() Failed", zap.Any("err", err.Error()))
		return "", "", errors.New("function AliyunOSS.NewBucket() Failed, err:" + err.Error())
	}

	// 读取本地文件。
	// 使用 multipart.FileHeader 提供的 Open 方法，能够和 gin 的文件上传流程无缝衔接，
	// 把 HTTP 细节限制在这一层，避免污染业务代码，符合「分层解耦」的设计思想。
	f, openError := file.Open()
	if openError != nil {
		global.GVA_LOG.Error("function file.Open() Failed", zap.Any("err", openError.Error()))
		return "", "", errors.New("function file.Open() Failed, err:" + openError.Error())
	}
	// 使用 defer 关闭文件句柄，确保即便中间出现错误，资源也能被正确回收，避免句柄泄露。
	defer f.Close()

	// 组装上传到阿里云的文件路径。
	// 这里的路径规则设计有以下考虑：
	// 1. global.GVA_CONFIG.AliyunOSS.BasePath：从配置中读取前缀，方便按环境/项目做隔离（比如 test/、prod/）；
	// 2. "uploads" 目录：与系统中其他上传方式保持一致，便于统一管理上传资源；
	// 3. 按日期分目录（2006-01-02）：避免所有文件都堆在一个目录中，降低 OSS 列表操作压力，也方便按日期排查问题；
	// 4. 直接使用原始文件名：对人类友好，便于快速识别文件；如果需要更强的唯一性，可以在这里拼接 UUID / 哈希值做扩展。
	// yunFileTmpPath := filepath.Join("uploads", time.Now().Format("2006-01-02")) + "/" + file.Filename
	yunFileTmpPath := global.GVA_CONFIG.AliyunOSS.BasePath + "/" + "uploads" + "/" + time.Now().Format("2006-01-02") + "/" + file.Filename

	// 把文件流直接上传到 OSS。
	// 这里选择「流式上传」而不是「先落本地临时文件后再上传」的好处：
	// 1. 减少一次磁盘 IO，提升性能；
	// 2. 不占用本地磁盘空间，降低清理临时文件的复杂度。
	err = bucket.PutObject(yunFileTmpPath, f)
	if err != nil {
		global.GVA_LOG.Error("function formUploader.Put() Failed", zap.Any("err", err.Error()))
		return "", "", errors.New("function formUploader.Put() Failed, err:" + err.Error())
	}

	// 返回给调用方「带域名的完整 URL」+「纯 key」：
	// - URL 方便前端或其他服务直接访问；
	// - key 用于后续删除、移动等操作，不受域名配置变化影响，这样如果以后切换访问域名，只需要改配置里的 BucketUrl 即可。
	return global.GVA_CONFIG.AliyunOSS.BucketUrl + "/" + yunFileTmpPath, yunFileTmpPath, nil
}

// DeleteFile 根据传入的 key 删除 OSS 上对应的对象。
// 删除逻辑抽象成独立方法的意义在于：
// 1. 业务层只需要记住「上传时返回的 key」，无需关心底层是阿里云、七牛还是本地存储；
// 2. 日志与错误处理统一，后续如果要做操作审计或回收站功能，也更容易接入。
func (*AliyunOSS) DeleteFile(key string) error {
	bucket, err := NewBucket()
	if err != nil {
		global.GVA_LOG.Error("function AliyunOSS.NewBucket() Failed", zap.Any("err", err.Error()))
		return errors.New("function AliyunOSS.NewBucket() Failed, err:" + err.Error())
	}

	// 删除单个文件。objectName表示删除OSS文件时需要指定包含文件后缀在内的完整路径，例如abc/efg/123.jpg。
	// 如需删除文件夹，请将objectName设置为对应的文件夹名称。如果文件夹非空，则需要将文件夹下的所有object删除后才能删除该文件夹。
	// 这里直接让调用方传入 key，可以支持：
	// - 精确删除某个文件；
	// - 通过业务层自己维护 key 列表，实现批量删除或清理目录。
	err = bucket.DeleteObject(key)
	if err != nil {
		global.GVA_LOG.Error("function bucketManager.Delete() failed", zap.Any("err", err.Error()))
		return errors.New("function bucketManager.Delete() failed, err:" + err.Error())
	}

	return nil
}

// NewBucket 根据全局配置创建并返回一个已经绑定好 BucketName 的 *oss.Bucket。
// 封装成函数有几方面好处：
// 1. 所有 OSS 相关配置集中读取，避免在多个地方硬编码 Endpoint / AccessKey / BucketName；
// 2. 以后如果要接入 STS、代理、重试策略等，只需要在这里修改创建逻辑即可，对上层调用完全透明。
func NewBucket() (*oss.Bucket, error) {
	// 创建 OSSClient 实例，使用配置文件中的 endpoint 和 AK/SK。
	// 把密钥放在配置文件而不是写死在代码里，可以：
	// - 提升安全性：源码泄露也不会直接暴露账号密码；
	// - 方便多环境切换：测试、预发布、生产只需改配置。
	client, err := oss.New(global.GVA_CONFIG.AliyunOSS.Endpoint, global.GVA_CONFIG.AliyunOSS.AccessKeyId, global.GVA_CONFIG.AliyunOSS.AccessKeySecret)
	if err != nil {
		return nil, err
	}

	// 获取存储空间（Bucket），BucketName 同样来自配置，避免在代码里出现多个写死的名称。
	// 统一从配置读取可以确保不同环境使用正确的存储空间，也便于后续迁移。
	bucket, err := client.Bucket(global.GVA_CONFIG.AliyunOSS.BucketName)
	if err != nil {
		return nil, err
	}

	return bucket, nil
}
