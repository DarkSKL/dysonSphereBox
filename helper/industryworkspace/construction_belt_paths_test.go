package industryworkspace

// 文件职责：从最终原版蓝图独立追踪配方之间的有向带路；使用：施工回归与验收证据；边界：不读取规划流图、生成器回执或物流塔库存。

import (
	"cmp"
	"slices"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// recipeBeltPath 为一种中间品记录可达的上下游设备对，以及可在原图逐栋复核的一条完整路径。
type recipeBeltPath struct {
	FromRecipeID    int16   `json:"fromRecipeId"`
	ToRecipeID      int16   `json:"toRecipeId"`
	ItemID          int     `json:"itemId"`
	MachinePairs    int     `json:"reachableMachinePairs"`
	BuildingIndices []int32 `json:"exampleBuildingIndices"`
}

// exportedRecipeBeltPaths 按原版输入/输出引用走带，分拣器必须同时具有两个端点且过滤器允许此物品。
func exportedRecipeBeltPaths(t *testing.T, doc blueprint.Document) []recipeBeltPath {
	t.Helper()
	dataset, err := quant.LoadVanillaDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	buildings := make(map[int32]blueprint.Building, len(doc.Buildings))
	for _, b := range doc.Buildings {
		buildings[b.Index] = b
	}
	belt := func(b blueprint.Building) bool { return b.ItemID >= 2001 && b.ItemID <= 2003 }
	sorter := func(b blueprint.Building) bool { return b.ItemID >= 2011 && b.ItemID <= 2014 }
	next := make(map[int32][]int32)
	for _, b := range doc.Buildings {
		from, hasInput := buildings[b.InputObjIdx]
		to, hasOutput := buildings[b.OutputObjIdx]
		if sorter(b) && hasInput && hasOutput && (belt(from) || belt(to)) {
			next[from.Index] = append(next[from.Index], b.Index)
			next[b.Index] = append(next[b.Index], to.Index)
		}
		if belt(b) {
			if hasInput && belt(from) {
				next[from.Index] = append(next[from.Index], b.Index)
			}
			if hasOutput && belt(to) {
				next[b.Index] = append(next[b.Index], to.Index)
			}
		}
	}
	paths := map[[3]int]*recipeBeltPath{}
	for _, source := range doc.Buildings {
		recipe, ok := dataset.RecipeByID[int(source.RecipeID)]
		if source.RecipeID <= 0 || !ok {
			continue
		}
		for _, item := range dataset.Recipes[recipe].Results {
			previous := map[int32]int32{source.Index: -1}
			queue := []int32{source.Index}
			for head := 0; head < len(queue); head++ {
				for _, index := range next[queue[head]] {
					if _, seen := previous[index]; seen {
						continue
					}
					b := buildings[index]
					if sorter(b) && b.FilterID != 0 && int(b.FilterID) != item {
						continue
					}
					previous[index] = queue[head]
					if belt(b) || sorter(b) {
						queue = append(queue, index)
						continue
					}
					consumer, ok := dataset.RecipeByID[int(b.RecipeID)]
					if !ok || b.RecipeID <= 0 || b.RecipeID == source.RecipeID || !slices.Contains(dataset.Recipes[consumer].Items, item) {
						continue
					}
					key := [3]int{int(source.RecipeID), int(b.RecipeID), item}
					path := paths[key]
					if path == nil {
						path = &recipeBeltPath{FromRecipeID: source.RecipeID, ToRecipeID: b.RecipeID, ItemID: item}
						for node := index; node >= 0; node = previous[node] {
							path.BuildingIndices = append(path.BuildingIndices, node)
						}
						slices.Reverse(path.BuildingIndices)
						paths[key] = path
					}
					path.MachinePairs++
				}
			}
		}
	}
	result := make([]recipeBeltPath, 0, len(paths))
	for _, path := range paths {
		result = append(result, *path)
	}
	slices.SortFunc(result, func(a, b recipeBeltPath) int {
		return cmp.Or(cmp.Compare(a.FromRecipeID, b.FromRecipeID), cmp.Compare(a.ToRecipeID, b.ToRecipeID), cmp.Compare(a.ItemID, b.ItemID))
	})
	return result
}

// TestExportedRecipeBeltPathsRejectsDisconnectedRoutes 用电路板输送线区分实际带路、塔库存、反向接口及错料过滤。
func TestExportedRecipeBeltPathsRejectsDisconnectedRoutes(t *testing.T) {
	base := []blueprint.Building{
		{Index: 10, ItemID: 2303, RecipeID: 50, InputObjIdx: -1, OutputObjIdx: -1},
		{Index: 20, ItemID: 2011, FilterID: 1301, InputObjIdx: 10, OutputObjIdx: 30},
		{Index: 30, ItemID: 2001, InputObjIdx: -1, OutputObjIdx: 40},
		{Index: 40, ItemID: 2001, InputObjIdx: -1, OutputObjIdx: -1},
		{Index: 50, ItemID: 2011, InputObjIdx: 40, OutputObjIdx: 60},
		{Index: 60, ItemID: 2901, RecipeID: 9, InputObjIdx: -1, OutputObjIdx: -1},
	}
	paths := exportedRecipeBeltPaths(t, blueprint.Document{Buildings: base})
	if len(paths) != 1 || paths[0].ItemID != 1301 || paths[0].MachinePairs != 1 || !slices.Equal(paths[0].BuildingIndices, []int32{10, 20, 30, 40, 50, 60}) {
		t.Fatalf("完整带路未按建筑引用准确重建：%v", paths)
	}
	for _, tc := range []struct {
		name   string
		change func([]blueprint.Building)
	}{
		{"断带", func(b []blueprint.Building) { b[2].OutputObjIdx = -1 }},
		{"隔塔", func(b []blueprint.Building) { b[3].ItemID = 2104 }},
		{"反向", func(b []blueprint.Building) { b[4].InputObjIdx, b[4].OutputObjIdx = 60, 40 }},
		{"错料", func(b []blueprint.Building) { b[1].FilterID = 1202 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			buildings := slices.Clone(base)
			tc.change(buildings)
			if paths := exportedRecipeBeltPaths(t, blueprint.Document{Buildings: buildings}); len(paths) != 0 {
				t.Fatalf("不可供料的线路出现实体直连回执：%v", paths)
			}
		})
	}
}
