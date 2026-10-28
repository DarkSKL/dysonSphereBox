"""戴森球计划建筑 buildCollider 数值目录提取器（开发期验证工具）。

职责：只读游戏安装目录，从 resources.assets 读取 ItemProtoSet / ModelProtoSet 与建筑 prefab 的
Collider 组件，按游戏 PrefabDesc 规则推导每个模型的 buildCollider / buildColliders，输出数值目录 JSON。
使用：
    python -I scripts/factory/extract-build-colliders.py \
        --game-dir "<Steam>/steamapps/common/Dyson Sphere Program" \
        --models helper/blueprint/build/models.json \
        --out helper/blueprint/build/placement/game-colliders-<buildid>.json
边界：
    - UnityPy 只是本脚本的开发依赖（pip install UnityPy），不是产品运行依赖；产品只消费输出的 JSON。
    - 不写游戏目录，不导出网格/贴图/反编译源码，只输出数值与 Resources 路径。
    - 输出中的 source 字段不含个人绝对路径；models 输入路径为相对路径时原样记录，否则只记录文件名。
    - 覆盖范围：models.json 中列出的物品，核对 ItemProto.ModelIndex 与 models.json 是否一致，
      以及这些物品引用的模型的 collider；不对未列出的物品做断言（只列出其 ID 供参考）。

复现的游戏规则（Assembly-CSharp 中的方法名，仅描述行为，不含源码）：
    ItemProto.Preload：prefabDesc = LDB.models.modelArray[ModelIndex].prefabDesc，modelArray 以 ModelProto.ID 为下标。
    ModelProto.Preload：Resources.Load(PrefabPath) 与 Resources.Load(PrefabPath + "-cl")；两者都存在时
        independentCollider = true 且从 -cl prefab 读 collider，否则从主 prefab 读。
    PrefabDesc.ReadPrefab：GetComponentsInChildren<Collider>(includeInactive: true)，不检查 enabled。
    ColliderData.InitFromCollider：usage = isTrigger ? (independent ? Through : Build) : Physics；
        Box：pos = transform.TransformPoint(center)，ext = size * 0.5（不乘缩放），q = transform.rotation。
    buildCollider = 最后一个 Build+Box；数量 n <= 2 时 buildColliders = 全部，n > 2 时 = 前 n-1 个。
"""
import argparse
import hashlib
import json
import math
import os
import re
import struct
import sys

STEAM_APP_ID = 1366540
SCHEMA = "dsp-build-colliders/v1"
USAGE_NAMES = ["Physics", "Build", "Through", "Belt"]


# ---------- float32 精确表示 ----------

def f32(x):
    """f32 返回能 roundtrip 回同一 float32 位模式的最短十进制（以 Python float 承载）。"""
    bits = struct.pack("<f", x)
    exact = struct.unpack("<f", bits)[0]
    if math.isnan(exact) or math.isinf(exact):
        raise ValueError(f"非有限 float32: {x}")
    for digits in range(1, 10):
        candidate = float("%.*g" % (digits, exact))
        if struct.pack("<f", candidate) == bits:
            # json 用 repr 输出，再校验一次输出文本也能 roundtrip
            if struct.pack("<f", float(repr(candidate))) != bits:
                raise ValueError(f"float32 输出无法 roundtrip: {x}")
            return candidate
    raise ValueError(f"9 位有效数字仍无法 roundtrip: {x}")


def f32v(values):
    return [f32(v) for v in values]


# ---------- 原始字节读取（无 type tree 的 MonoBehaviour） ----------

