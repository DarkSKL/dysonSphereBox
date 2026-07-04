import type { Alignment, Side } from 'driver.js';

export type TutorialPageKey =
  | 'global'
  | 'home'
  | 'headless'
  | 'mod'
  | 'blueprint'
  | 'environment'
  | 'log'
  | 'settings'
  | 'about';

export interface TutorialStep {
  readonly selector?: string;
  readonly title: string;
  readonly description: string;
  readonly side?: Side;
  readonly align?: Alignment;
}

export interface TutorialDefinition {
  readonly pageKey: TutorialPageKey;
  readonly version: number;
  readonly autoStart: boolean;
  readonly steps: readonly TutorialStep[];
}
