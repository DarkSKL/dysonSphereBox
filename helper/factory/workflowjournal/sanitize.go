package workflowjournal

// 文件职责：限制事件中的公开文本与参数载荷；使用：日志 Append；边界：不从实体或日志文本推测事件，不扩大引擎白名单。

import (
	"encoding/json"
	"sort"
	"strings"
	"unicode/utf8"

	"DysonSphereBox/helper/blueprint/build/workflow"
)

// validID 职责：限制身份及控制字符；使用：公开读写边界；边界：不接受空身份或超过一百二十八字节。
func validID(id string) bool {
	return id != "" && len(id) <= 128 && utf8.ValidString(id) && !strings.ContainsAny(id, "\x00\r\n\t")
}

// bounded 职责：限制 UTF-8 文本字节；使用：错误和身份字段；边界：不留下半个字符。
func bounded(value string, maximum int) string {
	if len(value) > maximum {
		value = value[:maximum]
	}
	for !utf8.ValidString(value) && len(value) > 0 {
		value = value[:len(value)-1]
	}
	return strings.Clone(value)
}

// sanitize 职责：深复制并收紧纯 DTO 文本及参数；使用：Append；边界：非法或超大参数整体省略并明确标记，不输出未知对象。
func sanitize(event workflow.ExecutionEvent) workflow.ExecutionEvent {
	diagnostics := make([]string, 0, min(len(event.Diagnostics), 64))
	diagnosticBytes := 0
	for index, item := range event.Diagnostics {
		value := bounded(item, 2048)
		raw, _ := json.Marshal(value)
		if index >= 64 || diagnosticBytes+len(raw)+1 > (16<<10)-2 {
			event.DiagnosticsTruncated = true
			break
		}
		diagnostics = append(diagnostics, value)
		diagnosticBytes += len(raw) + 1
		if value != item {
			event.DiagnosticsTruncated = true
		}
	}
	event.Diagnostics = diagnostics
	// 关联身份必须原样保存；整条事件超过预算时由 Append 明确拒绝，不能截断后错误地关联其他节点。
	event.Failure = bounded(event.Failure, 2048)
	event.FailureCode = bounded(event.FailureCode, 128)
	if len(event.EffectiveParameters) > 16<<10 || (len(event.EffectiveParameters) > 0 && !json.Valid(event.EffectiveParameters)) {
		event.EffectiveParameters = nil
		event.ParametersTruncated = true
	} else {
		event.EffectiveParameters = append(json.RawMessage(nil), event.EffectiveParameters...)
	}
	keys := make([]string, 0, len(event.ParameterSources))
	for key := range event.ParameterSources {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	sources := map[string]string{}
	for index, key := range keys {
		if index >= 128 {
			event.ParametersTruncated = true
			break
		}
		name, source := bounded(key, 128), bounded(event.ParameterSources[key], 256)
		if name != key || source != event.ParameterSources[key] {
			event.ParametersTruncated = true
		}
		sources[name] = source
	}
	if len(sources) == 0 {
		event.ParameterSources = nil
	} else {
		event.ParameterSources = sources
	}
	return event
}
