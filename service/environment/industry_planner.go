package environment

// 文件职责：管理跨星球规划会话的选档、取消和结果归属。业务请求只接收已列出的 saveId。

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"DysonSphereBox/core"
	envhelper "DysonSphereBox/helper/environment"
	"DysonSphereBox/helper/factory"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/industryplan/search"
	work "DysonSphereBox/helper/industryworkspace"
	"DysonSphereBox/helper/saveanalysis"
	systemhelper "DysonSphereBox/helper/system"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// IndustryPlannerService 负责只读存档与最新一次规划任务；注册时不读取配置或启动工作线程。
type IndustryPlannerService struct {
	mu          sync.Mutex
	loadMu      sync.Mutex
	root        string
	configured  bool
	epoch       int64
	readContext context.Context
	stopRead    context.CancelFunc
	snapshot    *saveanalysis.Snapshot
	selection   work.Selection
	world       industryplan.World
	tasks       factory.GenerationTasks
	nextID      int64
	task        work.Task
	cancel      context.CancelFunc
	// search 职责：保留暂停中的搜索栈；使用：Continue；边界：切档和新任务仍取消旧上下文。
	search *search.Control
	bundle *work.Bundle
}

// GetDirectory 仅解析可供玩家选择的目录，不读取存档。
func (s *IndustryPlannerService) GetDirectory() envhelper.RootResolution {
	return (envhelper.Resolver{}).ResolveSaveRoot()
}

// AuthorizeRead 接收界面刚展示的目录，对当前会话开放只读选档。
func (s *IndustryPlannerService) AuthorizeRead(directory string) ([]EnvironmentSaveItem, error) {
	root := s.GetDirectory()
	if !root.Exists || directory == "" || !strings.EqualFold(filepath.Clean(directory), filepath.Clean(root.Path)) {
		return nil, fmt.Errorf("存档目录已变更，请重新选择")
	}
	return s.useRoot(root.Path, true)
}

// ChooseDirectory 允许从原生窗口选择另一份存档目录，取消时保留现有会话。
func (s *IndustryPlannerService) ChooseDirectory() (string, error) {
	if core.GLOBAL.RuntimeContext == nil {
		return "", fmt.Errorf("请在桌面端选择存档目录")
	}
	path, err := runtime.OpenDirectoryDialog(core.GLOBAL.RuntimeContext, runtime.OpenDialogOptions{Title: "选择戴森球计划存档目录"})
	if err != nil || path == "" {
		return "", err
	}
	_, err = s.useRoot(path, false)
	return path, err
}

func (s *IndustryPlannerService) useRoot(path string, configured bool) ([]EnvironmentSaveItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invalidate()
	s.root, s.configured = filepath.Clean(path), configured
	items, err := (&EnvironmentManagerService{}).listSavesAtRoot(s.root, "")
	if err != nil {
		s.root = ""
	}
	return items, err
}

// RevokeRead 清除会话和后台任务，客户端列表变化不会写入游戏目录。
func (s *IndustryPlannerService) RevokeRead() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invalidate()
	s.root = ""
}

func (s *IndustryPlannerService) invalidate() {
	if s.stopRead != nil {
		s.stopRead()
		s.stopRead = nil
		s.readContext = nil
	}
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.epoch++
	s.snapshot, s.bundle = nil, nil
	s.search = nil
	s.world, s.selection, s.task = industryplan.World{}, work.Selection{}, work.Task{}
}

func (s *IndustryPlannerService) checkRoot() error {
	if s.root == "" {
		return fmt.Errorf("请先选择存档目录")
	}
	if s.configured {
		now := s.GetDirectory()
		if !now.Exists || !strings.EqualFold(filepath.Clean(now.Path), s.root) {
			s.invalidate()
			s.root = ""
			return fmt.Errorf("存档目录已变化，请重新选择")
		}
	}
	return nil
}

// ListSaves 刷新当前授权的同一目录，复用资源管理页的存档 ID。
func (s *IndustryPlannerService) ListSaves() ([]EnvironmentSaveItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkRoot(); err != nil {
		return nil, err
	}
	return (&EnvironmentManagerService{}).listSavesAtRoot(s.root, "")
}

