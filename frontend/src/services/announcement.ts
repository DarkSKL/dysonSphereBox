import {CaptureSession, GetHistory, GetHistoryItem, GetLatest, MarkHistoryRead, MarkRead} from '../../wailsjs/go/system/AnnouncementService'
import type {announcement, system} from '../../wailsjs/go/models'

export type NoticeItem = system.ClientNoticeItem
export type NoticeLatest = announcement.NoticeLatestDTO
export type NoticeReceipt = system.ClientNoticeMarkReadReceipt
export type NoticeHistoryEntry = system.ClientNoticeHistoryEntry
export type NoticeHistory = announcement.NoticeHistoryDTO

export const captureAnnouncementSession = (): Promise<string> => CaptureSession()
export const fetchLatestAnnouncement = (sessionID: string): Promise<NoticeLatest> => GetLatest(sessionID)
export const markAnnouncementRead = (sessionID: string, id: string, revision: number): Promise<NoticeReceipt> => MarkRead(sessionID, id, revision)
export const fetchAnnouncementHistory = (sessionID: string, page: number, pageSize: number): Promise<NoticeHistory> => GetHistory(sessionID, page, pageSize)
export const fetchAnnouncementHistoryItem = (sessionID: string, id: string, revision: number): Promise<NoticeHistoryEntry> => GetHistoryItem(sessionID, id, revision)
export const markAnnouncementHistoryRead = (sessionID: string, id: string, revision: number): Promise<NoticeReceipt> => MarkHistoryRead(sessionID, id, revision)
