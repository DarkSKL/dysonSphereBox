"""按蓝图原生 Area 坐标绘制极区俯视图；建筑轮廓为示意，不能替代游戏碰撞检测。"""
import argparse
import json
import math
from pathlib import Path

from PIL import Image, ImageDraw


def render(source_path, output_path):
    """保留真实球面中心和本地朝向，显示纬线弧度，避免将经纬坐标展开误看成实际长条。"""
    source = json.loads(source_path.read_text(encoding="utf8"))
    document, plan = source["document"], source["surfacePlan"]
    records = {b["index"]: b for b in document["buildings"]}
    areas = {a["index"]: a for a in document["areas"]}
    origins = {}
    lat_step = math.tau / (plan["gridSegment"] * 5)

    def origin(index):
        if index not in origins:
            area = areas[index]
            if index == plan["primaryAreaIndex"]:
                origins[index] = (0, plan["anchorLatitudeGrid"] * lat_step)
            else:
                parent = areas[area["parentIndex"]]
                lon, lat = origin(parent["index"])
                lon += area["tropicAnchor"] * math.tau / (parent["areaSegments"] * 5)
                lon -= area["anchorLocalOffsetX"] * math.tau / (area["areaSegments"] * 5)
                origins[index] = (lon, lat + area["anchorLocalOffsetY"] * lat_step)
        return origins[index]

    def world(record, key="localOffset"):
        p = record[key]
        lon, lat = origin(record["areaIndex"])
        lon += p["x"] * math.tau / (areas[record["areaIndex"]]["areaSegments"] * 5)
        lat += p["y"] * lat_step
        radius = 200.2 + p["z"] * 1.3333333
        return (radius * math.cos(lat) * math.sin(lon), -radius * math.cos(lat) * math.cos(lon), lon, lat)

    positions = {i: world(r) for i, r in records.items()}
    left, right = min(p[0] for p in positions.values()) - 9, max(p[0] for p in positions.values()) + 9
    bottom, top = min(p[1] for p in positions.values()) - 9, max(p[1] for p in positions.values()) + 9
    scale = min(14, 1400 / (right - left), 1000 / (top - bottom))
    image = Image.new("RGB", (round((right-left)*scale), round((top-bottom)*scale)+65), "#101824")
    draw = ImageDraw.Draw(image)

    def screen(x, z):
        return ((x-left)*scale, 65+(top-z)*scale)

    for grid in range(180, 251, 5):
        radius = 200.2 * math.cos(grid * lat_step)
        draw.ellipse((*screen(-radius, radius), *screen(radius, -radius)), outline="#253343")
    for degree in range(0, 360, 10):
        angle = math.radians(degree)
        draw.line((*screen(0, 0), *screen(95*math.sin(angle), -95*math.cos(angle))), fill="#253343")

    sizes = {2302: (1.8,1.8,"#5098b0"), 2303: (2.1,2.1,"#768de6"), 2901: (2.5,2.5,"#d7e5ef"),
             2101: (1.5,1.5,"#6db695"), 2107: (.8,.8,"#b0f5cb"), 2308: (5,2,"#e6b368"),
             2309: (5,3,"#be8ee6"), 2310: (6,5,"#dd8e95")}
    aliases = {2315:2302,2319:2302,2304:2303,2305:2303,2318:2303,2902:2901,2317:2309}
    for i, r in records.items():
        x,z,lon,lat = positions[i]
        if r.get("recipeId",0)>0 or r["itemId"] in (2101,2102,2107):
            width,depth,color = sizes.get(aliases.get(r["itemId"],r["itemId"]),(3,3,"#aabbcc"))
            yaw = math.radians(r["yaw"])
            def local(dx,dy):
                east = (dx*math.cos(yaw)+dy*math.sin(yaw))*math.tau*200.2/1000
                north = (-dx*math.sin(yaw)+dy*math.cos(yaw))*math.tau*200.2/1000
                return screen(x+east*math.cos(lon)-north*math.sin(lat)*math.sin(lon),
                              z+east*math.sin(lon)+north*math.sin(lat)*math.cos(lon))
            draw.polygon([local(dx*width/2,dy*depth/2) for dx,dy in ((-1,-1),(1,-1),(1,1),(-1,1))],fill=color,outline="#eff4fa")
            draw.line((local(0,0),local(0,depth/2)),fill="#263445",width=2)
        elif r["itemId"] in (2103,2104):
            draw.ellipse((*screen(x-6,z+6),*screen(x+6,z-6)),outline="#f0a965",width=3)
        if 2001<=r["itemId"]<=2003 and r["outputObjIdx"] in positions:
            q = positions[r["outputObjIdx"]]
            draw.line((*screen(x,z),*screen(q[0],q[1])),fill="#67bde0" if r["localOffset"]["z"]>.5 else "#cc883d",width=2)
        elif 2011<=r["itemId"]<=2014:
            q = world(r,"localOffset2")
            draw.line((*screen(x,z),*screen(q[0],q[1])),fill="#a0adb9")
    draw.rectangle((0,0,image.width,62),fill="#101824")
    # 箭头沿导出监测器在球面当地的真实朝向；最后绘制，避免被传送带线条遮挡。
    for i,r in records.items():
        if r["itemId"]!=2030:
            continue
        x,z,lon,lat=positions[i]
        yaw=math.radians(r["yaw"])
        dx=math.sin(yaw)*math.cos(lon)-math.cos(yaw)*math.sin(lat)*math.sin(lon)
        dz=math.sin(yaw)*math.sin(lon)+math.cos(yaw)*math.sin(lat)*math.cos(lon)
        length=math.hypot(dx,dz)
        if length<1e-8:
            continue
        ux,uy=dx/length,-dz/length
        px,py=screen(x,z)
        draw.polygon(((px+8*ux,py+8*uy),(px-4*ux-4*uy,py-4*uy+4*ux),(px-4*ux+4*uy,py-4*uy-4*ux)),fill="#ffeb6e",outline="#ffffff")
    draw.text((14,10),source_path.stem,fill="#f2f6fa")
    draw.text((14,30),"Polar top view / native spherical centers + local yaw / schematic bodies",fill="#b0c0d0")
    image.save(output_path)
    print(output_path)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source",type=Path)
    parser.add_argument("output",type=Path)
    args = parser.parse_args()
    render(args.source,args.output)
