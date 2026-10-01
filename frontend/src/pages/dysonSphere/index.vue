<script setup lang="ts">
import SphereScene from '../../components/dysonSphere/home/sphere-scene.vue'
import {useBuildCycle} from '../../composables/dysonSphere/useBuildCycle'

definePage({ meta: { layout: 'main', title: '首页' } })

// =============== 建造进度 ===============
// 说明：进度跟随本地时间日循环（08:00 为 0%，22:00 为 100%），由 useBuildCycle 定时刷新。
const {progress: buildProgress, progressText: buildProgressText} = useBuildCycle()
</script>

<template>
  <!-- ~ 首页主视觉 -->
  <!-- * 说明：戴森球场景铺满内容区，左侧叠加欢迎标题与建造进度 -->
  <!-- * 交互：文字层与遮罩不拦截指针事件，鼠标交互全部交给场景 -->
  <!-- * 依赖：依赖 SphereScene 组件与 --ds-scene-scrim 遮罩 Token -->
  <!-- * 边界：页面固定为内容区大小，不滚动，避免与场景滚轮缩放冲突；场景背景色与全局 --ds-bg-shell 一致 -->
  <section class="home" aria-labelledby="home-title">
    <SphereScene class="home-scene" :progress="buildProgress" />
    <div class="home-scrim" aria-hidden="true" />

    <div class="home-copy">
      <p class="home-eyebrow">DYSON SPHERE BOX</p>
      <h1 id="home-title" class="home-title">欢迎回来，工程师</h1>
      <p class="home-progress">
        戴森壳建造进度 <strong class="home-progress-value">{{ buildProgressText }}</strong>
      </p>
    </div>
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
  font-weight: 700;
  font-variant-numeric: tabular-nums;
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
