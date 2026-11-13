package blueprint

// 文件职责：为蓝图信封设置游戏内封面物品图标与布局。
// 使用：生成器在编码前按目标产物调用；边界：不生成图片、不改建筑负载，导入蓝图只有显式调用时才替换封面。

import "strconv"

// itemIconSlots 职责：限定原生蓝图封面的五个图标槽；使用：去重及截断；边界：不是目标或产线数量上限。
const itemIconSlots = 5

// itemIconLayouts 职责：按图标数量选择原生布局编号；使用：单图大图标、双图并排、三图三角、四图方阵、五图环列。
// 边界：编号与内置 dsp-blueprint-editor 的 BlueprintIcon 布局表一致，0 表示无图标，不可直接用数量替代布局编号。
var itemIconLayouts = [...]int{0, 10, 20, 30, 40, 51}

// WithItemIcons 职责：返回已设置物品封面的文档；使用：调用方按展示优先级传入游戏物品 ID，再交给 Encode 重算校验。
// 边界：忽略非正值、保序去重、最多五项，空列表清空封面；物品原型有效性由调用方保证，不修改原信封切片或建筑数据。
// 逻辑流：1. 构建兼容当前版本的信封 => 2. 清空旧槽位并写入有效物品 => 3. 按数量选择布局，保留其余元数据。
func WithItemIcons(d Document, itemIDs []int) (Document, error) {
	fields, err := envelopeFields(d, d.EnvelopeVersion)
	if err != nil {
		return Document{}, err
	}
	for index := 0; index < itemIconSlots; index++ {
		fields[2+index] = "0"
	}
	seen := make(map[int]bool, itemIconSlots)
	count := 0
	for _, id := range itemIDs {
		if id <= 0 || seen[id] {
			continue
		}
		fields[2+count] = strconv.Itoa(id)
		seen[id] = true
		count++
		if count == itemIconSlots {
			break
		}
	}
	fields[1] = strconv.Itoa(itemIconLayouts[count])
	d.EnvelopeFields = fields
	return d, nil
}
