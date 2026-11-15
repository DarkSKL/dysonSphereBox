import { Vector3 } from 'three';
import type { VizObject } from './objects';

// =============== 传送带路径与接缝 ===============
// 说明：按真实连接统一箭头相位，并为两侧带段提供完全相同的接缝截面。
// 边界：只计算前端表现，不修改蓝图节点；断开的远距离引用不绘制连接。
export const BELT_THICKNESS = 0.2;
export const BELT_WIDTH = 0.66;
const LINK_MAX = 6;
const FREQUENCY = 0.85;
const EPSILON = 1e-5;
const UP = new Vector3(0, 1, 0);

type Objects = ReadonlyMap<string, VizObject>;
type Inputs = ReadonlyMap<string, ReadonlySet<VizObject>>;

interface Flow {
  readonly phase: number;
  readonly frequency: number;
}

interface Frame {
  readonly direction: Vector3;
  readonly side: Vector3;
  readonly up: Vector3;
}

interface Section {
  readonly side: Vector3;
  readonly up: Vector3;
}

export interface BeltShape {
  readonly start: Vector3;
  readonly end: Vector3;
  readonly startSide: Vector3;
  readonly startUp: Vector3;
  readonly endSide: Vector3;
  readonly endUp: Vector3;
  readonly direction: Vector3;
  readonly side: Vector3;
  readonly up: Vector3;
  readonly length: number;
  readonly head: number;
}

const wrap = (phase: number): number => ((phase % 1) + 1) % 1;
const distance = (a: VizObject, b: VizObject): number => Math.hypot(b.target.x - a.target.x, b.target.y - a.target.y, b.target.z - a.target.z);
const position = (obj: VizObject): Vector3 => new Vector3(obj.pose.x, obj.pose.y + obj.lift + 0.02, obj.pose.z);
const heading = (obj: VizObject): Vector3 => new Vector3(Math.sin(Math.PI - obj.pose.yaw), 0, -Math.cos(Math.PI - obj.pose.yaw));

/** 只接受仍存在的带节点；目标姿态决定连接有效性，避免移动过程中长带突然断开。 */
function output(obj: VizObject, objects: Objects): VizObject | undefined {
  const next = obj.building.outputId ? objects.get(obj.building.outputId) : undefined;
  if (!next || next.kind !== 'belt' || next.removing || next === obj) return;
  const length = distance(obj, next);
  return length > EPSILON && length <= LINK_MAX ? next : undefined;
}

/** 相位按下游约束反推；已有末端保留相位，新末端从已有上游续接，追加节点不会让整条带跳相。 */
export class BeltFlow {
  private readonly flows = new Map<VizObject, Flow>();
  private readonly pending = new Set<VizObject>();

  invalidate(obj: VizObject): void {
    if (obj.kind === 'belt') this.pending.add(obj);
  }

  delete(obj: VizObject): void {
    this.flows.delete(obj);
    this.pending.delete(obj);
  }

  clear(): void {
    this.flows.clear();
    this.pending.clear();
  }

  get(obj: VizObject): Flow {
    return this.flows.get(obj) ?? { phase: 0, frequency: FREQUENCY };
  }

  /** 仅向相位实际变化的上游传播；静止帧不遍历全场，闭环用整数个箭头周期消除闭合缝。 */
  update(objects: Objects, inputs: Inputs, dirty: Set<VizObject>): void {
    while (this.pending.size) {
      const first = this.pending.values().next().value as VizObject;
      if (objects.get(first.id) !== first || first.removing) {
        this.delete(first);
        continue;
      }
      const path: VizObject[] = [];
      const visited = new Map<VizObject, number>();
      let node = first;
      let anchor: Flow;
      for (;;) {
        const cycle = visited.get(node);
        if (cycle !== undefined) {
          const loop = path.slice(cycle);
          const length = loop.reduce((sum, item) => sum + distance(item, output(item, objects)!), 0);
          anchor = { phase: this.get(node).phase, frequency: Math.max(1, Math.round(length * FREQUENCY)) / length };
          break;
        }
        const cached = this.flows.get(node);
        visited.set(node, path.length);
        path.push(node);
        const next = output(node, objects);
        if (next) {
          node = next;
          continue;
        }
        let phase = cached?.phase ?? 0;
        if (!cached) for (const previous of inputs.get(node.id) ?? []) {
          const prior = this.flows.get(previous);
          if (prior && output(previous, objects) === node) {
            phase = wrap(prior.phase + distance(previous, node) * FREQUENCY);
            break;
          }
        }
        anchor = { phase, frequency: FREQUENCY };
        path.pop();
        this.assign(node, anchor, inputs, visited, dirty);
        break;
      }
      for (let i = path.length - 1; i >= 0; i--) {
        const item = path[i];
        const next = output(item, objects)!;
        anchor = { phase: wrap(anchor.phase - distance(item, next) * anchor.frequency), frequency: anchor.frequency };
        this.assign(item, anchor, inputs, visited, dirty);
      }
    }
  }

  private assign(obj: VizObject, flow: Flow, inputs: Inputs, path: ReadonlyMap<VizObject, number>, dirty: Set<VizObject>): void {
    const before = this.flows.get(obj);
    this.flows.set(obj, flow);
    this.pending.delete(obj);
    if (before && Math.abs(wrap(before.phase - flow.phase + 0.5) - 0.5) < EPSILON && Math.abs(before.frequency - flow.frequency) < EPSILON) return;
    dirty.add(obj);
    for (const previous of inputs.get(obj.id) ?? []) if (!path.has(previous)) this.pending.add(previous);
  }
}

