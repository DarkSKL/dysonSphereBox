"""复核本目录参考蓝图的结构、机器间距和直接连接，不改动原图或应用源码。

使用：在应用根目录运行 python docs/超密度蓝图生成适配器方案/参考数据复核.py。
依赖：Python 标准库、当前项目的 Go 工具链及 blueprint 编解码包。
边界：坐标统计不等于球面距离、游戏放置或运行产能验证。
"""

from __future__ import annotations

import argparse
from collections import Counter, defaultdict
from datetime import date
import hashlib
import json
from pathlib import Path
import statistics
import subprocess
import tempfile


# 由项目自身严格解析，再强制重新压缩，避免仅复用原始压缩块完成往返。
GO_READER = r'''
package main

import (
    "encoding/json"
    "os"
    "reflect"

    bp "DysonSphereBox/helper/blueprint"
)

// main 只读解析指定参考文件并输出复核数据，不连接数据库或 Wails。
func main() {
    raw, err := os.ReadFile(os.Args[1])
    if err != nil { panic(err) }
    doc, err := bp.Decode(string(raw), true)
    if err != nil { panic(err) }
    doc.OriginalGzip = nil
    encoded, err := bp.Encode(doc, doc.EnvelopeVersion)
    if err != nil { panic(err) }
    back, err := bp.Decode(encoded.Blueprint, true)
    if err != nil { panic(err) }
    payload := map[string]interface{}{
        "document": doc,
        "verification": map[string]bool{
            "strictDecode": true,
            "checksumValid": doc.ChecksumValid,
            "reencodeWithoutOriginalGzipVerified": encoded.Verified,
            "redecodedBuildingRecordsEqual": reflect.DeepEqual(doc.Buildings, back.Buildings),
            "redecodedAreaRecordsEqual": reflect.DeepEqual(doc.Areas, back.Areas),
        },
    }
    data, err := json.Marshal(payload)
    if err != nil { panic(err) }
    if err = os.WriteFile(os.Args[2], data, 0600); err != nil { panic(err) }
}
'''

MACHINE_IDS = (2317, 2318, 2319)


def write_json(path: Path, value: object) -> None:
    """写入 UTF-8 研究数据，保证中文与原始数值可读。"""
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def read_document(root: Path, source: Path) -> dict:
    """使用受控临时目录调用 Go；临时目录退出时由标准库清理。"""
    temporary_root = Path(tempfile.gettempdir()).resolve()
    with tempfile.TemporaryDirectory(prefix="dsbox-density-") as directory:
        temporary = Path(directory).resolve()
        # 清理范围必须是系统临时目录下本次创建的独立子目录。
        if temporary.parent != temporary_root or not temporary.name.startswith("dsbox-density-"):
            raise RuntimeError("复核临时目录不在预期范围")
        reader = temporary / "main.go"
        result = temporary / "decoded.json"
        reader.write_text(GO_READER, encoding="utf-8")
        subprocess.run(["go", "run", str(reader), str(source), str(result)], cwd=root, check=True)
        return json.loads(result.read_text(encoding="utf-8"))


