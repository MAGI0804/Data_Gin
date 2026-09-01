import type { OfficeFeishuBot, OfficeMessage, OfficePushTarget } from './types'

export type PushTargetDraft = {
  id: number | null
  name: string
  messageId: number
  botAppId: string
  receiveIdType: OfficePushTarget['receiveIdType']
  receiveId: string
  enabled: boolean
  lockVersion: number
}
export function emptyPushTarget(messages: OfficeMessage[], bots: OfficeFeishuBot[]): PushTargetDraft {
  return { id: null, name: '', messageId: messages.find((item) => item.enabled)?.id ?? 0, botAppId: bots[0]?.id ?? '', receiveIdType: 'chat_id', receiveId: '', enabled: true, lockVersion: 0 }
}

export function targetDraftFrom(target: OfficePushTarget): PushTargetDraft {
  return { id: target.id, name: target.name, messageId: target.messageId, botAppId: target.botAppId, receiveIdType: target.receiveIdType, receiveId: target.receiveId, enabled: target.enabled, lockVersion: target.lockVersion }
}
