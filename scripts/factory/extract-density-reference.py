"""从用户本地蓝图提取密铺研究单元，保留原始端点、偏移、槽位和显式边界。

使用：在应用根目录运行 python scripts/factory/extract-density-reference.py。
边界：只读原图，输出研究 JSON；不把裁出的片段宣称为可直接粘贴的完整蓝图。
"""
from pathlib import Path
import hashlib
import importlib.util
import json
import sys

# 导入研究脚本仅为复用严格解码，避免向源目录写入字节码文件。
sys.dont_write_bytecode = True


def extract_unit(document: dict, name: str, machine_ids: tuple[int, int]) -> dict:
    """追踪机器、全部分拣器与邻近真实带链，所有跨界引用单独列出。"""
    by_index = {b["index"]: b for b in document["buildings"]}
    selected = set(machine_ids)
    arms = [b for b in document["buildings"] if 2011 <= b["itemId"] <= 2014
            and (b["inputObjIdx"] in selected or b["outputObjIdx"] in selected)]
    selected.update(b["index"] for b in arms)
    neighbors: dict[int, set[int]] = {}
    for b in document["buildings"]:
        if not 2001 <= b["itemId"] <= 2003:
            continue
        for field in ("inputObjIdx", "outputObjIdx"):
            other = by_index.get(b[field])
            if other and 2001 <= other["itemId"] <= 2003:
                neighbors.setdefault(b["index"], set()).add(other["index"])
                neighbors.setdefault(other["index"], set()).add(b["index"])
    belts = {b[field] for b in arms for field in ("inputObjIdx", "outputObjIdx")
             if b[field] in by_index and 2001 <= by_index[b[field]]["itemId"] <= 2003}
    frontier = set(belts)
    for _ in range(5):
        frontier = {other for index in frontier for other in neighbors.get(index, ())} - belts
        belts.update(frontier)
    selected.update(belts)
    records = [by_index[index] for index in sorted(selected)]
    boundary = []
    for b in document["buildings"]:
        for field in ("inputObjIdx", "outputObjIdx"):
            target = b[field]
            if target < 0 or target not in by_index or (b["index"] in selected) == (target in selected):
                continue
            outgoing = b["index"] in selected
            external = target if outgoing else b["index"]
            boundary.append({"recordIndex": b["index"], "field": field, "targetIndex": target,
                             "insideIndex": b["index"] if outgoing else target,
                             "externalIndex": external, "externalItemId": by_index[external]["itemId"],
                             "direction": "outgoing" if outgoing else "incoming"})
    first, second = (by_index[index] for index in machine_ids)
    origin = first["localOffset"]
    return {"name": name, "machineIndices": machine_ids,
            "machineItemId": first["itemId"], "modelIndex": first["modelIndex"],
            "sourceOrigin": origin,
            "secondMachineDelta": {axis: second["localOffset"][axis] - origin[axis]
                                   for axis in ("x", "y", "z")},
            "records": records, "boundaryReferences": boundary}


def main() -> None:
    """经项目严格编解码器读取原图后生成可复核研究单元；不改动源文件。"""
    root = Path(__file__).resolve().parents[2]
    research = root / "docs/超密度蓝图生成适配器方案"
    source = research / "73125增产剂v1.3（本地版）.txt"
    spec = importlib.util.spec_from_file_location("density_reader", research / "参考数据复核.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    payload = module.read_document(root, source)
    document = payload["document"]
    if not all(payload["verification"].values()):
        raise ValueError("参考图严格编解码复核未通过")
    units = [extract_unit(document, name, ids) for name, ids in (
        ("quantum-chemical-pair", (548, 518)),
        ("recomposing-assembler-pair", (1198, 1138)),
        ("negentropy-smelter-pair", (2173, 2122)),
    )]
    result = {"schema": "dsp-density-reference/v1", "sourceFile": source.name,
              "sourceSha256": hashlib.sha256(source.read_bytes()).hexdigest(),
              "gameVersion": document["gameVersion"], "areas": document["areas"],
              "primaryAreaIndex": document["primaryAreaIndex"],
              "coordinateSpace": "original-area-local; no translation, scaling or snapping",
              "verification": payload["verification"], "units": units,
              "boundary": "裁切边界尚未接回外围物流；这些是研究单元，不是完整可粘贴蓝图。"}
    target = research / "密铺参考单元.json"
    target.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    embedded = root / "helper/blueprint/density/data/reference-units.json"
    embedded.parent.mkdir(parents=True, exist_ok=True)
    embedded.write_text(json.dumps(result, ensure_ascii=False, separators=(",", ":")) + "\n", encoding="utf-8")
    print(json.dumps({"output": str(target.relative_to(root)), "units": [
        {"name": u["name"], "records": len(u["records"]), "boundaries": len(u["boundaryReferences"])}
        for u in units]}, ensure_ascii=False))


if __name__ == "__main__":
    main()
