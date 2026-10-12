<script setup lang="ts">
import {ArrowRight, KeyRound, LockKeyhole, Users, X} from '@lucide/vue'
import {computed, ref, watch} from 'vue'
import type {RoomResolveResult} from '../../types/multiplayer'
import {portText} from '../../utils/multiplayer'

// =============== 加入房间对话框 ===============
// 说明：父页面保存目标并执行 resolve/join；改目标立即清除旧预览，密码仅存在于此表单。
// 使用：mode=invite 时输入邀请码或房间 ID 查找；mode=room 时目标来自大厅列表，直接展示预览。
const props = defineProps<{show: boolean; mode: 'invite' | 'room'; target: string; resolved: RoomResolveResult | null; online: boolean; resolving: boolean; joining: boolean; busy: boolean}>()
const emit = defineEmits<{
  (event: 'update:show', value: boolean): void
  (event: 'update:target', value: string): void
  (event: 'preview'): void
  (event: 'join', password: string): void
}>()
const password = ref('')
watch(() => [props.target, props.online, props.show], () => { password.value = '' })
const full = computed(() => !!props.resolved && props.resolved.room.memberCount >= props.resolved.room.maxMembers)
const canJoin = computed(() => props.online && !!props.resolved && !props.busy && !props.resolving && !full.value && (!props.resolved.needPassword || !!password.value))
const title = computed(() => props.mode === 'invite' ? '邀请码加入' : '加入房间')
function submit(): void { if (canJoin.value) emit('join', password.value) }
function close(): void { if (!props.joining) emit('update:show', false) }
</script>

<template>
  <n-modal :show="show" :mask-closable="!joining" :close-on-esc="!joining" @update:show="emit('update:show', $event)">
    <!-- ~ 加入入口：邀请码查房与大厅选择共用预览；密码房只在需要时显示密码框 -->
    <section class="join-dialog" role="dialog" aria-modal="true" aria-labelledby="join-title">
      <header class="dialog-header">
        <div class="dialog-title"><KeyRound :size="18" /><h2 id="join-title">{{ title }}</h2></div>
        <n-button quaternary circle size="small" aria-label="关闭" :disabled="joining" @click="close"><template #icon><X :size="16" /></template></n-button>
      </header>

      <form v-if="mode === 'invite'" class="invite-form" @submit.prevent="emit('preview')">
        <label for="mp-invite">邀请码或房间 ID</label>
        <div class="invite-row">
          <n-input :value="target" class="type-mono" placeholder="XXXX-XXXX 或 r_…" :input-props="{id: 'mp-invite'}" :disabled="!online || joining" autofocus @update:value="emit('update:target', $event)" />
          <n-button attr-type="submit" secondary :disabled="!online || !target.trim() || busy || resolving" :loading="resolving">查找</n-button>
        </div>
        <p class="hint">有好友的邀请码？输入后查找房间，确认信息后加入。</p>
      </form>
      <p v-if="!online" class="offline-hint">连接大厅后即可查找和加入房间。</p>

      <!-- ~ 房间确认：展示房主、人数和密码要求，加入前明确网络地址共享 -->
      <div v-if="mode === 'room' && resolving && !resolved" class="preview-loading" role="status"><n-skeleton text :repeat="3" /><span>正在读取房间信息…</span></div>
      <div v-if="resolved" class="room-preview" aria-live="polite">
        <span class="preview-label">房间信息</span><h3>{{ resolved.room.name }}</h3>
        <dl>
          <div><dt>房主</dt><dd>{{ resolved.room.hostNickname }}</dd></div>
          <div><dt>成员</dt><dd><Users :size="14" /> {{ resolved.room.memberCount }} / {{ resolved.room.maxMembers }}</dd></div>
          <div><dt>游戏端口</dt><dd class="type-mono">{{ (resolved.room.ports ?? []).map(portText).join(' · ') || '—' }}</dd></div>
        </dl>
        <form @submit.prevent="submit">
          <div v-if="resolved.needPassword" class="password-field"><label for="mp-room-password"><LockKeyhole :size="13" />房间密码</label><n-input v-model:value="password" :input-props="{id: 'mp-room-password'}" type="password" show-password-on="click" placeholder="请输入房主设置的密码" :disabled="!online || joining" /></div>
          <p class="hint">加入后，房主可以看到你的网络地址，用于建立游戏连接。</p>
          <div class="dialog-actions">
            <n-button :disabled="joining" @click="close">取消</n-button>
            <n-button attr-type="submit" type="primary" :loading="joining" :disabled="!canJoin">{{ full ? '房间已满' : '加入房间' }}<template #icon><ArrowRight :size="16" /></template></n-button>
          </div>
        </form>
      </div>
      <p v-else-if="mode === 'room' && !resolving" class="hint">未能读取房间信息，请关闭后刷新大厅重试。</p>
    </section>
  </n-modal>
</template>

<style scoped lang="scss">
.join-dialog { width: 440px; max-width: calc(100vw - 32px); padding: 24px; border: 1px solid var(--ds-border-default); border-radius: 4px; background: var(--ds-bg-panel); color: var(--ds-text-primary); }
.dialog-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 20px; }
.dialog-title { display: flex; align-items: center; gap: 10px; }
.dialog-title svg { color: var(--ds-text-secondary); }
h2 { margin: 0; font-size: 16px; }
.invite-form { display: flex; flex-direction: column; gap: 8px; }
.invite-row { display: flex; gap: 8px; }
label { display: flex; gap: 6px; align-items: center; font-size: 12px; color: var(--ds-text-secondary); }
.hint { margin: 12px 0 0; color: var(--ds-text-secondary); font-size: 12px; line-height: 1.8; }
.invite-form .hint { margin-top: 4px; }
.offline-hint { margin: 14px 0 0; color: var(--ds-warning); font-size: 12px; }
.preview-loading { display: flex; flex-direction: column; gap: 12px; color: var(--ds-text-secondary); font-size: 12px; }
.room-preview { margin-top: 4px; }
.invite-form + .room-preview, .offline-hint + .room-preview { margin-top: 24px; padding-top: 20px; border-top: 1px solid var(--ds-border-default); }
.preview-label { font-size: 11px; color: var(--ds-text-secondary); }
h3 { margin: 6px 0 16px; font-size: 18px; overflow-wrap: anywhere; }
dl { margin: 0 0 20px; }
dl > div { display: flex; justify-content: space-between; gap: 16px; padding: 6px 0; font-size: 12px; border-bottom: 1px dashed var(--ds-border-subtle); }
dt { color: var(--ds-text-secondary); flex-shrink: 0; }
dd { display: flex; align-items: center; gap: 6px; text-align: right; margin: 0; overflow-wrap: anywhere; }
.password-field { display: flex; flex-direction: column; gap: 8px; }
.dialog-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 20px; }
</style>
