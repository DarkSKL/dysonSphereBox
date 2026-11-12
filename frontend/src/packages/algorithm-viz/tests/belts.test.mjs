import assert from 'node:assert/strict';
import test from 'node:test';
import { Vector3 } from 'three';
import { BeltFlow, BeltGeometry, BELT_WIDTH, BELT_THICKNESS } from '../src/internal/belts.ts';

// 职责：复现长带相位错位及三维拐角缺口；边界：验证路径与截面数学，着色器仍需浏览器渲染验证。
function fixture(points, links = points.map((_, i) => i + 1 < points.length ? i + 1 : -1)) {
  const nodes = points.map(([x, y, z], i) => {
    const pose = { x, y, z, yaw: Math.PI / 2 };
    return { id: `belt-${i}`, kind: 'belt', building: { outputId: links[i] >= 0 ? `belt-${links[i]}` : undefined },
      pose, target: { ...pose }, lift: 0, scale: 1, grow: 1, removing: false };
  });
  const objects = new Map(nodes.map(node => [node.id, node]));
  const inputs = new Map();
  for (const node of nodes) {
    const id = node.building.outputId;
    if (id) { if (!inputs.has(id)) inputs.set(id, new Set()); inputs.get(id).add(node); }
  }
  return { nodes, objects, inputs, flow: new BeltFlow(), geometry: new BeltGeometry() };
}

function solve(f) {
  for (const node of f.nodes) f.flow.invalidate(node);
  f.flow.update(f.objects, f.inputs, new Set());
}

function near(a, b, tolerance = 1e-6) { assert.ok(Math.abs(a - b) < tolerance, `${a} != ${b}`); }
function phaseEqual(a, b) { near(((a - b + 0.5) % 1 + 1) % 1 - 0.5, 0); }
function vectorEqual(a, b) { near(a.distanceTo(b), 0); }

function checkFlow(f) {
  for (const node of f.nodes) {
    const next = f.objects.get(node.building.outputId);
    if (!next) continue;
    const length = Math.hypot(next.target.x - node.target.x, next.target.y - node.target.y, next.target.z - node.target.z);
    const a = f.flow.get(node), b = f.flow.get(next);
    phaseEqual(a.phase + length * a.frequency, b.phase);
    near(a.frequency, b.frequency);
  }
}

function corner(shape, end, across, height) {
  return (end ? shape.end : shape.start).clone()
    .addScaledVector(end ? shape.endSide : shape.startSide, across)
    .addScaledVector(end ? shape.endUp : shape.startUp, height);
}

test('1、1.5 和非整数长度的连续带共享箭头相位，链首补长也不打断相位', () => {
  const f = fixture([[0, 0, 0], [1.5, 0, 0], [3, 0, 0], [4, 0, 0], [5.37, 0, 0]]);
  solve(f); checkFlow(f);
  const a = f.geometry.shape(f.nodes[0], f.objects, f.inputs);
  const flow = f.flow.get(f.nodes[0]);
  phaseEqual(flow.phase - a.head * flow.frequency + a.length * flow.frequency, f.flow.get(f.nodes[1]).phase);
});

test('追加末端时已有带段的箭头不跳相', () => {
  const f = fixture([[0, 0, 0], [1.5, 0, 0], [3, 0, 0]]);
  f.objects.delete(f.nodes[2].id);
  f.flow.invalidate(f.nodes[0]); f.flow.invalidate(f.nodes[1]);
  f.flow.update(f.objects, f.inputs, new Set());
  const before = f.flow.get(f.nodes[0]).phase;
  f.objects.set(f.nodes[2].id, f.nodes[2]);
  f.flow.invalidate(f.nodes[2]); f.flow.invalidate(f.nodes[1]);
  f.flow.update(f.objects, f.inputs, new Set());
  phaseEqual(before, f.flow.get(f.nodes[0]).phase); checkFlow(f);
});