// SelectSave 建立固定时刻的只读世界，解析期间切档或撤销时丢弃旧响应。
func (s *IndustryPlannerService) SelectSave(saveID string) (work.Selection, error) {
	s.mu.Lock()
	if err := s.checkRoot(); err != nil {
		s.mu.Unlock()
		return work.Selection{}, err
	}
	s.invalidate()
	readContext, stopRead := context.WithCancel(context.Background())
	s.readContext, s.stopRead = readContext, stopRead
	epoch, root := s.epoch, s.root
	s.mu.Unlock()
	s.loadMu.Lock()
	defer s.loadMu.Unlock()
	s.mu.Lock()
	valid := epoch == s.epoch
	s.mu.Unlock()
	if !valid {
		return work.Selection{}, context.Canceled
	}
	selected, err := (&EnvironmentManagerService{}).findSaveResource(root, saveID)
	if err != nil {
		return work.Selection{}, err
	}
	paths, err := envhelper.SaveBundlePaths(filepath.Join(root, filepath.FromSlash(selected.RelativePath)))
	if err != nil {
		return work.Selection{}, err
	}
	snapshot, err := saveanalysis.Read(paths[0])
	if err != nil {
		return work.Selection{}, err
	}
	if err := readContext.Err(); err != nil {
		return work.Selection{}, err
	}
	selection, world := work.Describe(saveID, snapshot)
	selection.SaveName = filepath.Base(selected.RelativePath)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkRoot(); err != nil {
		return work.Selection{}, err
	}
	if epoch != s.epoch {
		return work.Selection{}, context.Canceled
	}
	s.snapshot, s.selection, s.world = snapshot, selection, world
	work.RecordOperation("select", "已读取产业线规划存档", map[string]interface{}{"stars": selection.Stars, "planets": len(selection.Sites), "tick": selection.Tick})
	return selection, nil
}

