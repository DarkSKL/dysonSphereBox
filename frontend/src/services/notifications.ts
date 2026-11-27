import {GetInbox, GetUnread, MarkRead} from '../../wailsjs/go/system/NotificationService'
import type {notification, system} from '../../wailsjs/go/models'

export type InboxEntry = notification.InboxEntry
export type InboxPage = notification.InboxPage
export type UnreadCount = system.NotificationUnread

// =============== 收件箱 Wails 桥接 ===============
// 说明：以账号作用域保护查询和标读，投递 ID 在 WebView 中保持字符串。
export function fetchNotificationUnread(key: string): Promise<UnreadCount> {
  return GetUnread(key)
}

export function fetchNotificationInbox(key: string, page: number): Promise<InboxPage> {
  return GetInbox(key, page, 20)
}

export function markNotificationRead(key: string, id: string): Promise<system.NotificationReadResult> {
  return MarkRead(key, id)
}
