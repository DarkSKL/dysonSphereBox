<script setup lang="ts">
import {Settings2} from '@lucide/vue'
import {nextTick, ref, watch} from 'vue'
import BlueprintPanel from '../../components/dysonSphere/home/blueprint-panel.vue'
import SphereScene from '../../components/dysonSphere/home/sphere-scene.vue'
import {useBuildCycle} from '../../composables/dysonSphere/useBuildCycle'
import {useHomeSphereBlueprint} from '../../composables/dysonSphere/useHomeSphereBlueprint'

definePage({ meta: { layout: 'main', title: '首页' } })

// =============== 建造进度 ===============
// 说明：进度跟随本地时间日循环（08:00 为 0%，22:00 为 100%），由 useBuildCycle 定时刷新。
const {progress: buildProgress, progressText: buildProgressText} = useBuildCycle()
const scene = ref<InstanceType<typeof SphereScene> | null>(null)
const editorButton = ref<HTMLDivElement | null>(null)
const {appearance, savingAppearance, appearanceError, setPart, editing, restoring, parsing, saving, supported, hasSaved, error, applied, preview, showHidden,
  restore, inspect, discardPreview, toggleHidden, commit, close, setAvailable} = useHomeSphereBlueprint(model => {
  if (!scene.value) throw new Error('戴森球场景尚未就绪')
  scene.value.setBlueprint(model)
})
watch(editing, async open => {
  if (!open) {
    await nextTick()
    editorButton.value?.querySelector('button')?.focus()
  }
})
</script>

<template>
  <!-- ~ 首页主视觉 -->
  <!-- * 说明：戴森球场景铺满内容区，左侧叠加欢迎标题与建造进度 -->
  <!-- * 交互：文字层与遮罩让出场景操作，仅蓝图按钮和预览面板接收输入 -->
  <!-- * 依赖：依赖 SphereScene 组件与 --ds-scene-scrim 遮罩 Token -->
  <!-- * 边界：页面固定为内容区大小，不滚动，避免与场景滚轮缩放冲突；场景背景色与全局 --ds-bg-shell 一致 -->
  <section class="home" aria-label="戴森球主页">
    <SphereScene ref="scene" class="home-scene" :appearance="appearance" :progress="buildProgress" @ready="restore" @available="setAvailable" />
    <div class="home-scrim" aria-hidden="true" />

    <template v-if="!editing">
      <div class="home-copy">
        <p class="home-eyebrow">DYSON SPHERE BOX</p>
        <h1 id="home-title" class="home-title">欢迎回来，工程师</h1>
        <p v-if="applied || (hasSaved && !supported)" class="home-progress">
          {{ applied ? '球壳蓝图 · 设计完成态' : '已保存球壳蓝图' }}
          <strong v-if="applied" class="home-progress-value">{{ applied.layers.length }} 层</strong>
        </p>
        <p v-else class="home-progress">
          戴森壳建造进度 <strong class="home-progress-value">{{ buildProgressText }}</strong>
        </p>
        <p v-if="error && hasSaved" class="home-error" role="alert">{{ error }}</p>
      </div>

      <!-- ~ 球壳设置入口 -->
      <!-- * 说明：在首页内容区右上角提供图标入口 -->
      <!-- * 交互：悬停角落或键盘聚焦时显示图标，激活后打开设置面板 -->
      <!-- * 依赖：蓝图恢复期间由 restoring 禁用入口 -->
      <!-- * 边界：悬停区域局限在角落，其他位置继续响应场景操作 -->
      <div ref="editorButton" class="home-editor-corner">
        <n-tooltip placement="bottom-end">
          <template #trigger>
            <n-button class="home-editor-button" quaternary circle size="small" aria-label="自定义球壳"
              :disabled="restoring" @click="editing = true">
              <template #icon><Settings2 :size="18" aria-hidden="true" /></template>
            </n-button>
          </template>
          自定义球壳
        </n-tooltip>
      </div>
    </template>
    <BlueprintPanel v-else :preview="preview" :parsing="parsing" :saving="saving" :supported="supported"
      :applied="applied" :appearance="appearance" :saving-appearance="savingAppearance"
      :appearance-error="appearanceError" @part="setPart" :has-saved="hasSaved" :show-hidden="showHidden" :error="error" @inspect="inspect" @discard="discardPreview"
      @hidden="toggleHidden" @close="close" @apply="commit()" @reset="commit(true)" />
  </section>
</template>

<style scoped lang="scss">
.home {
  position: absolute;
  inset: 0;
  overflow: hidden;
  container-type: size;
}

.home-scene {
  position: absolute;
  inset: 0;
}

// 左侧遮罩：淡化文字背后的星云与粒子，日夜主题分别使用各自底色。
.home-scrim {
  position: absolute;
  inset: 0;
  background: var(--ds-scene-scrim);
  pointer-events: none;
}

// 文字层：恒星位于画面约 64% 处，左侧留出标题区域。
.home-copy {
  position: absolute;
  top: 50%;
  left: clamp(24px, 5cqw, 72px);
  max-width: min(38cqw, 420px);
  pointer-events: none;
  transform: translateY(-50%);
}

.home-eyebrow {
  margin: 0 0 12px;
  color: var(--ds-text-muted);
  font-size: 11px;
  font-family: var(--ds-font-display);
  font-weight: 600;
  letter-spacing: 0.24em;
}

.home-title {
  margin: 0;
  color: var(--ds-text-primary);
  font-size: clamp(22px, 3.2cqw, 34px);
  font-weight: 700;
  letter-spacing: 0.02em;
  line-height: 1.25;
}

.home-progress {
  margin: 12px 0 0;
  color: var(--ds-text-secondary);
  font-size: 13px;
}

.home-progress-value {
  color: var(--ds-text-primary);
  font-family: var(--ds-font-display);
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.home-editor-corner {
  position: absolute;
  top: 0;
  right: 0;
  padding: 12px;

  .home-editor-button {
    opacity: 0;
  }

  &:hover .home-editor-button,
  .home-editor-button:focus-visible {
    opacity: 1;
  }
}

.home-error {
  margin: 8px 0 0;
  color: var(--ds-error);
  font-size: 12px;
  overflow-wrap: anywhere;
}

// 窄内容区：戴森球占据更多横向空间，文字移到左上角并缩小，避免压住壳体。
@container (max-width: 720px) {
  .home-copy {
    top: 28px;
    max-width: 46cqw;
    transform: none;
  }

  .home-eyebrow {
    margin-bottom: 8px;
    font-size: 10px;
    letter-spacing: 0.18em;
  }

  .home-title {
    font-size: 20px;
  }

  .home-progress {
    margin-top: 8px;
    font-size: 12px;
  }
}

// 矮内容区：垂直居中会贴近上下边缘，改为靠上排布。
@container (max-height: 420px) {
  .home-copy {
    top: 20px;
    transform: none;
  }
}
</style>
