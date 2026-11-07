import type { AlgoVizBatch, AlgoVizBuilding, AlgoVizOperation } from './types';

/**
 * 演示数据：模拟一次生成算法的完整过程，用于独立调试与文档示例。
 *
 * 阶段：布局装配（临时对象）=> 候选回退与平移 => 连接物料路线 => 补齐供电与物流塔
 * => 提交正式结果（切换到区域坐标，全部对象落定）。
 */
export function createDemoBatches(): AlgoVizBatch[] {
  const batches: AlgoVizBatch[] = [];
  const all = new Map<string, AlgoVizBuilding>();
  const add = (ops: AlgoVizOperation[], id: string, building: AlgoVizBuilding): void => {
    all.set(id, building);
    ops.push({ type: 'add', id, building });
  };
  const rows = [
    { y: 0, item: 2304, group: 0, count: 6 },
    { y: 11, item: 2304, group: 1, count: 5 },
    { y: 22, item: 2302, group: 2, count: 7 },
  ];

  // 1. 布局装配：逐台放置生产设备
  const layout: AlgoVizOperation[] = [];
  rows.forEach((row, r) => {
    for (let i = 0; i < row.count; i++) {
      add(layout, `m-${r}-${i}`, { itemId: row.item, x: i * 4, y: row.y, z: 0, yaw: 0, group: row.group, temporary: true });
    }
  });
  batches.push({ operations: layout, coordinates: 'layout', stage: '布局装配' });

  // 2. 候选回退：第二行末尾两台撤下，整行右移一格重新尝试
  const retry: AlgoVizOperation[] = [];
  for (let i = 3; i < rows[1].count; i++) retry.push({ type: 'remove', id: `m-1-${i}` });
  for (let i = 0; i < 3; i++) {
    const building = { ...(all.get(`m-1-${i}`) as AlgoVizBuilding), x: i * 4 + 2 };
    all.set(`m-1-${i}`, building);
    retry.push({ type: 'replace', id: `m-1-${i}`, building });
  }
  for (let i = 3; i < rows[1].count; i++) {
    all.delete(`m-1-${i}`);
    add(retry, `m-1-${i}b`, { itemId: 2304, x: i * 4 + 2, y: 11, z: 0, yaw: 0, group: 1, temporary: true });
  }
  batches.push({ operations: retry, stage: '回退候选并重新排布' });

  // 3. 物料路线：每行两侧各一条传送带，分拣器连接带与设备
  const routing: AlgoVizOperation[] = [];
  rows.forEach((row, r) => {
    const shift = r === 1 ? 2 : 0;
    for (const side of [-1, 1]) {
      const by = row.y + side * 2.5;
      const length = row.count * 4 + 2;
      for (let i = 0; i < length; i++) {
        const forward = side < 0;
        const x = (forward ? i : length - 1 - i) - 2 + shift;
        const id = `b-${r}-${side}-${i}`;
        add(routing, id, {
          itemId: r === 2 ? 2002 : 2001,
          x,
          y: by,
          z: 0,
          yaw: forward ? 90 : 270,
          group: row.group,
          temporary: true,
          outputId: i < length - 1 ? `b-${r}-${side}-${i + 1}` : undefined,
        });
      }
      for (let i = 0; i < row.count; i++) {
        const mx = i * 4 + shift;
        add(routing, `s-${r}-${side}-${i}`, {
          itemId: 2012,
          x: mx,
          y: by,
          z: 0,
          x2: mx,
          y2: row.y + side * 1.4,
          z2: 0,
          yaw: side < 0 ? 0 : 180,
          group: row.group,
          temporary: true,
        });
      }
    }
  });
  batches.push({ operations: routing, stage: '连接物料路线' });

  // 4. 供电与物流：感应塔穿插在设备之间，物流塔居于左侧
  const assembly: AlgoVizOperation[] = [];
  rows.forEach((row, r) => {
    for (let i = 0; i < row.count; i += 2) add(assembly, `p-${r}-${i}`, { itemId: 2201, x: i * 4 + 2 + (r === 1 ? 2 : 0), y: row.y + 4.2, z: 0, yaw: 0, temporary: true });
  });
  add(assembly, 'station', { itemId: 2103, x: -10, y: 11, z: 0, yaw: 0, temporary: true });
  add(assembly, 'storage', { itemId: 2101, x: -10, y: 22, z: 0, yaw: 0, temporary: true });
  batches.push({ operations: assembly, stage: '补齐供电与物流塔' });

  // 5. 提交正式结果：切换到区域坐标（整体平移），全部对象落定
  const final: AlgoVizOperation[] = [];
  let index = 0;
  for (const [id, building] of all) {
    final.push({
      type: 'replace',
      id,
      building: {
        ...building,
        x: building.x + 6,
        y: building.y - 4,
        x2: building.x2 === undefined ? undefined : building.x2 + 6,
        y2: building.y2 === undefined ? undefined : building.y2 - 4,
        temporary: false,
        areaIndex: 0,
        index: index++,
      },
    });
  }
  batches.push({ operations: final, coordinates: 'area', areas: [{ index: 0, parentIndex: -1, anchorX: 0, anchorY: 0, width: 80, height: 40 }], stage: '提交正式结果' });
  return batches;
}
