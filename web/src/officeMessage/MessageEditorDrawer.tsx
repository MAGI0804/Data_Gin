import { Drawer } from '../ui'
import type { WorkspaceApiClient } from '../appShell/WorkspaceRouter'
import type { OfficeMessageDraft, OfficeMessageSourceType } from './types'
import { ProcedureSourceEditor, QuerySourceEditor } from './SourceEditors'
import styles from './OfficeMessage.module.css'

type Props = { client: WorkspaceApiClient; draft: OfficeMessageDraft | null; saving: boolean; error: string; onChange: (draft: OfficeMessageDraft) => void; onClose: () => void; onSave: () => void; onNotice: (notice: string, error?: boolean) => void }

export function MessageEditorDrawer({ client, draft, saving, error, onChange, onClose, onSave, onNotice }: Props) {
  function switchSource(sourceType: OfficeMessageSourceType) {
    if (!draft) return
    onChange({
      ...draft,
      sourceType,
      content: sourceType === 'EDITED' ? draft.content : '',
      fileNameTemplate: sourceType !== 'EDITED' && !draft.fileNameTemplate.trim() ? '办公消息_{{date:yyyyMMdd}}.xlsx' : draft.fileNameTemplate,
      parameters: sourceType === 'ORACLE_QUERY' ? draft.parameters : [],
      columnMapping: sourceType === 'EDITED' ? [] : draft.columnMapping,
    })
  }
  return <Drawer open={Boolean(draft)} title={draft?.id ? '编辑办公消息' : '新增办公消息'} description="配置消息内容或 Oracle Excel 来源。保存前建议完成元数据读取或 SELECT 测试。" size="wide" closeDisabled={saving} onClose={onClose} footer={<><button type="button" disabled={saving} onClick={onClose}>取消</button><button type="button" className={styles.primary} disabled={saving} onClick={onSave}>{saving ? '保存中…' : '保存消息'}</button></>}>
    {draft ? <form className={styles.form} onSubmit={(event) => { event.preventDefault(); onSave() }}>
      <div className={styles.twoColumns}><label>消息名称<input required maxLength={128} value={draft.name} disabled={saving} onChange={(event) => onChange({ ...draft, name: event.currentTarget.value })} /></label><label>消息来源<select value={draft.sourceType} disabled={saving} onChange={(event) => switchSource(event.currentTarget.value as OfficeMessageSourceType)}><option value="EDITED">自编辑消息</option><option value="ORACLE_PROCEDURE">存储过程结果 Excel</option><option value="ORACLE_QUERY">SELECT 结果 Excel</option></select></label></div>
      <label className={styles.checkbox}><input type="checkbox" checked={draft.enabled} disabled={saving} onChange={(event) => onChange({ ...draft, enabled: event.currentTarget.checked })} /><span>启用消息</span></label>
      {draft.sourceType === 'EDITED' ? <label>消息正文<textarea rows={12} maxLength={60000} value={draft.content} disabled={saving} onChange={(event) => onChange({ ...draft, content: event.currentTarget.value })} /></label> : null}
      {draft.sourceType !== 'EDITED' ? <label>Excel 文件名模板<input required maxLength={255} value={draft.fileNameTemplate} disabled={saving} onChange={(event) => onChange({ ...draft, fileNameTemplate: event.currentTarget.value })} /><small>必须以 .xlsx 结尾；支持 {'{{date:yyyyMMdd}}'} 或 {'{{date:yyyy-MM-dd}}'}，按上海时间生成推送当日日期。</small></label> : null}
      {draft.sourceType === 'ORACLE_PROCEDURE' ? <ProcedureSourceEditor client={client} draft={draft} disabled={saving} onChange={onChange} onNotice={onNotice} /> : null}
      {draft.sourceType === 'ORACLE_QUERY' ? <QuerySourceEditor client={client} draft={draft} disabled={saving} onChange={onChange} onNotice={onNotice} /> : null}
      {error ? <p className={styles.formError} role="alert">{error}</p> : null}
    </form> : null}
  </Drawer>
}
