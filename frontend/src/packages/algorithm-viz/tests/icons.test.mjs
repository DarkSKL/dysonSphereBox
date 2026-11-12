import assert from 'node:assert/strict';
import test from 'node:test';
import { IconSlots, iconRect } from '../src/internal/icon-slots.ts';

// 职责：复现图标槽位压实后的持有关系，以及精灵图 UV 的翻转与内缩；边界：不覆盖着色器，需在浏览器渲染验证。
class FakePool {
  constructor() { this.items = []; }
  get size() { return this.items.length; }
  alloc() { this.items.push(null); return this.items.length - 1; }
  // 与 InstancePool 同一契约：末端实例补位，返回其原槽号；释放末槽或无效槽不产生迁移
  release(slot) {
    if (!Number.isInteger(slot) || slot < 0 || slot >= this.items.length) return undefined;
    const last = this.items.length - 1;
    this.items.pop();
    return slot !== last ? last : undefined;
  }
}

const holder = (name) => ({ name, iconSlot: -1 });

test('释放中间槽位后，末端持有者迁移到空槽并保持一一对应', () => {
  const pool = new FakePool();
  const slots = new IconSlots(pool);
  const [a, b, c, d] = ['a', 'b', 'c', 'd'].map(holder);
  for (const owner of [a, b, c, d]) slots.alloc(owner);
  assert.deepEqual([a, b, c, d].map(o => o.iconSlot), [0, 1, 2, 3]);

  slots.release(b);
  assert.equal(b.iconSlot, -1);
  assert.equal(d.iconSlot, 1, '末端持有者补到被释放的槽位');
  assert.deepEqual([a.iconSlot, c.iconSlot, d.iconSlot], [0, 2, 1]);
  assert.equal(pool.size, 3);

  // 再释放迁移后的持有者，不能误伤其他对象
  slots.release(d);
  assert.deepEqual([a.iconSlot, c.iconSlot], [0, 1]);
  assert.equal(pool.size, 2);
});

test('释放末槽、重复释放与重复分配都不改变其他持有者', () => {
  const pool = new FakePool();
  const slots = new IconSlots(pool);
  const [a, b] = ['a', 'b'].map(holder);
  slots.alloc(a);
  slots.alloc(b);
  assert.equal(slots.alloc(a), 0, '已持有槽位时分配保持原槽号');
  assert.equal(pool.size, 2);

  slots.release(b);
  slots.release(b);
  assert.equal(a.iconSlot, 0);
  assert.equal(pool.size, 1);
});

test('reset 让全部持有者回到无图标状态', () => {
  const slots = new IconSlots(new FakePool());
  const owners = ['a', 'b', 'c'].map(holder);
  for (const owner of owners) slots.alloc(owner);
  slots.reset();
  assert.deepEqual(owners.map(o => o.iconSlot), [-1, -1, -1]);
});

test('精灵图 UV：左上角为原点，v 轴翻转，四周内缩半个像素', () => {
  const sheet = { width: 1472, height: 1472, cell: 64 };
  const [u0, v0, du, dv] = iconRect(sheet, { x: 0, y: 0 }, 0.5);
  assert.ok(Math.abs(u0 - 0.5 / 1472) < 1e-9);
  assert.ok(Math.abs(v0 - (1 - 63.5 / 1472)) < 1e-9, '第一行位于贴图顶部，即 v 接近 1');
  assert.ok(Math.abs(du - 63 / 1472) < 1e-9 && Math.abs(dv - 63 / 1472) < 1e-9);

  const [, lowV] = iconRect(sheet, { x: 64, y: 1408 }, 0);
  assert.ok(Math.abs(lowV - 0) < 1e-9, '最后一行位于贴图底部，即 v 为 0');
});

test('传入 out 时原地写入并返回同一数组，逐帧调用不分配', () => {
  const sheet = { width: 1472, height: 1472, cell: 64 };
  const out = [9, 9, 9, 9];
  const result = iconRect(sheet, { x: 128, y: 64 }, 0.5, out);
  assert.equal(result, out);
  assert.deepEqual(out, iconRect(sheet, { x: 128, y: 64 }, 0.5));
});