class Reader:
    """Reader 按 Unity 序列化规则顺序读取 MonoBehaviour 原始字节。"""

    def __init__(self, data):
        self.d = data
        self.p = 0

    def align(self):
        self.p = (self.p + 3) & ~3

    def _unpack(self, fmt, size):
        v = struct.unpack_from(fmt, self.d, self.p)[0]
        self.p += size
        return v

    def i32(self):
        return self._unpack("<i", 4)

    def i64(self):
        return self._unpack("<q", 8)

    def f32(self):
        return self._unpack("<f", 4)

    def u8_aligned(self):
        # MonoBehaviour 中 bool / byte 枚举字段各自 4 字节对齐（解析需消费完全部字节，以此校验）
        v = self.d[self.p]
        self.p += 1
        self.align()
        return v

    def string(self):
        n = self.i32()
        s = self.d[self.p:self.p + n].decode("utf-8")
        self.p += n
        self.align()
        return s

    def int_array(self):
        return [self.i32() for _ in range(self.i32())]

    def proto_table_header(self):
        self.i32(); self.i64()  # m_GameObject
        self.u8_aligned()  # m_Enabled
        self.i32(); self.i64()  # m_Script
        self.string()  # m_Name
        table = self.string()  # ProtoTable.TableName
        signature = self.string()  # ProtoTable.Signature
        return table, signature

    def expect_end(self, what):
        if self.p != len(self.d):
            raise ValueError(f"{what} 解析结束于 {self.p}，原始长度 {len(self.d)}，字段布局与游戏版本不符")


def parse_model_proto_set(raw):
    """parse_model_proto_set 解析 ModelProtoSet：Proto(Name, ID, SID) + ModelProto 序列化字段。"""
    r = Reader(raw)
    table, signature = r.proto_table_header()
    int_fields = ["Order", "ObjectType", "RuinType", "RendererType", "RotSymmetry", "HpMax",
                  "HpUpgrade", "HpRecover", "RuinId", "RuinCount", "RuinLifeTime"]
    protos = []
    for _ in range(r.i32()):
        m = {"Name": r.string(), "ID": r.i32(), "SID": r.string(), "OverrideName": r.string()}
        for f in int_fields:
            m[f] = r.i32()
        m["PrefabPath"] = r.string()
        protos.append(m)
    r.expect_end("ModelProtoSet")
    return table, signature, protos


def parse_item_proto_set(raw):
    """parse_item_proto_set 解析 ItemProtoSet，只保留核对所需字段，其余按布局跳过。"""
    r = Reader(raw)
    table, signature = r.proto_table_header()
    protos = []
    for _ in range(r.i32()):
        m = {"Name": r.string(), "ID": r.i32(), "SID": r.string()}
        r.i32()  # Type
        r.i32()  # SubID
        r.string(); r.string()  # MiningFrom, ProduceFrom
        r.i32(); r.i32()  # StackSize, Grade
        r.int_array()  # Upgrades
        r.u8_aligned(); r.u8_aligned()  # IsFluid, IsEntity
        m["CanBuild"] = bool(r.u8_aligned())
        r.u8_aligned()  # BuildInGas
        m["IconPath"] = r.string()
        r.string()  # IconTag
        m["ModelIndex"] = r.i32()
        m["ModelCount"] = r.i32()
        r.i32(); r.i32()  # HpMax, Ability
        r.i64(); r.i64()  # HeatValue, Potential
        r.f32()  # ReactorInc
        r.i32(); r.i32()  # FuelType, CatalystType
        r.u8_aligned()  # AmmoType (byte enum)
        r.i32()  # BombType
        for _f in ("CraftType", "BuildIndex", "BuildMode", "GridIndex", "UnlockKey", "PreTechOverride"):
            r.i32()
        r.u8_aligned()  # Productive
        r.i32(); r.f32(); r.i32()  # MechaMaterialID, DropRate, EnemyDropLevel
        r.f32(); r.f32()  # EnemyDropRange
        r.f32(); r.i32(); r.f32()  # EnemyDropCount, EnemyDropMask, EnemyDropMaskRatio
        r.int_array()  # DescFields
        r.string(); r.string(); r.string()  # Description, iconTagString, iconNameTagString
        protos.append(m)
    r.expect_end("ItemProtoSet")
    return table, signature, protos


# ---------- 变换（Unity 约定，q = x,y,z,w） ----------

def qmul(a, b):
    ax, ay, az, aw = a
    bx, by, bz, bw = b
    return (aw * bx + ax * bw + ay * bz - az * by,
            aw * by - ax * bz + ay * bw + az * bx,
            aw * bz + ax * by - ay * bx + az * bw,
            aw * bw - ax * bx - ay * by - az * bz)


def qrot(q, v):
    x, y, z, w = q
    vx, vy, vz = v
    tx = 2 * (y * vz - z * vy)
    ty = 2 * (z * vx - x * vz)
    tz = 2 * (x * vy - y * vx)
    return (vx + w * tx + (y * tz - z * ty),
            vy + w * ty + (z * tx - x * tz),
            vz + w * tz + (x * ty - y * tx))


