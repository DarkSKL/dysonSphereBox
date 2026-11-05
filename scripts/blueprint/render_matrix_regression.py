"""按严格解码的实体坐标绘制布局诊断图；矩形是示意轮廓，不代替游戏碰撞检测。"""
import argparse
import json
from pathlib import Path
from PIL import Image, ImageDraw

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("directory", type=Path, help="DSBOX_BLUEPRINT_TEST_OUTPUT 指定的导出目录")
root = parser.parse_args().directory
panels = []
for mode in (1, 3, 4):
    source = json.loads((root / f"matrix480-mode{mode}.json").read_text(encoding="utf8"))
    buildings = source["document"]["buildings"]
    records = {b["index"]: b for b in buildings}
    xs = [b["localOffset"]["x"] for b in buildings]
    ys = [b["localOffset"]["y"] for b in buildings]
    scale, margin, header = 14, 60, 70
    width = max(440, round((max(xs)-min(xs))*scale)+2*margin)
    height = round((max(ys)-min(ys))*scale)+2*margin+header
    panel = Image.new("RGB", (width, height), "#101824")
    draw = ImageDraw.Draw(panel)
    def point(p):
        return (margin+(p["x"]-min(xs))*scale, header+margin+(max(ys)-p["y"])*scale)
    for x in range(0, int(max(xs)-min(xs))+1, 5):
        draw.line((margin+x*scale,header+margin,margin+x*scale,height-margin),fill="#253343")
    for y in range(0, int(max(ys)-min(ys))+1, 5):
        draw.line((margin,header+margin+y*scale,width-margin,header+margin+y*scale),fill="#253343")
    for b in buildings:
        x,y=point(b["localOffset"])
        if b["itemId"] in (2302,2303,2901):
            size,color={2302:(1.8,"#5098b0"),2303:(2.1,"#768de6"),2901:(2.5,"#d7e5ef")}[b["itemId"]]
            half=size*scale/2
            draw.rectangle((x-half,y-half,x+half,y+half),fill=color,outline="#f5fafc")
        elif b["itemId"] in (2103,2104):
            radius=4.5*scale
            draw.ellipse((x-radius,y-radius,x+radius,y+radius),outline="#f0a965",width=3)
            draw.text((x-18,y-5),"TOWER",fill="#f0a965")
    for b in buildings:
        if 2001 <= b["itemId"] <= 2003:
            end=records.get(b["outputObjIdx"])
            if end:
                color="#67bde0" if b["localOffset"]["z"]>.5 else "#cc883d"
                draw.line((*point(b["localOffset"]),*point(end["localOffset"])),fill=color,width=2)
        elif 2011<=b["itemId"]<=2014:
            draw.line((*point(b["localOffset"]),*point(b["localOffset2"])),fill="#a0adb9")
    belts=sum(2001<=b["itemId"]<=2003 for b in buildings)
    towers=sum(b["itemId"] in (2103,2104) for b in buildings)
    draw.text((20,12),f"480/min matrix | mode {mode} | equator",fill="#f2f6fa")
    draw.text((20,33),f"{source['surfacePlan']['layoutWidth']} x {source['surfacePlan']['layoutHeight']} | {belts} belts | {towers} towers",fill="#b0c0d0")
    draw.text((20,52),"Decoded coordinates / schematic footprints",fill="#b0c0d0")
    panel.save(root/f"matrix480-mode{mode}.png")
    panels.append(panel)
combined=Image.new("RGB",(sum(p.width for p in panels),max(p.height for p in panels)),"#101824")
offset=0
for panel in panels:
    combined.paste(panel,(offset,0))
    offset+=panel.width
combined.save(root/"matrix480-baseline.png")
print(root/"matrix480-baseline.png")
