import { Color, MathUtils, Quaternion, Vector3 } from 'three';
import type { BlueprintLayer } from '../../blueprint';

/** 游戏中的位置使用左手系，所有场景对象在层局部空间统一翻转 Z。 */
export function layerRotation(q: number[]): Quaternion {
  if (q.length !== 4 || !q.every(Number.isFinite) || Math.hypot(...q) < 1e-6) {
    throw new Error('蓝图轨道的旋转无效');
  }
  return new Quaternion(-q[0], -q[1], q[2], q[3]).normalize();
}

export function objectColor(packed: number, fallback: number): Color {
  // 原版对象 alpha 为零表示使用原型颜色；其余值表示涂色强度。
  return new Color(fallback).lerp(new Color(packed >>> 8), (packed & 255) / 255);
}

export interface FrameArc {
  frame: BlueprintLayer['frames'][number];
  points: Vector3[];
}

const TAU = Math.PI * 2;
export const MAX_ARC = 0.045;

function sampleFrame(a: Vector3, b: Vector3, euler: boolean): Vector3[] {
  const angle = a.angleTo(b);
  if (angle < 1e-8) throw new Error('框架的两端位置重合');
  const latA = Math.asin(MathUtils.clamp(a.y, -1, 1));
  const latB = Math.asin(MathUtils.clamp(b.y, -1, 1));
  // Maths.Elerp 在极点沿用另一端的经度，并始终沿最短经度差插值。
  let lonA = Math.atan2(-a.z, a.x);
  let lonB = Math.atan2(-b.z, b.x);
  if (Math.abs(a.x) < 1e-6 && Math.abs(a.z) < 1e-6) lonA = lonB;
  if (Math.abs(b.x) < 1e-6 && Math.abs(b.z) < 1e-6) lonB = lonA;
  let delta = MathUtils.euclideanModulo(lonB - lonA, TAU);
  if (delta > Math.PI) delta -= TAU;
  const nearestEquator = MathUtils.clamp(0, Math.min(latA, latB), Math.max(latA, latB));
  const span = euler ? Math.hypot(latB - latA, delta * Math.cos(nearestEquator)) : angle;
  const count = Math.max(1, Math.ceil(span / MAX_ARC));
  const points = [a];
  const tangent = b.clone().addScaledVector(a, -a.dot(b));
  if (!euler && tangent.lengthSq() < 1e-18) throw new Error('对跖节点无法确定框架大圆');
  tangent.normalize();
  for (let i = 1; i < count; i++) {
    const t = i / count;
    if (euler) {
      const lat = MathUtils.lerp(latA, latB, t);
      const lon = lonA + t * delta;
      points.push(new Vector3(Math.cos(lat) * Math.cos(lon), Math.sin(lat), -Math.cos(lat) * Math.sin(lon)));
    } else {
      points.push(a.clone().multiplyScalar(Math.cos(angle * t)).addScaledVector(tangent, Math.sin(angle * t)));
    }
  }
  points.push(b);
  return points;
}

export function frameKey(a: number, b: number): string {
  return a < b ? `${a}/${b}` : `${b}/${a}`;
}

export function layerTopology(layer: BlueprintLayer) {
  const nodes = new Map<number, Vector3>();
  for (const node of layer.nodes) {
    if (node.pos.length !== 3 || !node.pos.every(Number.isFinite) || nodes.has(node.id)) {
      throw new Error('蓝图节点坐标或编号无效');
    }
    const v = new Vector3(node.pos[0], node.pos[1], -node.pos[2]);
    if (v.lengthSq() < 1e-12) throw new Error('蓝图节点位于球心');
    nodes.set(node.id, v.normalize());
  }
  const frames = new Map<string, FrameArc>();
  for (const frame of layer.frames) {
    const a = nodes.get(frame.a);
    const b = nodes.get(frame.b);
    const key = frameKey(frame.a, frame.b);
    if (!a || !b || frames.has(key)) throw new Error(`框架 ${frame.id} 的节点引用无效`);
    frames.set(key, { frame, points: sampleFrame(a, b, frame.euler) });
  }
  return { nodes, frames };
}