def v3(o):
    return (float(o.x), float(o.y), float(o.z))


def q4(o):
    return (float(o.x), float(o.y), float(o.z), float(o.w))


def is_identity(t):
    return (v3(t.m_LocalPosition) == (0.0, 0.0, 0.0) and q4(t.m_LocalRotation) == (0.0, 0.0, 0.0, 1.0)
            and v3(t.m_LocalScale) == (1.0, 1.0, 1.0))


def transform_point(chain, p):
    for t in reversed(chain):
        s = v3(t.m_LocalScale)
        p = qrot(q4(t.m_LocalRotation), (p[0] * s[0], p[1] * s[1], p[2] * s[2]))
        lp = v3(t.m_LocalPosition)
        p = (p[0] + lp[0], p[1] + lp[1], p[2] + lp[2])
    return p


def transform_vector(chain, v):
    for t in reversed(chain):
        s = v3(t.m_LocalScale)
        v = qrot(q4(t.m_LocalRotation), (v[0] * s[0], v[1] * s[1], v[2] * s[2]))
    return v


def world_rotation(chain):
    # 根节点的 rotation 即 localRotation，原样返回以保留精确位模式（含 -0.0）
    q = q4(chain[0].m_LocalRotation)
    for t in chain[1:]:
        q = qmul(q, q4(t.m_LocalRotation))
    return q


def derivation_exact(chain, center):
    """derivation_exact 判断 pos/q 是否无需浮点近似：单位变换，或单节点且 center 与位移均为零。"""
    if all(is_identity(t) for t in chain):
        return True
    zero = (0.0, 0.0, 0.0)
    return len(chain) == 1 and center == zero and v3(chain[0].m_LocalPosition) == zero


def round_f32(x):
    return struct.unpack("<f", struct.pack("<f", x))[0]


# ---------- 资源访问 ----------

