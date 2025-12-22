package upload

import (
	"errors"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"go.uber.org/zap"
)

// mu 用于在本地文件删除时加锁，避免多个协程并发删除同一个文件导致竞态问题或系统错误
// 使用包级别的互斥锁而不是文件级别锁实现简单，足够满足 gin-vue-admin 的业务场景
var mu sync.Mutex

type Local struct{}

//@author: [piexlmax](https://github.com/piexlmax)
//@author: [ccfish86](https://github.com/ccfish86)
//@author: [SliverHorn](https://github.com/SliverHorn)
//@object: *Local
//@function: UploadFile
//@description: 上传文件
//@param: file *multipart.FileHeader
//@return: string, string, error

func (*Local) UploadFile(file *multipart.FileHeader) (string, string, error) {
	// 读取文件后缀，保证后续保存的文件依旧保留原有类型（如 .jpg、.png），方便前端 / 浏览器识别
	ext := filepath.Ext(file.Filename)
	// 读取文件名并加密：
	// - 避免直接暴露原始文件名中的敏感信息
	// - 将名称转换为固定格式，减少特殊字符带来的路径问题
	// - 通过 MD5 保证相同文件名在同一时刻生成的基础名一致，便于排查和去重
	name := strings.TrimSuffix(file.Filename, ext)
	name = utils.MD5V([]byte(name))
	// 拼接新文件名：
	// - 增加时间戳，避免同名文件覆盖
	// - 时间戳可以用于后续问题排查（知道大概上传时间）
	filename := name + "_" + time.Now().Format("20060102150405") + ext
	// 尝试创建存储路径：
	// - 使用配置里的 StorePath 作为统一文件根目录，便于迁移和运维维护
	// - os.MkdirAll 保证多级目录不存在时自动创建，提升健壮性
	mkdirErr := os.MkdirAll(global.GVA_CONFIG.Local.StorePath, os.ModePerm)
	if mkdirErr != nil {
		global.GVA_LOG.Error("function os.MkdirAll() failed", zap.Any("err", mkdirErr.Error()))
		return "", "", errors.New("function os.MkdirAll() failed, err:" + mkdirErr.Error())
	}
	// 拼接服务器本地真实存储路径（p）和对外访问路径（filepath）：
	// - p：物理路径，操作系统使用
	// - filepath：URL 前缀 + 文件名，前端或其他服务通过这个路径访问
	p := global.GVA_CONFIG.Local.StorePath + "/" + filename
	filepath := global.GVA_CONFIG.Local.Path + "/" + filename

	// 读取上传文件内容：
	// - 通过 multipart.FileHeader.Open 获取流式数据，避免一次性读到内存导致大文件占用过高
	f, openError := file.Open()
	if openError != nil {
		global.GVA_LOG.Error("function file.Open() failed", zap.Any("err", openError.Error()))
		return "", "", errors.New("function file.Open() failed, err:" + openError.Error())
	}
	// 使用 defer 保证不论函数中途是否出错都会关闭文件句柄，避免 FD 泄漏
	defer f.Close()

	// 在本地磁盘上创建新文件：
	// - 使用 os.Create，如果文件不存在则新建，存在则覆盖（此处文件名是我们生成的，不会与旧文件冲突）
	// - 如创建失败，往往是权限或磁盘问题，直接返回错误便于调用方感知
	out, createErr := os.Create(p)
	if createErr != nil {
		global.GVA_LOG.Error("function os.Create() failed", zap.Any("err", createErr.Error()))

		return "", "", errors.New("function os.Create() failed, err:" + createErr.Error())
	}
	// 同样使用 defer 确保文件句柄被正确释放
	defer out.Close()

	// 通过 io.Copy 将上传流写入磁盘文件：
	// - io.Copy 使用内部缓冲区，高效并且对大文件友好
	// - 不需要手动循环 read/write，代码更简洁可靠
	_, copyErr := io.Copy(out, f)
	if copyErr != nil {
		global.GVA_LOG.Error("function io.Copy() failed", zap.Any("err", copyErr.Error()))
		return "", "", errors.New("function io.Copy() failed, err:" + copyErr.Error())
	}
	// 返回对外可访问路径和文件名：
	// - filepath：前端或其他服务访问用
	// - filename：有些业务只需要保存文件名，自行拼接路径
	return filepath, filename, nil
}

//@author: [piexlmax](https://github.com/piexlmax)
//@author: [ccfish86](https://github.com/ccfish86)
//@author: [SliverHorn](https://github.com/SliverHorn)
//@object: *Local
//@function: DeleteFile
//@description: 删除文件
//@param: key string
//@return: error

func (*Local) DeleteFile(key string) error {
	// 检查 key 是否为空：
	// - 避免无意义的删除请求
	// - 保持接口语义清晰，方便调用方排错
	if key == "" {
		return errors.New("key不能为空")
	}

	// 验证 key 是否包含非法字符或尝试访问存储路径之外的文件：
	// - 防止通过 "../" 等路径穿越，误删或恶意删除服务器其他目录的文件（目录遍历攻击）
	// - Windows / Linux 下的特殊字符过滤，减少路径解析带来的安全风险
	if strings.Contains(key, "..") || strings.ContainsAny(key, `\/:*?"<>|`) {
		return errors.New("非法的key")
	}

	// 使用 filepath.Join 组合路径：
	// - 兼容不同操作系统的路径分隔符
	// - 避免字符串拼接可能出现的多斜杠或漏斜杠问题
	p := filepath.Join(global.GVA_CONFIG.Local.StorePath, key)

	// 删除前先检查文件是否存在：
	// - 可以给出更友好的错误信息，而不是直接返回 os.Remove 的系统错误
	// - 避免在业务上造成「看起来删除成功但其实文件本就不存在」的误解
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return errors.New("文件不存在")
	}

	// 使用互斥锁防止并发删除：
	// - 多个请求同时删除同一个文件时，只允许一个真正执行删除操作
	// - 避免由于并发调用导致的系统层面错误，提高健壮性
	mu.Lock()
	defer mu.Unlock()

	// 调用 os.Remove 执行真正的删除：
	// - 如果失败，将系统错误透传出去，方便排查（如权限不足、文件被占用等）
	err := os.Remove(p)
	if err != nil {
		return errors.New("文件删除失败: " + err.Error())
	}

	return nil
}
