/** 动画用的小型数学工具：缓动、插值与帧率无关的阻尼。 */

export const clamp01 = (v: number): number => (v < 0 ? 0 : v > 1 ? 1 : v);

export const lerp = (a: number, b: number, t: number): number => a + (b - a) * t;

export const easeOutCubic = (t: number): number => 1 - Math.pow(1 - t, 3);

export const easeInCubic = (t: number): number => t * t * t;

export const easeInOutCubic = (t: number): number => (t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2);

/** 带回弹的缓出：落位时略微越过再回到原位，营造机关咬合的手感 */
export const easeOutBack = (t: number, s = 1.4): number => {
  const u = t - 1;
  return 1 + (s + 1) * u * u * u + s * u * u;
};

export const smoothstep = (e0: number, e1: number, x: number): number => {
  const t = clamp01((x - e0) / (e1 - e0));
  return t * t * (3 - 2 * t);
};

/** 帧率无关的指数阻尼：lambda 越大越快趋近目标 */
export const damp = (current: number, target: number, lambda: number, dt: number): number =>
  lerp(current, target, 1 - Math.exp(-lambda * dt));

/** 把角度差规范到 (-π, π]，用于沿最短方向转动 */
export const shortestAngle = (from: number, to: number): number => {
  let d = (to - from) % (Math.PI * 2);
  if (d > Math.PI) d -= Math.PI * 2;
  if (d < -Math.PI) d += Math.PI * 2;
  return d;
};

/** 简单可复现的哈希噪声，用于错开动画与装饰随机量 */
export const hash01 = (n: number): number => {
  const s = Math.sin(n * 127.1 + 311.7) * 43758.5453;
  return s - Math.floor(s);
};

/** 字符串哈希：对象 ID 映射为稳定的伪随机数 */
export const hashString = (text: string): number => {
  let h = 2166136261;
  for (let i = 0; i < text.length; i++) {
    h ^= text.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return (h >>> 0) / 4294967295;
};
