import { DysonSphereScene } from './scene';
import type { DysonSphereController, DysonSphereOptions } from './types';

/**
 * 在容器内创建戴森球场景。
 *
 * - 容器需要有确定的宽高（场景随容器尺寸自适应，建议 100% × 360~480px 的横幅）
 * - 画布、tooltip、成就卡片等 DOM 都会插入到容器内；容器会被加上 dyson-root 类
 * - 返回的控制器是场景对外的全部接口，组件卸载时务必调用 dispose()
 *
 * @example
 * const dyson = createDysonSphere(el, { theme: 'dark', achievements: list });
 * dyson.setParams({ progress: 0.5 });
 * dyson.dispose();
 */
export function createDysonSphere(container: HTMLElement, options: DysonSphereOptions = {}): DysonSphereController {
  return new DysonSphereScene(container, options);
}
