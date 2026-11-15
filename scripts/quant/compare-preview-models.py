"""Compare exported Unity prefab parts with preview GLB primitives and renderer rules."""
import argparse
import json
import re
import struct
from pathlib import Path


def read_glb(path):
    with path.open("rb") as stream:
        stream.seek(12)
        length, kind = struct.unpack("<II", stream.read(8))
        if kind != 0x4E4F534A:
            raise ValueError(f"No GLB JSON chunk: {path}")
        return json.loads(stream.read(length))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("reference", type=Path)
    parser.add_argument("frontend", type=Path)
    args = parser.parse_args()
    source = args.frontend / "src/packages/algorithm-viz/src"
    colors = (source / "render/native-colors.ts").read_text(encoding="utf-8")
    hidden = set(re.findall(r"'([^']+)'", colors.split("HIDDEN_PARTS", 1)[1].split("]);", 1)[0]))
    unlit = set(re.findall(r"'([^']+)': \{[^}]*\bunlit: true", colors))
    keys = sorted(set(re.findall(r"\d+: '([^']+)'", (source / "catalog.ts").read_text(encoding="utf-8"))))
    report = {}

    for key in keys:
        root = args.reference / "models" / key
        unity = json.loads((root / "index.json").read_text(encoding="utf-8"))
        path = args.frontend / "src/data/models" / f"{key}.glb"
        glb = read_glb(path) if path.is_file() else {}
        procedural = key.startswith("belt-")
        material_by_id = {m["pathId"]: m for m in unity["materials"].values()}
        glb_nodes = {n.get("name", "") for n in glb.get("nodes", []) if "mesh" in n}
        primitives = []
        for node in glb.get("nodes", []):
            if "mesh" not in node:
                continue
            node_name = node.get("name", "")
            for primitive in glb["meshes"][node["mesh"]]["primitives"]:
                mat = glb["materials"][primitive["material"]] if "material" in primitive else {}
                name = mat.get("name", "")
                kind = ("glass" if re.search("glass", name, re.I) else
                        "effect" if re.search("effect|-eff|mask|-water$", name, re.I) else "solid")
                reason = None
                if procedural:
                    reason = "procedural-belt"
                elif (re.search("lod[1-9]|-cl$|shadow", node_name, re.I) or
                      (re.search("collider", node_name, re.I) and not re.fullmatch(r"hadron-collider(?:-\d+)*", node_name, re.I))):
                    reason = "mesh-filter"
                elif name in hidden:
                    reason = "hidden-material"
                elif kind == "effect" and "baseColorTexture" not in mat.get("pbrMetallicRoughness", {}):
                    reason = "effect-without-base-texture"
                elif "POSITION" not in primitive.get("attributes", {}):
                    reason = "no-positions"
                primitives.append({"node": node_name, "material": name, "kind": kind, "skipped": reason,
                                   "emissiveTexture": mat.get("emissiveTexture"),
                                   "emissiveFactor": mat.get("emissiveFactor"),
                                   "emissiveStrength": mat.get("extensions", {}).get("KHR_materials_emissive_strength"),
                                   "emissiveTextureSuppressed": bool("emissiveTexture" in mat and not reason and
                                                                      (name in unlit or kind != "solid"))})
        glb_materials = {p["material"] for p in primitives}
        parts = []

        def walk(node, parent="", active=True):
            name = node["name"]
            full = f"{parent}/{name}" if parent else name
            active = active and bool(node["active"])
            for renderer in node["renderers"]:
                for mat_id in renderer["materials"]:
                    mat = material_by_id.get(mat_id)
                    if mat is None:
                        continue
                    emission = {slot: file for slot, file in mat["textures"].items() if re.search("emiss|glow", slot, re.I)}
                    effect = (renderer["type"] not in ("MeshRenderer", "SkinnedMeshRenderer") or
                              bool(re.search("effect|additive|glass|particle|-eff", f"{mat['name']} {mat['shader']}", re.I)))
                    matches = [p for p in primitives if p["material"] == mat["name"]]
                    parts.append({"node": full, "active": active, "rendererEnabled": bool(renderer["enabled"]),
                                  "renderer": renderer["type"], "mesh": renderer["mesh"] or node["mesh"],
                                  "material": mat["name"], "materialId": mat_id, "shader": mat["shader"],
                                  "originalEmission": emission, "effectCandidate": effect,
                                  "nodeNameInGlb": name in glb_nodes, "materialNameInGlb": mat["name"] in glb_materials,
                                  "glbTexturePresent": any(p["emissiveTexture"] is not None for p in matches)})
            for child in node["children"]:
                walk(child, full, active)

        for prefab in unity["prefabs"].values():
            walk(prefab["root"])
        report[key] = {"glbPresent": path.is_file(), "procedural": procedural,
                       "prefabIds": list(unity["prefabs"]), "missingPrefabs": unity["missingPrefabs"],
                       "meshCount": len(unity["meshes"]), "materialCount": len(unity["materials"]),
                       "glbPrimitives": primitives, "unityParts": parts}

    (args.reference / "blueprint-render-comparison.json").write_text(
        json.dumps(report, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
