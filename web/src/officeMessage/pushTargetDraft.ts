import type { OfficeMessage, OfficePushTarget } from './types'

export type PushTargetDraft = {
  id: number | null
  name: string
  messageId: number
  receiveIdType: OfficePushTarget['receiveIdType']
  receiveId: string
  enabled: boolean
  lockVersion: number
}
export function emptyPushTarget(messages: OfficeMessage[]): PushTargetDraft {
  return { id: null, name: '', messageId: messages.find((item) => item.enabled)?.id ?? 0, receiveIdType: 'chat_id', receiveId: '', enabled: true, lockVersion: 0 }
}

export function targetDraftFrom(target: OfficePushTarget): PushTargetDraft {
  return { id: target.id, name: target.name, messageId: target.messageId, receiveIdType: target.receiveIdType, receiveId: target.receiveId, enabled: target.enabled, lockVersion: target.lockVersion }
}
