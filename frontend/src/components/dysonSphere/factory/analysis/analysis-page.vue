<script setup lang="ts">
// =============== 量化分析图表页外壳 ===============
// 说明：统一图表区与可选工具栏，不额外展示结论标题、文字说明或来源底注。
// 使用：各子页把图表放入默认插槽，控件放入 controls 插槽，必要图例放入 legend 插槽。
// 边界：只负责排版，不读取统计数据；图表区高度由父级分配，内容超出时在图表区内滚动。
defineProps<{scroll?: boolean}>()
</script>

<template>
  <!-- ~ 图表页 -->
  <!-- * 说明：图表填满可用空间，仅在有控件或必要图例时展示工具栏 -->
  <!-- * 边界：scroll 为 true 时图表区纵向滚动，行数多的图表不压缩行高 -->
  <section class="analysis-page">
    <div v-if="$slots.controls || $slots.legend" class="page-toolbar">
      <div v-if="$slots.legend" class="page-legend"><slot name="legend" /></div>
      <div v-if="$slots.controls" class="page-controls"><slot name="controls" /></div>
    </div>
    <div class="page-body" :class="{'is-scroll': scroll}"><slot /></div>
  </section>
</template>

<style scoped lang="scss">
.analysis-page { display: flex; height: 100%; min-height: 0; flex-direction: column; }
.page-toolbar { display: flex; min-width: 0; flex: 0 0 auto; flex-wrap: wrap; align-items: center; gap: 4px 12px; padding: 10px 14px 4px; }
.page-legend { display: flex; min-width: 0; flex-wrap: wrap; align-items: center; gap: 4px 12px; color: var(--ds-text-muted); font-size: 11px; line-height: 1.5; }
.page-controls { display: flex; flex: 0 0 auto; flex-wrap: wrap; justify-content: flex-end; align-items: center; gap: 4px; margin-left: auto; }
.page-body { position: relative; min-height: 0; flex: 1; }
.page-body.is-scroll { overflow-x: hidden; overflow-y: auto; scrollbar-width: thin; }
</style>
