<script setup lang="ts">
import {
  ArrowLeft,
  ArrowUp,
  Flag,
  PenSquare,
  RefreshCw,
  RotateCcw,
  Send,
} from '@lucide/vue'
import {computed, onDeactivated, ref, watch, watchPostEffect} from 'vue'
import ForumBodyRenderer from '../../components/forum/forum-body-renderer.vue'
import ForumEditor from '../../components/forum/forum-editor.vue'
import ForumReportModal from '../../components/forum/forum-report-modal.vue'
import {useForumState} from '../../composables/forum/useForumState'
import type {ForumCategory} from '../../types/forum'

definePage({meta: {layout: 'main', title: '论坛', keepAlive: true}})

const forum = useForumState()
const {
  viewMode,
  isAuthenticated,
  categories,
  enabledCategories,
  selectedCategoryId,
  posts,
  postsTotal,
  postsPage,
  postsPageSize,
  postsLoading,
  postsError,
  currentPostId,
  postDetail,
  detailLoading,
  detailError,
  replies,
  repliesTotal,
  repliesPage,
  repliesPageSize,
  repliesLoading,
  repliesError,
  focusedReplyId,
  postDraft,
  replyDraft,
  busyResourceAction,
  reportModalOpen,
  reportTarget,
  reportSubmitting,
} = forum

const categorySelectOptions = computed(() =>
  enabledCategories.value.map((item) => ({
    label: item.name,
    value: item.id,
  })),
)

const titleCharCount = computed(() => Array.from(postDraft.title || '').length)

// =============== 分类色彩标识映射 ===============
// 说明：按服务端动态分类 code 或排序稳定映射瑞士网格左侧竖标色彩，仅复用项目内置语义 Token，日间克制、夜间通透。
const categoryTonePalette = ['tone-brand', 'tone-emerald', 'tone-gold', 'tone-muted'] as const

function categoryToneClass(category?: ForumCategory | null): string {
  if (!category) return 'tone-muted'
  const code = (category.code || '').toLowerCase()
  const name = category.name || ''
  if (code.includes('guide') || name.includes('攻略') || name.includes('教程')) return 'tone-brand'
  if (code.includes('work') || code.includes('show') || name.includes('作品') || name.includes('蓝图')) return 'tone-emerald'
  if (code.includes('chat') || code.includes('talk') || name.includes('闲聊') || name.includes('交流')) return 'tone-muted'
  const index = Math.abs((category.id || 0) + (category.sort || 0)) % categoryTonePalette.length
  return categoryTonePalette[index]
}

