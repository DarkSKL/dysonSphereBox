<script setup lang="ts">
import {Copy, Link2, LogOut, Plug, PlugZap, Plus, RefreshCw, RotateCcw, Trash2, UserX, Users, X} from '@lucide/vue'
import {useDialog, useMessage} from 'naive-ui'
import {computed, reactive, ref, watch} from 'vue'
import {ClipboardSetText} from '../../../wailsjs/runtime/runtime'
import {useMultiplayerState} from '../../composables/multiplayer/useMultiplayerState'
import {
  codeText, connectionStateTexts, linkErrorText, linkPathTexts, linkStateTexts,
  type ClassifiedAddr, type LinkView, type PortDecl, type PortProto, type RoomMember,
} from '../../types/multiplayer'

definePage({meta: {layout: 'main', title: '联机', keepAlive: true}})

const message = useMessage()
const dialog = useDialog()
const mp = useMultiplayerState()
const {state, lastAction, resolved, online, room, isHost, pending} = mp

watch(lastAction, (action) => {
  if (!action) return
  if (action.ok) message.success(action.text)
  else message.error(action.text)
})

// =============== 连接与诊断 ===============
// 说明：连接状态、最近错误与本机网络诊断；诊断只是提示，能否连上以实际拨号为准。
const connectionText = computed(() => connectionStateTexts[state.value?.connection.state ?? 'offline'] ?? '未连接')
const connectionError = computed(() => {
  const code = state.value?.connection.lastErrorCode ?? 0
  return code ? codeText(code) : ''
})
const network = computed(() => state.value?.network)
const diagnostics = computed(() => {
  const net = network.value
  if (!net || !online.value) return []
  const items: string[] = []
  if (!net.firewallRule) items.push('Windows 防火墙已开启且未允许盒子入站，作为房主时成员可能连不上；请在防火墙中允许 DysonSphereBox（开发构建为 DysonSphereBox-dev.exe），或使用安装包安装')
  if (!net.ipv6) items.push('本机没有公网 IPv6 地址')
  if (!net.upnp) items.push('路由器未提供 UPnP 端口映射，公网直连需依赖打洞')
  return items
})

// 说明：本机对称 NAT（两个观测节点看到的 QUIC 映射不同）时常驻提示，不等加入房间；natConsistent 为 null 时不提示。
const selfSymmetric = computed(() => online.value && network.value?.natConsistent === false)
const natMappingsText = computed(() => (network.value?.natMappings ?? []).join(' / '))

// 说明：地址分组展示（本机上报、observed、房主下发），按 IP 标注私网/公网；房主地址只在成员侧有值。
const addrKindTexts: Record<string, string> = {public: '公网', private: '私网', loopback: '回环', cgnat: 'CGNAT', unknown: '未知'}
const addrGroups = computed(() => {
  const net = network.value
  if (!net || !online.value) return []
  const groups: {key: string; title: string; addrs: ClassifiedAddr[]; empty: string; errors: string[]}[] = [
    {key: 'reported', title: '本机上报的地址', addrs: net.reportedAddrs ?? [], empty: '尚未上报', errors: []},
    {key: 'observed', title: 'observed 地址（观测节点看到的本机地址）', addrs: net.observed ?? [], empty: '未拿到', errors: net.observeErrors ?? []},
  ]
  if (room.value && !isHost.value) groups.push({key: 'host', title: '房主下发的地址', addrs: net.hostAddrs ?? [], empty: '房主没有下发地址', errors: []})
  return groups
})
const hostNoPublic = computed(() => !!network.value?.hostNoPublic && !!room.value && !isHost.value)

async function copy(text: string) {
  try {
    await ClipboardSetText(text)
    message.success('已复制')
  } catch {
    message.error('复制失败')
  }
}

// =============== 大厅 ===============
const keyword = ref('')
const lobby = computed(() => state.value?.lobby.list ?? [])

