"""提取电力节点和物流塔端口/槽位事实（仅开发期使用）。

职责：只读原版资源中的 PowerDesc、StationDesc 和 SlotConfig，输出数字及来源指纹。
使用：python scripts/factory/extract-infrastructure-facts.py --game-dir <游戏目录> --out <数值目录.json>。
边界：不写游戏目录、不导出模型/贴图/反编译源码；数值目录不覆盖未知构建、科技或 Mod。
"""

import argparse
import importlib.util
import json
import os


def load_extractor():
    """复用只读资源和坐标解析，不依赖游戏运行时。"""
    spec = importlib.util.spec_from_file_location("build_colliders", os.path.join(os.path.dirname(__file__), "extract-build-colliders.py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def component_reader(module, raw):
    """跳过 MonoBehaviour 固定头；调用方负责完整字段布局检查。"""
    reader = module.Reader(raw)
    reader.i32()
    reader.i64()
    reader.u8_aligned()
    reader.i32()
    reader.i64()
    reader.string()
    return reader


def power_fact(module, raw):
    """读取 PowerDesc 节点半径，并完整消费未输出字段以校验布局。"""
    reader = component_reader(module, raw)
    fact = {"node": bool(reader.u8_aligned()), "connectDistance": module.f32(reader.f32()), "coverRadius": module.f32(reader.f32())}
    for _ in range(3):
        reader.f32()
    for _ in range(5):
        reader.u8_aligned()
    reader.i64()
    reader.i64()
    for _ in range(4):
        reader.i32()
    reader.i64()
    reader.u8_aligned()
    for _ in range(3):
        reader.i64()
    reader.u8_aligned()
    reader.i64()
    reader.i32()
    reader.i32()
    reader.u8_aligned()
    reader.u8_aligned()
    reader.i64()
    reader.i64()
    reader.expect_end("PowerDesc")
    return fact


def station_fact(module, raw):
    """完整读取 StationDesc 数值；未知布局直接失败。"""
    reader = component_reader(module, raw)
    fact = {"isStellar": bool(reader.u8_aligned()), "maxItemCount": reader.i32(), "maxItemKinds": reader.i32(),
            "maxDroneCount": reader.i32(), "maxShipCount": reader.i32(), "maxEnergyAcc": reader.i64(),
            "dronePoint": [module.f32(reader.f32()) for _ in range(3)],
            "shipPoint": [module.f32(reader.f32()) for _ in range(3)],
            "isCollector": bool(reader.u8_aligned()), "collectSpeed": reader.i32(), "isVeinCollector": bool(reader.u8_aligned())}
    reader.expect_end("StationDesc")
    return fact


def main():
    """关联物品和描述组件，保存没有个人路径的来源证据。"""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--game-dir", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()
    import UnityPy
    module = load_extractor()
    assets = module.Assets(UnityPy, os.path.join(args.game_dir, "DSPGAME_Data"))
    _, _, items = module.parse_item_proto_set(assets.raw_of("prototypes/itemprotoset"))
    _, _, models = module.parse_model_proto_set(assets.raw_of("prototypes/modelprotoset"))
    models = {model["ID"]: model for model in models}
    power, stations = [], []
    for item in items:
        if item["ID"] not in (2201, 2103, 2104):
            continue
        model = models[item["ModelIndex"]]
        prefab = assets.game_object(model["PrefabPath"])
        for node in assets.walk(prefab):
            for component_ref in node["go"].m_Component:
                component = (component_ref.component if hasattr(component_ref, "component") else component_ref).deref()
                if component.type != assets.T.MonoBehaviour:
                    continue
                raw = component.get_raw_data()
                reader = module.Reader(raw)
                reader.p = 16
                script = assets.resolve({"m_FileID": reader.i32(), "m_PathID": reader.i64()})
                script_name = script.read_typetree().get("m_Name")
                fact = None
                if item["ID"] == 2201 and script_name == "PowerDesc":
                    fact = power_fact(module, raw)
                    power.append(fact)
                elif script_name == "StationDesc":
                    fact = station_fact(module, raw)
                    geometry = assets.slot_config(prefab)
                    fact["portCount"] = len(geometry["portPoses"])
                    stations.append(fact)
                if fact is not None:
                    fact.update({"itemId": item["ID"], "modelId": model["ID"], "prefabPath": model["PrefabPath"],
                                 "component": script_name, "assetPathId": str(component.path_id), "hierarchy": "/".join(node["path"])})
    if len(power) != 1 or len(stations) != 2:
        raise ValueError("电力节点或物流塔事实缺失/重复")
    build_id, _ = module.steam_build_id(args.game_dir, None)
    output = {"schema": "dsp-infrastructure-facts/v1", "source": {
        "steamBuildId": build_id, "gameVersion": module.game_version(args.game_dir), "unityVersion": assets.gg.unity_version,
        "assemblySha256": module.sha256_of(os.path.join(args.game_dir, "DSPGAME_Data/Managed/Assembly-CSharp.dll")),
        "resourcesSha256": module.sha256_of(os.path.join(args.game_dir, "DSPGAME_Data/resources.assets")),
        "methodsReviewed": ["PrefabDesc.ReadPrefab"], "stationPortSource": "SlotConfig.slotPoses 实际数组长度，由 PrefabDesc 转为物流塔端口"},
        "powerNodes": power, "stations": sorted(stations, key=lambda fact: fact["itemId"])}
    os.makedirs(os.path.dirname(os.path.abspath(args.out)), exist_ok=True)
    with open(args.out, "w", encoding="utf-8", newline="\n") as target:
        json.dump(output, target, ensure_ascii=False, indent=2)
        target.write("\n")


if __name__ == "__main__":
    main()
