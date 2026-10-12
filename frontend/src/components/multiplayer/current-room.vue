<script setup lang="ts">
import {Copy, Link2, LogOut, RotateCcw, UserX, X} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {computed} from 'vue'
import {ClipboardSetText} from '../../../wailsjs/runtime/runtime'
import {linkErrorText, linkPathTexts, linkStateTexts, type LinkView, type MultiplayerState, type RoomMember} from '../../types/multiplayer'
import {portText, protoLabel} from '../../utils/multiplayer'
// =============== 当前房间与用户操作 ===============
// 说明：呈现真实快照与链路，房间变更交给页面；复制仅写剪贴板，不启动游戏。
const props = defineProps<{state: MultiplayerState; pending: Set<string>; online: boolean; busy: boolean}>()
const emit = defineEmits<{(event: 'invite' | 'close' | 'leave' | 'retry'): void; (event: 'kick', userId: number): void}>()
const room = computed(() => props.state.room)
const isHost = computed(() => room.value?.role === 'host')
const message = useMessage()
// 说明：控制链路离线时不复制旧会话的邀请码或连接地址；在线时仅写入剪贴板。
async function copy(text: string): Promise<void> {
  if (!props.online) return
  try { await ClipboardSetText(text); message.success('已复制') }
  catch { message.error('复制失败，请手动复制地址') }
}
const snapshot = computed(() => props.state.room?.snapshot)
const linksByUser = computed(() => new Map((props.state?.links ?? []).map((link) => [link.userId, link])))
const selfLink = computed<LinkView | undefined>(() => (!isHost.value ? props.state?.links?.[0] : undefined))
const localPorts = computed(() => props.state?.localPorts ?? [])
const targets = computed(() => props.state.network?.targets ?? [])

type LinkLike = Pick<LinkView, 'state' | 'path' | 'ipFamily' | 'rttMs' | 'error'>

// 说明：房主侧优先用投影 links，其次用快照中成员最近一次 link.report。
const memberLink = (member: RoomMember): LinkLike | undefined => linksByUser.value.get(member.userId) ?? member.link ?? undefined

function linkText(link?: LinkLike): string {
  if (!link) return '等待上报'
  const parts = [linkStateTexts[link.state] ?? link.state]
  if (link.path) parts.push(linkPathTexts[link.path] ?? link.path)
  if (link.ipFamily) parts.push(link.ipFamily === 'v6' ? 'IPv6' : 'IPv4')
  if (link.rttMs) parts.push(`${link.rttMs} ms`)
  return parts.join(' · ')
}

function targetText(status: string, address: string): string {
  if (status === 'ok') return `已连通 ${address}`
  if (status === 'refused') return '未检测到监听'
  return '尚未探测'
}

const selfLinkError = computed(() => (selfLink.value?.error ? linkErrorText(selfLink.value.error, selfLink.value.natBlocked, true) : null))

</script>
<template>
  <!-- ~ 当前房间：成员与实际数据链路；退出与关闭不影响用户游戏文件 -->
    <section v-if="snapshot" class="room-section" aria-labelledby="room-title">
      <div class="section-heading room-heading">
        <div><p class="eyebrow">当前房间</p><h2 id="room-title">{{ snapshot.name }}</h2></div>
        <span class="meta">{{ isHost ? '房主' : '成员' }} · {{ snapshot.members.length + 1 }}/{{ snapshot.maxMembers }} · {{ snapshot.visibility === 'public' ? '公开' : '私密' }}</span>
        <div class="actions">
          <template v-if="isHost">
            <n-button size="small" secondary :disabled="!online || busy" :loading="pending.has('invite')" @click="emit('invite')"><template #icon><Link2 :size="15" /></template>邀请码</n-button>
            <n-button size="small" type="error" secondary :disabled="!online || busy" :loading="pending.has('close')" @click="emit('close')"><template #icon><X :size="15" /></template>关闭房间</n-button>
          </template>
          <template v-else>
            <n-button v-if="selfLink && selfLink.state !== 'connected'" size="small" secondary :disabled="!online" :loading="pending.has('retry')" @click="emit('retry')"><template #icon><RotateCcw :size="15" /></template>重试连接</n-button>
            <n-button size="small" secondary :disabled="!online || busy" :loading="pending.has('leave')" @click="emit('leave')"><template #icon><LogOut :size="15" /></template>退出房间</n-button>
          </template>
        </div>
      </div>

      <div v-if="isHost && room?.invite" class="invite-row">
        <span>邀请码</span><code class="type-mono">{{ room.invite.code }}</code>
        <n-button size="tiny" quaternary :disabled="!online" aria-label="复制邀请码" @click="copy(room.invite.code)"><template #icon><Copy :size="14" /></template></n-button>
        <span class="meta">有效至 {{ new Date(room.invite.expiresAtMs).toLocaleString('zh-CN', {hour12: false}) }}</span>
      </div>

      <!-- * 成员：本地端口；把地址填进游戏即可 -->
      <div v-if="!isHost" class="block">
        <h3>在游戏中连接</h3>
        <p class="hint">在游戏中连接下面的地址；房主游戏端口经数据链路透明转发。</p>
        <p v-if="!localPorts.length" class="hint">正在分配本地连接地址，请稍候…</p><ul class="port-list">
          <li v-for="item in localPorts" :key="item.proto + item.address">
            <span class="port-proto">{{ protoLabel(item.proto) }}</span>
            <span>{{ item.label || '端口' }}</span>
            <code class="type-mono">{{ item.address }}</code>
            <n-button size="tiny" quaternary :disabled="!online" :aria-label="`复制 ${item.address}`" @click="copy(item.address.split('-')[0])"><template #icon><Copy :size="14" /></template></n-button>
          </li>
        </ul>
        <p class="link-line" :class="`link-${selfLink?.state ?? 'connecting'}`">数据链路：{{ linkText(selfLink) }}</p>
        <p v-if="selfLinkError" class="notice notice-warning">{{ selfLinkError.title }}：{{ selfLinkError.hint }}</p>
        <!-- * 最近一次直连/打洞的真实错误（只在本机投影中），默认折叠 -->
        <details v-if="selfLink?.detail && selfLink.state !== 'connected'" class="link-detail">
          <summary>最近一次拨号错误</summary>
          <code>{{ selfLink.detail }}</code>
        </details>
      </div>

      <!-- * 房主：游戏端口与目标地址探测 -->
      <div v-else class="block"><p class="hint">先在游戏中开启联机，再分享邀请码。成员可通过盒子连接你的游戏端口。</p>
        <h3>房主游戏端口</h3>
        <ul class="port-list">
          <li v-for="(decl, index) in snapshot.ports" :key="index">
            <span class="port-proto">{{ portText(decl) }}</span>
            <span>{{ decl.label || '端口' }}</span>
            <span class="meta">{{ targets[index] ? targetText(targets[index].status, targets[index].address) : '尚未探测' }}</span>
          </li>
        </ul>
      </div>

      <div class="block">
        <h3>房间成员 <span class="meta">{{ snapshot.members.length + 1 }} / {{ snapshot.maxMembers }}</span></h3>
        <table class="member-table">
          <thead><tr><th scope="col">昵称</th><th scope="col">数据链路</th><th scope="col"><span class="sr-only">操作</span></th></tr></thead>
          <tbody>
            <tr>
              <td>{{ snapshot.host.nickname }} <span class="tag">房主</span></td>
              <td class="meta">—</td>
              <td />
            </tr>
            <tr v-for="member in snapshot.members" :key="member.userId">
              <td>{{ member.nickname }}<span v-if="member.userId === state?.account.userId" class="tag">我</span></td>
              <td>
                <span :class="`link-${memberLink(member)?.state ?? 'connecting'}`">{{ linkText(memberLink(member)) }}</span>
                <span v-if="memberLink(member)?.error" class="meta"> · {{ linkErrorText(memberLink(member)?.error ?? 0)?.title }}</span>
              </td>
              <td class="cell-actions">
                <n-button v-if="isHost" size="tiny" quaternary type="error" :aria-label="`踢出 ${member.nickname}`" :disabled="!online || busy" :loading="pending.has('kick')" @click="emit('kick', member.userId)"><template #icon><UserX :size="14" /></template></n-button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

