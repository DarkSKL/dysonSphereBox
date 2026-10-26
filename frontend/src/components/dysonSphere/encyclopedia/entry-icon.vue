<script setup lang="ts">
import {Icon} from '@iconify/vue'
import {computed} from 'vue'
import {encyclopediaManifest, encyclopediaSpriteReady} from '../../../services/encyclopedia/catalog'
import {encyclopediaKindIcons} from '../../../services/encyclopedia/categories'
import type {EncyclopediaEntry} from '../../../types/encyclopedia'

const props = withDefaults(defineProps<{entry: EncyclopediaEntry; size?: number}>(), {size: 52})
// =============== 图标绘制 ===============
// 说明：精灵坐标按统一单元缩放，保持原图颜色与透明轮廓；可选资源包缺失时呈现类别图标。
const style = computed(() => {
  const ratio = props.size / encyclopediaManifest.sprite.cell
  return {width: `${props.size}px`, height: `${props.size}px`, backgroundImage: `url("${encyclopediaManifest.sprite.path}")`,
    backgroundSize: `${encyclopediaManifest.sprite.width * ratio}px ${encyclopediaManifest.sprite.height * ratio}px`,
    backgroundPosition: `${-(props.entry.icon?.x ?? 0) * ratio}px ${-(props.entry.icon?.y ?? 0) * ratio}px`}
})
</script>

<template>
  <!-- ~ 实体图标 -->
  <!-- * 说明：显示可选的游戏实体缩略图，名称由外层按钮或标题提供 -->
  <!-- * 交互：图像不截获事件 -->
  <!-- * 依赖：统一 sprite 就绪状态与校验后的图标坐标 -->
  <!-- * 边界：无图标或资源包加载失败时使用本地图标，不请求第三方图片 -->
  <span v-if="encyclopediaSpriteReady && entry.icon" class="entry-sprite" :style="style" aria-hidden="true" />
  <span v-else class="entry-fallback" :style="{width: `${size}px`, height: `${size}px`}" aria-hidden="true">
    <Icon :icon="encyclopediaKindIcons[entry.kind]" :width="size * .75" :height="size * .75" />
  </span>
</template>

<style scoped>
.entry-sprite { display: inline-block; flex: 0 0 auto; background-repeat: no-repeat; pointer-events: none; }
.entry-fallback { display: inline-flex; flex: 0 0 auto; align-items: center; justify-content: center; color: var(--ds-text-secondary); pointer-events: none; }
</style>
