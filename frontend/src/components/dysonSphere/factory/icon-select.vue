<script setup lang="ts">
import {h, type VNodeChild} from 'vue'
import type {SelectOption} from 'naive-ui'
import ItemIcon from './item-icon.vue'

// =============== 图标选择栏 ===============
// 职责：在 Naive 选择框的选中项与下拉选项左侧显示游戏物品图标，并以括角卡片作为外框；使用：量化工作台的设备、配方、物品等选择。
// 边界：只负责展示，value / options 透传；选项的 icon 为图鉴中文名，缺省回退到 label，查不到时由 ItemIcon 显示通用图标；
//       icon 为空字符串表示该选项不显示图标（如「不喷涂」）。
export interface IconSelectOption {label: string; value: string | number; icon?: string; disabled?: boolean}
const props = withDefaults(defineProps<{options: IconSelectOption[]; size?: 'tiny' | 'small'; iconSize?: number}>(), {size: 'tiny', iconSize: 16})
const value = defineModel<string | number | null>('value')

const iconOf = (option: SelectOption): string => {
  const icon = (option as unknown as IconSelectOption).icon
  return icon === undefined ? String(option.label ?? '') : icon
}
// iconSize 为 0 时整列不显示图标（外部已有图标的场景，如目标卡片）。
const renderRow = (option: SelectOption, size: number): VNodeChild => h('span', {class: 'qf-select-row'}, [
  props.iconSize <= 0 ? null : iconOf(option) ? h(ItemIcon, {name: iconOf(option), size}) : h('span', {class: 'qf-select-blank', style: {width: `${size}px`}}),
  h('span', {class: 'qf-select-text'}, String(option.label ?? '')),
])
const renderLabel = (option: SelectOption): VNodeChild => renderRow(option, props.iconSize > 0 ? props.iconSize + 2 : 16)
const renderTag = ({option}: {option: SelectOption}): VNodeChild => renderRow(option, props.iconSize)
</script>

<template>
  <!-- ~ 图标选择栏 -->
  <!-- * 说明：选中项与选项均为「图标 + 名称」，外框使用括角卡片，悬停、聚焦与展开时括线生长 -->
  <!-- * 依赖：ItemIcon 按名称复用图鉴精灵 -->
  <!-- * 边界：其余属性（aria-label、disabled、filterable 等）透传给 n-select -->
  <n-select v-model:value="value" class="qf-select ds-bracket-hover" :size="size" :options="options as unknown as SelectOption[]" :render-label="renderLabel" :render-tag="renderTag" :consistent-menu-width="false" />
</template>

<style scoped lang="scss">
// 括角取代整圈描边：静止时底色区分，悬停 / 聚焦 / 展开时括线出现。
.qf-select { --bracket-size: 8px; --bracket-color: var(--ds-brand); }
.qf-select :deep(.n-base-selection) { --n-border: 1px solid transparent !important; --n-border-hover: 1px solid transparent !important; --n-border-focus: 1px solid transparent !important; --n-border-active: 1px solid transparent !important; --n-box-shadow-focus: none !important; --n-box-shadow-active: none !important; --n-border-radius: 2px !important; --n-color: var(--ds-hover-bg) !important; --n-color-active: var(--ds-active-bg) !important; --n-padding-single-left: 6px !important; }
.qf-select:has(.n-base-selection--active)::after { --bracket-len: var(--bracket-size); }
.qf-select :deep(.n-base-selection-input__content), .qf-select :deep(.n-base-selection-label__render-label) { display: flex; min-width: 0; align-items: center; }
:deep(.qf-select-row) { display: inline-flex; min-width: 0; max-width: 100%; align-items: center; gap: 6px; }
:deep(.qf-select-text) { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
:deep(.qf-select-blank) { display: inline-block; flex: 0 0 auto; }
</style>

<style lang="scss">
// 下拉菜单渲染在 body 下，选项行样式需全局声明（类名带前缀，不影响其它选择框）。
// 边界：类名不得使用 icon-select 等前缀——蓝图预览会按文本特征删除第三方编辑器注入的样式（见 viewer-panel 的 isBlueprintEditorGlobalStyle），
//       命中后本组件样式会在再次进入页面时被误删。
.qf-select-row { display: inline-flex; min-width: 0; max-width: 100%; align-items: center; gap: 6px; }
.qf-select-text { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.qf-select-blank { display: inline-block; flex: 0 0 auto; }
</style>