</template>
<style scoped lang="scss">
.room-section { padding: 28px; background: var(--ds-bg-panel); border: 1px solid var(--ds-border-default); border-radius: 4px; }
.section-heading, .actions { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.room-heading { padding-bottom: 24px; border-bottom: 1px solid var(--ds-border-default); }
.actions { margin-left: auto; gap: 8px; }
h2 { margin: 0; font-size: 22px; overflow-wrap: anywhere; }
h3 { margin: 0 0 12px; font-size: 14px; font-weight: 700; }
.eyebrow { margin: 0 0 6px; color: var(--ds-text-secondary); font-size: 12px; }
.block { margin-top: 28px; }
.meta, .hint { color: var(--ds-text-secondary); font-size: 12px; line-height: 1.8; }
.hint { margin: 0 0 12px; }
.invite-row { display: flex; align-items: center; flex-wrap: wrap; margin-top: 20px; padding: 16px; background: var(--ds-bg-panel-elevated); }
.notice { padding: 12px; font-size: 13px; border-radius: 4px; }
.notice-warning { color: var(--ds-warning); background: var(--ds-warning-bg); }
.invite-row { gap: 8px; font-size: 13px; }
code { padding: 2px 6px; border: 1px solid var(--ds-border-subtle); border-radius: 4px; font-family: var(--ds-font-mono); font-size: 12px; }
.port-list { margin: 0; padding: 0; list-style: none; }
.port-list li { display: flex; align-items: center; gap: 10px; padding: 6px 0; font-size: 13px; border-bottom: 1px dashed var(--ds-border-subtle); }
.port-proto { min-width: 72px; color: var(--ds-text-secondary); font-family: var(--ds-font-mono); font-size: 12px; }
.link-line { margin: 10px 0 0; font-size: 13px; }
.link-detail { margin: 6px 0 0; color: var(--ds-text-secondary); font-size: 12px; }
.link-detail summary { cursor: pointer; }
.link-detail code { display: block; margin-top: 4px; white-space: pre-wrap; word-break: break-all; font-family: var(--ds-font-mono, monospace); }
.link-connected { color: var(--ds-success); }
.link-reconnecting, .link-connecting { color: var(--ds-warning); }
.link-failed { color: var(--ds-error); }

.member-table { width: 100%; border-collapse: collapse; font-size: 13px; }
.member-table th { padding: 6px 8px; color: var(--ds-text-secondary); font-weight: 500; text-align: left; border-bottom: 1px solid var(--ds-border-default); }
.member-table td { padding: 7px 8px; border-bottom: 1px solid var(--ds-border-subtle); }
.cell-actions { width: 1%; white-space: nowrap; text-align: right; }
.tag { margin-left: 6px; padding: 1px 5px; border: 1px solid var(--ds-border-default); border-radius: 3px; color: var(--ds-text-secondary); font-size: 11px; }
.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); }


@media (max-width: 800px) { .room-section { padding: 18px; } .port-list li { flex-wrap: wrap; } .member-table td { overflow-wrap: anywhere; } }
</style>
