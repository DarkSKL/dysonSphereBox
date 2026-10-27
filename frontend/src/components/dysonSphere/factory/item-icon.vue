<script setup lang="ts">
import {Icon} from '@iconify/vue'
import cargo from '@iconify-icons/game-icons/cargo-crate'
import {computed} from 'vue'
import {factoryIconStyle} from '../../../services/dysonSphere/factory-icons'

const props = withDefaults(defineProps<{name: string; size?: number}>(), {size: 24})
const style = computed(() => factoryIconStyle(props.name, props.size))
</script>

<template>
  <!-- ~ 量化物品图标 -->
  <!-- * 说明：按名称复用图鉴精灵；未收录的 Mod 物品使用通用货箱图标 -->
  <!-- * 边界：名称由外层提供无障碍文本，图标本身不读屏 -->
  <span v-if="style" class="factory-item-icon" :style="style" aria-hidden="true" />
  <span v-else class="factory-item-icon is-fallback" :style="{width: `${size}px`, height: `${size}px`}" aria-hidden="true"><Icon :icon="cargo" :width="size * .7" :height="size * .7" /></span>
</template>

<style scoped>
.factory-item-icon { display: inline-flex; flex: 0 0 auto; align-items: center; justify-content: center; background-repeat: no-repeat; pointer-events: none; }
.is-fallback { color: var(--ds-text-muted); }
</style>
