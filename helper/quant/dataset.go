package quant

// 文件职责：加载授权内置数据并建立保序名称索引；使用：量化、检查与布局；边界：只读 go:embed，无运行时联网。
// 数据原样来自 dsp-calc（神不在的黄昏提供，MulanPSL-2.0），见 NOTICE.md。

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// dataFiles 职责：携带原版量化数据；使用：离线量化与蓝图生成；边界：不嵌入外部数据集、不联网、不读取用户 Mod 数据。
//
//go:embed data/Vanilla.json
var dataFiles embed.FS

// datasetCache 职责：保存只读缓存；使用：多次求解；边界：不得修改缓存内的切片或 map。
var datasetCache sync.Map

// Dataset 职责：保存归一后的数据与名称键索引；使用：量化及生成；边界：仅加载时初始化。
type Dataset struct {
	// Info 职责：摘要；使用：版本提示；边界：不代表当前安装游戏版本。
	Info DatasetInfo
	// Items 职责：原型序列；使用：选项和检查；边界：保源顺序。
	Items []Item
	// Recipes 职责：原配方序列；使用：方案序号对应；边界：不排序。
	Recipes []Recipe
	// ByID 职责：物品索引；使用：边界转换；边界：不驱动遍历顺序。
	ByID map[int]Item
	// ByName 职责：名称索引；使用：图模型；边界：名称区分 Unicode。
	ByName map[string]Item
	// RecipeByID 职责：配方反查序号；使用：生成与检查；边界：返回序号而非指针。
	RecipeByID map[int]int
	// Choices 职责：每个产物的可选配方序号；使用：一基选项转换；边界：按配方输入顺序。
	Choices map[string][]int
	// Order 职责：产物首次出现顺序；使用：复刻 JS 对象顺序；边界：不使用 map 遍历替代。
	Order []string
}

// IsVanillaDatasetKey 职责：判断数据集键是否属于原版数据；使用：服务层阻断外部 Mod 进入蓝图工作流；边界：只按稳定键判断，不检查文件内容。
func IsVanillaDatasetKey(key string) bool {
	return strings.TrimSpace(key) == "" || strings.EqualFold(strings.TrimSpace(key), "Vanilla")
}

// LoadVanillaDataset 职责：读取唯一允许进入用户量化/生成流程的原版数据；使用：Wails factory 服务边界；边界：外部数据集键明确失败。
func LoadVanillaDataset(key string) (*Dataset, error) {
	if !IsVanillaDatasetKey(key) {
		return nil, qerror("Q_E_DATASET_MOD_UNSUPPORTED", "蓝图生成仅支持原版游戏数据集")
	}
	return LoadDataset("Vanilla")
}

// LoadDataset 职责：读取并缓存唯一内置原版数据；使用：量化与生成内部统一入口；边界：非原版键明确失败，不保留外部数据回放通道。
func LoadDataset(key string) (*Dataset, error) {
	if !IsVanillaDatasetKey(key) {
		return nil, qerror("Q_E_DATASET_MOD_UNSUPPORTED", "蓝图生成仅支持原版游戏数据集")
	}
	key = "Vanilla"
	if v, ok := datasetCache.Load(key); ok {
		return v.(*Dataset), nil
	}
	raw, err := dataFiles.ReadFile("data/" + key + ".json")
	if err != nil {
		return nil, qerror("Q_E_DATASET", "原版内置数据集不存在")
	}
	var input struct {
		Items   []Item
		Recipes []Recipe
	}
	if err := json.Unmarshal(bytes.TrimPrefix(raw, []byte{239, 187, 191}), &input); err != nil {
		return nil, qerror("Q_E_DATASET", "内置数据损坏")
	}
	if len(input.Items) == 0 || len(input.Recipes) == 0 {
		return nil, qerror("Q_E_DATASET", "数据集为空")
	}
	sum := sha256.Sum256(raw)
	d := &Dataset{Info: DatasetInfo{Key: key, Name: key, GameVersion: "0.10.34.28524", DataVersion: hex.EncodeToString(sum[:8]), ItemCount: len(input.Items), RecipeCount: len(input.Recipes)}, Items: input.Items, Recipes: input.Recipes, ByID: map[int]Item{}, ByName: map[string]Item{}, RecipeByID: map[int]int{}, Choices: map[string][]int{}}
	d.Info.Name = "原版游戏"
	for _, item := range d.Items {
		d.ByID[item.ID] = item
		d.ByName[item.Name] = item
	}
	for i, r := range d.Recipes {
		if len(r.Items) != len(r.ItemCounts) || len(r.Results) != len(r.ResultCounts) || r.TimeSpend <= 0 {
			return nil, qerror("Q_E_DATASET", fmt.Sprintf("配方 %d 结构无效", r.ID))
		}
		d.RecipeByID[r.ID] = i
		for _, id := range append(append([]int{}, r.Items...), r.Results...) {
			if _, ok := d.ByID[id]; !ok {
				return nil, qerror("Q_E_DATASET", fmt.Sprintf("配方 %d 引用未知物品 %d", r.ID, id))
			}
		}
		for _, id := range r.Results {
			name := d.ByID[id].Name
			if _, ok := d.Choices[name]; !ok {
				d.Order = append(d.Order, name)
			}
			d.Choices[name] = append(d.Choices[name], i)
		}
	}
	actual, _ := datasetCache.LoadOrStore(key, d)
	return actual.(*Dataset), nil
}

// ListDatasets 职责：返回唯一可供用户选择的原版数据集；使用：工作区初始化；边界：不扫描目录或暴露外部数据组合。
func ListDatasets() ([]DatasetInfo, error) {
	d, err := LoadVanillaDataset("Vanilla")
	if err != nil {
		return nil, err
	}
	return []DatasetInfo{d.Info}, nil
}
