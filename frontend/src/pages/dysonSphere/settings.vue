<script setup lang="ts">
import BepInExSection from '../../components/dysonSphere/settings/bepinex-section.vue'
import GamePathSection from '../../components/dysonSphere/settings/game-path-section.vue'

definePage({ meta: { layout: 'main', title: '游戏设置' } })
</script>

<template>
  <!-- ~ 游戏设置页面 -->
  <!-- * 说明：管理戴森球计划专属配置，当前包含游戏目录与 BepInEx 前置框架两个分区 -->
  <!-- * 交互：BepInEx 分区依赖已保存的游戏目录，目录变化后自动重新检测 -->
  <!-- * 依赖：依赖 GamePathSection、BepInExSection 组件、services/dysonSphere/game 共享状态与 --ds-page-* 布局 Token -->
  <!-- * 边界：应用主题、服务器地址与应用更新等全局偏好放在全局设置页，不在此处展示 -->
  <section class="game-settings-page" aria-label="游戏设置">
    <GamePathSection />
    <BepInExSection />
  </section>
</template>

<style scoped lang="scss">
// 与全局设置页保持同一版心：宽于 --ds-page-max-width 时水平居中
.game-settings-page {
  width: 100%;
  max-width: var(--ds-page-max-width);
  margin-inline: auto;
  padding: var(--ds-page-padding-top) var(--ds-page-padding-x) var(--ds-page-padding-bottom);
  container: settings-page / inline-size;
}
</style>
