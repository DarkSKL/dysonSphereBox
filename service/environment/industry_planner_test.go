package environment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"DysonSphereBox/helper/industryplan"
	work "DysonSphereBox/helper/industryworkspace"
	"DysonSphereBox/helper/quant"
)

// TestIndustrySelectionLifecycle 确认选档只接受授权目录返回的 ID，坏档及撤销授权不会保留可用结果。
func TestIndustrySelectionLifecycle(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "测试.dsv"), []byte("未完成存档"), 0600); err != nil {
		t.Fatal(err)
	}
	s := IndustryPlannerService{}
	if _, err := s.ListSaves(); err == nil {
		t.Fatal("未选目录就列出了存档")
	}
	files, err := s.useRoot(dir, false)
	if err != nil || len(files) != 1 {
		t.Fatal("存档目录扫描失败", err)
	}
	if _, err := s.SelectSave(filepath.Join(dir, "测试.dsv")); err == nil {
		t.Fatal("接受了路径作为 ID")
	}
	if _, err := s.SelectSave(files[0].ID); err == nil {
		t.Fatal("损坏存档被接受")
	}
	if _, err := s.Start(work.Request{SaveID: files[0].ID, SnapshotID: "过期快照"}); err == nil {
		t.Fatal("没有有效快照就能发起规划")
	}
	s.RevokeRead()
	if _, err := s.GetTask(0); err == nil {
		t.Fatal("撤销授权后仍能读取任务")
	}
}

// TestIndustryDesktopSaveSession 通过实际绑定服务覆盖读档、替代任务、取消、预览分片及撤销后的过期响应。
func TestIndustryDesktopSaveSession(t *testing.T) {
	sample := os.Getenv("DSP_SAVE_SAMPLE")
	if sample == "" {
		t.Skip("未指定真实存档副本")
	}
	s := IndustryPlannerService{}
	files, err := s.useRoot(filepath.Dir(sample), false)
	if err != nil {
		t.Fatal(err)
	}
	var id string
	for _, f := range files {
		if filepath.Base(f.RelativePath) == filepath.Base(sample) {
			id = f.ID
			break
		}
	}
	if id == "" {
		t.Fatal("真实存档未进入目录清单")
	}
	selected, err := s.SelectSave(id)
	if err != nil {
		t.Fatal(err)
	}
	off := false
	r := work.Request{SaveID: id, SnapshotID: selected.SnapshotID, Planning: industryplan.Request{CenterPlanetID: 301, Profile: "low", EnableMining: &off, Quant: quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 30}}}}}
	poll := func(id int64, desired string) {
		t.Helper()
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			state, err := s.GetTask(id)
			if err != nil {
				t.Fatal(err)
			}
			if state.State == desired {
				return
			}
			if state.State != "running" && state.State != "cancelling" {
				t.Fatal("任务终态与预期不符", state)
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("等待状态超时", desired)
	}
	first, err := s.Start(r)
	if err != nil {
		t.Fatal(err)
	}
	s.Cancel(first.ID)
	poll(first.ID, "cancelled")
	if _, err = s.GetReport(first.ID); err == nil {
		t.Fatal("已取消的任务仍有可导出结果")
	}
	retired, err := s.Start(r)
	if err != nil {
		t.Fatal(err)
	}
	// 桥接每次重新反序列化请求，使用单独的目标切片以复现正常客户端调用。
	r.Planning.Quant.Targets = []quant.Target{{ItemID: 1101, RatePerMinute: 36}}
	current, err := s.Start(r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.GetTask(retired.ID); err == nil {
		t.Fatal("旧任务仍可查询")
	}
	s.Cancel(retired.ID)
	poll(current.ID, "ready")
	report, err := s.GetReport(current.ID)
	if err != nil || !report.Exportable || len(report.Planets) != 1 || report.Targets[0].RatePerMinute != 36 {
		t.Fatal("完整报告未归属最新任务", err)
	}
	chunk, err := s.ReadBlueprint(current.ID, report.Planets[0].ID, 0)
	if err != nil || !strings.HasPrefix(chunk.Text, "BLUEPRINT:") {
		t.Fatal("当前星球蓝图预览失败", err)
	}
	s.RevokeRead()
	if _, err = s.ReadBlueprint(current.ID, report.Planets[0].ID, 0); err == nil {
		t.Fatal("关闭存档后仍可读取旧蓝图")
	}
}
