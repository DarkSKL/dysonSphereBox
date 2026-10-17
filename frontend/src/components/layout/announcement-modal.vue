<script setup lang="ts">
import {onBeforeUnmount, onMounted, ref, watch} from 'vue'
import {useAnnouncements} from '../../composables/useAnnouncements'
// =============== 启动公告可见性 ===============
// 说明：弹窗实际可见只使按钮可用；普通关闭、正文渲染和图片加载均不提交已读。
const props = defineProps<{readonly open: boolean}>()
const emit = defineEmits<{(event: 'close'): void}>()
const body = ref<HTMLElement | null>(null)
const {item, title, html, open: noticeOpen, ready, pending, error, check, acknowledge, close: closeNotice, openLink, verifyVisible} = useAnnouncements()
const close = (): void => { closeNotice(); emit('close') }
const afterEnter = (): void => { void verifyVisible(body.value) }
watch(() => props.open, (hostVisible) => { if (hostVisible) afterEnter(); else void verifyVisible(null) })
const handleAcknowledge = async (): Promise<void> => { await verifyVisible(body.value); if (ready.value) await acknowledge() }
let observer: MutationObserver | undefined
onMounted(() => {
  observer = new MutationObserver(() => { if (props.open && noticeOpen.value) afterEnter() })
  observer.observe(document.body, {subtree: true, attributes: true, attributeFilter: ['class', 'style']})
  document.addEventListener('visibilitychange', afterEnter)
})
onBeforeUnmount(() => { observer?.disconnect(); document.removeEventListener('visibilitychange', afterEnter) })
</script>
<template>
  <!-- ~ 启动最新未读公告 -->
  <!-- * 说明：显示清洗后的最新正文，正文无法显示时保留错误提示 -->
  <!-- * 交互：用户点击已查阅才发送确认，关闭不产生已读副作用 -->
  <!-- * 依赖：共享公告状态与宿主可见性检查 -->
  <!-- * 边界：账号弹窗或教程遮挡时不解锁确认按钮 -->
  <n-modal :show="props.open && noticeOpen" preset="card" style="width:min(720px, calc(100vw - 32px));" :title="title || (error ? '公告检查失败' : '')" @after-enter="afterEnter" @update:show="(value) => !value && props.open && close()">
    <span v-if="item" class="announcement-kind">{{ item.kind === 'promotion' ? '推广' : item.kind === 'notice' ? '通知' : '公告' }}</span>
    <div ref="body" class="announcement-body" v-html="html" @click="openLink" @auxclick.prevent="openLink" @keydown.enter="openLink" />
    <p v-if="error" class="announcement-error" role="alert">{{ error }}</p>
    <template #footer>
      <n-space justify="end">
        <n-button secondary @click="close">关闭</n-button>
        <n-button v-if="error" secondary @click="check(true)">重试检查</n-button>
        <n-button type="primary" :loading="pending" :disabled="!props.open || !ready" @click="handleAcknowledge">已查阅</n-button>
      </n-space>
    </template>
  </n-modal>
</template>
<style scoped lang="scss">
@use '../../styles/announcement-body';
</style>
