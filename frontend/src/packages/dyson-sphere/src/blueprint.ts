/** 原版 DYBP 在主页场景使用的坐标和颜色。颜色是无符号 0xRRGGBBAA。 */
export interface SphereBlueprint {
  type: number;
  gameVersion: string;
  layers: BlueprintLayer[];
  orbits: BlueprintOrbit[];
}

export interface BlueprintOrbit {
  id: number;
  radius: number;
  rotation: number[];
  enabled: boolean;
  visible: boolean;
  hsva: number[];
}

export interface BlueprintLayer {
  id: number;
  radius: number;
  rotation: number[];
  visible: boolean;
  nodes: { id: number; pos: number[]; color: number; proto: number }[];
  frames: { id: number; a: number; b: number; euler: boolean; color: number; proto: number }[];
  shells: { id: number; nodes: number[]; seed: number; color: number; proto: number }[];
  gridMode: number;
  /** 每格四个 RGBA 字节的 Base64，空字符串表示无逐格涂装。 */
  cellColors: string;
}

/** 首页蓝图的显示偏好，独立于游戏蓝图原文。 */
export interface BlueprintAppearance {
  structure: boolean;
  surfaceDetail: boolean;
  orbits: boolean;
}

export const DEFAULT_BLUEPRINT_APPEARANCE: Readonly<BlueprintAppearance> = Object.freeze({
  structure: true, surfaceDetail: true, orbits: true,
});