// =============== 创建房间 ===============
// 说明：端口由房主手填，不内置游戏预设；端口表创建后不可修改。
const createForm = reactive({name: '', visibility: 'public', password: '', maxMembers: 8})
const ports = ref<PortDecl[]>([{proto: 'tcp', port: 8469, count: 1, label: '游戏'} as PortDecl])
const protoOptions = [{label: 'TCP', value: 'tcp'}, {label: 'UDP', value: 'udp'}, {label: 'TCP + UDP', value: 'both'}]
const visibilityOptions = [{label: '公开（大厅可见）', value: 'public'}, {label: '私密（仅邀请码）', value: 'private'}]
const maxPorts = computed(() => state.value?.session?.limits.maxPorts || 8)
const maxMembers = computed(() => state.value?.session?.limits.maxMembers || 16)

function addPort() {
  if (ports.value.length < maxPorts.value) ports.value.push({proto: 'udp', port: 0, count: 1, label: ''} as PortDecl)
}

const createError = computed(() => {
  const name = createForm.name.trim()
  if (name.length < 1 || name.length > 32) return '房间名需 1–32 个字符'
  if (createForm.password && (createForm.password.length < 4 || createForm.password.length > 32)) return '密码需 4–32 个字符'
  if (ports.value.length === 0) return '至少声明一个端口'
  for (const [index, decl] of ports.value.entries()) {
    if (!decl.port || decl.port < 1 || decl.port > 65535) return `第 ${index + 1} 条端口无效`
    if (!decl.count || decl.count < 1 || decl.count > 32 || decl.port + decl.count - 1 > 65535) return `第 ${index + 1} 条端口数量无效`
    if ((decl.label ?? '').length > 16) return `第 ${index + 1} 条标签过长`
  }
  return ''
})

async function doCreate() {
  await mp.create({...createForm, name: createForm.name.trim(), ports: ports.value.map((item) => ({...item}) as PortDecl)})
}

// 说明：本机对称 NAT 时创建前确认一次；确认后仍可创建（局域网成员仍能连入）。
function submitCreate() {
  if (createError.value) return
  if (!selfSymmetric.value) {
    void doCreate()
    return
  }
  dialog.warning({
    title: '你的网络无法打洞',
    content: '你的网络无法打洞，跨网络的成员可能无法连入（局域网成员仍可连入）。仍要创建房间吗？',
    positiveText: '仍然创建',
    negativeText: '取消',
    onPositiveClick: () => {
      void doCreate()
    },
  })
}

// =============== 加入房间 ===============
// 说明：先 room.resolve 预览，再按需输入密码加入；邀请码只去首尾空白，大小写由服务器比较。
const joinForm = reactive({target: '', password: ''})
const joinIsInvite = computed(() => !joinForm.target.trim().startsWith('r_'))

async function preview(roomId = '') {
  if (roomId) joinForm.target = roomId
  const target = joinForm.target.trim()
  if (!target) return
  await mp.resolve(joinIsInvite.value ? '' : target, joinIsInvite.value ? target : '')
}

async function submitJoin() {
  const target = joinForm.target.trim()
  if (!target) return
  const ok = await mp.join(joinIsInvite.value ? '' : target, joinIsInvite.value ? target : '', joinForm.password)
  if (ok) {
    joinForm.password = ''
    resolved.value = null
  }
}

// =============== 房间 ===============
const snapshot = computed(() => room.value?.snapshot)
const linksByUser = computed(() => new Map((state.value?.links ?? []).map((link) => [link.userId, link])))
const selfLink = computed<LinkView | undefined>(() => (!isHost.value ? state.value?.links?.[0] : undefined))
const localPorts = computed(() => state.value?.localPorts ?? [])
const targets = computed(() => network.value?.targets ?? [])

function portText(decl: PortDecl): string {
  const range = decl.count > 1 ? `${decl.port}-${decl.port + decl.count - 1}` : `${decl.port}`
  return `${decl.proto === 'both' ? 'TCP+UDP' : decl.proto.toUpperCase()} ${range}`
}

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
const protoLabel = (proto: PortProto | string) => (proto === 'tcp' ? 'TCP' : proto === 'udp' ? 'UDP' : 'TCP+UDP')
</script>

