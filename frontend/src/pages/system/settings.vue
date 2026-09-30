<script setup lang="ts">
import ServerSection from '../../components/system/settings/server-section.vue'
import UpdateSection from '../../components/system/settings/update-section.vue'

definePage({ meta: { layout: 'main', title: '全局设置' } })
</script>

<template>
  <!-- ~ 全局设置页面 -->
  <!-- * 说明：管理戴森球盒子本身的全局偏好，当前包含服务器连接地址与应用更新两个分区 -->
  <!-- * 交互：各分区独立加载和保存，互不阻塞；页面标题由布局标题栏展示 -->
  <!-- * 依赖：依赖 ServerSection、UpdateSection 组件与 style.scss 中的 --ds-page-* 布局 Token -->
  <!-- * 边界：戴森球计划专属配置（游戏目录等）放在游戏设置页，不在此处展示 -->
  <section class="settings-page" aria-label="全局设置">
    <ServerSection />
    <UpdateSection />
  </section>
</template>

<style scoped lang="scss">
.settings-page {
  max-width: var(--ds-page-max-width);
  padding: var(--ds-page-padding-top) var(--ds-page-padding-x) var(--ds-page-padding-bottom);
  container: settings-page / inline-size;
}
</style>
