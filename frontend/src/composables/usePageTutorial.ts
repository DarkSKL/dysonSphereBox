import { driver, type DriveStep, type Driver } from 'driver.js';
import 'driver.js/dist/driver.css';
import { nextTick, onBeforeUnmount, onDeactivated, ref } from 'vue';

import {
  GetTutorialProgress,
  MarkTutorialCompleted,
  SkipTutorial,
} from '../../wailsjs/go/system/TutorialService';
import { tutorialRegistry } from '../tutorials/registry';
import type { TutorialDefinition, TutorialPageKey, TutorialStep } from '../tutorials/types';

interface StartTutorialOptions {
  readonly ignoreProgress?: boolean;
  readonly delayMs?: number;
}

interface PageTutorialController {
  readonly isTutorialRunning: typeof isTutorialRunning;
  readonly tryAutoStartTutorial: () => Promise<void>;
  readonly restartTutorial: () => Promise<void>;
  readonly destroyTutorial: () => void;
}

const DEFAULT_START_DELAY_MS = 160;
const AUTO_RETRY_DELAY_MS = 900;
const MAX_AUTO_RETRY_COUNT = 40;
const TOUR_STAGE_PADDING = 4;
const TOUR_STAGE_RADIUS = 4;
const isTutorialRunning = ref(false);
let activeDriver: Driver | null = null;
let activePageKey: TutorialPageKey | null = null;
const autoRetryTimers = new Map<TutorialPageKey, number>();
const autoRetryCounts = new Map<TutorialPageKey, number>();

// =============== 教程实例清理 ===============
// 说明：销毁当前 Driver.js 实例，避免路由切换后残留遮罩。
const destroyTutorial = (): void => {
  if (activeDriver?.isActive()) {
    activeDriver.destroy();
  }
  activeDriver = null;
  activePageKey = null;
  isTutorialRunning.value = false;
};

// =============== 等待页面稳定 ===============
// 说明：等待 Vue 渲染和 Naive UI 控件挂载，减少教程启动时找不到目标元素的概率。
const waitForTutorialTargets = async (delayMs: number): Promise<void> => {
  await nextTick();
  await new Promise<void>((resolve) => {
    window.setTimeout(resolve, delayMs);
  });
};

// =============== 步骤过滤 ===============
// 说明：只保留当前 DOM 中存在的 selector；无 selector 的步骤作为页面级说明保留。
const resolveAvailableSteps = (steps: readonly TutorialStep[]): DriveStep[] => {
  const missingSelectors: string[] = [];
  const availableSteps = steps.flatMap<DriveStep>((step) => {
    if (step.selector && !document.querySelector(step.selector)) {
      missingSelectors.push(step.selector);
      return [];
    }

    return [
      {
        element: step.selector,
        popover: {
          title: step.title,
          description: step.description,
          side: step.side,
          align: step.align,
        },
      },
    ];
  });

  if (missingSelectors.length > 0) {
    console.warn('[tutorial] 当前页面缺少部分引导目标：', missingSelectors);
  }

  return availableSteps;
};

// =============== 状态写回 ===============
// 说明：完成或跳过教程后写入数据库；写入失败只记录控制台，避免遮罩无法关闭。
const markTutorialSkipped = async (definition: TutorialDefinition): Promise<void> => {
  try {
    await SkipTutorial(definition.pageKey, definition.version);
  } catch (error) {
    console.warn('[tutorial] 跳过状态写入失败：', error);
  }
};

const markTutorialCompleted = async (definition: TutorialDefinition): Promise<void> => {
  try {
    await MarkTutorialCompleted(definition.pageKey, definition.version);
  } catch (error) {
    console.warn('[tutorial] 完成状态写入失败：', error);
  }
};

// =============== 自动启动重试 ===============
// 说明：全局教程优先展示；页面教程遇到全局教程未结束或当前有教程运行时，短暂重试。
const clearAutoRetry = (pageKey: TutorialPageKey): void => {
  const timer = autoRetryTimers.get(pageKey);
  if (timer !== undefined) {
    window.clearTimeout(timer);
  }
  autoRetryTimers.delete(pageKey);
  autoRetryCounts.delete(pageKey);
};

