package factory

// 文件职责：把超过 1 MiB 的生成串暂存为受控文件并分片读取；使用：factory 服务；边界：只接受登记令牌，不提供任意路径读写。

import (
	"DysonSphereBox/helper/blueprint"
	contract "DysonSphereBox/helper/factory/contract"
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"sync"
)

// Artifacts 职责：维护有界临时文件登记；使用：服务实例持有；边界：不触碰蓝图库或游戏目录。
type Artifacts struct {
	// mu 职责：保护登记；使用：并发读写；边界：不跨业务计算持锁。
	mu sync.Mutex
	// paths 职责：令牌映射；使用：分片；边界：外部路径不可信。
	paths map[string]string
	// order 职责：淘汰顺序；使用：最多保留 8 个文件；边界：仅删本实例创建文件。
	order []string
}

// Put 职责：保存生成文本；使用：大对象响应；边界：写入失败不返回可用令牌。
func (a *Artifacts) Put(text string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.paths == nil {
		a.paths = map[string]string{}
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	f, err := os.CreateTemp("", "dsbox-factory-*.txt")
	if err != nil {
		return "", err
	}
	file := f.Name()
	if _, err = f.WriteString(text); err != nil {
		f.Close()
		os.Remove(file)
		return "", err
	}
	if err = f.Close(); err != nil {
		os.Remove(file)
		return "", err
	}
	token := hex.EncodeToString(id[:])
	a.paths[token] = file
	a.order = append(a.order, token)
	for len(a.order) > 8 {
		old := a.order[0]
		a.order = a.order[1:]
		_ = os.Remove(a.paths[old])
		delete(a.paths, old)
	}
	return token, nil
}

// Read 职责：读取登记文件的固定分片；使用：前端组装预览/导出；边界：拒绝未知令牌和非法偏移。
func (a *Artifacts) Read(req contract.ChunkRequest) (contract.ChunkResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := contract.ChunkResult{}
	path, ok := a.paths[req.Token]
	if !ok || req.Offset < 0 {
		return out, &blueprint.Error{Code: "BP_E_FORMAT", Message: "生成内容已过期，请重新生成"}
	}
	f, err := os.Open(path)
	if err != nil {
		return out, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return out, err
	}
	if int64(req.Offset) > info.Size() {
		return out, &blueprint.Error{Code: "BP_E_FORMAT", Message: "片段偏移超出范围"}
	}
	data := make([]byte, 256<<10)
	n, err := f.ReadAt(data, int64(req.Offset))
	if err != nil && err != io.EOF {
		return out, err
	}
	out.Text = string(data[:n])
	out.NextOffset = req.Offset + n
	out.Done = int64(out.NextOffset) >= info.Size()
	return out, nil
}
