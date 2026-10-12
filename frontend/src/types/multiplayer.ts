// =============== 联机类型与码值文案 ===============
// 说明：复用 Wails 生成的 multiplayer 模型，并集中维护联机结果码、原因码与数据链路失败码的中文文案。
// 使用：联机 services / composables / 页面按 code 映射提示，未收录的码直接展示服务器 msg。
// 边界：码值以协议设计错误码.md 为准，前端不新增码；本地码 209xx 只由 Go 后端产生。

import {multiplayer, p2p} from '../../wailsjs/go/models'

export type MultiplayerState = multiplayer.State
export type MultiplayerActionResult = multiplayer.ActionResult
export type PortDecl = multiplayer.PortDecl
export type RoomSummary = multiplayer.RoomSummary
export type RoomMember = multiplayer.Member
export type LinkView = multiplayer.LinkView
export type ClassifiedAddr = p2p.ClassifiedAddr

export type PortProto = 'tcp' | 'udp' | 'both'

export interface RoomResolveResult {
  readonly room: RoomSummary
  readonly needPassword: boolean
}

// =============== 大厅与创建表单契约 ===============
// 说明：页面传递搜索分页与创建意图；请求仍由 services 转交 Go，端口以服务器限制为准。
export interface LobbyQuery {
  keyword: string
  page: number
  pageSize: number
  sort: 'createdAt' | 'memberCount'
}

export interface RoomCreateForm {
  name: string
  visibility: 'public' | 'private'
  password: string
  maxMembers: number
  ports: PortDecl[]
}

// 说明：结果码、会话终止原因、关房/离开原因与握手拒绝码（7、10001、HTTP 404）共用一张表。
const codeTexts: Record<number, string> = {
  7: '登录已失效，请重新登录',
  404: '服务器未开启联机功能',
  10001: '需要先修改密码',
  20001: '帧无法解析（客户端缺陷）',
  20002: '不认识的协议码（客户端缺陷）',
  20003: '参数错误',
  20006: '协议版本不支持，请更新盒子',
  20007: '盒子版本过低，请更新',
  20008: '会话标识冲突，正在重试',
  20009: '操作过于频繁，请稍后再试',
  20099: '服务器内部错误，请稍后再试',
  20101: '登录已失效，请重新登录',
  20102: '账户已被禁用',
  20103: '需要修改密码',
  20104: '账户已在其他盒子上线',
  20105: '服务器维护中，稍后自动重连',
  20201: '房间不存在',
  20202: '房间已关闭',
  20203: '房间人数已满',
  20204: '房间密码错误',
  20205: '该房间需要密码',
  20206: '你已被移出该房间，无法再次加入',
  20207: '你已在一个房间中',
  20208: '没有权限',
  20209: '不在该房间中',
  20210: '邀请码无效或已过期',
  20211: '端口声明不合法',
  20212: '房间名不合法',
  20213: '人数上限不合法',
  20214: '不能踢出自己',
  20215: '成员不存在',
  20216: '私密房间只能通过邀请码加入',
  20220: '房主关闭了房间',
  20221: '房主已离线',
  20222: '服务器维护',
  20223: '房主账户在其他盒子上线',
  20224: '房主账户已失效',
  20230: '成员主动退出',
  20231: '被房主移出房间',
  20232: '成员已离线',
  20233: '成员账户在其他盒子上线',
  20234: '成员账户已失效',
  20240: '发起者在提交前离线',
  20241: '房主未就绪，加入已取消',
  20242: '打洞协调失败',
  20301: '房主尚未上报地址',
  20302: '打洞请求过于频繁',
  20303: '地址格式不合法或数量超限',
  20304: '尚未上报本机地址，请稍后再试',
  20305: '尚未上报本机地址，无法打洞',
  20901: '请求超时',
  20902: '控制链路已断开，请求未完成',
  20903: '控制链路未连接',
}

// 说明：数据链路失败码附带可操作的提示方向（方案第 7.5 节）。
const linkErrorTexts: Record<number, {title: string; hint: string}> = {
  20401: {title: '双方 IP 协议族不匹配', hint: '双方都开启 IPv6，或使用同一种网络'},
  20402: {title: 'NAT 映射随目标变化，直连升级失败', hint: '可换网络重试，或请管理员允许中继回退'},
  20403: {title: '运营商级 NAT（CGNAT）', hint: '联系运营商获取公网 IP，或开启 IPv6'},
  20404: {title: '数据链路建立超时', hint: '检查防火墙、UDP 和服务器中继设置'},
  20405: {title: '房主的 Windows 防火墙可能拦截了盒子', hint: '请房主在防火墙中允许 DysonSphereBox（开发构建为 DysonSphereBox-dev.exe），或使用安装包安装'},
  20406: {title: '房主拒绝了连接', hint: '请退出后重新加入；若反复出现请反馈'},
  20407: {title: '房主游戏端口拒绝连接', hint: '房主游戏未开启或端口填写错误'},
}

export function codeText(code: number, fallback = ''): string {
  return codeTexts[code] ?? (fallback || `操作失败（${code}）`)
}

// 说明：v2 仅把 NAT 观测作为失败诊断，不能据此断言原生 DCUtR 必然失败。
export function linkErrorText(code: number, _natBlocked = '', _selfView = false): {title: string; hint: string} | null {
  return linkErrorTexts[code] ?? null
}

export const linkStateTexts: Record<string, string> = {
  connecting: '连接中',
  connected: '已连接',
  reconnecting: '重连中',
  failed: '连接失败',
}

export const linkPathTexts: Record<string, string> = {
  lan: '局域网',
  direct: '直连',
  upnp: 'UPnP 映射',
  punch: '打洞（旧版）',
  relay: '中继',
}

export const connectionStateTexts: Record<string, string> = {
  offline: '未连接',
  connecting: '连接中',
  online: '在线',
  reconnecting: '重连中',
}
