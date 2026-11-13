/**
 * @dyson/algorithm-viz —— 蓝图生成算法可视化渲染包（纪念碑谷式 2.5D 等距场景）。
 *
 * 公开 API：
 * - createAlgorithmViz(container, options) → AlgorithmVizController
 * - 类型：AlgoVizOperation / AlgoVizBatch / AlgoVizBuilding / ...
 * - 目录：ALGORITHM_VIZ_MODEL_KEYS / resolveModelKey / buildingKind
 * - 常量：ALGORITHM_VIZ_CSS / GRID_METERS / HEIGHT_SCALE / SPEED_MIN / SPEED_MAX
 * - 演示：createDemoBatches
 *
 * 渲染实现（render/、internal/、scene.ts）不属于公开 API，可能随时调整。
 * 公开声明不引用 three 的类型，使用方无需安装 @types/three。
 */

export { createAlgorithmViz } from './create';
export { ALGORITHM_VIZ_MODEL_KEYS, buildingKind, resolveModelKey } from './catalog';
export { ALGORITHM_VIZ_CSS } from './styles';
export { GRID_METERS, HEIGHT_SCALE, SPEED_MAX, SPEED_MIN } from './defaults';
export { createDemoBatches } from './mock';

export type { BuildingKind } from './catalog';
export type {
  AlgorithmVizController,
  AlgoVizArea,
  AlgoVizBatch,
  AlgoVizBuilding,
  AlgoVizBuildingEvent,
  AlgoVizCoordinates,
  AlgoVizIcon,
  AlgoVizIconGroup,
  AlgoVizIconSheet,
  AlgoVizIconVisibility,
  AlgoVizOperation,
  AlgoVizOptions,
  AlgoVizProgress,
  AlgoVizQuality,
  AlgoVizSelection,
  AlgoVizStats,
  AlgoVizTheme,
} from './types';
