import { Drawer } from '../ui'
import type { OfficeFeishuBot, OfficeMessage } from './types'
import type { PushTargetDraft } from './pushTargetDraft'
import styles from './OfficeMessage.module.css'

type Props = { draft: PushTargetDraft | null; messages: OfficeMessage[]; bots: OfficeFeishuBot[]; saving: boolean; error: string; onChange: (draft: PushTargetDraft) => void; onClose: () => void; onSave: () => void }

export function PushTargetDrawer({ draft, messages, bots, saving, error, onChange, onClose, onSave }: Props) {
  return <Drawer open={Boolean(draft)} title={draft?.id ? '编辑飞书推送配置' : '新增飞书推送配置'} description="选择服务端环境变量配置的飞书自建应用机器人，再绑定消息和接收方。" closeDisabled={saving} onClose={onClose} footer={<><button type="button" disabled={saving} onClick={onClose}>取消</button><button type="button" className={styles.primary} disabled={saving || bots.length === 0} onClick={onSave}>{saving ? '保存中…' : '保存配置'}</button></>}>
    {draft ? <form className={styles.form} onSubmit={(event) => { event.preventDefault(); onSave() }}>
      <label>配置名称<input name="name" autoComplete="off" required maxLength={128} value={draft.name} disabled={saving} onChange={(event) => onChange({ ...draft, name: event.currentTarget.value })} /></label>
      <label>飞书机器人<select name="botAppId" required value={draft.botAppId} disabled={saving || bots.length === 0} onChange={(event) => onChange({ ...draft, botAppId: event.currentTarget.value })}><option value="" disabled>{bots.length === 0 ? '服务端未配置机器人' : '选择机器人'}</option>{bots.map((bot) => <option key={bot.id} value={bot.id}>{bot.name}（{bot.id}）</option>)}</select><small>机器人 App ID：<code>{draft.botAppId || '未配置'}</code>；App Secret 不会返回浏览器。</small></label>
      <label>消息<select name="messageId" required value={draft.messageId || ''} disabled={saving} onChange={(event) => onChange({ ...draft, messageId: Number(event.currentTarget.value) })}><option value="" disabled>选择消息</option>{messages.map((message) => <option key={message.id} value={message.id}>{message.name}{message.enabled ? '' : '（已停用）'}</option>)}</select></label>
      <label>接收 ID 类型<select name="receiveIdType" value={draft.receiveIdType} disabled={saving} onChange={(event) => onChange({ ...draft, receiveIdType: event.currentTarget.value as PushTargetDraft['receiveIdType'] })}><option value="chat_id">群聊 ID（chat_id）</option><option value="open_id">用户 open_id</option><option value="user_id">企业 user_id</option><option value="union_id">用户 union_id</option><option value="email">邮箱</option></select></label>
      <label>接收 ID<input name="receiveId" autoComplete="off" required maxLength={255} className={styles.mono} value={draft.receiveId} disabled={saving} placeholder={draft.receiveIdType === 'chat_id' ? 'oc_xxxxxxxxx' : ''} onChange={(event) => onChange({ ...draft, receiveId: event.currentTarget.value })} /></label>
      <label className={styles.checkbox}><input name="enabled" type="checkbox" checked={draft.enabled} disabled={saving} onChange={(event) => onChange({ ...draft, enabled: event.currentTarget.checked })} /><span>启用推送配置</span></label>
      {error ? <p className={styles.formError} role="alert">{error}</p> : null}
    </form> : null}
  </Drawer>
}
