package diagnostics

// 文件职责：统一日志脱敏与错误证据转换。使用：所有入口落盘前调用；边界：不推断故障根因、不记录请求正文。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// secretPattern 职责：拦截文本中的认证信息；使用：错误及 URL 脱敏；边界：不承诺识别任意自由文本秘密。
var secretPattern = regexp.MustCompile(`(?i)(bearer\s+)[^\s"']+|((?:password|passwd|token|secret|authorization|cookie|api[_-]?key)\s*[=:]\s*)[^\s&,;"']+`)

// urlPattern 职责：定位文本 URL；使用：移除凭据和查询值；边界：保留服务与路径用于诊断。
var urlPattern = regexp.MustCompile(`https?://[^\s"'<>\x{ff1a}\x{ff0c}\x{ff08}\x{ff09}\x{3002}]+`)

// CleanText 职责：脱敏并限长文本；使用：正文、错误和数据字符串；边界：用户名路径替换为占位符。
func CleanText(value string) string {
	value = urlPattern.ReplaceAllStringFunc(value, func(raw string) string {
		u, err := url.Parse(raw)
		if err != nil {
			return "[invalid-url]"
		}
		u.User = nil
		q := u.Query()
		for key := range q {
			q.Set(key, "[redacted]")
		}
		u.RawQuery = q.Encode()
		u.Fragment = ""
		return u.String()
	})
	value = secretPattern.ReplaceAllString(value, "${1}${2}[redacted]")
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		value = regexp.MustCompile(`(?i)`+regexp.QuoteMeta(home)).ReplaceAllString(value, "[user-home]")
		value = strings.ReplaceAll(value, strings.ReplaceAll(home, `\`, "/"), "[user-home]")
	}
	if len(value) > 16384 {
		value = strings.ToValidUTF8(value[:16384], "") + "[truncated]"
	}
	return value
}

// sensitiveKey 职责：判定敏感结构字段；使用：递归脱敏；边界：不隐藏用于排障的 packageKey。
func sensitiveKey(key string) bool {
	key = strings.ToLower(strings.NewReplacer("_", "", "-", "", ".", "").Replace(key))
	for _, part := range []string{"password", "passwd", "token", "secret", "authorization", "cookie", "apikey", "requestbody", "responsebody"} {
		if strings.Contains(key, part) {
			return true
		}
	}
	return false
}

// cleanValue 职责：递归清洗 JSON 值；使用：日志数据落盘；边界：限制深度与数组长度。
func cleanValue(value interface{}, depth int) interface{} {
	if depth > 10 {
		return "[depth-limit]"
	}
	switch item := value.(type) {
	case map[string]interface{}:
		out := map[string]interface{}{}
		for key, v := range item {
			if sensitiveKey(key) {
				out[key] = "[redacted]"
			} else {
				out[key] = cleanValue(v, depth+1)
			}
		}
		return out
	case []interface{}:
		out := []interface{}{}
		for i, v := range item {
			if i >= 200 {
				out = append(out, "[truncated]")
				break
			}
			out = append(out, cleanValue(v, depth+1))
		}
		return out
	case string:
		return CleanText(item)
	default:
		return item
	}
}

// CleanData 职责：规范化、复制并脱敏数据；使用：写入前隔离调用方 map；边界：序列化失败仍留下诊断证据。
func CleanData(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return map[string]interface{}{}
	}
	b, err := json.Marshal(data)
	if err != nil {
		return map[string]interface{}{"serializationError": CleanText(err.Error()), "result": "unknown"}
	}
	var value map[string]interface{}
	if err = json.Unmarshal(b, &value); err != nil {
		return map[string]interface{}{"serializationError": CleanText(err.Error())}
	}
	return cleanValue(value, 0).(map[string]interface{})
}

// ErrorData 职责：保留原始错误链与可验证分类；使用：失败记录 data.error；边界：堆栈只在异常边界另行采集。
func ErrorData(err error) map[string]interface{} {
	if err == nil {
		return nil
	}
	code := "unknown"
	var networkError net.Error
	switch {
	case errors.Is(err, context.Canceled):
		code = "cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		code = "timeout"
	case errors.As(err, &networkError) && networkError.Timeout():
		code = "timeout"
	case errors.Is(err, os.ErrPermission):
		code = "permission_denied"
	case errors.Is(err, os.ErrNotExist):
		code = "not_found"
	case errors.Is(err, os.ErrExist):
		code = "already_exists"
	}
	causes := []string{}
	for current, i := err, 0; current != nil && i < 12; current, i = errors.Unwrap(current), i+1 {
		causes = append(causes, CleanText(current.Error()))
	}
	return map[string]interface{}{"code": code, "type": fmt.Sprintf("%T", err), "message": CleanText(err.Error()), "causes": causes}
}