for (const [name, points] of [
  ['水平直角', [[0, 0, 0], [1.5, 0, 0], [1.5, 0, 1.5], [3, 0, 1.5]]],
  ['垂直升降', [[0, 0, 0], [1.5, 0, 0], [1.5, 1, 0], [1.5, 2, 0], [3, 2, 0]]],
  ['垂直下降', [[0, 2, 0], [1.5, 2, 0], [1.5, 0, 0], [3, 0, 0]]],
  ['升降后转向', [[0, 0, 0], [1.5, 0, 0], [1.5, 1, 0], [1.5, 2, 0], [1.5, 2, 1.5]]],
  ['斜坡三维转弯', [[0, 0, 0], [1.5, 1, 0], [1.5, 2, 1.5], [3, 2, 2.2]]],
]) test(`${name}的两侧截面四个顶点完全重合`, () => {
  const f = fixture(points); solve(f); checkFlow(f);
  const shapes = f.nodes.map(node => f.geometry.shape(node, f.objects, f.inputs));
  for (let i = 0; i < shapes.length - 1; i++) {
    for (const across of [-0.5, 0.5]) for (const height of [0, 1]) {
      vectorEqual(corner(shapes[i], true, across, height), corner(shapes[i + 1], false, across, height));
    }
  }
});

test('直角斜接保持两侧带宽，水平转垂直的顶面交于同一条边', () => {
  const f = fixture([[0, 0, 0], [1, 0, 0], [1, 1, 0], [2, 1, 0]]);
  const a = f.geometry.shape(f.nodes[0], f.objects, f.inputs);
  near(a.endSide.length(), BELT_WIDTH);
  vectorEqual(a.endUp, new Vector3(-BELT_THICKNESS, BELT_THICKNESS, 0));
  const g = fixture([[0, 0, 0], [1, 0, 0], [1, 0, 1]]);
  const b = g.geometry.shape(g.nodes[0], g.objects, g.inputs);
  near(b.endSide.x, -BELT_WIDTH); near(b.endSide.z, BELT_WIDTH);
});

test('已有开链闭合成环后，非整数周长仍能连续循环', () => {
  const f = fixture([[0, 0, 0], [1.5, 0, 0], [1.5, 0, 1], [0, 0, 1]]);
  solve(f);
  f.nodes[3].building.outputId = f.nodes[0].id;
  f.inputs.set(f.nodes[0].id, new Set([f.nodes[3]]));
  f.flow.invalidate(f.nodes[3]);
  f.flow.update(f.objects, f.inputs, new Set()); checkFlow(f);
  near(f.flow.get(f.nodes[0]).frequency * 5, Math.round(0.85 * 5));
});

test('汇流分支在同一节点共享相位', () => {
  const f = fixture([[0, 0, 0], [1.5, 0, 0], [3, 0, 0], [1.5, 0, 1]], [1, 2, -1, 1]);
  solve(f); checkFlow(f);
});

test('删除下游后恢复端帽，超过连接上限的引用不绘制长桥', () => {
  const f = fixture([[0, 0, 0], [1.5, 0, 0], [20, 0, 0]]);
  const a = f.geometry.shape(f.nodes[1], f.objects, f.inputs);
  near(a.length, 0.5);
  const far = f.geometry.shape(f.nodes[2], f.objects, f.inputs);
  near(far.head, 0.5);
  f.objects.delete(f.nodes[1].id); f.geometry.beginFrame();
  const b = f.geometry.shape(f.nodes[0], f.objects, f.inputs);
  near(b.length, 1);
});

test('接缝跟随当前显示姿态，升降抬升不会重复叠加；生长只缩短当前段', () => {
  const f = fixture([[0, 0, 0], [1.5, 0, 0], [1.5, 1, 0]]);
  f.nodes[0].lift = 0.1; f.nodes[1].lift = 0.2;
  const a = f.geometry.shape(f.nodes[0], f.objects, f.inputs);
  const b = f.geometry.shape(f.nodes[1], f.objects, f.inputs);
  vectorEqual(a.end, b.start); near(a.end.y, 0.22);
  f.nodes[0].grow = 0.4;
  const growing = f.geometry.shape(f.nodes[0], f.objects, f.inputs);
  near(growing.length, a.length * 0.4);
});

test('三万节点使用迭代路径解析，静止帧不重新写入实例', () => {
  const f = fixture(Array.from({ length: 30000 }, (_, i) => [i * 1.5, 0, 0]));
  solve(f); checkFlow(f);
  const dirty = new Set(); f.flow.update(f.objects, f.inputs, dirty);
  assert.equal(dirty.size, 0);
});

test('水平入口改变时，长垂直链及出口同步更新截面，避免旧横向轴残留', () => {
  const points = [[0, 0, 0], ...Array.from({length:8}, (_, i) => [1.5, i, 0]), [3, 7, 0]];
  const f = fixture(points), dirty = new Set();
  f.geometry.markVerticalRun(f.nodes[0], f.objects, f.inputs, dirty);
  for (const node of f.nodes.slice(1, -1)) assert.ok(dirty.has(node), `${node.id} 未更新`);
});