const isTutorialSettled = async (definition: TutorialDefinition): Promise<boolean> => {
  const progress = await GetTutorialProgress(definition.pageKey, definition.version);
  return progress.completed || progress.skipped;
};

// =============== 页面教程控制 ===============
// 说明：统一控制页面教程的自动展示、手动重看、关闭写库和生命周期销毁。
export const usePageTutorial = (pageKey: TutorialPageKey): PageTutorialController => {
  const definition = tutorialRegistry[pageKey];

  const scheduleAutoRetry = (): void => {
    const currentCount = autoRetryCounts.get(pageKey) ?? 0;
    if (currentCount >= MAX_AUTO_RETRY_COUNT) {
      clearAutoRetry(pageKey);
      return;
    }

    const timer = autoRetryTimers.get(pageKey);
    if (timer !== undefined) {
      window.clearTimeout(timer);
    }

    autoRetryCounts.set(pageKey, currentCount + 1);
    autoRetryTimers.set(pageKey, window.setTimeout(() => {
      void tryAutoStartTutorial();
    }, AUTO_RETRY_DELAY_MS));
  };

  const startTutorial = async (options: StartTutorialOptions = {}): Promise<void> => {
    if (!definition || isTutorialRunning.value) {
      return;
    }

    if (!options.ignoreProgress) {
      if (await isTutorialSettled(definition)) {
        clearAutoRetry(definition.pageKey);
        return;
      }
    }

    await waitForTutorialTargets(options.delayMs ?? DEFAULT_START_DELAY_MS);
    const steps = resolveAvailableSteps(definition.steps);
    if (steps.length === 0) {
      console.warn(`[tutorial] ${definition.pageKey} 没有可用引导目标，已跳过启动。`);
      return;
    }

    clearAutoRetry(definition.pageKey);
    destroyTutorial();
    isTutorialRunning.value = true;
    activePageKey = definition.pageKey;

    const driverObj = driver({
      animate: true,
      allowClose: true,
      allowKeyboardControl: true,
      allowScroll: true,
      disableActiveInteraction: false,
      overlayOpacity: 0.62,
      popoverClass: 'driverjs-theme-dsb',
      stagePadding: TOUR_STAGE_PADDING,
      stageRadius: TOUR_STAGE_RADIUS,
      showButtons: ['previous', 'next', 'close'],
      showProgress: true,
      progressText: '{{current}} / {{total}}',
      nextBtnText: '下一步',
      prevBtnText: '上一步',
      doneBtnText: '完成教程',
      steps,
      onCloseClick: () => {
        void (async () => {
          await markTutorialSkipped(definition);
          destroyTutorial();
        })();
      },
      onDoneClick: () => {
        void (async () => {
          await markTutorialCompleted(definition);
          destroyTutorial();
        })();
      },
      onDestroyed: () => {
        if (activePageKey === definition.pageKey) {
          activeDriver = null;
          activePageKey = null;
          isTutorialRunning.value = false;
        }
      },
    });

    activeDriver = driverObj;
    driverObj.drive();
  };

  const tryAutoStartTutorial = async (): Promise<void> => {
    if (!definition.autoStart) {
      return;
    }
    if (definition.pageKey !== 'global' && !(await isTutorialSettled(tutorialRegistry.global))) {
      scheduleAutoRetry();
      return;
    }
    if (isTutorialRunning.value) {
      scheduleAutoRetry();
      return;
    }

    await startTutorial();
  };

  const restartTutorial = async (): Promise<void> => {
    await startTutorial({ ignoreProgress: true, delayMs: 80 });
  };

  onDeactivated(() => {
    clearAutoRetry(pageKey);
    if (activePageKey === pageKey) {
      destroyTutorial();
    }
  });

  onBeforeUnmount(() => {
    clearAutoRetry(pageKey);
    if (activePageKey === pageKey) {
      destroyTutorial();
    }
  });

  return {
    isTutorialRunning,
    tryAutoStartTutorial,
    restartTutorial,
    destroyTutorial,
  };
};