/** 一帧内共享截面缓存；转弯、升降两侧使用同一组顶点，不靠加长方盒掩盖接缝。 */
export class BeltGeometry {
  private frames = new Map<VizObject, Frame>();
  private sections = new Map<VizObject, Section>();

  beginFrame(): void {
    this.frames.clear();
    this.sections.clear();
  }

  shape(obj: VizObject, objects: Objects, inputs: Inputs): BeltShape {
    const frame = this.frame(obj, objects, inputs);
    const next = output(obj, objects);
    const start = position(obj);
    const end = next ? position(next) : start.clone().addScaledVector(frame.direction, 0.5);
    const head = this.previous(obj, objects, inputs) ? 0 : 0.5;
    start.addScaledVector(frame.direction, -head);
    const a = this.section(obj, objects, inputs);
    const b = next ? this.section(next, objects, inputs) : a;
    const grow = Math.max(obj.grow, 1e-3);
    end.lerpVectors(start, end, grow);
    return {
      start, end, startSide: a.side, startUp: a.up,
      endSide: a.side.clone().lerp(b.side, grow), endUp: a.up.clone().lerp(b.up, grow),
      direction: frame.direction, side: frame.side, up: frame.up,
      length: Math.max(start.distanceTo(end), EPSILON), head,
    };
  }

  span(obj: VizObject, objects: Objects): number {
    const next = output(obj, objects);
    return next ? distance(obj, next) : 0.5;
  }

  /** 纯竖直段的横向轴继承水平邻居，入口变化时整段升降链及端帽都需同步重绘。 */
  markVerticalRun(obj: VizObject, objects: Objects, inputs: Inputs, dirty: Set<VizObject>): void {
    if (obj.kind !== 'belt') return;
    const vertical = (node: VizObject): boolean => {
      const next = output(node, objects);
      return Boolean(next && Math.hypot(next.target.x - node.target.x, next.target.z - node.target.z) < EPSILON);
    };
    const pending = new Set([obj]);
    for (const node of pending) {
      for (const previous of inputs.get(node.id) ?? []) {
        dirty.add(previous);
        if (vertical(previous)) pending.add(previous);
      }
      const next = output(node, objects);
      if (next) {
        dirty.add(next);
        if (vertical(next)) pending.add(next);
        else if (vertical(node)) for (const previous of inputs.get(next.id) ?? []) dirty.add(previous);
      }
    }
  }

  private direction(obj: VizObject, objects: Objects): Vector3 {
    const next = output(obj, objects);
    const direction = next ? position(next).sub(position(obj)) : heading(obj);
    return direction.lengthSq() > EPSILON * EPSILON ? direction.normalize() : heading(obj);
  }

  /** 汇流时优先延续最直的输入；所有接入带仍共享该节点的截面。 */
  private previous(obj: VizObject, objects: Objects, inputs: Inputs): VizObject | undefined {
    const direction = this.direction(obj, objects);
    let best: VizObject | undefined;
    let score = -Infinity;
    for (const candidate of inputs.get(obj.id) ?? []) {
      if (candidate.removing || objects.get(candidate.id) !== candidate || output(candidate, objects) !== obj) continue;
      const dot = this.direction(candidate, objects).dot(direction);
      if (dot > score) { best = candidate; score = dot; }
    }
    return best;
  }

  private frame(obj: VizObject, objects: Objects, inputs: Inputs): Frame {
    const cached = this.frames.get(obj);
    if (cached) return cached;
    const direction = this.direction(obj, objects);
    const side = new Vector3().crossVectors(direction, UP);
    if (side.lengthSq() < EPSILON) {
      // 纯竖直带没有世界朝上的横向轴，沿连接查找水平段，避免升降交界突然扭转 90 度。
      const visited = new Set<VizObject>([obj]);
      let node = this.previous(obj, objects, inputs);
      while (node && !visited.has(node)) {
        visited.add(node);
        side.crossVectors(this.direction(node, objects), UP);
        if (side.lengthSq() >= EPSILON) break;
        node = this.previous(node, objects, inputs);
      }
      if (side.lengthSq() < EPSILON) {
        node = output(obj, objects);
        while (node && !visited.has(node)) {
          visited.add(node);
          side.crossVectors(this.direction(node, objects), UP);
          if (side.lengthSq() >= EPSILON) break;
          node = output(node, objects);
        }
      }
      if (side.lengthSq() < EPSILON) side.set(0, 0, 1);
    }
    side.normalize();
    const frame = { direction, side, up: new Vector3().crossVectors(side, direction).normalize() };
    this.frames.set(obj, frame);
    return frame;
  }

  private section(obj: VizObject, objects: Objects, inputs: Inputs): Section {
    const cached = this.sections.get(obj);
    if (cached) return cached;
    const current = this.frame(obj, objects, inputs);
    const previous = this.previous(obj, objects, inputs);
    const incoming = previous ? this.frame(previous, objects, inputs) : current;
    const section = {
      side: this.miter(incoming.side, current.side).multiplyScalar(BELT_WIDTH * obj.scale),
      up: this.miter(incoming.up, current.up).multiplyScalar(BELT_THICKNESS),
    };
    this.sections.set(obj, section);
    return section;
  }

  /** 斜接向量在两侧带面上的投影均为原宽度；接近折返时限制伸长，避免尖刺。 */
  private miter(a: Vector3, b: Vector3): Vector3 {
    const denominator = 1 + a.dot(b);
    return denominator > 0.25 ? a.clone().add(b).multiplyScalar(1 / denominator) : b.clone();
  }
}
