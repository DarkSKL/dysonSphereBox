package thunderstoreindex

// 文件职责：提供 Thunderstore 分片索引响应的 gzip JSON 解码辅助。
// 使用：chunkedIndexProvider 解码 package-listing-index 和 chunk blob 时调用。
// 边界：只处理字节解压与 JSON 反序列化，不发起 HTTP 请求、不选择 provider、不写入缓存。

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
)

// decodeGzipJSON 将 gzip 压缩的 JSON 字节解码到目标对象。
// 职责：封装 Thunderstore 新版索引 blob 的 gzip 解压和 JSON 解码细节。
// 使用：chunkedIndexProvider 在读取 index blob 与 chunk blob 后调用。
// 边界：target 必须是可写入的指针；解压或 JSON 失败时返回带阶段上下文的错误。
// 逻辑流：1. 创建 gzip reader => 2. 解压读取 JSON => 3. 写入调用方提供的目标对象。
func decodeGzipJSON(data []byte, target interface{}) error {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer reader.Close()

	if err := json.NewDecoder(reader).Decode(target); err != nil {
		return fmt.Errorf("json decode: %w", err)
	}
	return nil
}
