import {computed, onBeforeUnmount, onMounted, ref} from 'vue'

// =============== 戴森壳建造周期 ===============
// 说明：首页建造进度跟随本地时间循环：08:00 为 0%，线性建造到 22:00 达到 100%，
// 夜间 22:00 至次日 08:00 线性回落到 0%，形成首尾相接的日循环。
// 边界：只提供展示用进度，不读写后端数据；页面卸载时停止计时器。

const MINUTES_PER_DAY = 24 * 60
const BUILD_START_MINUTE = 8 * 60
const BUILD_PEAK_MINUTE = 22 * 60
const BUILD_DURATION = BUILD_PEAK_MINUTE - BUILD_START_MINUTE
const DISMANTLE_DURATION = MINUTES_PER_DAY - BUILD_DURATION
// 刷新间隔：进度每分钟约变化 0.1%，30 秒刷新足够平滑且不会频繁触发场景更新
const REFRESH_INTERVAL_MS = 30_000

/** 按本地时间计算建造进度 0~1 */
export function resolveBuildProgress(date: Date): number {
  const minute = date.getHours() * 60 + date.getMinutes() + date.getSeconds() / 60
  const sinceStart = (minute - BUILD_START_MINUTE + MINUTES_PER_DAY) % MINUTES_PER_DAY
  if (sinceStart <= BUILD_DURATION) {
    return sinceStart / BUILD_DURATION
  }
  return 1 - (sinceStart - BUILD_DURATION) / DISMANTLE_DURATION
}

export function useBuildCycle() {
  const progress = ref(resolveBuildProgress(new Date()))
  const progressText = computed(() => `${Math.round(progress.value * 100)}%`)
  let timer: number | undefined

  const refresh = (): void => {
    progress.value = resolveBuildProgress(new Date())
  }

  onMounted(() => {
    refresh()
    timer = window.setInterval(refresh, REFRESH_INTERVAL_MS)
  })

  onBeforeUnmount(() => {
    window.clearInterval(timer)
    timer = undefined
  })

  return {progress, progressText}
}