def analyze_structure(document: dict, verification: dict, source: Path, revision: str, analysis_date: str) -> dict:
    """汇总全部建筑；最近距离仅扫描原始局部 XY，不解释球面物理距离。"""
    buildings = document["buildings"]
    by_id = {building["index"]: building for building in buildings}
    result = {
        "sourceFile": source.name,
        "sourceBytes": source.stat().st_size,
        "sha256": hashlib.sha256(source.read_bytes()).hexdigest(),
        "buildingCount": len(buildings),
        "areaCount": len(document["areas"]),
        "gameVersion": document["gameVersion"],
        "binaryVersion": document["binaryVersion"],
        "areas": document["areas"],
        "items": dict(sorted(Counter(b["itemId"] for b in buildings).items())),
        "recipes": dict(sorted(Counter(b["recipeId"] for b in buildings if b["recipeId"]).items())),
        "recordLayouts": dict(Counter(b["recordLayout"] for b in buildings)),
        "duplicateIndices": len(buildings) - len(by_id),
        "danglingReferences": sum(
            b[key] >= 0 and b[key] not in by_id
            for b in buildings for key in ("inputObjIdx", "outputObjIdx")
        ),
        "crossAreaReferences": sum(
            b[key] in by_id and by_id[b[key]]["areaIndex"] != b["areaIndex"]
            for b in buildings for key in ("inputObjIdx", "outputObjIdx")
        ),
        "perArea": [],
        "itemSpatial": [],
        "nearestSameTypeLocalXYExamples": [],
        "analysis": {
            "date": analysis_date,
            "sourceRevision": revision,
            **verification,
            "gamePlacementTestedByThisAnalysis": False,
            "fullGeometryRulesExecuted": False,
            "coordinateNote": "原始 Area 局部坐标；非整数容差 1e-4；范围仅统计首端；引用检查仅覆盖显式输入输出。",
        },
    }
    for area in document["areas"]:
        points = [b["localOffset"] for b in buildings if b["areaIndex"] == area["index"]]
        result["perArea"].append({
            "area": area["index"], "count": len(points),
            "firstEndpointBounds": {
                key: [min(p[key] for p in points), max(p[key] for p in points)]
                for key in ("x", "y", "z")
            } if points else None,
        })
    for item in sorted(result["items"]):
        group = [b for b in buildings if b["itemId"] == item]
        result["itemSpatial"].append({
            "item": item, "count": len(group),
            "modelCounts": dict(Counter(b["modelIndex"] for b in group)),
            "zRange": [min(b["localOffset"]["z"] for b in group), max(b["localOffset"]["z"] for b in group)],
            "nonIntegerXY": sum(
                any(abs(b["localOffset"][key] - round(b["localOffset"][key])) > 1e-4 for key in ("x", "y"))
                for b in group
            ),
            "yawDistinctRoundedTo3Decimals": len({round(b["yaw"], 3) for b in group}),
        })
    for item in MACHINE_IDS:
        group = [b for b in buildings if b["itemId"] == item]
        if len(group) < 2:
            continue
        best = (float("inf"), None, None)
        for index, left in enumerate(group):
            for right in group[index + 1:]:
                if left["areaIndex"] != right["areaIndex"]:
                    continue
                distance = sum((left["localOffset"][key] - right["localOffset"][key]) ** 2 for key in ("x", "y"))
                if distance < best[0]:
                    best = (distance, left, right)
        if best[1] is not None:
            result["nearestSameTypeLocalXYExamples"].append({
                "itemId": item, "localXYDistance": best[0] ** 0.5,
                "buildings": [
                    {key: b[key] for key in ("index", "itemId", "modelIndex", "areaIndex", "localOffset", "yaw", "recipeId")}
                    for b in best[1:]
                ],
            })
    return result


