"""Export selected vanilla building meshes, textures and material texture slots."""
import argparse
import json
import re
from pathlib import Path

import UnityPy
from PIL import Image
from UnityPy.enums import TextureFormat
from UnityPy.helpers.TypeTreeGenerator import TypeTreeGenerator


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("game", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("--building", action="append")
    parser.add_argument("--preview-catalog", type=Path, help="TypeScript 预览目录；导出登记模型的原版 prefab")
    args = parser.parse_args()
    data = args.game / "DSPGAME_Data"
    env = UnityPy.load(*(str(data / name) for name in (
        "resources.assets", "globalgamemanagers", "globalgamemanagers.assets", "sharedassets0.assets"
    )))
    manager = next(obj for obj in env.objects if obj.type.name == "ResourceManager")
    resources = manager.read_typetree()["m_Container"]
    files = {obj.assets_file.name: obj.assets_file for obj in env.objects}

    def resolve(ptr):
        file_id = ptr["m_FileID"]
        file = Path(manager.assets_file.externals[file_id - 1].path).name if file_id else manager.assets_file.name
        return files[file].objects[ptr["m_PathID"]]

    env.typetree_generator = TypeTreeGenerator(files["resources.assets"].unity_version)
    env.typetree_generator.load_local_game(str(args.game))
    resources_by_path = dict(resources)
    model_table = resolve(resources_by_path["prototypes/modelprotoset"]).read_typetree()["dataArray"]
    by_model = {row["ID"]: row for row in model_table}
    args.output.mkdir(parents=True, exist_ok=True)
    (args.output / "model-prototypes.json").write_text(json.dumps(model_table, ensure_ascii=False, indent=2), encoding="utf-8")

    prefixes = {"lighthouse": "lighthouse", "ray-receiver": "ray receiver", "hadron-collider": "hadron collider"}
    keys = {}
    if args.preview_catalog:
        source = args.preview_catalog.read_text(encoding="utf-8")
        for model_id, key in re.findall(r"(\d+): '([^']+)'", source):
            keys.setdefault(key, []).append(int(model_id))
    else:
        keys = {"lighthouse": [664], "ray-receiver": [73], "hadron-collider": [69]}

    def safe(name):
        return re.sub(r'[<>:"/\\|?*\x00-\x1f]', "_", name).rstrip(" .") or "unnamed"

    def collect_prefab(root, collected):
        def remember(ptr):
            if not ptr or not ptr.path_id:
                return None
            obj = ptr.deref()
            collected[(obj.assets_file.name, obj.path_id)] = (None, obj)
            return obj.path_id

        def vector(value, components="xyz"):
            return [getattr(value, component) for component in components]

        def walk(go, transform):
            row = {"name": go.m_Name, "active": go.m_IsActive, "position": vector(transform.m_LocalPosition),
                   "rotation": vector(transform.m_LocalRotation, "xyzw"), "scale": vector(transform.m_LocalScale),
                   "mesh": None, "renderers": [], "children": []}
            for entry in go.m_Component:
                obj = entry.component.deref()
                kind = obj.type.name
                if kind == "MeshFilter":
                    row["mesh"] = remember(obj.read().m_Mesh)
                elif kind in ("MeshRenderer", "SkinnedMeshRenderer", "ParticleSystemRenderer", "LineRenderer", "TrailRenderer"):
                    renderer = obj.read()
                    row["renderers"].append({"type": kind, "enabled": renderer.m_Enabled,
                                             "mesh": remember(getattr(renderer, "m_Mesh", None)),
                                             "materials": [remember(ptr) for ptr in renderer.m_Materials]})
            for ptr in transform.m_Children:
                child = ptr.read()
                row["children"].append(walk(child.m_GameObject.read(), child))
            return row

        go = root.read()
        transform = next(entry.component.read() for entry in go.m_Component if entry.component.type.name in ("Transform", "RectTransform"))
        return walk(go, transform)

    for name in args.building or keys:
        output = args.output / name
        output.mkdir(exist_ok=True)
        index = {"meshes": {}, "materials": {}, "rawTextures": {}, "prefabs": {}, "missingPrefabs": []}
        collected = {}
        for model_id in keys[name]:
            path = by_model[model_id]["PrefabPath"]
            ptr = resources_by_path.get(path.lower())
            if not ptr or not ptr["m_PathID"]:
                index["missingPrefabs"].append({"id": model_id, "path": path})
                continue
            index["prefabs"][str(model_id)] = {"path": path, "root": collect_prefab(resolve(ptr), collected)}

        # 三个既有样本保留资源目录中的废墟及辅助效果；传送带 prefab 本身没有网格。
        prefix = prefixes.get(name, "belt" if name.startswith("belt-") else None)
        if prefix:
            for path, ptr in resources:
                if path.startswith(f"entities/models/{prefix}/") and ptr["m_PathID"]:
                    obj = resolve(ptr)
                    if obj.type.name in ("Mesh", "Material"):
                        collected[(obj.assets_file.name, obj.path_id)] = (path, obj)

        def save_texture(texture, path_id):
            file = f"{safe(texture.m_Name)}-{path_id}.png"
            target = output / file
            if not target.exists():
                try:
                    if texture.m_TextureFormat == TextureFormat.R16:
                        # Unity R16 包含后续 mip；PNG 仅导出最高分辨率的一层。
                        size = (texture.m_Width, texture.m_Height)
                        top_mip = texture.get_image_data()[:size[0] * size[1] * 2]
                        Image.frombytes("I;16", size, top_mip).transpose(Image.Transpose.FLIP_TOP_BOTTOM).save(target)
                    else:
                        texture.image.save(target)
                except (ValueError, NotImplementedError) as error:
                    file = target.with_suffix(".bin").name
                    (output / file).write_bytes(texture.get_image_data())
                    index["rawTextures"][file] = {
                        "width": texture.m_Width, "height": texture.m_Height,
                        "unityFormat": int(texture.m_TextureFormat), "decodeError": str(error),
                    }
            return file

        for path, obj in collected.values():
            if obj.type.name == "Mesh":
                mesh = obj.read()
                filename = f"{safe(mesh.m_Name)}-{obj.path_id}.obj"
                (output / filename).write_text(mesh.export("obj"), encoding="utf-8")
                index["meshes"][filename] = path
            elif obj.type.name == "Material":
                material = obj.read()
                slots = {}
                for key, entry in material.m_SavedProperties.m_TexEnvs:
                    if entry.m_Texture.path_id:
                        texture = entry.m_Texture.read()
                        if hasattr(texture, "get_image_data"):
                            slots[key] = save_texture(texture, entry.m_Texture.path_id)
                props = material.m_SavedProperties
                shader = material.m_Shader.read() if material.m_Shader.path_id else None
                index["materials"][f"{safe(material.m_Name)}-{obj.path_id}"] = {
                    "name": material.m_Name, "pathId": obj.path_id,
                    "resource": path,
                    "shader": (shader.m_ParsedForm.m_Name or shader.m_Name) if shader else None,
                    "textures": slots,
                    "floats": dict(props.m_Floats),
                    "ints": dict(getattr(props, "m_Ints", []) or []),
                    "renderQueue": material.m_CustomRenderQueue,
                    "colors": {key: [value.r, value.g, value.b, value.a] for key, value in props.m_Colors},
                }
        (output / "index.json").write_text(json.dumps(index, indent=2), encoding="utf-8")
        print(name, len(index["meshes"]), "meshes;", len(index["materials"]), "materials")
    print(len(model_table), "model prototypes")


if __name__ == "__main__":
    main()
