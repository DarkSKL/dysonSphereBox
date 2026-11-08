import { AlgorithmVizScene } from './scene';
import type { AlgorithmVizController, AlgoVizOptions } from './types';

/**
 * 在容器内创建算法可视化场景。
 *
 * - 容器需要有确定的宽高；画布与覆盖层 DOM 会插入容器内，容器会被加上 algoviz-root 类
 * - 返回的控制器是场景对外的全部接口，组件卸载时务必调用 dispose()
 *
 * @example
 * const viz = createAlgorithmViz(el, { theme: 'light', modelUrls });
 * viz.enqueue({ coordinates: 'layout', stage: '布局装配', operations: [...] });
 * viz.dispose();
 */
export function createAlgorithmViz(container: HTMLElement, options: AlgoVizOptions = {}): AlgorithmVizController {
  return new AlgorithmVizScene(container, options);
}
