package upload

import (
	"errors"
	"fmt"
	"mime/multipart"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"go.uber.org/zap"
)

// AwsS3 为 AWS S3（以及兼容 S3 协议的对象存储）提供上传/删除能力的实现类型。
// 设计为一个空结构体，主要有两个好处：
// 1. 与本项目其他存储实现（本地、七牛、COS、MinIO 等）保持统一结构，方便在上层通过接口/工厂模式进行切换；
// 2. 结构体本身不持有状态，所有依赖来自全局配置和入参，避免并发场景下的共享状态问题，调用更加安全。
type AwsS3 struct{}

//@author: [WqyJh](https://github.com/WqyJh)
//@object: *AwsS3
//@function: UploadFile
//@description: Upload file to Aws S3 using aws-sdk-go. See https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/s3-example-basic-bucket-operations.html#s3-examples-bucket-ops-upload-file-to-bucket
//@param: file *multipart.FileHeader
//@return: string, string, error

func (*AwsS3) UploadFile(file *multipart.FileHeader) (string, string, error) {
	// 每次上传时通过 newSession() 创建一个 session：
	// - 避免在包级变量中长期持有 Session，减少资源泄露和生命周期管理的复杂度；
	// - 由 aws-sdk-go 负责底层连接池和重用策略，调用方代码保持简单。
	session := newSession()
	// 使用 s3manager.Uploader 封装上传逻辑：
	// - 内部已处理多部分上传、重试等细节，业务代码不需要关心大文件细节；
	// - 后续如果想做更复杂的上传控制，可以在这里统一扩展。
	uploader := s3manager.NewUploader(session)

	// 使用 Unix 时间戳 + 原始文件名作为对象 key，原因：
	// - 带有时间戳，可在绝大多数情况下避免同名冲突；
	// - 保留原始文件名信息，便于人工排查和定位。
	fileKey := fmt.Sprintf("%d%s", time.Now().Unix(), file.Filename)
	// 通过 PathPrefix 控制逻辑“目录”，把具体的存储路径抽象为配置：
	// - 可以方便地为不同业务配置不同前缀，实现逻辑隔离；
	// - 如果将来迁移目录层级，只需修改配置，不需要改代码。
	filename := global.GVA_CONFIG.AwsS3.PathPrefix + "/" + fileKey
	// 直接从 *multipart.FileHeader 打开文件句柄，避免中间文件拷贝，提升性能。
	f, openError := file.Open()
	if openError != nil {
		// 使用全局日志记录错误信息，方便统一收集和排查问题。
		global.GVA_LOG.Error("function file.Open() failed", zap.Any("err", openError.Error()))
		return "", "", errors.New("function file.Open() failed, err:" + openError.Error())
	}
	// defer 关闭文件句柄，确保资源在函数结束时被正确释放：
	// - 即使中间出现 return 也不会忘记 Close；
	// - 在高并发场景下防止句柄泄露。
	defer f.Close() // 创建文件 defer 关闭

	// 将文件内容上传到目标 Bucket + Key
	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(global.GVA_CONFIG.AwsS3.Bucket),
		Key:    aws.String(filename),
		Body:   f,
	})
	if err != nil {
		global.GVA_LOG.Error("function uploader.Upload() failed", zap.Any("err", err.Error()))
		return "", "", err
	}

	// 返回值设计为：完整访问 URL + 存储用的 key：
	// - URL 方便前端或客户端直接访问资源；
	// - key 用于后续删除、权限控制等操作，不需要从 URL 反解析路径。
	return global.GVA_CONFIG.AwsS3.BaseURL + "/" + filename, fileKey, nil
}

//@author: [WqyJh](https://github.com/WqyJh)
//@object: *AwsS3
//@function: DeleteFile
//@description: Delete file from Aws S3 using aws-sdk-go. See https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/s3-example-basic-bucket-operations.html#s3-examples-bucket-ops-delete-bucket-item
//@param: file *multipart.FileHeader
//@return: string, string, error

func (*AwsS3) DeleteFile(key string) error {
	// 删除逻辑同样通过 newSession() 创建 session，保证配置来源统一。
	session := newSession()
	svc := s3.New(session)
	// 使用与上传相同的 PathPrefix + key 规则拼接对象路径：
	// - 确保删除和上传使用同一命名规范，避免路径不一致导致的“删不掉”问题；
	// - 对上层调用方只暴露 key，本文件负责具体路径细节。
	filename := global.GVA_CONFIG.AwsS3.PathPrefix + "/" + key
	bucket := global.GVA_CONFIG.AwsS3.Bucket

	_, err := svc.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(filename),
	})
	if err != nil {
		global.GVA_LOG.Error("function svc.DeleteObject() failed", zap.Any("err", err.Error()))
		return errors.New("function svc.DeleteObject() failed, err:" + err.Error())
	}

	// WaitUntilObjectNotExists 等待对象真正从存储中删除：
	// - 保证函数返回时，对象已经在 S3 上不可见，提高语义上的“删除完成度”；
	// - 对于紧接着进行的写/读操作更安全，避免读到“刚删完还没完全同步”的旧数据。
	_ = svc.WaitUntilObjectNotExists(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(filename),
	})
	return nil
}

// newSession Create S3 session
func newSession() *session.Session {
	sess, _ := session.NewSession(&aws.Config{
		// Region：S3 所在的区域，做成配置是为了方便多环境（开发/测试/生产）切换。
		Region: aws.String(global.GVA_CONFIG.AwsS3.Region),
		// Endpoint：支持自定义 Endpoint，从而兼容 MinIO、Cloudflare R2 等 S3 协议服务；
		// 通过配置即可实现不同云厂商/自建对象存储之间的平滑切换。
		Endpoint: aws.String(global.GVA_CONFIG.AwsS3.Endpoint), // minio 在这里设置地址, 可以兼容
		// S3ForcePathStyle：开启 path-style 访问方式（http://endpoint/bucket/key）
		// 很多自建 S3 服务更依赖这种访问方式，同时也降低对 DNS 的要求。
		S3ForcePathStyle: aws.Bool(global.GVA_CONFIG.AwsS3.S3ForcePathStyle),
		// DisableSSL：是否禁用 HTTPS，部分内网或自签名证书场景下可以通过配置关闭。
		DisableSSL: aws.Bool(global.GVA_CONFIG.AwsS3.DisableSSL),
		// Credentials：使用静态凭证（AK/SK），从全局配置读取：
		// - 方便通过配置中心/环境变量管理不同环境的密钥；
		// - 不在代码中硬编码，符合安全最佳实践。
		Credentials: credentials.NewStaticCredentials(
			global.GVA_CONFIG.AwsS3.SecretID,
			global.GVA_CONFIG.AwsS3.SecretKey,
			"",
		),
	})
	return sess
}
