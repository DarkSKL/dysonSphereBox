"""Stage original light textures used by the blueprint preview's static materials."""
import argparse
import json
import re
from pathlib import Path

from PIL import Image, ImageChops


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("reference", type=Path)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    report = json.loads((args.reference / "blueprint-render-comparison.json").read_text(encoding="utf-8"))
    args.output.mkdir(parents=True, exist_ok=True)
    result = {}
    for key, row in report.items():
        root = args.reference / "models" / key
        index = json.loads((root / "index.json").read_text(encoding="utf-8"))
        sources = {m["name"]: m for m in index["materials"].values()}
        for primitive in row["glbPrimitives"]:
            name = primitive["material"]
            source = sources.get(name)
            if not source:
                continue
            slots = source["textures"]
            shader = source["shader"] or ""
            if primitive["skipped"] == "hidden-material":
                if "Black Mask" in shader:
                    continue
                if "Glass" in shader or "Spraycoater Water" in shader:
                    result[name] = {"kind": "glass", "color": source["colors"].get("_Color", [0.42, 0.6, 0.78])[:3]}
                    continue
                slot = next((s for s in ("_MainTex", "_FlareTex", "_OrbTex") if s in slots), None)
                if slot:
                    kind = "effect"
                elif "Fusion Sun" in shader:
                    result[name] = {"kind": "effect", "color": source["colors"]["_TintColor1"][:3]}
                    continue
                else:
                    continue
            elif any(s in slots for s in ("_EmissionTex", "_EmissionMap", "_GlowTex")):
                slot = next(s for s in ("_EmissionTex", "_EmissionMap", "_GlowTex") if s in slots)
                kind = "emission"
            else:
                continue
            source_file = slots[slot]
            if not source_file.endswith(".png"):
                continue
            texture_id = "light-" + re.search(r"-(\d+)\.png$", source_file).group(1)
            mask = slots.get("_MaskTex") if kind == "effect" else None
            if mask and mask.endswith(".png"):
                texture_id += "-mask-" + re.search(r"-(\d+)\.png$", mask).group(1)
            target = args.output / f"{texture_id}.webp"
            if not target.exists():
                with Image.open(root / source_file) as image:
                    image = image.convert("RGBA")
                    image.thumbnail((512, 512), Image.Resampling.LANCZOS)
                    if mask and mask.endswith(".png"):
                        with Image.open(root / mask) as mask_image:
                            if mask_image.mode.startswith("I;16"):
                                channel = mask_image.convert("I").point(lambda value: value / 257).convert("L")
                            else:
                                channel = mask_image.convert("RGB").getchannel("R")
                            channel = channel.resize(image.size, Image.Resampling.LANCZOS)
                            image.putalpha(ImageChops.multiply(image.getchannel("A"), channel))
                    image.save(target, lossless=True)
            result[name] = {"kind": kind, "texture": texture_id}
            if name.startswith("science-academy-"):
                result[name]["color"] = source["colors"]["_LabColor1"][:3]
                secondary = slots.get("_EmissionEffectTex")
                if secondary:
                    secondary_id = "light-" + re.search(r"-(\d+)\.png$", secondary).group(1)
                    with Image.open(root / secondary) as layer:
                        layer = layer.convert("RGBA")
                        layer.thumbnail((512, 512), Image.Resampling.LANCZOS)
                        layer.save(args.output / f"{secondary_id}.webp", lossless=True)
                    result[name]["secondary"] = secondary_id
    lighthouse = args.reference / "models/lighthouse"
    with Image.open(lighthouse / "lighthouse-eff-r-3543.png") as ring:
        ring.thumbnail((512, 512), Image.Resampling.LANCZOS)
        ring.convert("RGBA").save(args.output / "light-3543.webp", lossless=True)
    (args.output / "materials.json").write_text(json.dumps(result, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(len(result), "materials;", len(list(args.output.glob("light-*.webp"))), "textures")


if __name__ == "__main__":
    main()
