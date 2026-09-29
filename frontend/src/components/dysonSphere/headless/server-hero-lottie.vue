<script setup lang="ts">
import type lottie from 'lottie-web'
import type {AnimationConfigWithData, AnimationItem} from 'lottie-web'
import {computed, nextTick, onBeforeUnmount, ref, watch} from 'vue'
import serverAnimation from '../../../assets/lottie/Server.json'

interface Props {
  active: boolean
  tone?: 'emerald' | 'blue' | 'amber' | 'zinc'
}

type LottieColor = [number, number, number, number]
type LottieRecord = Record<string, unknown>
type LottiePlayer = typeof lottie

const props = defineProps<Props>()

const animationContainer = ref<HTMLElement | null>(null)
let animation: AnimationItem | null = null
let lottiePlayer: LottiePlayer | null = null
let renderToken = 0

const colorMap: Record<NonNullable<Props['tone']>, LottieColor> = {
  emerald: [0.062, 0.725, 0.506, 1],
  blue: [0.145, 0.388, 0.922, 1],
  amber: [0.851, 0.467, 0.024, 1],
  zinc: [0.443, 0.443, 0.478, 1],
}

const accessibleLabel = computed(() => (props.active ? '无头服务器运行中' : '无头服务器未运行'))
const currentColor = computed<LottieColor>(() => colorMap[props.tone ?? 'zinc'])

// =============== Lottie 配色 ===============
// 说明：克隆 Server.json 并替换静态填充/描边颜色，让 Hero 动画跟随开服状态。
// 边界：仅替换静态 RGBA 色值；关键帧颜色和渐变结构保持原样，避免破坏动画数据。
const cloneAnimationData = (color: LottieColor): unknown => {
  const data = JSON.parse(JSON.stringify(serverAnimation)) as unknown
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
// 说明：运行中循环播放 Server 动画；非运行状态停在首帧，作为静态状态图标。
// 边界：使用动态导入复用轻量 lottie 播放器，卸载时销毁实例避免残留 DOM。
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
    class="h-[84px] w-[84px] overflow-hidden"
    role="img"
    :aria-label="accessibleLabel"
  ></div>
</template>