// =============== 时间与计数格式化 ===============
// 说明：列表行采用紧凑自适应时间（当天仅展示时分，跨天展示月日时分，跨年补全年份），详情与 title 保留完整时间。
function formatDateTime(raw: string): string {
  if (!raw) return '—'
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw
  return date.toLocaleString('zh-CN', {
    hour12: false,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatChronicleTime(raw: string): string {
  if (!raw) return '—'
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw
  const now = new Date()
  const pad = (n: number): string => String(n).padStart(2, '0')
  const hm = `${pad(date.getHours())}:${pad(date.getMinutes())}`
  if (date.toDateString() === now.toDateString()) {
    return `今天 ${hm}`
  }
  const md = `${pad(date.getMonth() + 1)}/${pad(date.getDate())}`
  if (date.getFullYear() === now.getFullYear()) {
    return `${md} ${hm}`
  }
  return `${date.getFullYear()}/${md}`
}

function formatReplyCount(count: number): string {
  return String(Math.max(0, count || 0)).padStart(2, '0')
}

// =============== 详情页滚动与右下角悬浮控制 ===============
// 说明：监听详情滚动容器位置，超过阈值后展示「回到顶部」悬浮按钮；切帖或离开详情时重置滚动位置。
const detailScrollRef = ref<HTMLElement | null>(null)
const showBackToTop = ref(false)

function handleDetailScroll(): void {
  showBackToTop.value = (detailScrollRef.value?.scrollTop ?? 0) > 120
}

function scrollDetailToTop(): void {
  detailScrollRef.value?.scrollTo({top: 0, behavior: 'smooth'})
}

watch([viewMode, currentPostId], () => {
  showBackToTop.value = false
  if (detailScrollRef.value) {
    detailScrollRef.value.scrollTop = 0
  }
})

// 回复分页与帖子正文独立返回，等两者完成渲染后再滚动并聚焦通知对应的回复。
watchPostEffect(() => {
  if (!focusedReplyId.value || !postDetail.value) return
  const target = detailScrollRef.value?.querySelector<HTMLElement>(`[data-reply-id="${focusedReplyId.value}"]`)
  if (!target) return
  target.scrollIntoView({block: 'center'})
  target.focus({preventScroll: true})
})

onDeactivated(() => {
  reportModalOpen.value = false
})
</script>

<template>
  <!-- ~ 社区论坛页面（瑞士网格编年栏） -->
  <!-- * 说明：无外层实心面板框、无冗余顶部标题，直接浮于宇宙星云之上；以单行分类命令轨开场，三列对称排版呈现帖子列表 -->
  <!-- * 交互：点击分类横线切换板块，悬停帖子行触发 .ds-bracket-hover 括角生长，详情与发帖均采用居中无框沉浸画布 -->
  <!-- * 边界：依赖已验证账号会话；未登录时仅展示居中提示状态，不发起远端请求 -->
  <section class="forum-page" aria-label="社区论坛">
    <div v-if="!isAuthenticated" class="forum-empty-gate">
      <h2>登录后进入社区论坛</h2>
      <p>点击左侧栏头像登录戴森球盒子账号，即可浏览讨论、分享蓝图与模组，或参与交流。</p>
    </div>

    <!-- ~ 视图一：瑞士网格编年列表 -->
    <template v-else-if="viewMode === 'list'">
      <header class="forum-command-rail">
        <nav class="category-tabs" role="tablist" aria-label="论坛分类">
          <button
            type="button"
            role="tab"
            class="category-tab"
            :class="{'is-active': selectedCategoryId === 0}"
            :aria-selected="selectedCategoryId === 0"
            @click="forum.selectCategory(0)"
          >
            <span>全部</span>
          </button>
          <button
            v-for="cat in categories"
            :key="cat.id"
            type="button"
            role="tab"
            class="category-tab"
            :class="{'is-active': selectedCategoryId === cat.id}"
            :aria-selected="selectedCategoryId === cat.id"
            @click="forum.selectCategory(cat.id)"
          >
            <span>{{ cat.name }}</span>
          </button>
        </nav>

        <div class="rail-actions">
          <n-tooltip>
            <template #trigger>
              <n-button class="rail-icon-btn" :loading="postsLoading" aria-label="刷新帖子列表" @click="forum.refreshList()">
                <template #icon><RefreshCw :size="15" /></template>
              </n-button>
            </template>
            刷新列表
          </n-tooltip>
          <n-button type="primary" @click="forum.openCompose()">
            <template #icon><PenSquare :size="15" /></template>
            发布帖子
          </n-button>
        </div>
      </header>

      <div class="chronicle-stage">
        <div v-if="postsError" class="forum-state-box is-error">
          <p>{{ postsError }}</p>
          <n-button size="small" secondary @click="forum.refreshList()">重新加载</n-button>
        </div>

        <div v-else-if="postsLoading && posts.length === 0" class="chronicle-loading" role="status" aria-label="正在读取帖子列表">
          <div v-for="index in 5" :key="index" class="chronicle-row is-skeleton">
            <div class="row-left">
              <n-skeleton width="56px" height="14px" />
              <n-skeleton width="76px" height="11px" />
            </div>
            <div class="row-center">
              <n-skeleton width="62%" height="18px" />
              <n-skeleton width="22%" height="12px" />
            </div>
            <div class="row-right">
              <n-skeleton width="32px" height="20px" />
            </div>
          </div>
        </div>

        <div v-else-if="posts.length === 0" class="forum-state-box is-empty">
          <h3>当前分类下还没有讨论</h3>
          <p>发布第一篇帖子，支持在正文中原位嵌入截图、MP4 演示、蓝图、Thunderstore 模组与主存档。</p>
          <n-button type="primary" secondary @click="forum.openCompose()">
            <template #icon><PenSquare :size="15" /></template>
            发布第一篇帖子
          </n-button>
        </div>

        <ul v-else class="chronicle-list">
          <li
            v-for="item in posts"
            :key="item.id"
            class="chronicle-row ds-bracket-hover"
            tabindex="0"
            @click="forum.openPostDetail(item.id)"
            @keydown.enter.prevent="forum.openPostDetail(item.id)"
          >
            <div class="row-left">
              <span class="cat-tag">
                <i class="cat-bar-mark" :class="categoryToneClass(item.category)" aria-hidden="true" />
                <span class="cat-name">{{ item.category?.name || '未分类' }}</span>
              </span>
              <time class="row-time type-mono" :title="formatDateTime(item.createdAt)">
                {{ formatChronicleTime(item.createdAt) }}
              </time>
            </div>

            <div class="row-center">
              <h2 class="row-title" :title="item.title">{{ item.title }}</h2>
              <div class="row-sub">
                <span class="row-inline-cat">
                  <i class="cat-bar-mark" :class="categoryToneClass(item.category)" aria-hidden="true" />
                  {{ item.category?.name || '未分类' }}
                </span>
                <span class="row-author">{{ item.authorName }}</span>
                <span class="row-inline-time type-mono">{{ formatChronicleTime(item.createdAt) }}</span>
              </div>
            </div>

            <div class="row-right" :aria-label="`${item.replyCount} 条回复`">
              <span class="reply-num type-display" :class="{'is-zero': !item.replyCount}">
                {{ formatReplyCount(item.replyCount) }}
              </span>
              <span class="reply-unit">回复</span>
            </div>
          </li>
        </ul>

        <footer v-if="postsTotal > postsPageSize" class="chronicle-pager">
          <n-pagination
            :page="postsPage"
            :page-size="postsPageSize"
            :item-count="postsTotal"
            @update:page="forum.changePostsPage"
          />
        </footer>
      </div>
    </template>

    <!-- ~ 视图二：无框居中长文阅读与回复 -->
    <template v-else-if="viewMode === 'detail'">
      <div ref="detailScrollRef" class="editorial-scroll" @scroll.passive="handleDetailScroll">
        <div class="editorial-container">
          <div v-if="detailError && !postDetail" class="forum-state-box is-error">
            <p>{{ detailError }}</p>
            <n-button size="small" secondary @click="forum.backToList(true)">返回列表</n-button>
          </div>

          <div v-else-if="detailLoading && !postDetail" class="editorial-loading">
            <n-skeleton width="24%" height="14px" />
            <n-skeleton width="78%" height="28px" />
            <n-skeleton text :repeat="5" />
          </div>

          <template v-else-if="postDetail">
            <article class="editorial-article">
              <div class="article-kicker">
                <span class="cat-tag">
                  <i class="cat-bar-mark" :class="categoryToneClass(postDetail.category)" aria-hidden="true" />
                  <span>{{ postDetail.category?.name || '未分类' }}</span>
                </span>
                <span v-if="postDetail.category && !postDetail.category.enabled" class="disabled-cat-badge">原分类已停用</span>
              </div>

              <h1 class="article-title">{{ postDetail.title }}</h1>

              <div class="article-meta">
                <span class="meta-author">{{ postDetail.authorName }}</span>
                <span class="meta-sep" aria-hidden="true">/</span>
                <time class="type-mono">{{ formatDateTime(postDetail.createdAt) }}</time>
                <span class="meta-sep" aria-hidden="true">/</span>
                <span class="type-display">{{ postDetail.replyCount }} 条回复</span>
              </div>

              <div class="article-body">
                <ForumBodyRenderer
                  :body="postDetail.body"
                  :body-format="postDetail.bodyFormat"
                  :body-blocks="postDetail.bodyBlocks"
                  :resources="postDetail.resources"
                  :busy-action="busyResourceAction"
                  @copy-blueprint="forum.handleCopyBlueprint"
                  @import-blueprint="forum.handleImportBlueprint"
                  @import-save="forum.handleImportSave"
                  @import-mod="forum.handleImportMod"
                  @open-thunderstore="forum.handleOpenThunderstore"
                  @report-resource="(res) => forum.openReport({targetType: 'resource', targetId: res.id, title: res.name || `资源 #${res.id}`})"
                />
              </div>
            </article>

            <section class="editorial-replies" aria-label="回复列表">
              <div class="section-rule-head">
                <h2>全部回复</h2>
                <span class="section-count type-display">{{ formatReplyCount(repliesTotal) }}</span>
              </div>

              <div v-if="repliesError" class="forum-state-box is-error">
                <p>{{ repliesError }}</p>
                <n-button size="small" secondary @click="forum.loadReplies()">重试</n-button>
              </div>

              <div v-else-if="repliesLoading && replies.length === 0" class="replies-hint">
                正在加载回复…
              </div>

              <div v-else-if="replies.length === 0" class="replies-hint">
                暂无回复，在下方写下第一条讨论。
              </div>

              <ul v-else class="reply-stream">
                <li v-for="reply in replies" :key="reply.id" class="reply-entry" :data-reply-id="reply.id"
                  :class="{'is-review-target': focusedReplyId === reply.id}" tabindex="-1"
                >
                  <div class="reply-head">
                    <div class="reply-author">
                      <strong>{{ reply.authorName }}</strong>
                      <time class="reply-time type-mono">{{ formatDateTime(reply.createdAt) }}</time>
                    </div>
                    <button
                      type="button"
                      class="reply-report-btn"
                      @click="forum.openReport({targetType: 'reply', targetId: reply.id, title: `${reply.authorName} 的回复`})"
                    >
                      <Flag :size="12" />举报
                    </button>
                  </div>
                  <div class="reply-body">
                    <ForumBodyRenderer
                      :body="reply.body"
                      :body-format="reply.bodyFormat"
                      :body-blocks="reply.bodyBlocks"
                      :resources="reply.resources"
                      :busy-action="busyResourceAction"
                      @report-resource="(res) => forum.openReport({targetType: 'resource', targetId: res.id, title: res.name || `资源 #${res.id}`})"
                    />
                  </div>
                </li>
              </ul>

              <footer v-if="repliesTotal > repliesPageSize" class="chronicle-pager">
                <n-pagination
                  :page="repliesPage"
                  :page-size="repliesPageSize"
                  :item-count="repliesTotal"
                  @update:page="forum.changeRepliesPage"
                />
              </footer>
            </section>

            <section class="editorial-composer" aria-label="发表回复">
              <div class="section-rule-head">
                <h2>发表回复</h2>
                <span v-if="replyDraft.lastError" class="composer-error">{{ replyDraft.lastError }}</span>
              </div>

              <ForumEditor
                v-model="replyDraft.body"
                :bindings="replyDraft.bindings"
                mode="reply"
                :min-height="140"
                :disabled="replyDraft.submitting"
                placeholder="输入回复内容（支持 Markdown 与最多 4 张图片）…"
              />

              <div class="composer-actions">
                <n-button secondary size="small" :disabled="replyDraft.submitting" @click="forum.resetReplyDraft()">
                  <template #icon><RotateCcw :size="14" /></template>
                  清空草稿
                </n-button>
                <n-button type="primary" :loading="replyDraft.submitting" @click="forum.submitReply()">
                  <template #icon><Send :size="14" /></template>
                  {{ replyDraft.lastError ? '重试发送' : '发送回复' }}
                </n-button>
              </div>
            </section>
          </template>
        </div>
      </div>

      <!-- ~ 详情页右下角悬浮操作坞 -->
      <!-- * 说明：承接回到顶部、刷新帖子、举报帖子与返回列表操作，释放正文顶部空间 -->
      <!-- * 交互：滚动超过 120px 后平滑浮现回到顶部按钮；悬停图标展示左侧提示并生长括角 -->
      <aside class="detail-floating-dock" aria-label="帖子操作">
        <Transition name="float-btn">
          <n-tooltip v-if="showBackToTop" placement="left">
            <template #trigger>
              <button
                type="button"
                class="floating-icon-btn ds-bracket-hover"
                aria-label="回到顶部"
                @click="scrollDetailToTop"
              >
                <ArrowUp :size="16" />
              </button>
            </template>
            回到顶部
          </n-tooltip>
        </Transition>

        <n-tooltip placement="left">
          <template #trigger>
            <button
              type="button"
              class="floating-icon-btn ds-bracket-hover"
              :class="{'is-spinning': detailLoading || repliesLoading}"
              :disabled="detailLoading || repliesLoading"
              aria-label="刷新帖子"
              @click="forum.openPostDetail(currentPostId)"
            >
              <RefreshCw :size="15" />
            </button>
          </template>
          刷新帖子
        </n-tooltip>

        <n-tooltip v-if="postDetail" placement="left">
          <template #trigger>
            <button
              type="button"
              class="floating-icon-btn is-danger ds-bracket-hover"
              aria-label="举报帖子"
              @click="forum.openReport({targetType: 'post', targetId: postDetail.id, title: postDetail.title})"
            >
              <Flag :size="15" />
            </button>
          </template>
          举报帖子
        </n-tooltip>

        <n-tooltip placement="left">
          <template #trigger>
            <button
              type="button"
              class="floating-icon-btn ds-bracket-hover"
              aria-label="返回列表"
              @click="forum.backToList(false)"
            >
              <ArrowLeft :size="16" />
            </button>
          </template>
          返回列表
        </n-tooltip>
      </aside>
    </template>

    <!-- ~ 视图三：无框居中发帖画布 -->
    <template v-else>
      <div class="editorial-scroll">
        <div class="editorial-container is-compose">
          <header class="editorial-topbar">
            <n-button secondary size="small" :disabled="postDraft.submitting" @click="forum.backToList(false)">
              <template #icon><ArrowLeft :size="15" /></template>
              返回列表
            </n-button>

            <div class="rail-actions">
              <n-button secondary size="small" :disabled="postDraft.submitting" @click="forum.resetPostDraft()">
                <template #icon><RotateCcw :size="14" /></template>
                清空草稿
              </n-button>
              <n-button type="primary" size="small" :loading="postDraft.submitting" @click="forum.submitPost()">
                <template #icon><Send :size="14" /></template>
                {{ postDraft.lastError ? '重试发布' : '发布帖子' }}
              </n-button>
            </div>
          </header>

          <div v-if="postDraft.lastError" class="compose-error-banner" role="alert">
            发布失败：{{ postDraft.lastError }}（已为你保留原稿、已选文件和同一请求标识，可直接点击「重试发布」。）
          </div>

          <div class="compose-category-row">
            <span class="compose-field-label">板块</span>
            <div v-if="enabledCategories.length <= 8" class="compose-cat-chips" role="radiogroup" aria-label="选择发帖板块">
              <button
                v-for="cat in enabledCategories"
                :key="cat.id"
                type="button"
                role="radio"
                class="compose-cat-chip ds-bracket-hover"
                :class="{'is-selected': postDraft.categoryId === cat.id}"
                :aria-checked="postDraft.categoryId === cat.id"
                :disabled="postDraft.submitting"
                @click="postDraft.categoryId = cat.id"
              >
                <i class="cat-bar-mark" :class="categoryToneClass(cat)" aria-hidden="true" />
                <span>{{ cat.name }}</span>
              </button>
            </div>
            <div v-else class="compose-cat-select">
              <n-select
                v-model:value="postDraft.categoryId"
                :options="categorySelectOptions"
                placeholder="选择分类"
                :disabled="postDraft.submitting"
              />
            </div>
          </div>

          <div class="compose-title-wrap">
            <input
              v-model="postDraft.title"
              type="text"
              class="compose-title-input"
              maxlength="120"
              placeholder="输入帖子标题…"
              :disabled="postDraft.submitting"
              aria-label="帖子标题"
            />
            <span class="compose-title-count type-mono" :class="{'is-over': titleCharCount > 120}">
              {{ titleCharCount }} / 120
            </span>
          </div>

          <ForumEditor
            v-model="postDraft.body"
            :bindings="postDraft.bindings"
            mode="post"
            :min-height="320"
            :disabled="postDraft.submitting"
            placeholder="使用所见即所得写作，或点击工具栏在光标处插入图片、MP4、蓝图、模组或存档…"
          />
        </div>
      </div>
    </template>

    <ForumReportModal
      v-model:show="reportModalOpen"
      :target="reportTarget"
      :submitting="reportSubmitting"
      @submit="forum.submitReport"
    />
  </section>
</template>

<style scoped lang="scss">
// 说明：整体遵循瑞士网格编年栏设计，去外层灰框、去顶部冗余标题，直接透出 MainLayout 宇宙星云与日间白昼画布。
.forum-page {
  position: relative;
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  padding: clamp(16px, 2vw, 28px) var(--ds-page-padding-x) 0;
  color: var(--ds-text-primary);
}

.forum-empty-gate {
  margin: auto;
  text-align: center;
  max-width: 420px;

  h2 {
    margin: 0 0 10px;
    font-size: 19px;
    font-weight: 700;
  }

  p {
    margin: 0;
    font-size: 13px;
    color: var(--ds-text-muted);
    line-height: 1.8;
  }
}

// =============== 单行分类命令轨（无冗余标题） ===============
.forum-command-rail {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  border-bottom: 1px solid var(--ds-border-default);
  flex-shrink: 0;
}

.category-tabs {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 24px;
  min-width: 0;
}

.category-tab {
  position: relative;
  display: inline-flex;
  align-items: center;
  padding: 6px 2px 13px;
  border: none;
  background: transparent;
  color: var(--ds-text-muted);
  font: inherit;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: color 150ms ease;

  &:hover {
    color: var(--ds-text-primary);
  }

  &.is-active {
    color: var(--ds-text-primary);
    font-weight: 700;

    &::after {
      content: '';
      position: absolute;
      left: 0;
      right: 0;
      bottom: -1px;
      height: 2px;
      background: var(--ds-brand);
    }
  }
}

.rail-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-bottom: 9px;
  flex-shrink: 0;
}

.rail-icon-btn {
  width: 34px;
  height: 34px;
  padding: 0;
  flex-shrink: 0;

  &.is-sm {
    width: 28px;
    height: 28px;
  }
}

// =============== 瑞士网格三列帖子流 ===============
.chronicle-stage {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}

.chronicle-list,
.chronicle-loading {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  list-style: none;
  margin: 0;
  padding: 0 0 24px;
}

.chronicle-row {
  display: grid;
  grid-template-columns: 148px minmax(0, 1fr) 76px;
  align-items: center;
  gap: 22px;
  padding: 18px 16px;
  border-bottom: 1px solid var(--ds-border-subtle);
  cursor: pointer;
  transition: background-color 150ms ease;

  &:hover {
    background: var(--ds-hover-bg);
  }

  &.is-skeleton {
    cursor: default;
    &:hover {
      background: transparent;
    }
  }
}

.row-left {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding-right: 18px;
  border-right: 1px solid var(--ds-border-subtle);
  min-width: 0;
}

.cat-tag {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font-size: 12.5px;
  font-weight: 600;
  color: var(--ds-text-primary);
  min-width: 0;
}

.cat-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cat-bar-mark {
  display: inline-block;
  width: 2px;
  height: 12px;
  border-radius: 1px;
  flex-shrink: 0;

  &.tone-brand { background: var(--ds-brand); }
  &.tone-emerald { background: var(--ds-res-on); }
  &.tone-gold { background: var(--ds-folder-accent); }
  &.tone-muted { background: var(--ds-text-muted); }
}

.row-time {
  font-size: 11.5px;
  color: var(--ds-text-muted);
  white-space: nowrap;
}

.row-center {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.row-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  line-height: 1.42;
  color: var(--ds-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.row-sub {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 12.5px;
  color: var(--ds-text-muted);
}

.row-inline-cat,
.row-inline-time {
  display: none;
}

.row-right {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
}

.reply-num {
  font-size: 20px;
  font-weight: 600;
  line-height: 1.1;
  color: var(--ds-text-primary);

  &.is-zero {
    color: var(--ds-text-muted);
    font-weight: 500;
  }
}

.reply-unit {
  font-size: 10.5px;
  color: var(--ds-text-muted);
}

.chronicle-pager {
  display: flex;
  justify-content: flex-end;
  padding: 14px 4px 20px;
  border-top: 1px solid var(--ds-border-subtle);
  flex-shrink: 0;
}

.forum-state-box {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 64px 20px;
  text-align: center;
  color: var(--ds-text-muted);
  font-size: 13px;

  h3 {
    margin: 0;
    font-size: 17px;
    font-weight: 700;
    color: var(--ds-text-primary);
  }

  p {
    margin: 0;
    max-width: 420px;
    line-height: 1.75;
  }

  &.is-error {
    color: var(--ds-error);
  }
}

// =============== 无框居中长文阅读与发帖画布 ===============
// 说明：阅读列宽按容器宽度自适应，在保持 600px 舒适最小阅读宽度的同时，宽屏下平滑扩展至 1240px。
.editorial-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding-inline: 8px;
  scrollbar-width: none;
  -ms-overflow-style: none;

  &::-webkit-scrollbar {
    display: none;
  }
}

.editorial-container {
  width: clamp(min(100%, 600px), 84%, 1240px);
  margin: 0 auto;
  padding: 4px 0 88px;

  &.is-compose {
    width: clamp(min(100%, 600px), 86%, 1240px);
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
}

// =============== 详情页右下角悬浮操作坞 ===============
.detail-floating-dock {
  position: absolute;
  right: max(18px, calc(var(--ds-page-padding-x) * 0.65));
  bottom: 24px;
  z-index: 20;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.floating-icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  padding: 0;
  border-radius: 4px;
  border: 1px solid var(--ds-border-default);
  background: var(--ds-bg-panel-elevated);
  color: var(--ds-text-secondary);
  cursor: pointer;
  transition: background-color 150ms ease, border-color 150ms ease, color 150ms ease, transform 150ms ease;

  &:hover:not(:disabled) {
    border-color: transparent;
    background: var(--ds-bg-panel);
    color: var(--ds-text-primary);
    transform: translateY(-1px);
  }

  &.is-danger:hover:not(:disabled) {
    color: var(--ds-error);
    background: var(--ds-error-bg);
  }

  &:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }

  &.is-spinning svg {
    animation: forum-spin 0.9s linear infinite;
  }
}

@keyframes forum-spin {
  to {
    transform: rotate(360deg);
  }
}

.float-btn-enter-active,
.float-btn-leave-active {
  transition: opacity 180ms ease, transform 180ms cubic-bezier(0.22, 1, 0.36, 1);
}

.float-btn-enter-from,
.float-btn-leave-to {
  opacity: 0;
  transform: translateY(6px);
}

.editorial-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 12px;
  margin-bottom: 20px;
  border-bottom: 1px solid var(--ds-border-subtle);

  .rail-actions {
    padding-bottom: 0;
  }

  .is-compose & {
    margin-bottom: 2px;
  }
}

.editorial-loading {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 12px 0;
}

.editorial-article {
  padding-bottom: 28px;
}

.article-kicker {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}

.disabled-cat-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 7px;
  border-radius: 3px;
  background: var(--ds-warning-bg);
  color: var(--ds-warning);
  font-size: 11px;
}

.article-title {
  margin: 0 0 12px;
  font-size: 24px;
  font-weight: 700;
  line-height: 1.38;
  color: var(--ds-text-primary);
  word-break: break-word;
}

.article-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  padding-bottom: 18px;
  margin-bottom: 20px;
  border-bottom: 1px solid var(--ds-border-subtle);
  font-size: 12.5px;
  color: var(--ds-text-muted);
}

