"""绘制Go原生几何导出的真实建造盒、世界端点和连接；不读取游戏美术资源。"""

import argparse
import itertools
import json
import math
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont


def hull(points):
    """取得定向盒投影的凸包，保留建筑高度在当前观察方向的影响。"""
    points = sorted(set(points))

    def cross(origin, first, second):
        return ((first[0] - origin[0]) * (second[1] - origin[1])
                - (first[1] - origin[1]) * (second[0] - origin[0]))

    lower, upper = [], []
    for point in points:
        while len(lower) > 1 and cross(lower[-2], lower[-1], point) <= 0:
            lower.pop()
        lower.append(point)
    for point in reversed(points):
        while len(upper) > 1 and cross(upper[-2], upper[-1], point) <= 0:
            upper.pop()
        upper.append(point)
    return lower[:-1] + upper[:-1]


def render(source_path, output_path, title, subtitle, projection):
    """采用切平面、极轴俯视或经纬展开；输入盒必须来自Go最终坐标的原生几何。"""
    source = json.loads(source_path.read_text(encoding="utf-8"))
    records, points = source["records"], source["points"]
    centers = [point for record, point in zip(records, points)
               if record.get("recipeId", 0) > 0]
    center = [sum(point[axis] for point in centers) / len(centers)
              for axis in range(3)]
    anchors = source.get("longitudes", [])
    if projection == "unwrapped" and len(anchors) != len(records):
        raise ValueError("经纬展开需要每条记录的原生经度，避免整圈布局在180度处折返")
    mean_latitude = sum(math.asin(point[1] / math.sqrt(sum(
        value * value for value in point))) for point in centers) / len(centers)
    if projection == "polar":
        horizontal, vertical = (1, 0, 0), (0, 0, 1)
    else:
        length = math.sqrt(sum(value * value for value in center))
        up = [value / length for value in center]
        longitude = math.atan2(up[0], -up[2])
        horizontal = (math.cos(longitude), 0, math.sin(longitude))
        vertical = (-up[1] * horizontal[2],
                    up[0] * horizontal[2] - up[2] * horizontal[0],
                    up[1] * horizontal[0])

    def project(point, anchor=0):
        if projection == "unwrapped":
            longitude = math.atan2(point[0], -point[2])
            longitude = anchor + math.remainder(longitude - anchor, 2 * math.pi)
            latitude = math.asin(point[1] / math.sqrt(sum(value * value for value in point)))
            return longitude * 200.2 * math.cos(mean_latitude), latitude * 200.2
        delta = [point[axis] - center[axis] for axis in range(3)]
        return (sum(delta[axis] * horizontal[axis] for axis in range(3)),
                sum(delta[axis] * vertical[axis] for axis in range(3)))

    def record_project(point, index):
        return project(point, anchors[index] if anchors else 0)

    polygons, projected = [], [record_project(point, index) for index, point in enumerate(points)]
    for item in source["boxes"]:
        box = item["box"]
        corners = []
        for signs in itertools.product((-1, 1), repeat=3):
            point = [box["Center"][axis] + sum(
                signs[index] * box["HalfSize"][index] * box["Axes"][index][axis]
                for index in range(3)) for axis in range(3)]
            corners.append(record_project(point, item["index"]))
        polygons.append((item["itemId"], hull(corners)))
        projected.extend(corners)
    left, right = min(point[0] for point in projected) - 4, max(point[0] for point in projected) + 4
    bottom, top = min(point[1] for point in projected) - 4, max(point[1] for point in projected) + 4
    scale = min(24, 1280 / (right - left), 900 / (top - bottom))
    width, height = max(860, round((right - left) * scale)), round((top - bottom) * scale) + 145
    image = Image.new("RGB", (width, height), "#101923")
    draw = ImageDraw.Draw(image)
    font_path = Path("C:/Windows/Fonts/msyh.ttc")
    font = ImageFont.truetype(str(font_path), 22) if font_path.exists() else ImageFont.load_default()
    small = ImageFont.truetype(str(font_path), 17) if font_path.exists() else ImageFont.load_default()

    def screen(point):
        return ((point[0] - left) * scale, 95 + (top - point[1]) * scale)

    colors = {2302: "#598fa5", 2303: "#8d9cde", 2308: "#dba76c", 2309: "#b29acc",
              2901: "#d7e5eb", 2104: "#aa8bd3", 2103: "#aa8bd3", 2201: "#6498b5"}
    aliases = {2315: 2302, 2319: 2302, 2304: 2303, 2305: 2303, 2318: 2303,
               2317: 2309, 2902: 2901}
    for item, polygon in polygons:
        draw.polygon([screen(point) for point in polygon],
                     fill=colors.get(aliases.get(item, item), "#9aacba"), outline="#e9eef3")
    indices = {record["index"]: index for index, record in enumerate(records)}
    for index, record in enumerate(records):
        first = screen(record_project(points[index], index))
        if 2001 <= record["itemId"] <= 2003:
            other = indices.get(record["outputObjIdx"])
            if other is not None and 2001 <= records[other]["itemId"] <= 2003:
                color = "#62bbdb" if record["localOffset"]["z"] > .4 else "#e3ad55"
                draw.line((*first, *screen(record_project(points[other], other))), fill=color, width=2)
        elif 2011 <= record["itemId"] <= 2014:
            second = screen(record_project(source["points2"][index], index))
            draw.line((*first, *second), fill="#e8edf2")
    # 箭头直接取当地经纬基和导出Yaw，避免把投影曲线误读成机器姿态倾斜。
    for index, (record, point) in enumerate(zip(records, points)):
        if record.get("recipeId", 0) <= 0:
            continue
        radius = math.sqrt(sum(value * value for value in point))
        latitude = math.asin(point[1] / radius)
        longitude = math.atan2(point[0], -point[2])
        east = (math.cos(longitude), 0, math.sin(longitude))
        north = (-math.sin(latitude) * math.sin(longitude), math.cos(latitude),
                 math.sin(latitude) * math.cos(longitude))
        yaw = math.radians(record["yaw"])
        forward = [math.sin(yaw) * east[axis] + math.cos(yaw) * north[axis]
                   for axis in range(3)]
        tip = [point[axis] + forward[axis] * 1.2 for axis in range(3)]
        draw.line((*screen(record_project(point, index)), *screen(record_project(tip, index))), fill="#243343", width=2)
    draw.text((18, 10), title, font=font, fill="#f5f7fa")
    draw.text((18, 44), subtitle, font=small, fill="#b5c5d4")
    caption = "原生盒的经纬展开投影；离线检查，游戏内铺设尚未实测" if projection == "unwrapped" else "原生建造盒与导出连接投影；离线检查，游戏内铺设尚未实测"
    draw.text((18, height - 35), caption,
              font=small, fill="#b5c5d4")
    image.save(output_path)
    print(output_path)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("--title", default="Native blueprint geometry")
    parser.add_argument("--subtitle", default="World positions and build colliders")
    parser.add_argument("--projection", choices=("tangent", "polar", "unwrapped"), default="tangent")
    arguments = parser.parse_args()
    render(arguments.source, arguments.output, arguments.title, arguments.subtitle, arguments.projection)
