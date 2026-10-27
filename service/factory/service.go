package factory

// 文件职责：本地量化/蓝图公共 API 编排；使用：生成器页面调用；边界：无数据库、账号或服务器依赖，所有失败通过响应 code 传递。

import (
	"DysonSphereBox/core"
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build"
	factoryHelper "DysonSphereBox/helper/factory"
	contract "DysonSphereBox/helper/factory/contract"
	"DysonSphereBox/helper/quant"
	systemHelper "DysonSphereBox/helper/system"
	"context"
	"fmt"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
)

// Service 职责：校验输入、保存有限方案快照并编排纯引擎；使用：Wails 绑定；边界：构造无副作用，不将账号或真实游戏资源纳入计算。
type Service struct {
	// mu 职责：保护快照；使用：Solve/Generate；边界：计算不持锁。
	mu sync.Mutex
	// plans 职责：后端可信求解结果；使用：防篡改生成；边界：仅最近 16 个。
	plans map[string]quant.Result
	// order 职责：快照淘汰顺序；使用：限制内存；边界：不持久化。
	order []string
	// busy 职责：限制同一时刻一个生成任务；使用：Generate；边界：失败也必须释放。
	busy atomic.Bool
	// artifacts 职责：管理大串临时文件；使用：分片读取；边界：不访问用户指定路径。
	artifacts factoryHelper.Artifacts
}

// ListDatasets 职责：返回 12 个内置组合；使用：页面初始化；边界：不运行联网发现。
func (s *Service) ListDatasets() contract.DatasetResponse {
	data, err := quant.ListDatasets()
	return contract.DatasetResponse{Status: factoryHelper.Status(err), Data: data}
}

// DescribeDataset 职责：返回当前物品与配方选项；使用：目标及人工方案控件；边界：前端不执行配方计算。
func (s *Service) DescribeDataset(key string) contract.DatasetDetailResponse {
	d, err := quant.LoadDataset(key)
	if err != nil {
		return contract.DatasetDetailResponse{Status: factoryHelper.Status(err)}
	}
	return contract.DatasetDetailResponse{Status: factoryHelper.Status(nil), Data: &contract.DatasetDetail{Info: d.Info, Items: d.Items, Recipes: d.Recipes}}
}

// Solve 职责：校验并执行量化，保存可信快照；使用：工作区量化按钮；边界：失败不缓存部分结果，不修改数据集。
func (s *Service) Solve(req quant.Request) contract.QuantResponse {
	data, err := quant.Solve(context.Background(), req)
	if err != nil {
		systemHelper.RecordFailure("factory", "quant", "solve", "量化求解失败", err, map[string]interface{}{"targets": len(req.Targets), "dataset": req.Dataset})
		return contract.QuantResponse{Status: factoryHelper.Status(err)}
	}
	s.mu.Lock()
	if s.plans == nil {
		s.plans = map[string]quant.Result{}
	}
	if _, ok := s.plans[data.PlanID]; !ok {
		s.order = append(s.order, data.PlanID)
	}
	s.plans[data.PlanID] = data
	for len(s.order) > 16 {
		delete(s.plans, s.order[0])
		s.order = s.order[1:]
	}
	s.mu.Unlock()
	return contract.QuantResponse{Status: factoryHelper.Status(nil), Data: &data}
}

// Decode 职责：严格解析蓝图；使用：导入/诊断；边界：缺省校验 MD5F，忽略校验必须显式选择。
func (s *Service) Decode(req contract.DecodeRequest) contract.DecodeResponse {
	strict := true
	if req.Strict != nil {
		strict = *req.Strict
	}
	data, err := blueprint.Decode(req.Blueprint, strict)
	if err != nil {
		systemHelper.RecordFailure("factory", "blueprint", "decode", "蓝图解码失败", err, map[string]interface{}{"length": len(req.Blueprint)})
		return contract.DecodeResponse{Status: factoryHelper.Status(err)}
	}
	return contract.DecodeResponse{Status: factoryHelper.Status(nil), Data: &data}
}

// Encode 职责：编码并自检；使用：无损写回；边界：始终重算校验，大串分片，不进行资源落盘。
func (s *Service) Encode(req contract.EncodeRequest) contract.EncodeResponse {
	envelope := req.Document.EnvelopeVersion
	if req.Envelope != nil {
		envelope = *req.Envelope
	}
	data, err := blueprint.Encode(req.Document, envelope)
	if err != nil {
		return contract.EncodeResponse{Status: factoryHelper.Status(err)}
	}
	if len(data.Blueprint) > 1<<20 {
		data.BlueprintRef, err = s.artifacts.Put(data.Blueprint)
		if err != nil {
			return contract.EncodeResponse{Status: factoryHelper.Status(err)}
		}
		data.BlueprintLength = len(data.Blueprint)
		data.Blueprint = ""
	}
	return contract.EncodeResponse{Status: factoryHelper.Status(nil), Data: &data}
}