class Assets:
    """Assets 封装 Resources 容器查找、prefab 层级遍历与 collider 读取。"""

    def __init__(self, unitypy, data_dir):
        from UnityPy.enums import ClassIDType
        self.T = ClassIDType
        self.collider_types = {ClassIDType.BoxCollider: "Box", ClassIDType.SphereCollider: "Sphere",
                               ClassIDType.CapsuleCollider: "Capsule", ClassIDType.MeshCollider: "Mesh"}
        for name in ("WheelCollider", "TerrainCollider"):
            if hasattr(ClassIDType, name):
                self.collider_types[getattr(ClassIDType, name)] = name.replace("Collider", "")
        # 职责：加载当前版本用于解析 PrefabDesc 的完整 Unity 外部资源集合。
        # 使用：resolve 通过 globalgamemanagers 的 m_Externals 解析 MonoScript 和 prefab 引用。
        # 边界：只读加载原版资源；缺失的可选 sharedassets 文件仍由 UnityPy 自身报出，不能伪造脚本字段。
        resource_files = [
            "globalgamemanagers",
            "globalgamemanagers.assets",
            "resources.assets",
            "sharedassets0.assets",
        ]
        existing_files = [os.path.join(data_dir, name) for name in resource_files
                          if os.path.isfile(os.path.join(data_dir, name))]
        self.env = unitypy.load(*existing_files)
        self.files = {n.replace("\\", "/").split("/")[-1].lower(): f for n, f in self.env.files.items()}
        self.gg = self.files["globalgamemanagers"]
        rm = next(o for o in self.env.objects if o.type == ClassIDType.ResourceManager)
        self.container = {}
        for path, ptr in rm.read_typetree()["m_Container"]:
            self.container.setdefault(path, []).append(ptr)
        self.cache = {}

    def resolve(self, ptr, source_file=None):
        fid, pid = ptr["m_FileID"], ptr["m_PathID"]
        target = source_file if fid == 0 and source_file is not None else self.gg if fid == 0 else self.files[self.gg.externals[fid - 1].path.split("/")[-1].lower()]
        return target.objects[pid]

    def raw_of(self, resource_path):
        return self.resolve(self.container[resource_path.lower()][0]).get_raw_data()

    def game_object(self, resource_path):
        for ptr in self.container.get(resource_path.lower(), []):
            o = self.resolve(ptr)
            if o.type == self.T.GameObject:
                return o
        return None

    def read(self, obj):
        key = (obj.assets_file.name, obj.path_id)
        if key not in self.cache:
            self.cache[key] = obj.read()
        return self.cache[key]

    def walk(self, go_obj):
        """walk 先序遍历 prefab 层级（根，再按 m_Children 顺序），返回节点列表。"""
        go = self.read(go_obj)
        root_tr_obj = None
        for c in go.m_Component:
            o = (c.component if hasattr(c, "component") else c).deref()
            if o.type in (self.T.Transform, self.T.RectTransform):
                root_tr_obj = o
                break
        if root_tr_obj is None:
            raise ValueError("prefab 根节点缺少 Transform")
        nodes = []

        def rec(tr_obj, chain, path):
            tr = self.read(tr_obj)
            g_obj = tr.m_GameObject.deref()
            g = self.read(g_obj)
            node = {"go": g, "active": bool(g.m_IsActive), "chain": chain + [tr],
                    "transformPathId": tr_obj.path_id, "transformFile": tr_obj.assets_file.name,
                    "path": path + [g.m_Name]}
            nodes.append(node)
            for ch in tr.m_Children:
                child = ch.deref()
                rec(child, node["chain"], node["path"])

        rec(root_tr_obj, [], [])
        return nodes

    def transform_pose(self, go_obj, pointer, source_file):
        """transform_pose 返回 PrefabDesc 读取的 Transform.position/rotation。"""
        target = self.resolve(pointer, source_file)
        if target.type not in (self.T.Transform, self.T.RectTransform):
            raise ValueError(f"SlotConfig 引用了非 Transform 对象: {target.type}")
        for node in self.walk(go_obj):
            if node["transformPathId"] != target.path_id or node["transformFile"] != target.assets_file.name:
                continue
            transform = node["chain"][-1]
            return {
                "position": f32v(transform_point(node["chain"], (0.0, 0.0, 0.0))),
                "rotation": f32v(world_rotation(node["chain"])),
                "localScale": f32v(v3(transform.m_LocalScale)),
                "hierarchy": "/".join(node["path"]),
            }
        raise ValueError(f"找不到 SlotConfig 引用的 Transform: {target.path_id}")

    def slot_config(self, go_obj):
        """slot_config 读取 PrefabDesc.ReadPrefab 所需的 SlotConfig 数值。"""
        slot_component = None
        slot_source_file = None
        for node in self.walk(go_obj):
            for c in node["go"].m_Component:
                component = (c.component if hasattr(c, "component") else c).deref()
                if component.type != self.T.MonoBehaviour:
                    continue
                raw = component.get_raw_data()
                file_id, path_id = struct.unpack_from("<iq", raw, 16)
                script = self.resolve({"m_FileID": file_id, "m_PathID": path_id})
                if script.read_typetree().get("m_Name") == "SlotConfig":
                    slot_component = raw
                    slot_source_file = component.assets_file
                    break
            if slot_component is not None:
                break
        if slot_component is None:
            return None

        raw = slot_component
        p = 28
        name_length = struct.unpack_from("<i", raw, p)[0]
        p += 4 + name_length
        p = (p + 3) & ~3
        p += 48  # selectCenter、selectSize 以及 SlotConfig 的 6 个标量字段。

        def pointer_array():
            nonlocal p
            count = struct.unpack_from("<i", raw, p)[0]
            p += 4
            pointers = []
            for _ in range(count):
                file_id, path_id = struct.unpack_from("<iq", raw, p)
                p += 12
                pointers.append({"m_FileID": file_id, "m_PathID": path_id})
            return pointers

        slot_pointers = pointer_array()
        insert_pointers = pointer_array()
        addon_pointers = pointer_array()
        center_count = struct.unpack_from("<i", raw, p)[0]
        p += 4
        addon_centers = []
        for _ in range(center_count):
            addon_centers.append(f32v(struct.unpack_from("<fff", raw, p)))
            p += 12
        if p != len(raw):
            raise ValueError(f"SlotConfig 解析结束于 {p}，原始长度 {len(raw)}，字段布局与游戏版本不符")

        return {
            "portPoses": [self.transform_pose(go_obj, pointer, slot_source_file) for pointer in slot_pointers],
            "slotPoses": [self.transform_pose(go_obj, pointer, slot_source_file) for pointer in insert_pointers],
            "addonAreaColPoses": [self.transform_pose(go_obj, pointer, slot_source_file) for pointer in addon_pointers],
            "addonAreaPoses": [{"position": center} for center in addon_centers],
        }

    def colliders(self, go_obj, independent):
        """colliders 按假定的 GetComponentsInChildren 顺序（先序层级 + m_Component 顺序）返回 collider 记录。"""
        result = []
        for node_index, node in enumerate(self.walk(go_obj)):
            chain = node["chain"]
            identity = all(is_identity(t) for t in chain)
            for comp_index, c in enumerate(node["go"].m_Component):
                o = (c.component if hasattr(c, "component") else c).deref()
                if o.type not in self.collider_types:
                    continue
                data = self.read(o)
                shape = self.collider_types[o.type]
                trigger = bool(getattr(data, "m_IsTrigger", False))
                usage = (2 if independent else 1) if trigger else 0
                e = {"order": len(result), "hierarchy": "/".join(node["path"]), "hierarchyNodeIndex": node_index,
                     "componentIndex": comp_index, "assetPathId": str(o.path_id),
                     "componentType": shape + "Collider", "isTrigger": trigger,
                     "enabled": bool(getattr(data, "m_Enabled", True)), "gameObjectActive": node["active"],
                     "usage": USAGE_NAMES[usage], "transformIdentity": identity}
                if not identity:
                    e["localTRS"] = [{"pos": f32v(v3(t.m_LocalPosition)), "rot": f32v(q4(t.m_LocalRotation)),
                                      "scl": f32v(v3(t.m_LocalScale))} for t in chain]
                if shape == "Box":
                    center, size = v3(data.m_Center), v3(data.m_Size)
                    # 单位变换下 TransformPoint 与 rotation 精确；ext = size * 0.5 在 float32 中精确
                    e["raw"] = {"center": f32v(center), "size": f32v(size)}
                    e["pos"] = f32v(center if identity else transform_point(chain, center))
                    e["ext"] = f32v(round_f32(s * 0.5) for s in size)
                    e["q"] = f32v((0.0, 0.0, 0.0, 1.0) if identity else world_rotation(chain))
                elif shape == "Sphere":
                    center = v3(data.m_Center)
                    e["raw"] = {"center": f32v(center), "radius": f32(float(data.m_Radius))}
                    e["pos"] = f32v(center if identity else transform_point(chain, center))
                    e["radius"] = f32(float(data.m_Radius))
                elif shape == "Capsule":
                    center = v3(data.m_Center)
                    h, rad, d = float(data.m_Height), float(data.m_Radius), int(data.m_Direction)
                    local_ext = [0.0, 0.0, 0.0]
                    if d in (0, 1, 2):
                        local_ext[d] = round_f32(round_f32(h * 0.5) - rad)
                    e["raw"] = {"center": f32v(center), "height": f32(h), "radius": f32(rad), "direction": d}
                    e["pos"] = f32v(center if identity else transform_point(chain, center))
                    e["ext"] = f32v(local_ext if identity else transform_vector(chain, local_ext))
                    e["radius"] = f32(rad)
                else:
                    e["usage"] = "Physics"
                    e["note"] = "InitFromCollider 不处理该形状：idType 为 0，按 Physics/无形状处理"
                # 需要真实变换运算时以 float64 计算再舍入到 float32，可能与 Unity 内部 float32 运算差 1 ulp
                if shape in ("Box", "Sphere"):
                    e["derivationExact"] = derivation_exact(chain, v3(data.m_Center))
                else:
                    e["derivationExact"] = identity or shape == "Mesh"
                result.append(e)
        return result


