package digest

// 文件职责：在不复制完整序列化结果的情况下计算JSON内容摘要。
// 使用：大量重复检查不可变内容；边界：保持标准库JSON兼容字节，错误不返回部分摘要。

import (
	"crypto/sha256"
	"hash"

	json "github.com/goccy/go-json"
)

// jsonHashWriter 职责：保留流的最后一个字节以排除Encoder附加的换行；使用：JSON；边界：不缓存整个内容，Write必须报告全部已消费字节。
type jsonHashWriter struct {
	digest  hash.Hash
	last    [1]byte
	pending bool
}

// Write 职责：将当前块之外的最后一个待定字节保留；使用：标准JSON编码器；边界：仅省略最终终止换行，不改变字符串内部换行或空白。
func (writer *jsonHashWriter) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if writer.pending {
		_, _ = writer.digest.Write(writer.last[:])
	}
	if len(data) > 1 {
		_, _ = writer.digest.Write(data[:len(data)-1])
	}
	writer.last[0] = data[len(data)-1]
	writer.pending = true
	return len(data), nil
}

// JSON 职责：返回与sha256.Sum256(json.Marshal(value))完全相同的摘要；使用：对象内容校验；边界：保持标准库排序/转义/非有限数拒绝，不使用自制序列化。
func JSON(value any) ([32]byte, error) {
	writer := jsonHashWriter{digest: sha256.New()}
	if err := json.NewEncoder(&writer).Encode(value); err != nil {
		return [32]byte{}, err
	}
	// Encoder成功后必有一个末尾换行；保留其它末字节以避免依赖写入分块方式。
	if writer.pending && writer.last[0] != '\n' {
		_, _ = writer.digest.Write(writer.last[:])
	}
	var result [32]byte
	writer.digest.Sum(result[:0])
	return result, nil
}