def analyze_spacing(document: dict, source_sha: str) -> dict:
    """按固定容差分行，并保留代表机器的完整记录及相连分拣器和带。"""
    buildings = document["buildings"]
    by_id = {b["index"]: b for b in buildings}
    machines = {b["index"]: b for b in buildings if b["itemId"] in MACHINE_IDS}
    arms = [b for b in buildings if 2011 <= b["itemId"] <= 2014]
    # 当前研究的全部机器都在 Area 0，其他输入不能隐式混用不同局部坐标。
    if {b["areaIndex"] for b in machines.values()} != {0}:
        raise ValueError("该复核脚本的行距分析只适用于全部生产机器引用 Area 0 的样例")
    result = {
        "rowTolerance": 0.002, "rows": [], "selectedMachineConnections": [],
        "analysis": {
            "sourceSha256": source_sha,
            "coordinateSpace": "raw Area 0 local coordinates; not game world distance",
            "rowClustering": "同原型按 Y 升序；分组最大 Y 与起点 Y 差不超过 0.002；组内按 X 排序；ΔX 为相邻差中位数。",
            "orientationGrouping": "仅统计朝向时四舍五入到整数度并模 360；原始样本保持原值。",
            "selectedConnections": "每类原型前两条 Y 行内按 X 排序的第三台机器，以及引用它的分拣器和对应带。",
        },
    }
    for item in MACHINE_IDS:
        groups = []
        for building in sorted((b for b in machines.values() if b["itemId"] == item), key=lambda b: b["localOffset"]["y"]):
            if not groups or building["localOffset"]["y"] - groups[-1][0]["localOffset"]["y"] > 0.002:
                groups.append([])
            groups[-1].append(building)
        previous = None
        for index, group in enumerate(groups):
            group.sort(key=lambda b: b["localOffset"]["x"])
            center_y = statistics.mean(b["localOffset"]["y"] for b in group)
            deltas = [right["localOffset"]["x"] - left["localOffset"]["x"] for left, right in zip(group, group[1:])]
            result["rows"].append({
                "itemId": item, "row": index + 1, "y": center_y, "count": len(group),
                "previousRowDeltaY": None if previous is None else center_y - previous,
                "xMin": group[0]["localOffset"]["x"], "xMax": group[-1]["localOffset"]["x"],
                "deltaXMedian": statistics.median(deltas) if deltas else None,
                "deltaXRange": [min(deltas), max(deltas)] if deltas else None,
                "yawCountsRoundedDegrees": dict(Counter(round(b["yaw"]) % 360 for b in group)),
                "recipeCounts": dict(Counter(b["recipeId"] for b in group)),
                "firstThreeIndices": [b["index"] for b in group[:3]],
                "lastThreeIndices": [b["index"] for b in group[-3:]],
            })
            previous = center_y
            if index < 2 and len(group) >= 3:
                machine = group[2]
                linked_arms = [a for a in arms if machine["index"] in (a["inputObjIdx"], a["outputObjIdx"])]
                belt_ids = {
                    target for arm in linked_arms for target in (arm["inputObjIdx"], arm["outputObjIdx"])
                    if target in by_id and 2001 <= by_id[target]["itemId"] <= 2003
                }
                result["selectedMachineConnections"].append({
                    "machine": machine, "inserters": linked_arms,
                    "connectedBelts": [by_id[target] for target in sorted(belt_ids)],
                })
    counts = Counter()
    degree = defaultdict(lambda: [0, 0])
    for arm in arms:
        for key, direction, position in (("outputObjIdx", "input", 0), ("inputObjIdx", "output", 1)):
            machine = machines.get(arm[key])
            if machine is not None:
                counts[(machine["itemId"], machine["recipeId"], direction, arm["itemId"], arm["filterId"])] += 1
                degree[machine["index"]][position] += 1
    result["machineInserterConnections"] = [
        {"machineItemId": item, "recipeId": recipe, "direction": direction, "inserterItemId": arm, "filterId": filter_id, "count": count}
        for (item, recipe, direction, arm, filter_id), count in sorted(counts.items())
    ]
    result["machineInOutCounts"] = [
        {"inputCount": incoming, "outputCount": outgoing, "machineCount": count}
        for (incoming, outgoing), count in sorted(Counter(tuple(degree[index]) for index in machines).items())
    ]
    result["machineParametersByRecipe"] = {
        str(recipe): {str(list(parameters)): count for parameters, count in sorted(Counter(tuple(b["parameters"]) for b in machines.values() if b["recipeId"] == recipe).items())}
        for recipe in sorted({b["recipeId"] for b in machines.values()})
    }
    return result


def main() -> None:
    """从应用根定位源码，更新仅属于本方案的两份研究数据。"""
    folder = Path(__file__).resolve().parent
    root = folder.parents[1]
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, default=folder / "73125增产剂v1.3（本地版）.txt")
    parser.add_argument("--output-dir", type=Path, default=folder)
    parser.add_argument("--analysis-date", default=date.today().isoformat())
    options = parser.parse_args()
    source = options.source.resolve()
    output = options.output_dir.resolve()
    payload = read_document(root, source)
    revision = subprocess.run(["git", "rev-parse", "HEAD"], cwd=root, check=True, capture_output=True, text=True).stdout.strip()
    structure = analyze_structure(payload["document"], payload["verification"], source, revision, options.analysis_date)
    spacing = analyze_spacing(payload["document"], structure["sha256"])
    output.mkdir(parents=True, exist_ok=True)
    write_json(output / "参考蓝图结构统计.json", structure)
    write_json(output / "参考蓝图间距与连接数据.json", spacing)
    print(f"建筑 {structure['buildingCount']}，机器 Y 分组 {len(spacing['rows'])}；结构与间距数据已写入方案目录。")
    print(f"严格编解码核验：{all(payload['verification'].values())}；游戏放置与完整几何终检未执行。")


if __name__ == "__main__":
    main()
