"""Project a verified 0.10.35.29151 dump onto the calculator's existing Vanilla schema."""
import argparse
import json
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("dump", type=Path)
    parser.add_argument("baseline", type=Path)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()

    def load(name):
        return json.loads((args.dump / f"{name}.json").read_text(encoding="utf-8"))

    manifest = load("manifest")
    if (manifest["gameVersion"], manifest["steamBuildId"]) != ("0.10.35.29151", "25955035"):
        raise SystemExit("Expected Steam build 25955035 / 0.10.35.29151")

    raw = args.baseline.read_bytes()
    data = json.loads(raw.decode("utf-8-sig"))
    data.pop("gameVersion", None)
    data = {"gameVersion": manifest["gameVersion"], **data}
    items = {row["ID"]: row for row in load("items")}
    recipes = {row["ID"]: row for row in load("recipes")}
    recipe_fields = ("Type", "Items", "ItemCounts", "Results", "ResultCounts", "TimeSpend")

    # 保持既有顺序、名称键和合成规则，旧工作区按序号保存的配方仍可定位。
    for recipe in data["recipes"]:
        source = recipes.get(recipe["ID"])
        if source is None:
            continue
        for field in recipe_fields:
            # 原版分馏记录使用 100:1 概率编码；量化以 100 次尝试的耗时模拟成功转化。
            if recipe["ID"] == 115 and field in ("ItemCounts", "TimeSpend"):
                if (source["ItemCounts"], source["TimeSpend"]) != ([100], 1):
                    raise SystemExit("Fractionation encoding changed")
                continue
            if recipe[field] != source[field]:
                recipe[field] = float(source[field]) if field == "TimeSpend" else source[field]
    for item in data["items"]:
        source = items.get(item["ID"])
        if source:
            for field in ("Type", "GridIndex"):
                item[field] = source[field]

    if not any(row["ID"] == 162 for row in data["recipes"]):
        lens = recipes[162]
        normal_lens = next(row for row in data["recipes"] if row["ID"] == 101)
        if lens["Type"] != normal_lens["Type"] or lens["NonProductive"] or lens["Results"] != [1211]:
            raise SystemExit("Unexpected Dark Fog Lens recipe")
        data["recipes"].append({
            "ID": 162, "Type": lens["Type"], "Factories": normal_lens["Factories"], "Name": lens["Name"],
            **{field: lens[field] for field in recipe_fields[1:]},
            "Proliferator": 3, "IconName": "darkfog-lens",
        })
    existing_ids = {item["ID"] for item in data["items"]}
    for item_id in (1099, 1211):
        if item_id not in existing_ids:
            source = items[item_id]
            data["items"].append({
                **{field: source[field] for field in ("ID", "Type", "Name", "GridIndex")},
                "IconName": source["IconPath"].rsplit("/", 1)[-1],
            })

    result = json.dumps(data, ensure_ascii=False, indent=2)
    if raw.endswith(b"\n"):
        result += "\n"
    if b"\r\n" in raw:
        result = result.replace("\n", "\r\n")
    args.output.write_bytes(result.encode("utf-8-sig" if raw.startswith(b"\xef\xbb\xbf") else "utf-8"))


if __name__ == "__main__":
    main()