// Generate 职责：复核方案后串行生成、自检；使用：生成按钮；边界：只接受当前进程求解的未篡改方案，不重启过期任务。
func (s *Service) Generate(req build.Request) contract.GenerateResponse {
	if !s.busy.CompareAndSwap(false, true) {
		return contract.GenerateResponse{Status: contract.Status{Code: "GEN_E_BUSY", Message: "已有生成任务运行中"}}
	}
	defer s.busy.Store(false)
	s.mu.Lock()
	plan, ok := s.plans[req.Plan.PlanID]
	s.mu.Unlock()
	if !ok || !reflect.DeepEqual(plan, req.Plan) {
		return contract.GenerateResponse{Status: contract.Status{Code: "GEN_E_PLAN_STALE", Message: "方案已过期或内容改变，请重新量化"}}
	}
	req.Plan = plan
	data, err := build.Generate(context.Background(), req)
	if err != nil {
		systemHelper.RecordFailure("factory", "blueprint", "generate", "蓝图生成失败", err, map[string]interface{}{"planId": plan.PlanID, "mode": req.Mode, "machines": plan.MachineCount})
		return contract.GenerateResponse{Status: factoryHelper.Status(err)}
	}
	if len(data.Blueprint) > 1<<20 {
		data.BlueprintRef, err = s.artifacts.Put(data.Blueprint)
		if err != nil {
			return contract.GenerateResponse{Status: factoryHelper.Status(err)}
		}
		data.BlueprintLength = len(data.Blueprint)
		data.Blueprint = ""
	}
	return contract.GenerateResponse{Status: factoryHelper.Status(nil), Data: &data}
}

// Inspect 职责：只读反算配方理论速率；使用：导入或库内回放；边界：不伪造量化结果，不修改原资源。
func (s *Service) Inspect(req contract.InspectRequest) contract.InspectResponse {
	data, err := factoryHelper.Inspect(req)
	if err != nil {
		return contract.InspectResponse{Status: factoryHelper.Status(err)}
	}
	return contract.InspectResponse{Status: factoryHelper.Status(nil), Data: &data}
}

// ReadBlueprintChunk 职责：分片读取本实例生成的大串；使用：预览及导出；边界：未知令牌拒绝，不接受路径。
func (s *Service) ReadBlueprintChunk(req contract.ChunkRequest) contract.ChunkResponse {
	data, err := s.artifacts.Read(req)
	if err != nil {
		return contract.ChunkResponse{Status: factoryHelper.Status(err)}
	}
	return contract.ChunkResponse{Status: factoryHelper.Status(nil), Data: &data}
}

// ExportBlueprint 职责：通过原生对话框导出校验后的文本；使用：页面导出按钮；边界：取消不写入，只能保存 .txt，不依赖浏览器下载策略。
func (s *Service) ExportBlueprint(name, text string) (string, error) {
	if _, err := blueprint.Decode(text, true); err != nil {
		return "", err
	}
	if core.GLOBAL.RuntimeContext == nil {
		return "", fmt.Errorf("桌面运行时尚未初始化")
	}
	target, err := runtime.SaveFileDialog(core.GLOBAL.RuntimeContext, runtime.SaveDialogOptions{Title: "导出蓝图文本", DefaultFilename: factoryHelper.ExportName(name), Filters: []runtime.FileFilter{{DisplayName: "蓝图文本 (*.txt)", Pattern: "*.txt"}}})
	if err != nil || target == "" {
		return "", err
	}
	if err = factoryHelper.ExportText(target, text); err != nil {
		systemHelper.RecordFailure("factory", "blueprint", "export", "导出蓝图失败", err, map[string]interface{}{"length": len(text)})
		return "", err
	}
	factoryHelper.RecordExport(len(text))
	return target, nil
}

// ExportWorkspace 职责：把经重新求解复核的方案保存为 JSON；使用：页面导出方案；边界：原生对话框取消不写文件，不新增持久化模型。
func (s *Service) ExportWorkspace(req contract.WorkspaceRequest) (string, error) {
	text, err := factoryHelper.MarshalWorkspace(req)
	if err != nil {
		return "", err
	}
	if core.GLOBAL.RuntimeContext == nil {
		return "", fmt.Errorf("桌面运行时尚未初始化")
	}
	name := strings.TrimSuffix(factoryHelper.ExportName(req.Name), ".txt") + ".json"
	target, err := runtime.SaveFileDialog(core.GLOBAL.RuntimeContext, runtime.SaveDialogOptions{Title: "导出计算方案", DefaultFilename: name, Filters: []runtime.FileFilter{{DisplayName: "计算方案 (*.json)", Pattern: "*.json"}}})
	if err != nil || target == "" {
		return "", err
	}
	if err = factoryHelper.ExportJSON(target, text); err != nil {
		systemHelper.RecordFailure("factory", "workspace", "export", "导出计算方案失败", err, map[string]interface{}{"targets": len(req.Request.Targets)})
		return "", err
	}
	factoryHelper.RecordWorkspaceExport(len(req.Request.Targets))
	return target, nil
}