def build_rules(cols, resource_path):
    """build_rules 复现 PrefabDesc.ReadPrefab 的 buildCollider / buildColliders 选择。"""
    builds = [c for c in cols if c["usage"] == "Build" and c["componentType"] == "BoxCollider"]
    if not builds:
        return None, []
    take = builds if len(builds) <= 2 else builds[:-1]

    def pick(c):
        return {"pos": c["pos"], "ext": c["ext"], "q": c["q"],
                "source": {"resourcePath": resource_path, "hierarchy": c["hierarchy"], "colliderOrder": c["order"],
                           "componentIndex": c["componentIndex"], "assetPathId": c["assetPathId"],
                           "componentEnabled": c["enabled"], "derivationExact": c["derivationExact"]}}

    return pick(builds[-1]), [pick(c) for c in take]


# ---------- 来源信息 ----------

def sha256_of(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def file_fact(game_dir, rel):
    path = os.path.join(game_dir, *rel.split("/"))
    return {"path": "<GameDirectory>/" + rel, "size": os.path.getsize(path), "sha256": sha256_of(path)}


def steam_build_id(game_dir, manifest):
    path = manifest or os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(game_dir))),
                                    f"appmanifest_{STEAM_APP_ID}.acf")
    if not os.path.isfile(path):
        return None, None
    with open(path, encoding="utf-8", errors="replace") as f:
        text = f.read()
    m = re.search(r'"buildid"\s+"(\d+)"', text)
    return (m.group(1) if m else None), f"<SteamLibrary>/steamapps/appmanifest_{STEAM_APP_ID}.acf"


