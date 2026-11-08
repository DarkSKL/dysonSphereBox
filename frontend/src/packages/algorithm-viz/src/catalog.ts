/**
 * 建筑目录：物品 ID / 蓝图模型序号 → 模型文件键、渲染类别与占位尺寸。
 *
 * 模型序号与物品 ID 的对应来自生成器内置的 models.json 与游戏碰撞表（游戏版本 0.10.35）。
 * 模型键即 GLB 文件名去掉扩展名，例如 assembler-1 对应 assembler-1.glb。
 */

/** 渲染类别：决定配色、动画方式与占位体形状 */
export type BuildingKind =
  | 'belt'
  | 'sorter'
  | 'splitter'
  | 'machine'
  | 'station'
  | 'storage'
  | 'power'
  | 'extractor'
  | 'defense'
  | 'other';

/** 模型序号 → 模型键 */
const MODEL_INDEX_KEYS: Readonly<Record<number, string>> = {
  35: 'belt-1', 36: 'belt-2', 37: 'belt-3',
  38: 'splitter-a', 39: 'splitter-b', 40: 'splitter-c',
  41: 'inserter-1', 42: 'inserter-2', 43: 'inserter-3', 483: 'inserter-4',
  44: 'tesla-coil', 45: 'energy-exchanger', 46: 'accumulator', 47: 'accumulator',
  49: 'logistic-station', 50: 'interstellar-logistic-station',
  51: 'storage-1', 52: 'storage-2', 53: 'wind-turbine', 54: 'fuel-plant', 55: 'solar-panel',
  56: 'fusion-reactor', 57: 'mining-drill', 60: 'water-pump', 61: 'oil-extractor',
  62: 'smelter', 63: 'oil-refinery', 64: 'chemical-plant',
  65: 'assembler-1', 66: 'assembler-2', 67: 'assembler-3', 456: 'assembler-4',
  68: 'orbital-substation', 69: 'hadron-collider', 70: 'lab', 455: 'lab-2',
  71: 'charging-pole', 72: 'em-rail-ejector', 73: 'ray-receiver', 74: 'vertical-launching-silo',
  117: 'orbital-collector', 118: 'fusion-power-station', 119: 'fractionator', 120: 'spray-coater',
  121: 'storage-tank', 194: 'smelter-2', 457: 'smelter-3', 208: 'monitor',
  255: 'geothermal-power-station', 256: 'mining-drill-mk2', 257: 'piler', 371: 'delivery-machine',
  373: 'turret-laser', 374: 'turret-gauss', 375: 'turret-cannon', 376: 'chemical-plant-2',
  402: 'turret-shield', 403: 'turret-signal', 407: 'turret-missile', 408: 'turret-plasma',
  422: 'turret-disturb', 453: 'battle-base', 482: 'turret-plasma-ground', 664: 'lighthouse',
};

/** 物品 ID → 默认模型序号（与生成器 models.json 一致） */
const ITEM_MODEL_INDEX: Readonly<Record<number, number>> = {
  2001: 35, 2002: 36, 2003: 37, 2011: 41, 2012: 42, 2013: 43, 2014: 483, 2020: 38, 2030: 208, 2040: 257,
  2101: 51, 2102: 52, 2103: 49, 2104: 50, 2105: 117, 2106: 121, 2107: 371,
  2201: 44, 2202: 71, 2203: 53, 2204: 54, 2205: 55, 2206: 46, 2207: 46, 2208: 73, 2209: 45, 2210: 56,
  2211: 118, 2212: 68, 2213: 255,
  2301: 57, 2302: 62, 2303: 65, 2304: 66, 2305: 67, 2306: 60, 2307: 61, 2308: 63, 2309: 64, 2310: 69,
  2311: 72, 2312: 74, 2313: 120, 2314: 119, 2315: 194, 2316: 256, 2317: 376, 2318: 456, 2319: 457,
  2401: 664, 2901: 70, 2902: 455,
  3001: 374, 3002: 373, 3003: 375, 3004: 408, 3005: 407, 3006: 422, 3007: 403, 3008: 402, 3009: 453, 3010: 482,
};

/**
 * 全部可能用到的模型键（去重）。使用方据此准备 modelUrls，
 * 例如只打包这些 GLB，排除无人机、火箭等非建筑模型。
 */
export const ALGORITHM_VIZ_MODEL_KEYS: readonly string[] = Array.from(new Set(Object.values(MODEL_INDEX_KEYS)));

/** 传送带没有可用网格（GLB 为空节点），由场景程序化绘制 */
const PROCEDURAL_KEYS = new Set(['belt-1', 'belt-2', 'belt-3']);

/** 解析建筑对应的模型键；无法识别时返回 null（使用占位体） */
export function resolveModelKey(itemId: number, modelIndex?: number): string | null {
  if (modelIndex !== undefined && MODEL_INDEX_KEYS[modelIndex]) return MODEL_INDEX_KEYS[modelIndex];
  const fallback = ITEM_MODEL_INDEX[itemId];
  return fallback !== undefined ? MODEL_INDEX_KEYS[fallback] ?? null : null;
}

/** 模型键是否由场景程序化绘制（不加载 GLB） */
export function isProceduralModel(key: string | null): boolean {
  return key !== null && PROCEDURAL_KEYS.has(key);
}

/** 按物品 ID 归类 */
export function buildingKind(itemId: number): BuildingKind {
  if (itemId >= 2001 && itemId <= 2003) return 'belt';
  if (itemId >= 2011 && itemId <= 2014) return 'sorter';
  if (itemId === 2020 || itemId === 2030 || itemId === 2040) return 'splitter';
  if (itemId === 2103 || itemId === 2104 || itemId === 2105 || itemId === 2107) return 'station';
  if (itemId === 2101 || itemId === 2102 || itemId === 2106) return 'storage';
  if (itemId >= 2201 && itemId <= 2213) return 'power';
  if (itemId === 2301 || itemId === 2306 || itemId === 2307 || itemId === 2316) return 'extractor';
  if ((itemId >= 2302 && itemId <= 2319) || itemId === 2901 || itemId === 2902) return 'machine';
  if (itemId >= 3001 && itemId <= 3010) return 'defense';
  return 'other';
}

/** 占位体尺寸（蓝图格）：宽、深、高。模型加载前或缺失时使用。 */
export function placeholderSize(itemId: number): readonly [number, number, number] {
  switch (itemId) {
    case 2104: return [7, 7, 6];
    case 2103: return [6, 6, 4.5];
    case 2107: return [2.4, 2.4, 2.6];
    case 2101: case 2102: return [2.4, 2.4, 2];
    case 2106: return [2.6, 2.6, 2.4];
    case 2201: case 2202: return [0.7, 0.7, 4];
    case 2020: case 2030: case 2040: return [1.4, 1.4, 1.4];
  }
  switch (buildingKind(itemId)) {
    case 'machine': return [3, 3, 3];
    case 'station': return [5, 5, 4];
    case 'power': return [2.6, 2.6, 3];
    case 'extractor': return [3, 3, 2.6];
    case 'defense': return [2.4, 2.4, 2.6];
    case 'sorter': return [0.6, 1.4, 0.8];
    default: return [2, 2, 2];
  }
}
