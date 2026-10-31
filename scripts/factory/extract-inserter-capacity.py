"""提取原版分拣器额定周期事实（仅开发期使用）。

职责：只读 ItemProtoSet、ModelProtoSet 和 InserterDesc，输出零科技容量所需的原始数值与来源指纹。
使用：python scripts/factory/extract-inserter-capacity.py --game-dir <游戏目录> --out <数值目录.json>。
边界：UnityPy 仅为开发依赖；不写游戏目录、不导出模型或反编译源码；周期状态机的解释另由人工核对固定 DLL 方法。
"""

import argparse
import importlib.util
import json
import os


def load_extractor():
    """复用同目录只读资源解析器；不复制或修改游戏资源。"""
    path = os.path.join(os.path.dirname(__file__), "extract-build-colliders.py")
    spec = importlib.util.spec_from_file_location("build_colliders", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def parse_inserter_desc(module, raw):
    """按已核对的 InserterDesc 序列字段读取；布局不匹配时拒绝输出。"""
    reader = module.Reader(raw)
    reader.i32()
    reader.i64()
    reader.u8_aligned()
    reader.i32()
    reader.i64()
    reader.string()
    fact = {"grade": reader.i32(), "sttf": module.f32(reader.f32()),
            "delayf": module.f32(reader.f32()), "canStack": bool(reader.u8_aligned()),
            "stackSize": reader.i32()}
    reader.expect_end("InserterDesc")
    return fact


def main():
    """关联物品、模型与分拣器描述组件，并以无个人路径的元数据写出事实。"""
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
    facts = []
    for item in items:
        if item["ID"] not in (2011, 2012, 2013, 2014):
            continue
        model = models[item["ModelIndex"]]
        prefab = assets.game_object(model["PrefabPath"])
        components = []
        for node in assets.walk(prefab):
            for component_ref in node["go"].m_Component:
                component = (component_ref.component if hasattr(component_ref, "component") else component_ref).deref()
                if component.type != assets.T.MonoBehaviour:
                    continue
                reader = module.Reader(component.get_raw_data())
                reader.p = 16
                script = assets.resolve({"m_FileID": reader.i32(), "m_PathID": reader.i64()})
                if script.read_typetree().get("m_Name") == "InserterDesc":
                    fact = parse_inserter_desc(module, component.get_raw_data())
                    fact["stt"] = round(module.round_f32(module.round_f32(fact["sttf"]) * 600000))
                    fact.update({"itemId": item["ID"], "modelId": model["ID"], "prefabPath": model["PrefabPath"],
                                 "assetPathId": str(component.path_id), "hierarchy": "/".join(node["path"])})
                    components.append(fact)
        if len(components) != 1:
            raise ValueError(f"物品 {item['ID']} 的 InserterDesc 数量不是 1：{len(components)}")
        facts.extend(components)
    if len(facts) != 4:
        raise ValueError("缺少 2011..2014 分拣器事实")
    build_id, _ = module.steam_build_id(args.game_dir, None)
    output = {"schema": "dsp-inserter-capacity/v1", "source": {
        "steamBuildId": build_id, "gameVersion": module.game_version(args.game_dir),
        "unityVersion": assets.gg.unity_version,
        "assemblySha256": module.sha256_of(os.path.join(args.game_dir, "DSPGAME_Data/Managed/Assembly-CSharp.dll")),
        "resourcesSha256": module.sha256_of(os.path.join(args.game_dir, "DSPGAME_Data/resources.assets")),
        "methodsReviewed": ["PrefabDesc.ReadPrefab", "PlanetFactory.CreateEntityLogicComponents", "FactorySystem.NewInserterComponent", "GameHistoryData.SetForNewGame", "InserterComponent.InternalUpdate", "InserterComponent.InternalUpdateNoAnim"]},
        "profile": {"id": "vanilla-no-tech-unstacked-full-power/v1", "ticksPerSecond": 60,
                    "timeUnitsPerTick": 10000, "obsoleteStackInput": 1, "pileStackInput": 2,
                    "stackOutput": 1, "bidirectional": False, "inputCargoStack": 1,
                    "power": 1, "distances": [1, 2, 3],
                    "scope": "连续供料且接收端无阻塞时的零科技额定上限；不承诺带上货物相位、缺电或端口等待后的实际吞吐"},
        "inserters": sorted(facts, key=lambda fact: fact["itemId"])}
    os.makedirs(os.path.dirname(os.path.abspath(args.out)), exist_ok=True)
    with open(args.out, "w", encoding="utf-8", newline="\n") as target:
        json.dump(output, target, ensure_ascii=False, indent=2)
        target.write("\n")


if __name__ == "__main__":
    main()
