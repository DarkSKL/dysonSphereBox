package environment

// 文件职责：提供目录读取授权、选档解析与快照查询的 Wails 服务。
// 使用：后续存档解析页面先探测目录，再明确授权，选定 saveId 才读取主档。
// 边界：复用环境管理目录和列表；不写存档、不启动游戏、不持久化授权。

import (
	environmentHelper "DysonSphereBox/helper/environment"
	"DysonSphereBox/helper/saveanalysis"
	"DysonSphereBox/utils"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

// SaveAnalysisService 职责：编排用户授权与只读解析；使用：Wails 注册后调用；边界：同一时刻只保留一个选档快照，初始化不访问数据库。
type SaveAnalysisService struct {
	// mu 职责：串行化授权、选档和查询；使用：所有状态方法；边界：一次解析完成后释放。
	mu sync.Mutex
	// root 职责：保存本次会话授权的目录；使用：每次读取前复核；边界：配置变化后授权失效。
	root string
	// saveID 职责：标识当前成功解析的存档；使用：拒绝过期页面查询；边界：分析失败时清空。
	saveID string
	// snapshot 职责：保存只读查询索引；使用：重复物品查询；边界：撤销或切档时释放。
	snapshot *saveanalysis.Snapshot
}

// GetDirectory 职责：返回现有存档目录解析结果；使用：页面展示授权对象；边界：不列档、不读取主档内容。
func (service *SaveAnalysisService) GetDirectory() environmentHelper.RootResolution {
	return (environmentHelper.Resolver{}).ResolveSaveRoot()
}

// AuthorizeRead 职责：确认目录读取授权并立即列档；使用：玩家同意后传入刚展示的目录；边界：目录选择和持久化沿用环境管理入口，探测结果变化时拒绝授权。
func (service *SaveAnalysisService) AuthorizeRead(directory string) ([]EnvironmentSaveItem, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.root, service.saveID, service.snapshot = "", "", nil
	root := service.GetDirectory()
	if !root.Exists || root.Path == "" || !strings.EqualFold(filepath.Clean(root.Path), utils.CleanPath(directory)) {
		return nil, fmt.Errorf("存档目录已变化或无效，请重新确认读取目录")
	}
	service.root = root.Path
	items, err := (&EnvironmentManagerService{}).listSavesAtRoot(service.root, "")
	if err != nil {
		service.root = ""
		return nil, err
	}
	if err := service.checkAuthorization(); err != nil {
		return nil, err
	}
	saveanalysis.RecordEvent("authorize", "玩家已授权读取存档目录", nil)
	return items, nil
}

// ListSaves 职责：刷新已授权目录列表；使用：授权后刷新或搜索；边界：不自动解析任何档。
func (service *SaveAnalysisService) ListSaves(query string) ([]EnvironmentSaveItem, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.checkAuthorization(); err != nil {
		return nil, err
	}
	items, err := (&EnvironmentManagerService{}).listSavesAtRoot(service.root, query)
	if authorizationErr := service.checkAuthorization(); authorizationErr != nil {
		return nil, authorizationErr
	}
	return items, err
}

// Analyze 职责：按现有 saveId 读取选中主档并返回汇总；使用：玩家选择具体存档后；边界：失败清除旧快照，附档只提示配对。
func (service *SaveAnalysisService) Analyze(saveID string, seconds int64) (saveanalysis.Analysis, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.snapshot = nil
	service.saveID = ""
	if err := service.checkAuthorization(); err != nil {
		return saveanalysis.Analysis{}, err
	}
	selected, err := (&EnvironmentManagerService{}).findSaveResource(service.root, saveID)
	if err != nil {
		return saveanalysis.Analysis{}, err
	}
	path := filepath.Join(service.root, filepath.FromSlash(selected.RelativePath))
	bundle, err := environmentHelper.SaveBundlePaths(path)
	if err != nil {
		return saveanalysis.Analysis{}, err
	}
	snapshot, err := saveanalysis.Read(path)
	if err != nil {
		saveanalysis.RecordEvent("parse", "读取选中存档失败", err)
		return saveanalysis.Analysis{}, err
	}
	summary, err := snapshot.Summary(seconds)
	if err != nil {
		return saveanalysis.Analysis{}, err
	}
	if err := service.publishSnapshot(saveID, snapshot); err != nil {
		return saveanalysis.Analysis{}, err
	}
	saveanalysis.RecordEvent("parse", "选中存档解析完成", nil)
	return saveanalysis.Analysis{SaveID: saveID, HasServerCompanion: len(bundle) > 1, Summary: summary}, nil
}

// publishSnapshot 职责：在读取及构建完成后发布快照；使用：Analyze 持锁调用；边界：配置可能被其他服务修改，重新复核后才返回分析结果。
func (service *SaveAnalysisService) publishSnapshot(saveID string, snapshot *saveanalysis.Snapshot) error {
	if err := service.checkAuthorization(); err != nil {
		return err
	}
	service.saveID, service.snapshot = saveID, snapshot
	return nil
}

// GetSummary 职责：按窗口返回已解析全档物品；使用：切换历史窗口；边界：不重新读取磁盘，选档身份必须匹配。
func (service *SaveAnalysisService) GetSummary(saveID string, seconds int64) (saveanalysis.Summary, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.checkSnapshot(saveID); err != nil {
		return saveanalysis.Summary{}, err
	}
	return service.snapshot.Summary(seconds)
}

// GetItem 职责：按星球展开物品的产耗与物流；使用：选中物品；边界：只查询当前快照，不接受任意文件路径。
func (service *SaveAnalysisService) GetItem(saveID string, itemID, seconds int64) (saveanalysis.ItemDetail, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.checkSnapshot(saveID); err != nil {
		return saveanalysis.ItemDetail{}, err
	}
	return service.snapshot.Item(itemID, seconds)
}

// GetWorld 职责：返回已确认的星球归属和星系参数；使用：选档后定位星球；边界：仅支持已验证生成算法，坐标单位及名称来源随结果返回。
func (service *SaveAnalysisService) GetWorld(saveID string) (saveanalysis.WorldView, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.checkSnapshot(saveID); err != nil {
		return saveanalysis.WorldView{}, err
	}
	return service.snapshot.World(), nil
}

// GetInventory 职责：按星球和物品返回分类库存；使用：零标识选择全部；边界：复用当前授权快照，不包含未覆盖库存位置。
func (service *SaveAnalysisService) GetInventory(saveID string, planetID, itemID int64) (saveanalysis.InventoryView, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.checkSnapshot(saveID); err != nil {
		return saveanalysis.InventoryView{}, err
	}
	return service.snapshot.Inventory(planetID, itemID)
}

// GetSphere 职责：查询指定恒星的球体和建设量；使用：层为零返回全部层；边界：只读，不修改样式或写回存档。
func (service *SaveAnalysisService) GetSphere(saveID string, starID, layerID int64) (saveanalysis.SphereView, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.checkSnapshot(saveID); err != nil {
		return saveanalysis.SphereView{}, err
	}
	return service.snapshot.Sphere(starID, layerID)
}

// RevokeRead 职责：撤销本次目录授权并释放快照；使用：玩家撤销授权或退出功能；边界：不更改现有目录配置。
func (service *SaveAnalysisService) RevokeRead() {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.root = ""
	service.saveID = ""
	service.snapshot = nil
	saveanalysis.RecordEvent("revoke", "存档读取授权已撤销", nil)
}

// checkAuthorization 职责：复核当前配置与授权目录；使用：任何列表或内容查询前；边界：变化时立即清除授权和缓存。
func (service *SaveAnalysisService) checkAuthorization() error {
	current := service.GetDirectory()
	if service.root == "" || !current.Exists || !strings.EqualFold(filepath.Clean(current.Path), service.root) {
		service.root = ""
		service.saveID = ""
		service.snapshot = nil
		return fmt.Errorf("请先授权读取当前存档目录")
	}
	return nil
}

// checkSnapshot 职责：验证查询身份；使用：汇总和详情调用；边界：未解析、已撤销或旧页面 saveId 均返回错误。
func (service *SaveAnalysisService) checkSnapshot(saveID string) error {
	if err := service.checkAuthorization(); err != nil {
		return err
	}
	if service.snapshot == nil || saveID != service.saveID {
		return fmt.Errorf("请先选中并解析这份存档")
	}
	return nil
}

// GetCircuitLine 职责：查询所选星球的电路板生产状态；使用：须授权并选档；边界：查询沿用固定快照。
func (service *SaveAnalysisService) GetCircuitLine(saveID string, planetID int64) (saveanalysis.CircuitReport, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.checkSnapshot(saveID); err != nil {
		return saveanalysis.CircuitReport{}, err
	}
	return service.snapshot.CircuitLine(planetID)
}

// GetProductionItems 列出授权快照内所选星球实际配置或有生产记录的物品。
func (service *SaveAnalysisService) GetProductionItems(saveID string, planetID int64) ([]saveanalysis.ProductionItem, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.checkSnapshot(saveID); err != nil {
		return nil, err
	}
	return service.snapshot.ProductionItems(planetID)
}

// GetProductionLine 读取同一快照内目标物品的逐机诊断、检查顺序及连接证据。
func (service *SaveAnalysisService) GetProductionLine(saveID string, planetID, itemID int64) (saveanalysis.CircuitReport, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.checkSnapshot(saveID); err != nil {
		return saveanalysis.CircuitReport{}, err
	}
	return service.snapshot.ProductionLine(planetID, itemID)
}

// GetPlanetProductionDevices 查询授权快照中的采集、分馏与矩阵站，包含无法确定产物的设备。
func (service *SaveAnalysisService) GetPlanetProductionDevices(saveID string, planetID int64) (saveanalysis.DeviceReport, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.checkSnapshot(saveID); err != nil {
		return saveanalysis.DeviceReport{}, err
	}
	return service.snapshot.PlanetDevices(planetID)
}

// GetProductionInputSupply 逆查授权快照中已确认缺料的设备，可限定为上一级可达的实体。
func (service *SaveAnalysisService) GetProductionInputSupply(saveID string, planet, product, item int64, scope saveanalysis.SupplyScope) (saveanalysis.SupplyReport, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.checkSnapshot(saveID); err != nil {
		return saveanalysis.SupplyReport{}, err
	}
	return service.snapshot.InputSupply(planet, product, item, scope.Entities)
}

// GetStationSupply 查询指定物流货槽的上游带线、模式候选站和实际活动任务。
func (service *SaveAnalysisService) GetStationSupply(saveID string, planet, station int64, index int) (saveanalysis.SupplyReport, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.checkSnapshot(saveID); err != nil {
		return saveanalysis.SupplyReport{}, err
	}
	return service.snapshot.StationSupply(planet, station, index)
}

// DiagnoseProduction 职责：返回目标产品的检查顺序和供应证据；使用：授权并选档后按星球查询；边界：旧 saveId 拒绝读取。
func (service *SaveAnalysisService) DiagnoseProduction(saveID string, planet, product int64) (saveanalysis.ProductionDiagnosis, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.checkSnapshot(saveID); err != nil {
		return saveanalysis.ProductionDiagnosis{}, err
	}
	return service.snapshot.DiagnoseProduction(planet, product)
}
