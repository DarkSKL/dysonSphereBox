<script setup lang="ts">
import {onBeforeUnmount, onMounted, ref, watch} from 'vue'
import {useThemeMode} from '../../../composables/useThemeMode'
import {
  createDysonSphere,
  DEFAULT_PARAMS,
  QUALITY_PRESETS,
  type DysonSphereController,
} from '../../../packages/dyson-sphere/src'

interface SphereSceneProps {
  /** 戴森壳建造进度 0~1 */
  readonly progress: number
}

const props = defineProps<SphereSceneProps>()
const containerRef = ref<HTMLDivElement | null>(null)
const {themeMode, loadThemeMode} = useThemeMode()
let controller: DysonSphereController | null = null

// =============== 太阳帆粒子 ===============
// 说明：首页只保留 2 道太阳帆轨道；粒子数随建造进度在画质档位上限的 10%~100% 之间变化。
// 边界：粒子数按 1% 步进取整，避免进度细微变化时频繁重建粒子几何体。
const ORBIT_LAYERS = 2
const MIN_PARTICLE_RATIO = 0.1

const resolveParticleCount = (progress: number): number => {
  const maxCount = QUALITY_PRESETS[DEFAULT_PARAMS.quality].particleCount
  const clamped = Math.min(Math.max(progress, 0), 1)
  const ratio = MIN_PARTICLE_RATIO + (1 - MIN_PARTICLE_RATIO) * clamped
  const step = maxCount / 100
  return Math.round((maxCount * ratio) / step) * step
}

// =============== 场景生命周期 ===============
// 说明：挂载时创建 three.js 场景，卸载时释放 WebGL 资源、注入样式与事件监听。
onMounted(async () => {
  // 主题配置异步读取完成后再创建 WebGL 场景，避免默认日间调色板先初始化并覆盖夜间首屏。
  await loadThemeMode()
  if (!containerRef.value) {
    return
  }
  controller = createDysonSphere(containerRef.value, {
    theme: themeMode.value,
    params: {
      progress: props.progress,
      orbitLayers: ORBIT_LAYERS,
      particleCount: resolveParticleCount(props.progress),
    },
  })
})

onBeforeUnmount(() => {
  controller?.dispose()
  controller = null
})

// =============== 外部状态同步 ===============
// 说明：只读应用主题驱动场景配色，不反向修改全局主题；进度变化时场景播放壳板落位动画并同步粒子数。
// 主题切换立即生效：应用自身已有圆形揭示动画，场景再做 400ms 渐变会在揭示圈内出现新旧配色混杂。
watch(themeMode, (mode) => {
  controller?.setTheme(mode, true)
})

watch(() => props.progress, (progress) => {
  controller?.setParams({progress, particleCount: resolveParticleCount(progress)})
})
</script>

<template>
  <!-- ~ 戴森球场景 -->
  <!-- * 说明：承载 @dyson/sphere 渲染的恒星、戴森壳与太阳帆粒子 -->
  <!-- * 交互：鼠标吸引粒子、点击散开、滚轮缩放、中键拖动旋转，均由场景包内部处理 -->
  <!-- * 依赖：依赖 packages/dyson-sphere 与 useThemeMode -->
  <!-- * 边界：场景会向容器写入 class 与 DOM，因此使用独立内层元素，避免与 Vue 管理的根节点属性冲突 -->
  <div class="sphere-scene">
    <div ref="containerRef" class="sphere-scene-canvas" />
  </div>
</template>

<style scoped lang="scss">
.sphere-scene,
.sphere-scene-canvas {
  width: 100%;
  height: 100%;
}
</style>
