"""按严格解码的实体坐标绘制布局诊断图；矩形是示意轮廓，不代替游戏碰撞检测。"""
import argparse
import json
import math
from pathlib import Path
from PIL import Image, ImageDraw

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("directory", type=Path, help="DSBOX_BLUEPRINT_TEST_OUTPUT 指定的导出目录")
parser.add_argument("--pattern", default="matrix480-mode[134].json", help="导出JSON文件匹配规则")
parser.add_argument("--columns", type=int, default=3, help="每行图片数量")
parser.add_argument("--output", default="matrix480-baseline.png", help="对照图文件名")
args = parser.parse_args()
root = args.directory
sources = sorted(root.glob(args.pattern))
if not sources or args.columns < 1:
    parser.error("未找到JSON样例，或每行图片数量小于1")
panels = []
for path in sources:
    source = json.loads(path.read_text(encoding="utf8"))
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
        if b.get("recipeId", 0) > 0 or b["itemId"] in (2101,2102,2107):
            sizes={2302:(1.8,1.8,"#5098b0"),2303:(2.1,2.1,"#768de6"),2901:(2.5,2.5,"#d7e5ef"),2101:(1.5,1.5,"#6db695"),2102:(2,2,"#6db695"),2107:(.8,.8,"#b0f5cb"),2308:(5,2,"#e6b368"),2309:(5,3,"#be8ee6"),2310:(6,5,"#dd8e95")}
            aliases={2315:2302,2319:2302,2304:2303,2305:2303,2318:2303,2902:2901,2317:2309}
            sx,sy,color=sizes.get(aliases.get(b["itemId"],b["itemId"]),(3,3,"#aaaabb"))
            hx,hy=sx*scale/2,sy*scale/2
            angle=math.radians(b["yaw"])
            corners=[(x+dx*math.cos(angle)+dy*math.sin(angle),y+dx*math.sin(angle)-dy*math.cos(angle)) for dx,dy in ((-hx,-hy),(hx,-hy),(hx,hy),(-hx,hy))]
            draw.polygon(corners,fill=color,outline="#f5fafc")
            draw.line((x,y,x+hy*math.sin(angle),y-hy*math.cos(angle)),fill="#273d50",width=2)
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
    # 监测器最后画，箭头沿导出Yaw；不使用宿主原来的默认朝向。
    for b in buildings:
        if b["itemId"] == 2030:
            x,y=point(b["localOffset"])
            a=math.radians(b["yaw"])
            tip=(x+6*math.sin(a),y-6*math.cos(a))
            left=(x-4*math.sin(a)+4*math.cos(a),y+4*math.cos(a)+4*math.sin(a))
            right=(x-4*math.sin(a)-4*math.cos(a),y+4*math.cos(a)-4*math.sin(a))
            draw.polygon((tip,left,right),fill="#ffeb6e",outline="#ffffff")
    belts=sum(2001<=b["itemId"]<=2003 for b in buildings)
    towers=sum(b["itemId"] in (2103,2104) for b in buildings)
    draw.text((20,12),path.stem,fill="#f2f6fa")
    draw.text((20,33),f"{source['surfacePlan']['layoutWidth']} x {source['surfacePlan']['layoutHeight']} | {belts} belts | {towers} towers",fill="#b0c0d0")
    draw.text((20,52),"Decoded position + yaw / schematic footprints",fill="#b0c0d0")
    panel.save(path.with_suffix(".png"))
    panels.append(panel)
cell_width,cell_height=max(p.width for p in panels),max(p.height for p in panels)
columns=min(args.columns,len(panels))
combined=Image.new("RGB",(columns*cell_width,math.ceil(len(panels)/columns)*cell_height),"#101824")
for i,panel in enumerate(panels):
    combined.paste(panel,((i%columns)*cell_width,(i//columns)*cell_height))
combined.save(root/args.output)
print(root/args.output)
