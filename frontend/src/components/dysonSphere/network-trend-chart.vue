<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {useThemeVars} from 'naive-ui'
import {graphic, use} from 'echarts/core'
import {LineChart} from 'echarts/charts'
import {GraphicComponent, GridComponent, TooltipComponent} from 'echarts/components'
import {CanvasRenderer} from 'echarts/renderers'
import type {EChartsOption} from 'echarts'
import VChart from 'vue-echarts'
import type {contract} from '../../../wailsjs/go/models'
import {formatMbps, mibpsToMbps} from '../../utils/format'

// =============== 图表数据与主题 ===============
// 说明：只接收真实网卡采样；Naive UI 主题变量与全局日夜色调同步。
// 边界：不生成占位采样，不参与后端轮询；实例尺寸与销毁由 vue-echarts 管理。
use([LineChart, GraphicComponent, GridComponent, TooltipComponent, CanvasRenderer])
const props = defineProps<{samples: readonly contract.HeadlessNetworkSample[]}>()
const theme = useThemeVars()

// 说明：Canvas 不解析 CSS 变量；读取实际字体栈并在 Oxanium 加载后更新刻度，避免首帧固定使用回退字体。
const axisFontFamily = ref('')
onMounted(() => {
  const displayFont = getComputedStyle(document.documentElement).getPropertyValue('--ds-font-display').trim()
  void document.fonts.load('400 10px Oxanium').then(() => {
    axisFontFamily.value = displayFont || theme.value.fontFamily
  }).catch(() => {
    axisFontFamily.value = theme.value.fontFamily
  })
})

// =============== 坐标与填充辅助 ===============
// 说明：纵轴上限取 1/2/5×10^n 的整齐刻度；面积填充自上而下渐隐，避免两条曲线叠加时遮挡。
const niceCeil = (value: number): number => {
  const magnitude = 10 ** Math.floor(Math.log10(value))
  const step = [1, 2, 5, 10].find((factor) => factor * magnitude >= value) ?? 10
  return step * magnitude
}
const formatAxisValue = (value: number): string => (value >= 10 || Number.isInteger(value) ? value.toFixed(0) : value.toFixed(1))
const areaGradient = (color: string, opacity: number) => new graphic.LinearGradient(0, 0, 0, 1, [
  {offset: 0, color: withAlpha(color, opacity)},
  {offset: 1, color: withAlpha(color, 0)},
])
// 说明：Naive UI 主题色为 #rrggbb 或 #rrggbbaa，转换为 rgba 以控制渐变透明度；其他格式原样返回。
const withAlpha = (color: string, alpha: number): string => {
  const hex = /^#([0-9a-f]{6})([0-9a-f]{2})?$/i.exec(color)?.[1]
  if (!hex) return color
  const [r, g, b] = [0, 2, 4].map((offset) => Number.parseInt(hex.slice(offset, offset + 2), 16))
  return `rgba(${r}, ${g}, ${b}, ${alpha})`
}

// =============== 网络趋势配置 ===============
// 说明：统一以 Mbps 比较上下行速率，曲线平滑并保持单调避免出现负值过冲；单个采样显示圆点，空数据保留坐标与提示。
const uploadData = computed(() => props.samples.map((sample) => mibpsToMbps(sample.uploadMBps)))
const downloadData = computed(() => props.samples.map((sample) => mibpsToMbps(sample.downloadMBps)))
const option = computed<EChartsOption>(() => {
  const colors = theme.value
  const numericFont = axisFontFamily.value || colors.fontFamily
  const singleSample = props.samples.length === 1
  return {
    animationDuration: 300,
    animationDurationUpdate: 300,
    animationEasingUpdate: 'cubicOut',
    backgroundColor: 'transparent',
    textStyle: {fontFamily: colors.fontFamily, color: colors.textColor3},
    grid: {left: 12, right: 16, top: 30, bottom: 12, containLabel: true},
    tooltip: {
      show: props.samples.length > 0,
      trigger: 'axis',
      confine: true,
      backgroundColor: colors.popoverColor,
      borderColor: colors.borderColor,
      borderWidth: 1,
      padding: [8, 10],
      textStyle: {color: colors.textColor1, fontSize: 12},
      extraCssText: 'border-radius: 4px; box-shadow: none;',
      axisPointer: {type: 'line', lineStyle: {color: colors.textColor3, type: 'dashed'}},
      valueFormatter: (value) => formatMbps(Number(value)),
    },
    xAxis: {
      type: 'category',
      data: props.samples.map((sample) => sample.label),
      boundaryGap: false,
      axisLine: {lineStyle: {color: colors.dividerColor}},
      axisTick: {show: false},
      axisLabel: {color: colors.textColor3, fontFamily: numericFont, fontSize: 10, hideOverlap: true, margin: 10},
      axisPointer: {label: {show: false}},
    },
    yAxis: {
      type: 'value',
      min: 0,
      // 说明：最小量程 1 Mbps，避免空闲时微小波动被放大成满幅曲线。
      max: (extent) => niceCeil(Math.max(1, extent.max * 1.15)),
      splitNumber: 3,
      name: 'Mbps',
      nameTextStyle: {color: colors.textColor3, fontFamily: numericFont, fontSize: 10, align: 'left'},
      nameGap: 12,
      axisLine: {show: false},
      axisTick: {show: false},
      axisLabel: {color: colors.textColor3, fontFamily: numericFont, fontSize: 10, formatter: (value: number) => formatAxisValue(value)},
      splitLine: {lineStyle: {color: colors.dividerColor, type: 'dashed', opacity: 0.6}},
    },
    series: [
      {
        id: 'upload', name: '上行', type: 'line',
        data: uploadData.value,
        smooth: 0.35, smoothMonotone: 'x',
        showSymbol: singleSample, symbol: 'circle', symbolSize: 6,
        lineStyle: {color: colors.primaryColor, width: 1.75, cap: 'round', join: 'round'},
        itemStyle: {color: colors.primaryColor},
        areaStyle: {color: areaGradient(colors.primaryColor, 0.16)},
        emphasis: {focus: 'series', lineStyle: {width: 2.25}},
      },
      {
        id: 'download', name: '下行', type: 'line',
        data: downloadData.value,
        smooth: 0.35, smoothMonotone: 'x',
        showSymbol: singleSample, symbol: 'circle', symbolSize: 6,
        lineStyle: {color: colors.successColor, width: 1.75, cap: 'round', join: 'round'},
        itemStyle: {color: colors.successColor},
        areaStyle: {color: areaGradient(colors.successColor, 0.16)},
        emphasis: {focus: 'series', lineStyle: {width: 2.25}},
      },
    ],
    graphic: {
      elements: [{
        id: 'empty-state', type: 'text', left: 'center', top: 'middle',
        silent: true, invisible: props.samples.length > 0,
        style: {text: '尚无足够样本', fill: colors.textColor3, fontSize: 12, fontFamily: colors.fontFamily},
      }],
    },
  }
})
</script>

<template>
  <!-- ~ 网络趋势图 -->
  <!-- * 说明：使用 ECharts 展示本机上下行速率，提示框与坐标轴跟随日夜主题 -->
  <!-- * 交互：悬停查看对应时间的两组速率，容器尺寸变化时自动调整 -->
  <!-- * 边界：零速率正常显示，缺少采样时不绘制模拟曲线 -->
  <VChart class="network-chart" :option="option" autoresize role="img" aria-label="本机网络上下行速率趋势图" />
</template>

<style scoped lang="scss">
.network-chart { width: 100%; height: 100%; min-width: 0; }
</style>