.meta-author {
  color: var(--ds-text-secondary);
  font-weight: 500;
}

.meta-sep {
  opacity: 0.4;
}

.article-body {
  min-width: 0;
}

// =============== 回复流与回复写作区 ===============
.editorial-replies,
.editorial-composer {
  padding-top: 22px;
  border-top: 1px solid var(--ds-border-default);
}

.editorial-composer {
  margin-top: 22px;
}

.section-rule-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;

  h2 {
    margin: 0;
    font-size: 14.5px;
    font-weight: 700;
    color: var(--ds-text-primary);
  }
}

.section-count {
  font-size: 14px;
  font-weight: 600;
  color: var(--ds-text-muted);
}

.replies-hint {
  padding: 24px 0;
  font-size: 13px;
  color: var(--ds-text-muted);
}

.reply-stream {
  list-style: none;
  margin: 0;
  padding: 0;
}

.reply-entry {
  padding: 16px 0;
  border-bottom: 1px solid var(--ds-border-subtle);

  &.is-review-target {
    outline: 1px solid var(--ds-brand);
    outline-offset: 4px;
  }

  &:last-child {
    border-bottom: none;
  }
}

.reply-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 8px;
}

.reply-author {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;

  strong {
    color: var(--ds-text-primary);
    font-weight: 600;
  }
}

