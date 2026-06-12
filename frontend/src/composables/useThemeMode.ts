import { computed, ref } from 'vue';
import { GetConfig, SetConfig } from '../../wailsjs/go/system/ConfigurationService';

export type ThemeMode = 'light' | 'dark';

interface ThemeTogglePoint {
  readonly x: number;
  readonly y: number;
}

const THEME_CONFIG_GROUP = 'System';
const THEME_CONFIG_KEY = 'AppTheme';
const DEFAULT_THEME_MODE: ThemeMode = 'light';
const themeMode = ref<ThemeMode>(DEFAULT_THEME_MODE);
const isThemeReady = ref(false);
const isSavingThemeMode = ref(false);
let loadThemeModePromise: Promise<void> | null = null;

// =============== 主题值归一 ===============
// 说明：约束数据库读取值只落到 light/dark，避免历史脏数据影响渲染。
const normalizeThemeMode = (value: string): ThemeMode => {
  return value === 'dark' ? 'dark' : DEFAULT_THEME_MODE;
};

// =============== 根节点主题应用 ===============
// 说明：把主题写入 html dataset 与 color-scheme，供全局 CSS token 和原生控件读取。
const applyThemeMode = (mode: ThemeMode): void => {
  const root = document.documentElement;
  root.dataset.theme = mode;
  root.style.colorScheme = mode;
};

// =============== 圆形扩散动画 ===============
// 说明：使用 View Transitions API 从触发点向外揭示新主题；不支持时由调用方降级为瞬切。
const animateThemeReveal = async (point: ThemeTogglePoint): Promise<void> => {
  const endRadius = Math.hypot(
    Math.max(point.x, window.innerWidth - point.x),
    Math.max(point.y, window.innerHeight - point.y),
  );

  await document.documentElement.animate(
    {
      clipPath: [
        `circle(0px at ${point.x}px ${point.y}px)`,
        `circle(${endRadius}px at ${point.x}px ${point.y}px)`,
      ],
    },
    {
      duration: 460,
      easing: 'cubic-bezier(0.16, 1, 0.3, 1)',
      pseudoElement: '::view-transition-new(root)',
    },
  ).finished;
};

// =============== 主题状态服务 ===============
// 说明：负责从数据库加载主题、写回配置，并暴露布局可用的切换动作。
export const useThemeMode = () => {
  const isDarkMode = computed(() => themeMode.value === 'dark');

  const loadThemeMode = async (): Promise<void> => {
    if (!loadThemeModePromise) {
      loadThemeModePromise = GetConfig(THEME_CONFIG_GROUP, THEME_CONFIG_KEY)
        .then((storedTheme) => {
          themeMode.value = normalizeThemeMode(storedTheme);
          applyThemeMode(themeMode.value);
        })
        .catch(() => {
          themeMode.value = DEFAULT_THEME_MODE;
          applyThemeMode(themeMode.value);
        })
        .finally(() => {
          isThemeReady.value = true;
        });
    }

    await loadThemeModePromise;
  };

  const saveThemeMode = async (mode: ThemeMode): Promise<void> => {
    isSavingThemeMode.value = true;
    try {
      await SetConfig(THEME_CONFIG_GROUP, THEME_CONFIG_KEY, mode);
    } finally {
      isSavingThemeMode.value = false;
    }
  };

  const setThemeMode = async (mode: ThemeMode, point?: ThemeTogglePoint): Promise<void> => {
    if (themeMode.value === mode) {
      return;
    }

    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    const canAnimate = Boolean(document.startViewTransition && point && !reduceMotion);

    if (!canAnimate) {
      themeMode.value = mode;
      applyThemeMode(mode);
      await saveThemeMode(mode);
      return;
    }

    const transition = document.startViewTransition?.(() => {
      themeMode.value = mode;
      applyThemeMode(mode);
    });

    if (!transition || !point) {
      await saveThemeMode(mode);
      return;
    }

    await transition.ready;
    await animateThemeReveal(point);
    await transition.finished;
    await saveThemeMode(mode);
  };

  const toggleThemeMode = async (point?: ThemeTogglePoint): Promise<void> => {
    await setThemeMode(isDarkMode.value ? 'light' : 'dark', point);
  };

  return {
    themeMode,
    isDarkMode,
    isThemeReady,
    isSavingThemeMode,
    loadThemeMode,
    setThemeMode,
    toggleThemeMode,
  };
};
