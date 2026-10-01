<script setup lang="ts">
import type {ResourceCardTone} from '../../../types/environment'

interface ResourceCardProps {
  readonly name: string
  readonly meta?: string
  readonly status?: string
  readonly tone?: ResourceCardTone
  readonly operating?: boolean
}

withDefaults(defineProps<ResourceCardProps>(), {
  meta: '',
  status: '',
  tone: 'default',
  operating: false,
})
</script>

<template>
  <!-- ~ 资源卡片 -->
  <!-- * 说明：正方形卡片，左上角图标，底部为名称、元信息与状态点 -->
  <!-- * 交互：点击、右键与拖拽事件透传到根按钮，由页面决定具体动作 -->
  <!-- * 依赖：图标通过默认插槽传入；tone 决定边框与状态点颜色 -->
  <!-- * 边界：只负责展示，不调用任何后端接口 -->
  <button type="button" class="resource-card" :class="`is-${tone}`" :aria-busy="operating">
    <span class="resource-card-icon">
      <n-spin :show="operating" size="small">
        <slot />
      </n-spin>
    </span>
    <span class="resource-card-name">{{ name }}</span>
    <span class="resource-card-meta">
      <span class="resource-card-meta-text">{{ meta }}</span>
      <em v-if="status" class="resource-card-status">
        <i class="resource-card-dot" aria-hidden="true"></i>
        {{ status }}
      </em>
    </span>
  </button>
</template>

<style scoped lang="scss">
// 注册为 <angle> 后渐变起始角才能被关键帧插值，否则只会在首尾两帧之间跳变
@property --resource-ring-angle {
  syntax: '<angle>';
  inherits: false;
  initial-value: 0deg;
}

@keyframes resource-ring-spin {
  to {
    --resource-ring-angle: 360deg;
  }
}

.resource-card {
  --card-accent: var(--ds-text-muted);
  position: relative;
  display: flex;
  min-width: 0;
  aspect-ratio: 1;
  flex-direction: column;
  border: 1px solid var(--ds-border-default);
  border-radius: 4px;
  background: var(--ds-bg-panel);
  padding: 12px 10px 10px;
  color: var(--card-accent);
  text-align: left;
  transition:
    border-color 160ms ease,
    background-color 160ms ease,
    box-shadow 160ms ease,
    transform 120ms ease;

  // 环绕渐变描边：仅启用态显示；覆盖在 1px 边框上，用遮罩挖空内容区只留边框环，
  // 锥形渐变的起始角持续旋转，使颜色沿边框流动
  // 边界：环宽取整数像素，小数宽度在圆角与缩放下会出现抗锯齿断点；渐变首尾同色保证旋转一周无接缝
  &::before {
    position: absolute;
    inset: -1px;
    padding: 2px;
    border-radius: inherit;
    background: conic-gradient(
      from var(--resource-ring-angle),
      var(--ds-res-on-ring-a),
      var(--ds-res-on-ring-b) 25%,
      var(--ds-res-on-ring-c) 50%,
      var(--ds-res-on-ring-b) 75%,
      var(--ds-res-on-ring-a)
    );
    content: '';
    opacity: 0;
    pointer-events: none;
    transition: opacity 160ms ease;
    -webkit-mask:
      linear-gradient(#000 0 0) content-box,
      linear-gradient(#000 0 0);
    -webkit-mask-composite: xor;
    mask:
      linear-gradient(#000 0 0) content-box,
      linear-gradient(#000 0 0);
    mask-composite: exclude;
  }

  &:hover {
    border-color: var(--ds-border-strong);
    background: var(--ds-hover-bg);
  }

  &:active {
    transform: translateY(1px);
  }

  &:focus-visible {
    outline: 2px solid var(--ds-focus-ring);
    outline-offset: 2px;
  }

  &[aria-disabled='true'] {
    cursor: not-allowed;
    opacity: 0.6;

    &:active {
      transform: none;
    }
  }

  &.is-dragging {
    opacity: 0.42;
  }

  &.is-drop-target {
    border-color: var(--ds-info);
    background: color-mix(in srgb, var(--ds-info) 12%, var(--ds-bg-panel));
  }

  // 说明：启用态保持纯色底，只用流动的环绕描边、状态标签和图标强调色区分；色值来自 --ds-res-* token
  &.is-enabled {
    --card-accent: var(--ds-res-on);
    border-color: transparent;
    background: var(--ds-res-on-surface);
    box-shadow: var(--ds-res-on-glow);

    &::before {
      opacity: 1;
      animation: resource-ring-spin var(--ds-res-on-ring-duration) linear infinite;
    }

    // 悬停时环更亮、流动更快
    &:hover {
      border-color: transparent;
      background: var(--ds-res-on-surface);

      &::before {
        opacity: 1;
        animation-duration: calc(var(--ds-res-on-ring-duration) / 2);
      }
    }

    .resource-card-status {
      background: var(--ds-res-on-soft);
      color: var(--ds-res-on);
    }

    .resource-card-dot {
      box-shadow: var(--ds-res-on-dot-glow);
    }
  }

  // 说明：关闭态（未启用 / 已禁用）降低图标与状态的存在感，只保留中性描边
  &.is-default,
  &.is-disabled {
    --card-accent: var(--ds-res-off);
    border-color: var(--ds-res-off-border);

    // 悬停时预示可启用：描边转为启用强调色
    &:hover:not([aria-disabled='true']) {
      border-color: var(--ds-res-on-hover-border);
    }

    .resource-card-status {
      background: var(--ds-res-off-soft);
      color: var(--ds-res-off);
    }
  }

  // 说明：已禁用比未启用多一道虚线描边，表示“曾启用、已从游戏侧移除”
  &.is-disabled {
    border-style: dashed;
  }

  &.is-missing {
    --card-accent: var(--ds-warning);
    border-color: var(--ds-warning-border);
  }

  &.is-conflict,
  &.is-unsupported {
    --card-accent: var(--ds-error);
    border-color: var(--ds-error-border);
  }

  &.is-folder {
    --card-accent: var(--ds-warning);
  }

  &.is-save,
  &.is-market {
    --card-accent: var(--ds-info);
  }

  &.is-market {
    border-color: color-mix(in srgb, var(--ds-info) 38%, var(--ds-border-default));
  }
}

.resource-card-icon {
  display: inline-flex;
  margin-bottom: auto;
}

.resource-card-name {
  display: block;
  overflow: hidden;
  margin-bottom: 6px;
  color: var(--ds-text-primary);
  font-size: 12px;
  line-height: 1.3;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.resource-card-meta {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  color: var(--ds-text-muted);
  font-size: 11px;
  line-height: 1.2;
}

.resource-card-meta-text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.resource-card-status {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 4px;
  border-radius: 999px;
  padding: 1px 6px 1px 5px;
  font-style: normal;
  white-space: nowrap;
}

.resource-card-dot {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: var(--card-accent);
}

// 关闭态状态点为空心圆，与启用态实心点在无色彩感知时也能区分
.is-default .resource-card-dot,
.is-disabled .resource-card-dot {
  border: 1.5px solid var(--ds-res-off);
  background: transparent;
}
</style>
