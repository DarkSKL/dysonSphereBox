"""Extract vanilla prototype tables and referenced sprites from a local Steam install."""
import argparse
import hashlib
import json
import re
from pathlib import Path

import UnityPy
from UnityPy.helpers.TypeTreeGenerator import TypeTreeGenerator


def write_json(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2, allow_nan=False) + "\n", encoding="utf-8")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("game", type=Path)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    game, output = args.game.resolve(), args.output.resolve()
    version = (game / "Updates/Versions.txt").read_text(encoding="utf-8-sig").splitlines()[-1].split(",")[0]
    steam_manifest = game.parent.parent / "appmanifest_1366540.acf"
    build = re.search(r'"buildid"\s+"(\d+)"', steam_manifest.read_text(encoding="utf-8-sig")) if steam_manifest.is_file() else None
    data = game / "DSPGAME_Data"
    assets = ("resources.assets", "globalgamemanagers", "globalgamemanagers.assets", "sharedassets0.assets")
    env = UnityPy.load(*(str(data / name) for name in assets))
    manager = next(obj for obj in env.objects if obj.type.name == "ResourceManager")
    resources = dict(manager.read_typetree()["m_Container"])
    files = {obj.assets_file.name: obj.assets_file for obj in env.objects}

    def lookup(path):
        ptr = resources[path.lower()]
        file_id = ptr["m_FileID"]
        name = Path(manager.assets_file.externals[file_id - 1].path).name if file_id else manager.assets_file.name
        return files[name].objects[ptr["m_PathID"]]

    env.typetree_generator = TypeTreeGenerator(files["resources.assets"].unity_version)
    env.typetree_generator.load_local_game(str(game))
    output.mkdir(parents=True, exist_ok=True)
    tables = {}
    signatures = {}
    for key, proto in (("items", "item"), ("recipes", "recipe"), ("technologies", "tech")):
        obj = lookup(f"prototypes/{proto}protoset")
        table = obj.read_typetree()
        tables[key] = table["dataArray"]
        signatures[key] = table["Signature"]
        write_json(output / f"{key}.json", tables[key])
    paths = sorted({r["IconPath"] for rows in tables.values() for r in rows if r.get("IconPath")})
    for language, locale in (("zh", "2052"), ("en", "1033")):
        strings = {}
        raw = (game / "Locale" / locale / "prototype.txt").read_bytes()
        encoding = "utf-16" if raw.startswith((b"\xff\xfe", b"\xfe\xff")) else "utf-8-sig"
        for line in raw.decode(encoding).splitlines():
            fields = line.split("\t", 3)
            if len(fields) == 4 and fields[3]:
                strings[fields[0]] = fields[3]
        write_json(output / f"prototype-{language}.json", strings)
    exported, missing = {}, []
    for path in paths:
        try:
            obj = lookup(path)
        except KeyError:
            missing.append(path)
            continue
        if obj.type.name not in ("Sprite", "Texture2D"):
            missing.append(path)
            continue
        relative = Path("icons") / (path.lower() + ".png")
        target = output / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        obj.read().image.save(target)
        exported[path] = relative.as_posix()
    with (data / "resources.assets").open("rb") as f:
        asset_hash = hashlib.file_digest(f, "sha256").hexdigest()
    manifest = {
        "gameVersion": version,
        "steamBuildId": build.group(1) if build else None,
        "resourcesSha256": asset_hash,
        "unityVersion": files["resources.assets"].unity_version,
        "signatures": signatures,
        "counts": {key: len(rows) for key, rows in tables.items()},
        "icons": exported,
        "missingIcons": missing,
    }
    write_json(output / "manifest.json", manifest)
    print(manifest["counts"], f"icons={len(exported)} missing={len(missing)}")


if __name__ == "__main__":
    main()
