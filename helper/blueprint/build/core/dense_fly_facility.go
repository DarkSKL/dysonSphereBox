package build

import "DysonSphereBox/helper/blueprint"

// 文件职责：创建可独立选址的配送箱、配送器、并联臂和最短实体尾带。
// 使用：超密度配送装配；边界：所需臂数由真实计划流量和所选等级决定，机器侧尚未接线时不能单独发布。

// makeDenseFlyFacility 职责：创建真实独立设施集合；使用：全场占位前；边界：不附加固定长度机器接驳长尾。
func (b *builder) makeDenseFlyFacility(port flyIndependentPort, rate float64) (*flyTransportEndpoint, []*placed, []*placed, error) {
	begin := len(b.all)
	planningRate := rate
	if b.nativeCapacityReserve {
		planningRate *= inserterCapacityPerMinute(b.setting.InserterLv, 1) / inserterCapacityPerMinute(b.setting.InserterLv, 3)
	}
	storage, err := planFlyStorageArms(planningRate, b.setting.InserterLv)
	if err != nil {
		return nil, nil, nil, err
	}
	if rate > float64(len(storage.slots))*inserterCapacityPerMinute(b.setting.InserterLv, storage.reach)+1e-6 {
		return nil, nil, nil, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "密铺配送箱的真实并联臂能力不足，应继续拆分机器行"}
	}
	box := b.make(2101, 2, 0, 0)
	box.record.Yaw, box.record.Yaw2 = 0, 0
	box.record.OutputToSlot, box.record.InputFromSlot, box.record.OutputFromSlot, box.record.InputToSlot = 14, 15, 15, 14
	box.record.Parameters = make([]int32, 110)
	box.record.Parameters[0] = 29
	for _, fluid := range []int{1000, 1007, 1114, 1116, 1120, 1121} {
		if fluid == port.item {
			box.record.Parameters[0] = 0
		}
	}
	delivery := b.make(2107, 2, 0, 1.875)
	delivery.record.Yaw, delivery.record.Yaw2 = 0, 0
	delivery.input = box
	delivery.record.InputFromSlot, delivery.record.InputToSlot = 13, 0
	delivery.record.FilterID = int16(port.item)
	delivery.record.Parameters = make([]int32, 128)
	delivery.record.Parameters[0], delivery.record.Parameters[1], delivery.record.Parameters[2], delivery.record.Parameters[3] = 2, 1, 30000, 1
	if port.input {
		delivery.record.Parameters[1] = 2
	}
	tail := b.byPath(storage.beltPoints(box.a))
	if port.input {
		reverse(tail)
	}
	endpoint := &flyTransportEndpoint{owner: b.owner, item: port.item, input: port.input, rate: rate, link: port.link, box: box, delivery: delivery, belts: tail}
	for i, slot := range storage.slots {
		arm := b.make(2010+b.setting.InserterLv, 0, 0, 0)
		if err := flyArmGeometry(arm, box, tail[i], slot, port.input); err != nil {
			return nil, nil, nil, err
		}
		if !port.input {
			arm.record.FilterID = int16(port.item)
		}
		endpoint.arms = append(endpoint.arms, arm)
		if endpoint.arm == nil {
			endpoint.arm = arm
		}
	}
	tail[len(tail)-1].record.Parameters = []int32{int32(port.item), 0}
	return endpoint, tail, b.all[begin:], nil
}

// makeDenseFlyReturnArm 职责：从出货箱未占用的真实西侧槽取回自产剂；使用：保留毛流回路；边界：槽位不得与入箱臂重复，分拣能力不足直接失败。
func (b *builder) makeDenseFlyReturnArm(endpoint *flyTransportEndpoint, rate float64) (*placed, *placed, error) {
	used := map[int8]bool{}
	for _, arm := range endpoint.arms {
		used[arm.record.OutputToSlot] = true
	}
	slot := int8(-1)
	for _, candidate := range []int8{9, 10, 11} {
		if !used[candidate] {
			slot = candidate
			break
		}
	}
	if slot < 0 || rate > inserterCapacityPerMinute(b.setting.InserterLv, 1)+1e-6 {
		return nil, nil, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "自产剂回路缺少独立且足够能力的真实箱体槽位"}
	}
	point, _ := storageSlotPoint(endpoint.box.a, endpoint.box.record.Yaw, int(slot))
	head := b.make(2000+b.setting.BeltLv, point.x-1, point.y, 0)
	arm := b.make(2010+b.setting.InserterLv, 0, 0, 0)
	if err := flyArmGeometry(arm, endpoint.box, head, int(slot), true); err != nil {
		return nil, nil, err
	}
	arm.record.FilterID = int16(endpoint.item)
	head.record.Parameters = []int32{int32(endpoint.item), 0}
	return head, arm, nil
}
