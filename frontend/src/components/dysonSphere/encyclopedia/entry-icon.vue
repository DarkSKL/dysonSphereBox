<script setup lang="ts">
import {Icon} from '@iconify/vue'
import {computed} from 'vue'
import {encyclopediaManifest, encyclopediaSpriteReady} from '../../../services/encyclopedia/catalog'
import {encyclopediaKindIcons} from '../../../services/encyclopedia/categories'
import type {EncyclopediaEntry} from '../../../types/encyclopedia'

const props = withDefaults(defineProps<{entry: EncyclopediaEntry; size?: number}>(), {size: 52})
// =============== 图标绘制 ===============
// 说明：精灵坐标按统一单元缩放，保持原图颜色与透明轮廓；可选资源包缺失时呈现类别图标。
// 边界：科技与研究配方的原图是纯白单色字形，改用遮罩按主题文字色着色，避免日间主题呈现白块。
const monochrome = computed(() => props.entry.kind === 'technology'
  || props.entry.relations.some(relation => relation.type === 'product' && relation.target.startsWith('technology:')))
const style = computed(() => {
  const ratio = props.size / encyclopediaManifest.sprite.cell
  const image = `url("${encyclopediaManifest.sprite.path}")`
  const size = `${encyclopediaManifest.sprite.width * ratio}px ${encyclopediaManifest.sprite.height * ratio}px`
  const position = `${-(props.entry.icon?.x ?? 0) * ratio}px ${-(props.entry.icon?.y ?? 0) * ratio}px`
  const box = {width: `${props.size}px`, height: `${props.size}px`}
  if (monochrome.value) return {...box, maskImage: image, WebkitMaskImage: image, maskSize: size, WebkitMaskSize: size, maskPosition: position, WebkitMaskPosition: position}
  return {...box, backgroundImage: image, backgroundSize: size, backgroundPosition: position}
})
</script>

<template>
  <!-- ~ 实体图标 -->
  <!-- * 说明：显示可选的游戏实体缩略图，名称由外层按钮或标题提供 -->
  <!-- * 交互：图像不截获事件 -->
  <!-- * 依赖：统一 sprite 就绪状态与校验后的图标坐标 -->
  <!-- * 边界：无图标或资源包加载失败时使用本地图标，不请求第三方图片 -->
  <span v-if="encyclopediaSpriteReady && entry.icon" class="entry-sprite" :class="{'is-monochrome': monochrome}" :style="style" aria-hidden="true" />
  <span v-else class="entry-fallback" :style="{width: `${size}px`, height: `${size}px`}" aria-hidden="true">
    <Icon :icon="encyclopediaKindIcons[entry.kind]" :width="size * .75" :height="size * .75" />
  </span>
</template>

<style scoped>
.entry-sprite { display: inline-block; flex: 0 0 auto; background-repeat: no-repeat; pointer-events: none; }
.entry-sprite.is-monochrome { background: var(--ds-text-primary); mask-repeat: no-repeat; -webkit-mask-repeat: no-repeat; }
.entry-fallback { display: inline-flex; flex: 0 0 auto; align-items: center; justify-content: center; color: var(--ds-text-secondary); pointer-events: none; }
</style>