// InspectResources 根据玩家选择精算一颗星的资源。查询期间换档时丢弃旧结果。
func (s *IndustryPlannerService) InspectResources(saveID, snapshotID string, planetID int64) (saveanalysis.PlanetResourceSummary, error) {
	s.mu.Lock()
	if err := s.checkRoot(); err != nil {
		s.mu.Unlock()
		return saveanalysis.PlanetResourceSummary{}, err
	}
	snapshot, epoch, ctx := s.snapshot, s.epoch, s.readContext
	valid := snapshot != nil && s.selection.SaveID == saveID && s.selection.SnapshotID == snapshotID
	s.mu.Unlock()
	if !valid {
		return saveanalysis.PlanetResourceSummary{}, fmt.Errorf("存档快照已变更，请重新选择星球")
	}
	summary, err := snapshot.ResolvePlanetResources(ctx, planetID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.epoch != epoch {
		return saveanalysis.PlanetResourceSummary{}, context.Canceled
	}
	return summary, err
}

// Start 在返回任务号前注册取消令牌，等待中的旧请求不能取代新请求。
func (s *IndustryPlannerService) Start(request work.Request) (work.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkRoot(); err != nil {
		return work.Task{}, err
	}
	if s.snapshot == nil || s.selection.SnapshotID != request.SnapshotID || s.selection.SaveID != request.SaveID {
		return work.Task{}, fmt.Errorf("存档快照已变更，请重新选档")
	}
	if err := request.Validate(); err != nil {
		return work.Task{}, err
	}
	s.nextID++
	id := s.nextID
	snapshot, selection, world := s.snapshot, s.selection, s.world
	s.bundle = nil
	s.task = work.Task{ID: id}
	s.task.SetState("running", time.Now())
	var control *search.Control
	control = search.New(3*time.Minute, func() {
		if request.Options.Continuous {
			control.Continue()
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.task.ID == id && s.task.State == "running" {
			s.task.SetState("paused", time.Now())
		}
	})
	s.search = control
	s.cancel = s.tasks.RunAsync(s.readContext, func(ctx context.Context) {
		ctx = search.WithControl(ctx, control)
		bundle, err := work.Generate(ctx, request, selection, world, snapshot, func(p industryplan.Progress) {
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.task.ID == id && s.task.State == "running" {
				s.task.Observe(p, time.Now())
			}
		})
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.task.ID != id {
			return
		}
		s.cancel = nil
		switch {
		case ctx.Err() != nil:
			s.task.SetState("cancelled", time.Now())
		case err != nil:
			issue := work.PlayerIssue(err.Error())
			s.task.SetState("failed", time.Now())
			s.task.Error = strings.TrimSpace(issue.Cause + "。" + issue.Action)
			systemhelper.RecordFailure("industryplan", "workspace", "generate", "跨星球规划失败", err, map[string]interface{}{"task": id})
		default:
			s.task.SetState("ready", time.Now())
			s.bundle = bundle
			work.RecordOperation("generate", "完成跨星球规划计算", map[string]interface{}{"task": id, "status": bundle.Report.Status, "planets": len(bundle.Report.Planets), "exportable": bundle.Report.Exportable})
		}
	})
	return s.task.Snapshot(time.Now()), nil
}

// Continue 职责：继续同一存档与参数下已暂停的搜索；使用：玩家选择继续计算；边界：不新建任务，不重复已验证布局。
func (s *IndustryPlannerService) Continue(id int64) (work.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkRoot(); err != nil {
		return work.Task{}, err
	}
	if s.task.ID != id || s.task.State != "paused" || s.search == nil {
		return work.Task{}, fmt.Errorf("当前没有可继续的规划")
	}
	if s.search.Continue() {
		s.task.SetState("running", time.Now())
	}
	return s.task.Snapshot(time.Now()), nil
}

// Cancel 只取消指定任务；旧页面发来的请求不影响当前工作。
func (s *IndustryPlannerService) Cancel(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.task.ID == id && s.cancel != nil {
		s.cancel()
		s.task.SetState("cancelling", time.Now())
	}
}

// GetTask 按编号读取进度。零号仅供页面恢复最近任务。
func (s *IndustryPlannerService) GetTask(id int64) (work.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkRoot(); err != nil {
		return work.Task{}, err
	}
	if id != 0 && id != s.task.ID {
		return work.Task{}, fmt.Errorf("该规划已被新的任务替代")
	}
	return s.task.Snapshot(time.Now()), nil
}

func (s *IndustryPlannerService) completed(id int64) (*work.Bundle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkRoot(); err != nil {
		return nil, err
	}
	if id == 0 || id != s.task.ID || s.task.State != "ready" || s.bundle == nil {
		return nil, fmt.Errorf("此任务尚无完整结果")
	}
	return s.bundle, nil
}

// GetReport 返回净交付、外供与各星球施工摘要，完整编码仍保留在本机。
func (s *IndustryPlannerService) GetReport(id int64) (work.Report, error) {
	b, err := s.completed(id)
	if err != nil {
		return work.Report{}, err
	}
	return b.Report, nil
}

// ReadBlueprint 仅读取被点选星球的一段编码，大方案不会挤占 Wails 消息队列。
func (s *IndustryPlannerService) ReadBlueprint(id, planetID int64, offset int) (work.Chunk, error) {
	b, err := s.completed(id)
	if err != nil {
		return work.Chunk{}, err
	}
	return b.ReadChunk(planetID, offset)
}

// ExportPack 使用已复核的同一份结果，单图保存原版文本，多图保存分享压缩包。
func (s *IndustryPlannerService) ExportPack(id int64) (string, error) {
	b, err := s.completed(id)
	if err != nil {
		return "", err
	}
	if core.GLOBAL.RuntimeContext == nil {
		return "", fmt.Errorf("请在桌面端导出蓝图包")
	}
	ext := b.ExportExtension()
	title := "保存分享 ZIP"
	if ext == ".txt" {
		title = "保存游戏蓝图 TXT"
	}
	options := runtime.SaveDialogOptions{Title: title, DefaultFilename: b.Report.ExportName + ext, Filters: []runtime.FileFilter{{DisplayName: title, Pattern: "*" + ext}}}
	root := (envhelper.Resolver{}).ResolveBlueprintRoot()
	if ext == ".txt" && root.Exists && !root.NeedConfirm {
		options.DefaultDirectory = root.Path
	}
	target, err := runtime.SaveFileDialog(core.GLOBAL.RuntimeContext, options)
	if err != nil || target == "" {
		return "", err
	}
	if current, err := s.completed(id); err != nil || current != b {
		return "", fmt.Errorf("规划已变更，请导出当前方案")
	}
	if filepath.Ext(target) == "" {
		target += ext
	}
	if err := b.Export(target); err != nil {
		return "", err
	}
	work.RecordOperation("export", "已导出跨星球产业线蓝图包", map[string]interface{}{"task": id, "planets": len(b.Report.Planets)})
	return target, nil
}

// ExportToLibrary 将多张原版施工图写入游戏蓝图库下的同名子目录，未确认的目录由玩家选定。
func (s *IndustryPlannerService) ExportToLibrary(id int64) (string, error) {
	b, err := s.completed(id)
	if err != nil {
		return "", err
	}
	root := (envhelper.Resolver{}).ResolveBlueprintRoot()
	directory := root.Path
	if !root.Exists || root.NeedConfirm {
		if core.GLOBAL.RuntimeContext == nil {
			return "", fmt.Errorf("请在桌面端选择游戏蓝图库目录")
		}
		directory, err = runtime.OpenDirectoryDialog(core.GLOBAL.RuntimeContext, runtime.OpenDialogOptions{Title: "选择游戏蓝图库目录"})
		if err != nil || directory == "" {
			return "", err
		}
	}
	if current, err := s.completed(id); err != nil || current != b {
		return "", fmt.Errorf("规划已变更，请导出当前方案")
	}
	path, err := b.WriteBook(directory)
	if err == nil {
		work.RecordOperation("export", "已写入游戏蓝图库", map[string]interface{}{"task": id, "sheets": b.Report.BlueprintCount})
	}
	return path, err
}
