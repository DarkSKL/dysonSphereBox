<script setup lang="ts">
import type lottie from 'lottie-web'
import type {AnimationConfigWithData, AnimationItem} from 'lottie-web'
import {computed, nextTick, onBeforeUnmount, ref, watch} from 'vue'
import pulseAnimation from '../../../assets/lottie/Pulse.json'

interface Props {
  active: boolean
  tone?: 'emerald' | 'amber' | 'zinc'
}

type LottieColor = [number, number, number, number]
type LottieRecord = Record<string, unknown>
type LottiePlayer = typeof lottie

const props = defineProps<Props>()

const animationContainer = ref<HTMLElement | null>(null)
let animation: AnimationItem | null = null
let lottiePlayer: LottiePlayer | null = null
let renderToken = 0

const activeColor: LottieColor = [0.062, 0.725, 0.506, 1]
const pendingColor: LottieColor = [0.851, 0.467, 0.024, 1]
const idleColor: LottieColor = [0.443, 0.443, 0.478, 1]

const accessibleLabel = computed(() => (props.active ? '服务运行中' : '服务未启动'))
const currentColor = computed<LottieColor>(() => {
  if (props.tone === 'amber') {
    return pendingColor
  }
  if (props.tone === 'emerald') {
    return activeColor
  }

  return idleColor
})

// =============== Lottie 配色 ===============
// 说明：运行时克隆动画 JSON 并替换填充/描边颜色，避免修改源动画文件。
// 边界：当前 Pulse 动画使用静态颜色；若后续换成关键帧渐变动画，会保留无法安全识别的结构。
const cloneAnimationData = (color: LottieColor): unknown => {
  const data = JSON.parse(JSON.stringify(pulseAnimation)) as unknown
  replaceLottieColor(data, color)

  return data
}

const replaceLottieColor = (value: unknown, color: LottieColor): void => {
  if (Array.isArray(value)) {
    value.forEach((item) => replaceLottieColor(item, color))
    return
  }
  if (!isRecord(value)) {
    return
  }

  if ((value.ty === 'fl' || value.ty === 'st') && isRecord(value.c)) {
    const colorNode = value.c
    if (colorNode.a === 0 && isColorTuple(colorNode.k)) {
      colorNode.k = color
    }
  }

  Object.values(value).forEach((child) => replaceLottieColor(child, color))
}

const isRecord = (value: unknown): value is LottieRecord => {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

const isColorTuple = (value: unknown): value is LottieColor => {
  return Array.isArray(value)
    && value.length === 4
    && value.every((item) => typeof item === 'number')
}

// =============== 动画生命周期 ===============
// 说明：根据服务状态重新创建动画；启动时循环播放，未启动时停在第一帧。
// 边界：容器未挂载时延迟到下一个渲染周期，卸载时销毁 lottie 实例释放 DOM 资源。
const renderAnimation = async (): Promise<void> => {
  const currentToken = renderToken + 1
  renderToken = currentToken
  await nextTick()
  if (!animationContainer.value) {
    return
  }

  if (!lottiePlayer) {
    const module = await import('lottie-web/build/player/lottie_light')
    lottiePlayer = module.default
  }
  if (currentToken !== renderToken || !animationContainer.value) {
    return
  }

  animation?.destroy()
  const config: AnimationConfigWithData = {
    container: animationContainer.value,
    renderer: 'svg',
    loop: props.active,
    autoplay: props.active,
    animationData: cloneAnimationData(currentColor.value),
    rendererSettings: {
      preserveAspectRatio: 'xMidYMid meet',
    },
  }

  animation = lottiePlayer.loadAnimation(config)
  animation.setSubframe(false)
  if (!props.active) {
    animation.goToAndStop(0, true)
  }
}

watch(
  () => [props.active, props.tone] as const,
  () => {
    void renderAnimation()
  },
  {immediate: true},
)

onBeforeUnmount(() => {
  renderToken += 1
  animation?.destroy()
  animation = null
})
</script>

<template>
  <div
    ref="animationContainer"
    class="h-9 w-9 overflow-hidden"
    role="img"
    :aria-label="accessibleLabel"
  ></div>
</template>