<template>
  <main class="mp-page" aria-labelledby="mp-title">
    <!-- ~ 页头：连接状态 -->
    <!-- * 说明：控制链路状态与连接/断开；进入页面自动连接，离开页面不断开 -->
    <!-- * 边界：断开会同时离开或关闭当前房间 -->
    <header class="mp-header">
      <div class="title-lockup">
        <Users :size="22" />
        <h1 id="mp-title">联机</h1>
        <span class="state-label" :class="`state-${state?.connection.state ?? 'offline'}`"><i />{{ connectionText }}</span>
        <span v-if="state?.account.nickname" class="meta">{{ state.account.nickname }}</span>
      </div>
      <div class="actions">
        <n-button v-if="state?.connection.state === 'offline'" size="small" type="primary" @click="mp.connect()">
          <template #icon><PlugZap :size="16" /></template>连接
        </n-button>
        <n-button v-else size="small" quaternary @click="mp.disconnect()">
          <template #icon><Plug :size="16" /></template>断开
        </n-button>
      </div>
    </header>

    <p v-if="connectionError && !online" class="notice notice-error" role="alert">{{ connectionError }}</p>
    <p v-if="!state?.account.loggedIn" class="notice notice-warning">联机需要先登录账号</p>
    <!-- * 对称 NAT：本机两个观测映射不同，常驻提示并列出两个映射 -->
    <div v-if="selfSymmetric" class="notice notice-warning" role="status">
      <p>你的网络无法打洞：作为房主时跨网络成员无法连入；作为成员仍会尝试打洞，但成功率很低。建议换网络，或让对方当房主</p>
      <p v-if="natMappingsText" class="nat-mappings">观测节点看到的映射：<code>{{ natMappingsText }}</code></p>
    </div>
    <ul v-if="diagnostics.length" class="diagnostics" aria-label="网络诊断">
      <li v-for="item in diagnostics" :key="item">{{ item }}</li>
    </ul>
    <!-- * 网络地址：本机上报、observed、房主下发，按 IP 标注私网/公网；用于判断地址能否从外网访问 -->
    <p v-if="hostNoPublic" class="notice notice-warning" role="alert">房主没有可从外网访问的地址：检查服务器 observer-public-ip 配置、安全组 UDP 4001，或房主路由器 UPnP</p>
    <details v-if="addrGroups.length" class="addr-diagnostics" :open="hostNoPublic">
      <summary>网络地址</summary>
      <div v-for="group in addrGroups" :key="group.key" class="addr-group">
        <h3>{{ group.title }}</h3>
        <ul v-if="group.addrs.length" class="addr-list">
          <li v-for="item in group.addrs" :key="item.addr" :title="item.addr">
            <span class="addr-kind" :class="`kind-${item.kind}`">{{ addrKindTexts[item.kind] ?? item.kind }}</span>
            <code>{{ item.display }}</code>
          </li>
        </ul>
        <p v-else class="hint">{{ group.empty }}</p>
        <p v-for="error in group.errors" :key="error" class="hint addr-error">{{ error }}</p>
      </div>
    </details>

    <!-- ~ 房间 -->
    <!-- * 说明：成员与链路状态、本地端口（成员）、目标地址探测（房主）、邀请/踢人/退出/关房 -->
    <section v-if="snapshot" class="mp-section" aria-labelledby="room-title">
      <div class="section-heading">
        <h2 id="room-title">{{ snapshot.name }}</h2>
        <span class="meta">{{ isHost ? '房主' : '成员' }} · {{ snapshot.members.length + 1 }}/{{ snapshot.maxMembers }} · {{ snapshot.visibility === 'public' ? '公开' : '私密' }}</span>
        <div class="actions">
          <template v-if="isHost">
            <n-button size="small" secondary :loading="pending.has('invite')" @click="mp.invite()"><template #icon><Link2 :size="15" /></template>邀请码</n-button>
            <n-button size="small" type="error" secondary :loading="pending.has('close')" @click="mp.close()"><template #icon><X :size="15" /></template>关闭房间</n-button>
          </template>
          <template v-else>
            <n-button v-if="selfLink && selfLink.state !== 'connected'" size="small" secondary @click="mp.retry()"><template #icon><RotateCcw :size="15" /></template>重试连接</n-button>
            <n-button size="small" secondary :loading="pending.has('leave')" @click="mp.leave()"><template #icon><LogOut :size="15" /></template>退出房间</n-button>
          </template>
        </div>
      </div>

      <div v-if="isHost && room?.invite" class="invite-row">
        <span>邀请码</span><code class="type-mono">{{ room.invite.code }}</code>
        <n-button size="tiny" quaternary aria-label="复制邀请码" @click="copy(room.invite.code)"><template #icon><Copy :size="14" /></template></n-button>
        <span class="meta">有效至 {{ new Date(room.invite.expiresAtMs).toLocaleString('zh-CN', {hour12: false}) }}</span>
      </div>

      <!-- * 成员：本地端口；把地址填进游戏即可 -->
      <div v-if="!isHost" class="block">
        <h3>本地端口</h3>
        <p class="hint">在游戏中连接下面的地址；房主游戏端口经数据链路透明转发。</p>
        <ul class="port-list">
          <li v-for="item in localPorts" :key="item.proto + item.address">
            <span class="port-proto">{{ protoLabel(item.proto) }}</span>
            <span>{{ item.label || '端口' }}</span>
            <code class="type-mono">{{ item.address }}</code>
            <n-button size="tiny" quaternary :aria-label="`复制 ${item.address}`" @click="copy(item.address.split('-')[0])"><template #icon><Copy :size="14" /></template></n-button>
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
      <div v-else class="block">
        <h3>游戏端口</h3>
        <ul class="port-list">
          <li v-for="(decl, index) in snapshot.ports" :key="index">
            <span class="port-proto">{{ portText(decl) }}</span>
            <span>{{ decl.label || '端口' }}</span>
            <span class="meta">{{ targets[index] ? targetText(targets[index].status, targets[index].address) : '尚未探测' }}</span>
          </li>
        </ul>
      </div>

      <div class="block">
        <h3>成员</h3>
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
                <n-button v-if="isHost" size="tiny" quaternary type="error" :aria-label="`踢出 ${member.nickname}`" :loading="pending.has('kick')" @click="mp.kick(member.userId)"><template #icon><UserX :size="14" /></template></n-button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <template v-else>
      <!-- ~ 大厅 -->
      <section class="mp-section" aria-labelledby="lobby-title">
        <div class="section-heading">
          <h2 id="lobby-title">大厅</h2>
          <span class="meta">{{ state?.lobby.total ?? 0 }} 个公开房间</span>
          <div class="actions">
            <n-input v-model:value="keyword" size="small" placeholder="搜索房间名" clearable :disabled="!online" style="width: 180px" @keydown.enter="mp.refreshLobby(keyword)" />
            <n-button size="small" quaternary :disabled="!online" :loading="pending.has('lobbyList')" aria-label="刷新大厅" @click="mp.refreshLobby(keyword)"><template #icon><RefreshCw :size="15" /></template></n-button>
          </div>
        </div>
        <table v-if="lobby.length" class="member-table">
          <thead><tr><th scope="col">房间</th><th scope="col">房主</th><th scope="col">人数</th><th scope="col">端口</th><th scope="col"><span class="sr-only">操作</span></th></tr></thead>
          <tbody>
            <tr v-for="item in lobby" :key="item.roomId">
              <td>{{ item.name }}<span v-if="item.hasPassword" class="tag">密码</span></td>
              <td>{{ item.hostNickname }}</td>
              <td>{{ item.memberCount }}/{{ item.maxMembers }}</td>
              <td class="meta">{{ (item.ports ?? []).map(portText).join('，') }}</td>
              <td class="cell-actions"><n-button size="tiny" secondary @click="preview(item.roomId)">加入</n-button></td>
            </tr>
          </tbody>
        </table>
        <p v-else class="hint">{{ online ? '暂无公开房间' : '连接后显示大厅' }}</p>
      </section>

      <div class="split">
        <!-- ~ 加入房间 -->
        <section class="mp-section" aria-labelledby="join-title">
          <div class="section-heading"><h2 id="join-title">加入房间</h2></div>
          <div class="fields">
            <label class="field"><span>邀请码或房间 ID</span><n-input v-model:value="joinForm.target" placeholder="XXXX-XXXX 或 r_…" :disabled="!online" @keydown.enter="preview()" /></label>
            <n-button secondary :disabled="!online || !joinForm.target.trim()" :loading="pending.has('resolveRoom')" @click="preview()">预览</n-button>
          </div>
          <div v-if="resolved" class="preview">
            <p><strong>{{ resolved.room.name }}</strong> <span class="meta">房主 {{ resolved.room.hostNickname }} · {{ resolved.room.memberCount }}/{{ resolved.room.maxMembers }}</span></p>
            <p class="meta">端口：{{ (resolved.room.ports ?? []).map(portText).join('，') }}</p>
            <label v-if="resolved.needPassword" class="field"><span>房间密码</span><n-input v-model:value="joinForm.password" type="password" show-password-on="click" @keydown.enter="submitJoin()" /></label>
            <p class="hint">加入后房主可以看到你的网络地址，用于建立直连。</p>
            <n-button type="primary" :loading="pending.has('joinRoom')" @click="submitJoin()">加入</n-button>
          </div>
        </section>

        <!-- ~ 创建房间 -->
        <!-- * 说明：端口声明表单；同一端口号同时声明 TCP 与 UDP 时成员端使用同一个本地端口号 -->
        <section class="mp-section" aria-labelledby="create-title">
          <div class="section-heading"><h2 id="create-title">创建房间</h2></div>
          <div class="fields-grid">
            <label class="field"><span>房间名</span><n-input v-model:value="createForm.name" maxlength="32" :disabled="!online" /></label>
            <label class="field"><span>可见性</span><n-select v-model:value="createForm.visibility" :options="visibilityOptions" :disabled="!online" /></label>
            <label class="field"><span>密码（可选）</span><n-input v-model:value="createForm.password" type="password" show-password-on="click" maxlength="32" :disabled="!online" /></label>
            <label class="field"><span>人数上限（含房主）</span><n-input-number v-model:value="createForm.maxMembers" :min="2" :max="maxMembers" :disabled="!online" /></label>
          </div>
          <h3>游戏端口</h3>
          <div v-for="(decl, index) in ports" :key="index" class="port-row">
            <n-select v-model:value="decl.proto" :options="protoOptions" size="small" style="width: 120px" :aria-label="`第 ${index + 1} 条协议`" />
            <n-input-number v-model:value="decl.port" size="small" :min="1" :max="65535" placeholder="端口" :show-button="false" style="width: 110px" :aria-label="`第 ${index + 1} 条端口`" />
            <n-input-number v-model:value="decl.count" size="small" :min="1" :max="32" style="width: 96px" :aria-label="`第 ${index + 1} 条连续端口数`" />
            <n-input v-model:value="decl.label" size="small" maxlength="16" placeholder="标签" style="width: 120px" :aria-label="`第 ${index + 1} 条标签`" />
            <n-button size="small" quaternary :disabled="ports.length <= 1" :aria-label="`删除第 ${index + 1} 条端口`" @click="ports.splice(index, 1)"><template #icon><Trash2 :size="14" /></template></n-button>
          </div>
          <n-button size="small" dashed :disabled="ports.length >= maxPorts" @click="addPort()"><template #icon><Plus :size="14" /></template>添加端口</n-button>
          <p v-if="createForm.visibility === 'public'" class="hint">公开房间的加入者可以看到房主的网络地址。</p>
          <p v-if="createError && createForm.name" class="notice notice-warning">{{ createError }}</p>
          <n-button type="primary" :disabled="!online || !!createError" :loading="pending.has('createRoom')" @click="submitCreate()">创建</n-button>
        </section>
      </div>
    </template>
  </main>
