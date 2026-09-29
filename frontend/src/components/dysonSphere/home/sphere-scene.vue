<script setup lang="ts">
import {onBeforeUnmount, onMounted, ref, watch} from 'vue'
import {useThemeMode} from '../../../composables/useThemeMode'
import {createDysonSphere, type DysonSphereController} from '../../../packages/dyson-sphere/src'

interface SphereSceneProps {
  /** 戴森壳建造进度 0~1 */
  readonly progress: number
}

const props = defineProps<SphereSceneProps>()
const containerRef = ref<HTMLDivElement | null>(null)
const {themeMode} = useThemeMode()
let controller: DysonSphereController | null = null

// =============== 场景生命周期 ===============
// 说明：挂载时创建 three.js 场景，卸载时释放 WebGL 资源、注入样式与事件监听。
onMounted(() => {
  if (!containerRef.value) {
    return
  }
  controller = createDysonSphere(containerRef.value, {
    theme: themeMode.value,
    params: {progress: props.progress},
  })
})

onBeforeUnmount(() => {
  controller?.dispose()
  controller = null
})

// =============== 外部状态同步 ===============
// 说明：只读应用主题驱动场景配色，不反向修改全局主题；进度变化时场景播放壳板落位动画。
// 主题切换立即生效：应用自身已有圆形揭示动画，场景再做 400ms 渐变会在揭示圈内出现新旧配色混杂。
watch(themeMode, (mode) => {
  controller?.setTheme(mode, true)
})

watch(() => props.progress, (progress) => {
  controller?.setParams({progress})
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
