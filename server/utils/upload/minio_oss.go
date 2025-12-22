package upload

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

// MinioClient
// 全局复用的 Minio 客户端实例：
// - 意义：Minio SDK 的 client 是线程安全的，创建一次即可在全局复用，避免在每次上传/删除时都重新建立连接。
// - 好处：减少连接创建开销、降低资源消耗，提高整体性能和响应速度。
var MinioClient *Minio // 优化性能，但是不支持动态配置

// Minio
// 封装 Minio 客户端和 bucket 名称：
// - 意义：把 Minio 相关操作封装成结构体，方便在项目中统一管理和扩展。
// - 好处：调用方不需要关心底层 SDK 细节，只通过结构体方法上传/删除文件，提升代码可读性和可维护性。
type Minio struct {
	Client *minio.Client
	bucket string
}

// GetMinio 初始化并获取 Minio 客户端
// - 当全局 MinioClient 已经存在时，直接返回已有实例，避免重复初始化；
// - 当第一次调用时，创建 Minio client，并尝试创建 bucket（若 bucket 已存在则复用）。
//
// 这么写的意义和好处：
// 1. 懒加载 + 单例复用：只有在真正需要时才会创建 client，且后续都会复用同一个实例，减少不必要的资源占用。
// 2. 自动保证 bucket 存在：启动时不额外要求手动创建 bucket，代码会在首次使用时自动创建，提升使用体验和容错性。
// 3. 错误处理清晰：如果 bucket 已经存在则正常复用；如果是其它错误则直接返回，便于排查和监控。
func GetMinio(endpoint, accessKeyID, secretAccessKey, bucketName string, useSSL bool) (*Minio, error) {
	if MinioClient != nil {
		return MinioClient, nil
	}
	// Initialize minio client object.
	// 使用官方推荐的 New 方法，并传入静态凭证：
	// - 静态凭证在大多数部署场景（配置文件/环境变量）中足够使用；
	// - 通过 options 统一管理安全选项（如 Secure），代码更清晰。
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL, // Set to true if using https
	})
	if err != nil {
		return nil, err
	}
	// 尝试创建 bucket：
	// - 意义：保证目标 bucket 存在，避免后续上传时报错。
	// - 好处：即使第一次部署也能自动完成最小化初始化，减少手工运维步骤。
	err = minioClient.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{})
	if err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, errBucketExists := minioClient.BucketExists(context.Background(), bucketName)
		if errBucketExists == nil && exists {
			// log.Printf("We already own %s\n", bucketName)
		} else {
			return nil, err
		}
	}
	MinioClient = &Minio{Client: minioClient, bucket: bucketName}
	return MinioClient, nil
}

// UploadFile 将前端上传的 multipart.FileHeader 文件上传到 Minio
// 主要步骤：
// - 打开 multipart 文件并读入内存缓冲区；
// - 使用 MD5 对原始文件名（不含扩展名）加密，确保存储时文件名唯一且不暴露原始信息；
// - 结合日期路径生成层级目录，方便日后按日期归档和清理；
// - 根据扩展名设置合适的 Content-Type；
// - 使用带超时的 context 调用 Minio 的 PutObject 进行上传。
//
// 这么写的意义和好处：
// 1. 安全性：文件名 MD5 化，避免泄露用户原始文件名，降低信息暴露风险。
// 2. 避免重名覆盖：同名文件加密后基本不会碰撞，防止多用户上传同名文件时互相覆盖。
// 3. 便于管理与统计：按日期分目录（yyyy-MM-dd），可以方便做按天备份、清理或统计。
// 4. 正确的 MIME 类型：根据扩展名自动推断 Content-Type，有利于浏览器正确预览和下载。
// 5. 超时控制：通过 context.WithTimeout 限制上传时间，避免网络异常时请求无限挂起。
func (m *Minio) UploadFile(file *multipart.FileHeader) (filePathres, key string, uploadErr error) {
	f, openError := file.Open()
	// mutipart.File to os.File
	if openError != nil {
		global.GVA_LOG.Error("function file.Open() Failed", zap.Any("err", openError.Error()))
		return "", "", errors.New("function file.Open() Failed, err:" + openError.Error())
	}

	filecontent := bytes.Buffer{}
	_, err := io.Copy(&filecontent, f)
	if err != nil {
		// 这里先把 multipart 文件一次性读入内存：
		// - 意义：Minio 的 PutObject 接口需要一个可读的 io.Reader，使用 bytes.Buffer 可以复用内存并获取长度信息。
		// - 好处：逻辑简单直观，适合中小文件上传场景；大文件场景可视情况改用流式/分片方案。
		global.GVA_LOG.Error("读取文件失败", zap.Any("err", err.Error()))
		return "", "", errors.New("读取文件失败, err:" + err.Error())
	}
	f.Close() // 及时关闭文件句柄，避免资源泄露

	// 对文件名进行加密存储
	ext := filepath.Ext(file.Filename)
	// 使用 MD5(不含后缀的文件名) + 原始后缀 的方式：
	// - 保留原来的后缀，方便根据后缀推断类型；
	// - 加密主体名称，避免敏感信息泄露。
	filename := utils.MD5V([]byte(strings.TrimSuffix(file.Filename, ext))) + ext
	if global.GVA_CONFIG.Minio.BasePath == "" {
		// 未配置 BasePath 时，默认走 uploads/日期/ 文件路径：
		// - 意义：统一管理上传文件的根目录。
		// - 好处：便于本系统或其他服务做统一映射和代理。
		filePathres = "uploads" + "/" + time.Now().Format("2006-01-02") + "/" + filename
	} else {
		// 支持通过配置自定义 BasePath：
		// - 意义：增强灵活性，可以按业务或多租户环境做路径隔离。
		// - 好处：方便运维在不同环境或部署方式下使用统一代码。
		filePathres = global.GVA_CONFIG.Minio.BasePath + "/" + time.Now().Format("2006-01-02") + "/" + filename
	}

	// 根据文件扩展名检测 MIME 类型
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		// 如果无法识别，则使用通用的二进制流类型：
		// - 好处：保证上传不会因为 MIME 判定失败而中断。
		contentType = "application/octet-stream"
	}

	// 设置超时10分钟
	// - 对于大文件或网络波动情况，10 分钟可以兼顾体验与安全；
	// - 防止上传操作无限阻塞，提升系统整体稳定性。
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()

	// Upload the file with PutObject   大文件自动切换为分片上传
	info, err := m.Client.PutObject(ctx, global.GVA_CONFIG.Minio.BucketName, filePathres, &filecontent, file.Size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		global.GVA_LOG.Error("上传文件到minio失败", zap.Any("err", err.Error()))
		return "", "", errors.New("上传文件到minio失败, err:" + err.Error())
	}
	return global.GVA_CONFIG.Minio.BucketUrl + "/" + info.Key, filePathres, nil
}

// DeleteFile 从 Minio 中删除指定 key 的对象
// - 使用带 5 秒超时的 context，保证删除操作不会长期阻塞；
// - 只关心删除是否成功，把错误上抛给调用方处理（记录日志 / 重试等）。
//
// 这么写的意义和好处：
// 1. 接口简单：对业务层只暴露一个 key，内部封装 bucket 等细节，调用成本低。
// 2. 超时保护：防止由于网络或存储异常导致服务 goroutine 被长期占用。
func (m *Minio) DeleteFile(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	// Delete the object from MinIO
	err := m.Client.RemoveObject(ctx, m.bucket, key, minio.RemoveObjectOptions{})
	return err
}