</template>

<style scoped lang="scss">
.mp-page {
  width: 100%;
  max-width: 1680px;
  min-height: 100%;
  margin-inline: auto;
  padding: var(--ds-page-padding-top) var(--ds-page-padding-x) var(--ds-page-padding-bottom);
  color: var(--ds-text-primary);
}

.mp-header, .title-lockup, .actions, .section-heading, .invite-row, .fields, .port-row { display: flex; align-items: center; }
.mp-header { gap: 14px; padding-bottom: 16px; border-bottom: 1px solid var(--ds-border-subtle); }
.title-lockup { gap: 10px; }
.title-lockup > svg { color: var(--ds-brand); }
h1 { margin: 0; font-size: 20px; }
h2 { margin: 0; font-size: 15px; }
h3 { margin: 16px 0 8px; font-size: 13px; color: var(--ds-text-secondary); }
.actions { gap: 6px; margin-left: auto; }
.meta, .hint { color: var(--ds-text-secondary); font-size: 12px; }
.hint { margin: 6px 0; }

.state-label { display: inline-flex; align-items: center; gap: 7px; padding: 3px 8px; border: 1px solid var(--ds-border-default); border-radius: 4px; font-size: 11px; }
.state-label i { width: 6px; height: 6px; border-radius: 50%; background: var(--ds-text-muted); }
.state-online { color: var(--ds-success); background: var(--ds-success-bg); border-color: var(--ds-success-border); }
.state-online i { background: var(--ds-success); }
.state-connecting, .state-reconnecting { color: var(--ds-warning); background: var(--ds-warning-bg); border-color: var(--ds-warning-border); }
.state-connecting i, .state-reconnecting i { background: var(--ds-warning); }

