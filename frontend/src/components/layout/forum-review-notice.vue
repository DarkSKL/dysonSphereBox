<script setup lang="ts">
import {onBeforeUnmount, ref} from 'vue'
import {isNavigationFailure, NavigationFailureType, useRouter} from 'vue-router'
import {accountSessionKey} from '../../pages/services/account'
import {fetchForumPublicationReview, forumReviewTarget} from '../../services/forum'
import {formatErrorMessage} from '../../utils/format'

const props = defineProps<{readonly reviewId: string}>()
const emit = defineEmits<{(event: 'close'): void}>()
const router = useRouter()
const loading = ref(false)
const statusLabel = ref('')
const reason = ref('')
const error = ref('')
const sessionKey = accountSessionKey.value
let active = true

// =============== 审核详情与去向 ===============
// 说明：通过时打开论坛目标，其他状态在当前弹窗展示；旧账号或已关闭弹窗的响应不再导航。
async function loadReview(): Promise<void> {
  if (!sessionKey || loading.value) return
  loading.value = true
  error.value = ''
  try {
    const review = await fetchForumPublicationReview(props.reviewId)
    if (!active || accountSessionKey.value !== sessionKey) return
    if (review.status === 'approved') {
      const replyId = review.targetType === 'reply' ? review.targetId : 0
      if (!Number.isSafeInteger(review.postId) || review.postId <= 0 ||
          (review.targetType !== 'post' && (!Number.isSafeInteger(replyId) || replyId <= 0))) {
        throw new Error('审核已通过，但无法定位发布内容。')
      }
      const failure = await router.push({name: '/community/forum'})
      if (!active || accountSessionKey.value !== sessionKey) return
      if (failure && !isNavigationFailure(failure, NavigationFailureType.duplicated)) throw new Error('无法打开论坛，请重试。')
      forumReviewTarget.value = {sessionKey, postId: review.postId, replyId}
      emit('close')
    } else {
      statusLabel.value = review.status === 'rejected' ? '审核未通过' : '审核中'
      reason.value = review.reason
    }
  } catch (cause) {
    if (active && accountSessionKey.value === sessionKey) error.value = formatErrorMessage(cause)
  } finally {
    if (active) loading.value = false
  }
}

onBeforeUnmount(() => { active = false })
void loadReview()
</script>

<template>
  <!-- ~ 论坛发布审核结果 -->
  <!-- * 说明：显示当前作者可见的审核状态与理由 -->
  <!-- * 交互：审核通过时打开对应内容；请求失败时可重试 -->
  <!-- * 依赖：通知中的审核记录 ID 与当前账号 -->
  <!-- * 边界：关闭或切换账号时丢弃迟到结果 -->
  <n-modal :show="true" preset="card" title="论坛发布审核" style="width:min(460px, calc(100vw - 32px));" @update:show="value => !value && emit('close')">
    <p v-if="loading" role="status">正在读取审核结果…</p>
    <p v-else-if="error" role="alert" class="text-sm" style="color: var(--ds-error)">{{ error }}</p>
    <template v-else>
      <p class="font-bold">{{ statusLabel }}</p>
      <p v-if="reason" class="mt-2 whitespace-pre-wrap break-words">{{ reason }}</p>
    </template>
    <template #footer>
      <n-space justify="end">
        <n-button v-if="error" secondary @click="loadReview">重试</n-button>
        <n-button secondary @click="emit('close')">关闭</n-button>
      </n-space>
    </template>
  </n-modal>
</template>