.reply-time {
  font-size: 11.5px;
  color: var(--ds-text-muted);
}

.reply-report-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border: none;
  background: transparent;
  color: var(--ds-text-muted);
  font-size: 12px;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 3px;
  opacity: 0.72;
  transition: opacity 140ms ease, color 140ms ease, background-color 140ms ease;

  &:hover {
    opacity: 1;
    color: var(--ds-error);
    background: var(--ds-error-bg);
  }
}

.composer-error {
  font-size: 12px;
  color: var(--ds-error);
}

.composer-actions {
  margin-top: 12px;
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

// =============== 发帖画布表单控件 ===============
.compose-error-banner {
  padding: 10px 14px;
  border-radius: 4px;
  border: 1px solid var(--ds-error-border);
  background: var(--ds-error-bg);
  color: var(--ds-error);
  font-size: 13px;
}

.compose-category-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.compose-field-label {
  font-size: 12.5px;
  color: var(--ds-text-muted);
  flex-shrink: 0;
}

.compose-cat-chips {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.compose-cat-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 30px;
  padding: 0 12px;
  border-radius: 4px;
  border: 1px solid var(--ds-border-subtle);
  background: transparent;
  color: var(--ds-text-secondary);
  font: inherit;
  font-size: 12.5px;
  cursor: pointer;
  transition: background-color 140ms ease, border-color 140ms ease, color 140ms ease;

  &:hover:not(:disabled) {
    background: var(--ds-hover-bg);
    color: var(--ds-text-primary);
  }

  &.is-selected {
    border-color: transparent;
    background: var(--ds-active-bg);
    color: var(--ds-text-primary);
    font-weight: 600;
  }

  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
}

.compose-cat-select {
  width: 220px;
}

.compose-title-wrap {
  display: flex;
  align-items: baseline;
  gap: 12px;
  border-bottom: 1px solid var(--ds-border-default);
  transition: border-color 160ms ease;

  &:focus-within {
    border-bottom-color: var(--ds-brand);
  }
}

.compose-title-input {
  flex: 1;
  min-width: 0;
  border: none;
  background: transparent;
  color: var(--ds-text-primary);
  font: inherit;
  font-size: 22px;
  font-weight: 700;
  line-height: 1.4;
  padding: 6px 0 12px;
  outline: none;

  &::placeholder {
    color: var(--ds-text-muted);
    font-weight: 500;
  }
}

.compose-title-count {
  font-size: 11.5px;
  color: var(--ds-text-muted);
  flex-shrink: 0;

  &.is-over {
    color: var(--ds-error);
    font-weight: 600;
  }
}

// =============== 窄窗自适应 ===============
@media (max-width: 1080px) {
  .editorial-scroll {
    padding-right: 54px;
  }
}

@media (max-width: 760px) {
  .editorial-scroll {
    padding-inline: 0;
    padding-right: 48px;
  }

  .detail-floating-dock {
    right: 12px;
    bottom: 16px;
  }

  .floating-icon-btn {
    width: 34px;
    height: 34px;
  }
  .forum-command-rail {
    flex-wrap: wrap;
    align-items: center;
  }

  .category-tabs {
    gap: 16px;
  }

  .chronicle-row {
    grid-template-columns: minmax(0, 1fr) 56px;
    gap: 14px;
    padding: 15px 10px;
  }

  .row-left {
    display: none;
  }

  .row-inline-cat {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    color: var(--ds-text-secondary);
    font-weight: 500;
  }

  .row-inline-time {
    display: inline;
  }

  .article-title {
    font-size: 20px;
  }
}
</style>
