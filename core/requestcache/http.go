package requestcache

// 文件职责：根据 HTTP 响应缓存约束计算 HTTP 复用及本地视图数据的内存有效期。
// 使用：请求层确认响应成功后结合 Method 配置调用。
// 边界：按已消耗的响应年龄扣减有效期，不执行条件请求或保存 HTTP 响应头。

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// FreshFor 职责：按响应约束及查询上限计算剩余缓存时间；使用：只传成功响应；边界：显式禁止复用、变种未知或缓存指令无效时返回零。
func FreshFor(header http.Header, now time.Time, limit time.Duration) time.Duration {
	return remainingFreshness(header, now, limit, false)
}

// ViewFreshFor 职责：计算已解码视图数据的本地复用期；使用：业务显式允许同会话、同视图重读时调用；边界：不用于 HTTP 响应缓存，仍采纳重新验证、变种和有效期约束。
func ViewFreshFor(header http.Header, now time.Time, limit time.Duration) time.Duration {
	return remainingFreshness(header, now, limit, true)
}

// remainingFreshness 共享有效期解析，由调用方选择 HTTP 缓存或业务视图数据的策略。
func remainingFreshness(header http.Header, now time.Time, limit time.Duration, viewSnapshot bool) time.Duration {
	if limit <= 0 || len(header.Values("Vary")) > 0 {
		return 0
	}
	var maxAge time.Duration
	hasMaxAge := false
	for _, value := range header.Values("Cache-Control") {
		for directive := range strings.SplitSeq(value, ",") {
			name, value, _ := strings.Cut(directive, "=")
			switch strings.ToLower(strings.TrimSpace(name)) {
			case "no-store":
				if !viewSnapshot {
					return 0
				}
			case "no-cache":
				return 0
			case "max-age":
				age, valid := seconds(strings.TrimSpace(value))
				if !valid || hasMaxAge {
					return 0
				}
				maxAge, hasMaxAge = age, true
			}
		}
	}

	var age time.Duration
	if value := header.Get("Age"); value != "" {
		parsed, valid := seconds(value)
		if !valid {
			return 0
		}
		age = parsed
	}
	date, dateErr := http.ParseTime(header.Get("Date"))
	if dateErr == nil {
		age = max(age, now.Sub(date))
	}
	if hasMaxAge {
		return max(0, min(limit, maxAge-age))
	}
	if value := header.Get("Expires"); value != "" {
		expires, err := http.ParseTime(value)
		if err != nil {
			return 0
		}
		if dateErr != nil {
			date = now
		}
		return max(0, min(limit, expires.Sub(date)-age))
	}
	return max(0, limit-age)
}

// seconds 解析非负整数秒并检查 time.Duration 的范围。
func seconds(value string) (time.Duration, bool) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed > uint64((1<<63-1)/time.Second) {
		return 0, false
	}
	return time.Duration(parsed) * time.Second, true
}
