<script setup lang="ts">
definePage({
  meta: {
    layout: 'main',
    keepAlive: true,
    title: '蓝图生成器',
  },
})

// =============== 蓝图生成器来源 ===============
// 说明：集中维护内嵌页面地址，方便后续切换到镜像站或本地托管版本。
const blueprintGeneratorUrl = 'https://sgwswordemperor.github.io/'

// =============== 内嵌视口尺寸 ===============
// 说明：外部蓝图工具是信息密度较高的桌面工具，过窄时保留最小工作宽度并交给外层滚动容器处理。
const embeddedViewportStyle = {
  minWidth: '1080px',
  minHeight: '680px',
}
</script>

<template>
  <section class="flex h-full min-h-[calc(100vh-4rem)] flex-col bg-zinc-100">
    <!-- ~ 蓝图生成器内嵌页 -->
    <!-- * 说明：直接承载外部蓝图生成器页面，保留原站点完整交互能力 -->
    <!-- * 交互：用户在嵌入页面内完成蓝图编辑与生成操作；窗口较窄时外层提供横向滚动 -->
    <!-- * 依赖：依赖 sgwswordemperor.github.io 可访问，并由 WebView 承载 iframe -->
    <!-- * 边界：外部站点不可访问或主动拒绝嵌入时，浏览器会在 iframe 内展示加载失败状态；小窗口下不压缩外部工具布局 -->
    <div class="min-h-0 flex-1 overflow-auto">
      <iframe
        class="block h-full w-full border-0 bg-white"
        :style="embeddedViewportStyle"
        :src="blueprintGeneratorUrl"
        title="蓝图生成器"
        allow="clipboard-read; clipboard-write"
        loading="eager"
        referrerpolicy="no-referrer-when-downgrade"
        sandbox="allow-downloads allow-forms allow-popups allow-same-origin allow-scripts"
      ></iframe>
    </div>
  </section>
</template>
