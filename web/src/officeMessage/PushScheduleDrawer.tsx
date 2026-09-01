import { Drawer } from '../ui'
import type { OfficeMessage, OfficePushTarget, OfficeScheduleParameter } from './types'
import { parametersForTarget, type PushScheduleDraft } from './pushScheduleDraft'
import styles from './OfficeMessage.module.css'

type Props = { draft: PushScheduleDraft | null; targets: OfficePushTarget[]; messages: OfficeMessage[]; saving: boolean; error: string; onChange: (draft: PushScheduleDraft) => void; onClose: () => void; onSave: () => void }

export function PushScheduleDrawer({ draft, targets, messages, saving, error, onChange, onClose, onSave }: Props) {
  const target = targets.find((item) => item.id === draft?.targetId)
  const message = messages.find((item) => item.id === target?.messageId)
  function updateParameter(code: string, value: OfficeScheduleParameter) {
    if (draft) onChange({ ...draft, parameters: { ...draft.parameters, [code]: value } })
  }

  return <Drawer open={Boolean(draft)} title={draft?.id ? '编辑定时计划' : '新增定时计划'} description="按上海时区使用五段 Cron 触发推送，日期参数可基于计划执行日自动计算。" size="medium" closeDisabled={saving} onClose={onClose} footer={<><button type="button" disabled={saving} onClick={onClose}>取消</button><button type="button" className={styles.primary} disabled={saving || targets.length === 0} onClick={onSave}>{saving ? '保存中…' : '保存计划'}</button></>}>
    {draft ? <form className={styles.form} onSubmit={(event) => { event.preventDefault(); onSave() }}>
      <label>计划名称<input name="scheduleName" autoComplete="off" required maxLength={128} value={draft.name} disabled={saving} onChange={(event) => onChange({ ...draft, name: event.currentTarget.value })} /></label>
      <label>推送配置<select name="targetId" required value={draft.targetId || ''} disabled={saving} onChange={(event) => { const targetId = Number(event.currentTarget.value); onChange({ ...draft, targetId, parameters: parametersForTarget(targetId, targets, messages, {}) }) }}><option value="" disabled>选择推送配置</option>{targets.map((item) => <option key={item.id} value={item.id}>{item.name}{item.enabled ? '' : '（已停用）'}</option>)}</select><small>绑定消息：{message?.name ?? '未找到消息'}</small></label>
      <div className={styles.twoColumns}><label>五段 Cron<input name="cronExpr" required maxLength={128} className={styles.mono} value={draft.cronExpr} disabled={saving} placeholder="0 9 * * *" onChange={(event) => onChange({ ...draft, cronExpr: event.currentTarget.value })} /><small>依次为分钟、小时、日、月、星期，例如每天 09:00：<code>0 9 * * *</code></small></label><label>时区<input name="timeZone" value={draft.timeZone} disabled readOnly /></label></div>
      {message?.parameters.length ? <div className={styles.editorBlock}><div className={styles.editorHeading}><div><h3>消息参数</h3><p>日期参数可使用执行日期并配置偏移天数，其他参数使用固定值。</p></div></div><div className={styles.parameterList}>{message.parameters.map((parameter) => {
        const value = draft.parameters[parameter.code] ?? { mode: parameter.valueType === 'date' ? 'SCHEDULED_DATE' : 'LITERAL', value: '', offsetDays: 0 }
        return <div className={styles.sourceGrid} key={parameter.code}><div className={styles.runForm}><small>参数</small><strong>{parameter.label}</strong><code>{parameter.code}</code></div>{parameter.valueType === 'date' ? <label>取值方式<select name={`parameter_${parameter.code}_mode`} value={value.mode} disabled={saving} onChange={(event) => { const mode = event.currentTarget.value as OfficeScheduleParameter['mode']; updateParameter(parameter.code, mode === 'SCHEDULED_DATE' ? { mode, value: '', offsetDays: value.offsetDays } : { mode, value: value.value, offsetDays: 0 }) }}><option value="SCHEDULED_DATE">执行日期</option><option value="LITERAL">固定值</option></select></label> : <label>取值方式<input name={`parameter_${parameter.code}_mode`} value="固定值" disabled readOnly /></label>}{value.mode === 'SCHEDULED_DATE' && parameter.valueType === 'date' ? <label>偏移天数<input name={`parameter_${parameter.code}_offsetDays`} type="number" min={-3660} max={3660} step="1" value={value.offsetDays} disabled={saving} onChange={(event) => updateParameter(parameter.code, { ...value, offsetDays: Number(event.currentTarget.value) })} /></label> : <label>固定值<input name={`parameter_${parameter.code}_value`} value={value.value} required={parameter.required} disabled={saving} onChange={(event) => updateParameter(parameter.code, { ...value, mode: 'LITERAL', offsetDays: 0, value: event.currentTarget.value })} /></label>}</div>
      })}</div></div> : <p className={styles.contractNote}>该消息不需要运行参数，计划会按 Cron 直接触发。</p>}
      <label className={styles.checkbox}><input name="scheduleEnabled" type="checkbox" checked={draft.enabled} disabled={saving} onChange={(event) => onChange({ ...draft, enabled: event.currentTarget.checked })} /><span>启用定时计划</span></label>
      {error ? <p className={styles.formError} role="alert">{error}</p> : null}
    </form> : null}
  </Drawer>
}
