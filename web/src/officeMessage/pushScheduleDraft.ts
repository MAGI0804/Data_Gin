import type { OfficeMessage, OfficePushSchedule, OfficePushTarget, OfficeScheduleParameter } from './types'

export type PushScheduleDraft = {
  id: number | null
  name: string
  targetId: number
  cronExpr: string
  timeZone: 'Asia/Shanghai'
  parameters: Record<string, OfficeScheduleParameter>
  enabled: boolean
  lockVersion: number
}

export function emptyPushSchedule(targets: OfficePushTarget[], messages: OfficeMessage[]): PushScheduleDraft {
  const targetId = targets.find((item) => item.enabled)?.id ?? targets[0]?.id ?? 0
  return { id: null, name: '', targetId, cronExpr: '0 9 * * *', timeZone: 'Asia/Shanghai', parameters: parametersForTarget(targetId, targets, messages, {}), enabled: true, lockVersion: 0 }
}

export function scheduleDraftFrom(schedule: OfficePushSchedule): PushScheduleDraft {
  return { id: schedule.id, name: schedule.name, targetId: schedule.targetId, cronExpr: schedule.cronExpr, timeZone: schedule.timeZone, parameters: schedule.parameters, enabled: schedule.enabled, lockVersion: schedule.lockVersion }
}

export function parametersForTarget(targetId: number, targets: OfficePushTarget[], messages: OfficeMessage[], current: Record<string, OfficeScheduleParameter>) {
  const target = targets.find((item) => item.id === targetId)
  const message = messages.find((item) => item.id === target?.messageId)
  return Object.fromEntries((message?.parameters ?? []).map((parameter) => {
    const existing = current[parameter.code]
    if (existing && (parameter.valueType === 'date' || existing.mode === 'LITERAL')) return [parameter.code, existing]
    return [parameter.code, { mode: parameter.valueType === 'date' ? 'SCHEDULED_DATE' : 'LITERAL', value: '', offsetDays: 0 } satisfies OfficeScheduleParameter]
  }))
}
