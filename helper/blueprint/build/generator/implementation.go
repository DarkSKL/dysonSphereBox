package generator

// 文件职责：为反射工具目录绑定实际执行构建身份。
// 使用：注册和执行计划指纹；边界：只读取当前进程可执行文件一次，不把固定契约版本当作实现指纹，不输出源码或环境信息。

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"sync"
)

// implementationCache 职责：缓存实际二进制摘要；使用：同进程多个方案编译；边界：只读构建身份，不持有任务状态。
var implementationCache struct {
	sync.Once
	value string
	err   error
}

// implementationIdentity 职责：区分契约相同但算法修复不同的构建；使用：ToolSpec.Implementation；边界：无法确定运行构建时拒绝初始化，不编造稳定版本。
func implementationIdentity() (string, error) {
	implementationCache.Do(func() {
		path, err := os.Executable()
		if err != nil {
			implementationCache.err = err
			return
		}
		file, err := os.Open(path)
		if err != nil {
			implementationCache.err = err
			return
		}
		defer file.Close()
		hash := sha256.New()
		if _, err = io.Copy(hash, file); err != nil {
			implementationCache.err = err
			return
		}
		implementationCache.value = "blueprint-core-v1/sha256:" + hex.EncodeToString(hash.Sum(nil))
	})
	return implementationCache.value, implementationCache.err
}
