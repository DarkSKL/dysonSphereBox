package diagnostic

// 文件职责：按安装器已串行保护的 packageKey 关联阶段、回滚和服务结果。使用：服务 Begin、进度 Observe、审计 Apply；边界：注册冲突不覆盖现有操作，不猜测取消与冲突为失败。

import (
	"DysonSphereBox/constant/logging"
	"DysonSphereBox/core/diagnostics"
	"DysonSphereBox/helper/modDependence/installer/v2/progress/payload"
	systemHelper "DysonSphereBox/helper/system"
	"sync"
	"time"
)

// operation 职责：记录一次安装器请求上下文；使用：内部阶段关联；边界：保护并发访问。
type operation struct {
	id      string
	handler string
	begin   time.Time
	stage   string
}

// operations 职责：按资源关联当前请求；使用：跨阶段诊断；边界：服务 defer 删除。
var operations = struct {
	sync.Mutex
	items map[string]*operation
}{items: map[string]*operation{}}

// Begin 职责：建立操作关联；使用：服务执行前；边界：同包已有操作时返回无副作用释放函数。
func Begin(key, action, handler string) func() {
	operations.Lock()
	if operations.items[key] != nil {
		operations.Unlock()
		return func() {}
	}
	op := &operation{id: diagnostics.NewID(), handler: handler, begin: time.Now()}
	operations.items[key] = op
	operations.Unlock()
	return func() {
		operations.Lock()
		if operations.items[key] == op {
			delete(operations.items, key)
		}
		operations.Unlock()
	}
}

// Apply 职责：继承资源操作关联与发起者；使用：结果及清理埋点；边界：没有注册上下文不伪造关联号。
func Apply(entry systemHelper.LogEntry, key string) systemHelper.LogEntry {
	operations.Lock()
	defer operations.Unlock()
	if op := operations.items[key]; op != nil {
		entry.OperationID, entry.Handler = op.id, op.handler
		if entry.Data == nil {
			entry.Data = map[string]interface{}{}
		}
		entry.Data["durationMs"] = time.Since(op.begin).Milliseconds()
	}
	return entry
}

// Observe 职责：仅在阶段或状态变化时记录进度；使用：安装器 event；边界：不记录每个下载百分比，保持证据可读。
func Observe(data payload.Payload) {
	operations.Lock()
	op := operations.items[data.PackageKey]
	if op == nil {
		operations.Unlock()
		return
	}
	stage := data.Stage + ":" + data.State
	if stage == op.stage {
		operations.Unlock()
		return
	}
	op.stage = stage
	id, handler, elapsed := op.id, op.handler, time.Since(op.begin).Milliseconds()
	operations.Unlock()
	level := logging.Info
	if data.State == "failed" {
		level = logging.Error
	} else if data.State == "conflict" {
		level = logging.Warn
	}
	info := map[string]interface{}{"packageKey": data.PackageKey, "stage": data.Stage, "state": data.State, "percent": data.Percent, "message": data.Message, "durationMs": elapsed, "result": data.State}
	if data.ErrorMessage != "" {
		info["error"] = map[string]interface{}{"message": data.ErrorMessage}
	}
	if data.Conflict != nil {
		info["conflict"] = data.Conflict
	}
	diagnostics.Default.Record(logging.Entry{Action: "安装器阶段变化", Level: level, Source: "backend.installer", Reason: "安装器报告阶段或状态变化", Namespace: "modDependence.installer.stage", Handler: handler, OperationID: id, Data: info})
}