.notice { margin: 12px 0 0; padding: 8px 12px; border: 1px solid; border-radius: 4px; font-size: 12px; }
.notice-error { color: var(--ds-error); background: var(--ds-error-bg); border-color: var(--ds-error-border); }
.notice-warning { color: var(--ds-warning); background: var(--ds-warning-bg); border-color: var(--ds-warning-border); }
.notice p { margin: 0; }
.notice .nat-mappings { margin-top: 4px; }
.diagnostics { margin: 12px 0 0; padding-left: 18px; color: var(--ds-text-secondary); font-size: 12px; line-height: 1.8; }
.addr-diagnostics { margin: 12px 0 0; font-size: 12px; color: var(--ds-text-secondary); }
.addr-diagnostics summary { cursor: pointer; }
.addr-group h3 { margin: 10px 0 4px; font-size: 12px; }
.addr-list { display: flex; flex-wrap: wrap; gap: 6px 14px; margin: 0; padding: 0; list-style: none; }
.addr-list li { display: inline-flex; align-items: center; gap: 6px; }
.addr-kind { padding: 0 5px; border: 1px solid var(--ds-border-default); border-radius: 3px; font-size: 11px; }
.kind-public { color: var(--ds-success); border-color: var(--ds-success-border); }
.kind-private, .kind-cgnat { color: var(--ds-warning); border-color: var(--ds-warning-border); }
.addr-error { margin: 4px 0 0; word-break: break-all; }

.mp-section { padding: 18px 0; border-bottom: 1px solid var(--ds-border-subtle); }
.section-heading { gap: 12px; margin-bottom: 10px; }
.split { display: grid; grid-template-columns: repeat(auto-fit, minmax(360px, 1fr)); gap: 0 40px; }

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

.fields { gap: 8px; align-items: flex-end; }
.fields .field { flex: 1; }
.fields-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px 14px; }
.field { display: flex; flex-direction: column; gap: 5px; font-size: 12px; color: var(--ds-text-secondary); }
.preview { display: flex; flex-direction: column; align-items: flex-start; gap: 6px; margin-top: 12px; padding-top: 12px; border-top: 1px dashed var(--ds-border-subtle); }
.preview p { margin: 0; }
.port-row { gap: 6px; margin-bottom: 6px; }
.mp-section > .n-button { margin-top: 10px; }
</style>