def game_version(game_dir):
    path = os.path.join(game_dir, "Updates", "Versions.txt")
    if not os.path.isfile(path):
        return None
    rows = [ln.strip() for ln in open(path, encoding="utf-8", errors="replace") if ln.strip()]
    return rows[-1].split(",")[0] if rows else None


def display_path(path):
    return path.replace("\\", "/") if not os.path.isabs(path) else os.path.basename(path)


# ---------- 主流程 ----------

def compact_number_arrays(text):
    """compact_number_arrays 把纯数字数组折叠为单行，便于审阅 diff。"""
    num = r"-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?"
    return re.sub(r"\[\s*(%s(?:,\s*%s)*)\s*\]" % (num, num),
                  lambda m: "[" + ", ".join(x.strip() for x in m.group(1).split(",")) + "]", text)


def main():
    ap = argparse.ArgumentParser(description="只读提取戴森球计划建筑 buildCollider 数值目录")
    ap.add_argument("--game-dir", required=True, help="游戏安装目录（含 DSPGAME_Data）")
    ap.add_argument("--models", required=True, help="物品 ID -> 模型 ID 的 models.json")
    ap.add_argument("--out", required=True, help="输出 JSON 路径")
    ap.add_argument("--steam-manifest", help=f"appmanifest_{STEAM_APP_ID}.acf，默认按 Steam 库结构推断")
    args = ap.parse_args()

    try:
        import UnityPy
    except ImportError:
        sys.exit("需要开发依赖 UnityPy：pip install UnityPy")

    data_dir = os.path.join(args.game_dir, "DSPGAME_Data")
    with open(args.models, "rb") as f:
        models_bytes = f.read()
    item_to_model = {int(k): int(v) for k, v in json.loads(models_bytes.decode("utf-8")).items()}

    assets = Assets(UnityPy, data_dir)
    model_table, model_sig, model_protos = parse_model_proto_set(assets.raw_of("prototypes/modelprotoset"))
    item_table, item_sig, item_protos = parse_item_proto_set(assets.raw_of("prototypes/itemprotoset"))
    models_by_id = {m["ID"]: m for m in model_protos}
    items_by_id = {m["ID"]: m for m in item_protos}

    # 物品 -> 模型 核对：ItemProto.ModelIndex 即 ModelProto.ID（modelArray 以 ID 为下标）
    mapping = []
    variant_items = {}
    for item_id, model_id in item_to_model.items():
        ip = items_by_id.get(item_id)
        row = {"item": item_id, "expectedModel": model_id}
        if ip is None:
            row.update(status="itemMissing")
        else:
            row.update(itemSid=ip["SID"], modelIndex=ip["ModelIndex"], modelCount=ip["ModelCount"],
                       canBuild=ip["CanBuild"])
            if ip["ModelIndex"] != model_id:
                row["status"] = "mismatch"
            elif ip["ModelCount"] < 1:
                row["status"] = "invalidModelCount"
            else:
                variants = list(range(ip["ModelIndex"], ip["ModelIndex"] + ip["ModelCount"]))
                row["modelVariants"] = variants
                missing_variants = [variant for variant in variants if variant not in models_by_id]
                if missing_variants:
                    row.update(status="modelMissing", missingVariants=missing_variants)
                else:
                    row["status"] = "match"
                    for variant in variants:
                        variant_items.setdefault(variant, set()).add(item_id)
        mapping.append(row)
    mapping_ok = all(r["status"] == "match" for r in mapping)
    uncovered = sorted(i["ID"] for i in item_protos
                       if i["CanBuild"] and i["ModelIndex"] > 0 and i["ID"] not in item_to_model)

    models = {}
    for model_id in sorted(variant_items):
        items = sorted(variant_items[model_id])
        rows = [r for r in mapping if model_id in r.get("modelVariants", [])]
        entry = {"modelId": model_id, "items": items,
                 "mappingVerified": all(r["status"] == "match" for r in rows)}
        models[str(model_id)] = entry
        mp = models_by_id.get(model_id)
        if mp is None:
            entry["error"] = "ModelProtoSet 中无此模型 ID"
            continue
        prefab_path = mp["PrefabPath"]
        entry["modelProto"] = {"name": mp["Name"], "sid": mp["SID"], "objectType": mp["ObjectType"],
                               "prefabPath": prefab_path}
        entry["prefabPath"] = prefab_path
        go = assets.game_object(prefab_path) if prefab_path else None
        cl = assets.game_object(prefab_path + "-cl") if prefab_path else None
        if go is None:
            entry["error"] = "Resources 容器中找不到 prefab"
            continue
        independent = cl is not None
        source_path = prefab_path + "-cl" if independent else prefab_path
        entry["colliderSource"] = {"resourcePath": source_path, "independentCollider": independent,
                                   "clResourcePath": prefab_path + "-cl", "clPrefabFound": independent}
        cols = assets.colliders(cl if independent else go, independent)
        bc, bcs = build_rules(cols, source_path)
        entry["buildBoxCount"] = sum(1 for c in cols if c["usage"] == "Build" and c["componentType"] == "BoxCollider")
        entry["buildCollider"] = bc
        entry["buildColliders"] = bcs
        entry["colliders"] = cols
        try:
            # PrefabDesc.ReadPrefab 始终从主 prefab 读取 SlotConfig；-cl 只提供独立碰撞盒。
            geometry = assets.slot_config(go)
            if geometry is not None:
                geometry["addonAreaSize"] = [
                    f32v(round_f32(value * 0.5) for value in pose["localScale"])
                    for pose in geometry["addonAreaColPoses"]
                ]
                entry["prefabGeometry"] = geometry
        except (KeyError, ValueError, struct.error) as error:
            entry["prefabGeometryError"] = str(error)

    build_id, manifest_display = steam_build_id(args.game_dir, args.steam_manifest)
    out = {
        "schema": SCHEMA,
        "source": {
            "game": "Dyson Sphere Program",
            "steamAppId": STEAM_APP_ID,
            "steamBuildId": build_id,
            "steamManifest": manifest_display,
            "gameVersion": game_version(args.game_dir),
            "gameVersionSource": "<GameDirectory>/Updates/Versions.txt 最后一行",
            "unityVersion": assets.gg.unity_version,
            "resourcesSha256": None,
            "assemblySha256": None,
            "files": [file_fact(args.game_dir, rel) for rel in
                      ("DSPGAME_Data/resources.assets", "DSPGAME_Data/globalgamemanagers",
                       "DSPGAME_Data/Managed/Assembly-CSharp.dll")],
            "modelsInput": {"path": display_path(args.models), "sha256": hashlib.sha256(models_bytes).hexdigest(),
                            "items": len(item_to_model), "models": len(set(item_to_model.values()))},
            "protoTables": {"Item": {"signature": item_sig, "count": len(item_protos), "table": item_table},
                            "Model": {"signature": model_sig, "count": len(model_protos), "table": model_table}},
            "extractor": "scripts/factory/extract-build-colliders.py",
            "unityPyVersion": UnityPy.__version__,
            "assetsReadOnly": True,
        },
        "numberFormat": "所有小数为能 roundtrip 到原始 float32 位模式的最短十进制；按 float32 解析",
        "mappingVerified": mapping_ok,
        "coverage": {
            "scope": "仅 models.json 中的物品及其 ModelCount 声明的模型变体；未列出的可建造物品不做断言",
            "itemModelMapping": {"checked": len(mapping), "matched": sum(r["status"] == "match" for r in mapping),
                                 "notMatched": [r for r in mapping if r["status"] != "match"],
                                 "modelCountNotOne": [r["item"] for r in mapping if r.get("modelCount", 1) != 1],
                                 "method": "解析 ItemProtoSet 原始字节（解析须恰好消费全部字节），比较 ModelIndex 并核对 ModelCount 覆盖的每个 ModelProto"},
            "buildableItemsNotInModelsInput": uncovered,
            "buildBoxesWithNonIdentityQ": [
                {"model": m["modelId"], "colliderOrder": b["source"]["colliderOrder"], "q": b["q"]}
                for m in models.values() for b in m.get("buildColliders", [])
                if b["q"] != [0.0, 0.0, 0.0, 1.0]],
            "nonRootColliders": [
                {"model": m["modelId"], "hierarchy": c["hierarchy"], "usage": c["usage"]}
                for m in models.values() for c in m.get("colliders", []) if "/" in c["hierarchy"]],
            "componentDisabledBuildBoxes": [
                {"model": m["modelId"], "colliderOrder": b["source"]["colliderOrder"]}
                for m in models.values() for b in m.get("buildColliders", []) if not b["source"]["componentEnabled"]],
        },
        "rules": [
            "ItemProto.Preload: prefabDesc = LDB.models.modelArray[ModelIndex].prefabDesc；modelArray 以 ModelProto.ID 为下标",
            "ModelProto.Preload: 同时存在 PrefabPath 与 PrefabPath+\"-cl\" 时 independentCollider=true，collider 读自 -cl prefab",
            "PrefabDesc.ReadPrefab: GetComponentsInChildren<Collider>(includeInactive: true)，不检查组件 enabled",
            "ColliderData.InitFromCollider: usage = isTrigger ? (independent ? Through : Build) : Physics；"
            "Box pos=TransformPoint(center), ext=size*0.5（不含缩放）, q=transform.rotation",
            "buildCollider = 最后一个 Build+Box；n<=2 时 buildColliders 为全部，n>2 时为前 n-1 个"
            "（最后一个是整体包围盒，其 colliders[] 副本 ext 被改为 0.1，buildCollider 保留原值）",
        ],
        "caveats": [
            "Unity GetComponentsInChildren 的运行时顺序未在游戏内验证；此处假定为先序层级遍历 + m_Component 顺序，"
            "colliderOrder 即该假定顺序。当前模型的 collider 若全在根节点上，则只依赖 m_Component 顺序",
            "静态值不含运行时调整：分拣器/传送带的建造盒在 BuildTool 中按长度/端点拉伸，传送带 Belt usage 由其他路径生成",
            "BepInEx 等 Mod 可在运行时修改 prefab；这里读取的是磁盘上的原版资源",
            "derivationExact=false 的数值以 float64 计算后舍入到 float32，可能与 Unity 的 float32 运算相差 1 ulp",
            "q 按 prefab 中的 float32 原样保留，可能含极小的非单位分量（见 coverage.buildBoxesWithNonIdentityQ）",
        ],
        "models": models,
    }
    for fact in out["source"]["files"]:
        if fact["path"].endswith("resources.assets"):
            out["source"]["resourcesSha256"] = fact["sha256"]
        elif fact["path"].endswith("Assembly-CSharp.dll"):
            out["source"]["assemblySha256"] = fact["sha256"]

    text = compact_number_arrays(json.dumps(out, ensure_ascii=False, indent=1)) + "\n"
    out_dir = os.path.dirname(os.path.abspath(args.out))
    os.makedirs(out_dir, exist_ok=True)
    with open(args.out, "w", encoding="utf-8", newline="\n") as f:
        f.write(text)
    print(f"wrote {args.out}: {len(models)} models, mapping {out['coverage']['itemModelMapping']['matched']}"
          f"/{len(mapping)} matched, mappingVerified={mapping_ok}")
    return 0 if mapping_ok else 2


if __name__ == "__main__":
    sys.exit(main())
