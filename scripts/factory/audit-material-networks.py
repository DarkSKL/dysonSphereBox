"""独立统计实际蓝图的材料带网络与机器连接。

职责：从严格解码的最终记录统计带分量、跨配方消费、带间搬运及有向可达关系。
使用：python -B scripts/factory/audit-material-networks.py 蓝图.txt --output 报告.json。
边界：不执行生成器，不把连通分量或机型数量当作实际吞吐/游戏放置证明。
"""

import argparse
import hashlib
import importlib.util
import json
import sys
from collections import Counter, defaultdict
from pathlib import Path
from urllib.parse import unquote

sys.dont_write_bytecode = True


def audit(document: dict, recipes: dict[int, dict]) -> dict:
    """按真实输出连接归集带分量；以有向追踪单独区分潜在共享与实际可达。"""
    records = {record["index"]: record for record in document["buildings"]}
    belts = {index: record for index, record in records.items() if 2001 <= record["itemId"] <= 2003}
    parents = {index: index for index in belts}

    def find(index: int) -> int:
        while parents[index] != index:
            parents[index] = parents[parents[index]]
            index = parents[index]
        return index

    for index, belt in belts.items():
        output = belt["outputObjIdx"]
        if output in belts:
            parents[find(output)] = find(index)

    groups = defaultdict(lambda: {"belts": [], "producers": set(), "consumers": set()})
    producers, consumers = defaultdict(set), defaultdict(set)
    transfers, direct = [], []
    for index in belts:
        groups[find(index)]["belts"].append(index)
    for arm in records.values():
        if not 2011 <= arm["itemId"] <= 2014:
            continue
        source, target = records.get(arm["inputObjIdx"]), records.get(arm["outputObjIdx"])
        if source is None or target is None:
            continue
        if source["recipeId"] and target["index"] in belts:
            groups[find(target["index"])]["producers"].add(source["index"])
            producers[target["index"]].add(source["index"])
        if target["recipeId"] and source["index"] in belts:
            groups[find(source["index"])]["consumers"].add(target["index"])
            consumers[source["index"]].add(target["index"])
        if source["index"] in belts and target["index"] in belts:
            transfers.append(arm["index"])
        if source["recipeId"] and target["recipeId"]:
            direct.append(arm["index"])

    reachable = set()
    for origin, machines in producers.items():
        current, seen = origin, set()
        while current in belts and current not in seen:
            seen.add(current)
            for source in machines:
                for target in consumers[current]:
                    reachable.add((records[source]["recipeId"], records[target]["recipeId"]))
            current = belts[current]["outputObjIdx"]

    rows = []
    for identity, group in groups.items():
        source_recipes = sorted({records[index]["recipeId"] for index in group["producers"]})
        target_recipes = sorted({records[index]["recipeId"] for index in group["consumers"]})
        row = {"root": identity, "belts": len(group["belts"]), "producerMachines": len(group["producers"]),
               "consumerMachines": len(group["consumers"]), "producerRecipes": source_recipes, "consumerRecipes": target_recipes}
        if recipes and all(recipe in recipes for recipe in source_recipes + target_recipes):
            common = set.intersection(*(set(recipes[recipe]["items"]) for recipe in target_recipes)) if target_recipes else set()
            produced = {item for recipe in source_recipes for item in recipes[recipe]["results"]}
            row["candidateItems"] = sorted(common & produced if produced else common)
            row["consumerNames"] = [recipes[recipe]["name"] for recipe in target_recipes]
        rows.append(row)
    rows.sort(key=lambda row: (-len(row["consumerRecipes"]), -row["belts"], row["root"]))
    return {"buildingCount": len(records), "machineCount": sum(bool(record["recipeId"]) for record in records.values()),
            "items": dict(Counter(record["itemId"] for record in records.values())), "areas": document["areas"],
            "beltComponents": len(rows), "beltToBeltInserters": len(transfers), "machineToMachineInserters": len(direct),
            "componentsFeedingMultipleRecipes": sum(len(row["consumerRecipes"]) > 1 for row in rows),
            "directedRecipePairsThroughBelts": [list(pair) for pair in sorted(reachable)], "components": rows,
            "scope": "带分量按带到带输出的无向连通性统计；有向配方可达单列，不穿越加工、库存或带间分拣器。candidateItems是配方交集推断，非运行货物证明。"}


def main() -> None:
    """只读用户指定源文件并保存可重复的审计结果；临时严格解码复用项目已有研究读取器。"""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("blueprint", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--dataset-json", type=Path, help="可选：包含recipes数组的只读数据集研究快照")
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[2]
    spec = importlib.util.spec_from_file_location("density_reader", root / "docs/超密度蓝图生成适配器方案/参考数据复核.py")
    reader = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(reader)
    source = args.blueprint.resolve()
    payload = reader.read_document(root, source)
    if not all(payload["verification"].values()):
        raise RuntimeError("严格编解码未通过")
    recipes = {} if args.dataset_json is None else {recipe["id"]: recipe for recipe in json.loads(args.dataset_json.read_text(encoding="utf-8-sig"))["recipes"]}
    result = audit(payload["document"], recipes)
    result.update(source=str(source), sourceSha256=hashlib.sha256(source.read_bytes()).hexdigest(), verification=payload["verification"])
    fields = payload["document"].get("envelopeFields", [])
    result["additionalEnvelopeHint"] = unquote(fields[13]) if len(fields) > 13 and len(fields[13]) < 4096 else ""
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({key: result[key] for key in ("machineCount", "beltComponents", "beltToBeltInserters", "componentsFeedingMultipleRecipes")}, ensure_ascii=True))


if __name__ == "__main__":
    main()
