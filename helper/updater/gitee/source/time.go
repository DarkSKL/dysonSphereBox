package source

// 文件职责：提供 Gitee Release 时间字段解码和缺失 ID 稳定生成工具。
// 使用：由 giteeRelease 和 giteeAsset 在 JSON 解码、normalize 过程中处理时间与 ID 兼容性。
// 边界：不访问网络、不写入文件、不决定版本是否可更新。

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"
)

// giteeTime 表示 Gitee API 返回的时间字段。
// 职责：兼容 Gitee Release 中可能为空、null 或 RFC3339 字符串的时间值。
// 使用：giteeRelease.CreatedAt 和 PublishedAt 字段解码时使用。
// 边界：只处理 JSON 解码，不负责格式化给前端展示。
type giteeTime time.Time

// UnmarshalJSON 解析 Gitee 返回的 RFC3339 时间字段。
// 职责：把 Gitee 时间字符串转换为 time.Time 兼容值。
// 使用：encoding/json 解码 giteeRelease 时自动调用。
// 边界：空字符串和 null 会被视为零值时间；非法格式返回带上下文的解析错误。
func (model *giteeTime) UnmarshalJSON(data []byte) error {
	value := strings.Trim(string(data), `"`)
	if value == "" || value == "null" {
		*model = giteeTime(time.Time{})
		return nil
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return fmt.Errorf("parse gitee time %q: %w", value, err)
	}

	*model = giteeTime(parsed)
	return nil
}

// stableID 根据文本生成非负稳定 ID。
// 职责：为缺失 ID 的 Gitee Release 或附件生成 go-selfupdate 可用标识。
// 使用：giteeRelease.normalize 和 giteeAsset.normalize 在远端 ID 缺失时调用。
// 边界：只保证同一输入稳定，不保证跨不同输入绝对无碰撞；不用于数据库主键。
func stableID(value string) int64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(value))

	return int64(hash.Sum64() & 0x7fffffffffffffff)
}
