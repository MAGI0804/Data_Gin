import { useCallback, useEffect, useMemo, useState } from 'react'
import { Pencil, Plus, RefreshCw, Send, Trash2 } from 'lucide-react'
import type { WorkspaceApiClient } from '../appShell/WorkspaceRouter'
import { DataTable, Dialog, FeedbackState, PageCanvas, PageHeader, Section, StatusTag } from '../ui'
import { createOfficeRun, deleteOfficeTarget, getOfficeMessages, getOfficeRuns, getOfficeTargets, saveOfficeTarget } from './api'
import type { OfficeMessage, OfficePushRun, OfficePushTarget } from './types'
import { PushTargetDrawer } from './PushTargetDrawer'
import { emptyPushTarget, targetDraftFrom, type PushTargetDraft } from './pushTargetDraft'
import { PushRunDialog } from './PushRunDialog'
import styles from './OfficeMessage.module.css'

type Props = { client: WorkspaceApiClient; permissions: string[] }
type Notice = { text: string; error: boolean }

export function PushManagementPage({ client, permissions }: Props) {
  const [messages, setMessages] = useState<OfficeMessage[]>([])
  const [targets, setTargets] = useState<OfficePushTarget[]>([])
  const [runs, setRuns] = useState<OfficePushRun[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [reload, setReload] = useState(0)
  const [draft, setDraft] = useState<PushTargetDraft | null>(null)
  const [draftError, setDraftError] = useState('')
  const [pendingDelete, setPendingDelete] = useState<OfficePushTarget | null>(null)
  const [pushTarget, setPushTarget] = useState<OfficePushTarget | null>(null)
  const [pushValues, setPushValues] = useState<Record<string, string>>({})
  const [pushRequestId, setPushRequestId] = useState('')
  const [pushError, setPushError] = useState('')
  const [notice, setNotice] = useState<Notice | null>(null)
  const canManage = permissions.includes('office_message.manage')
  const canPush = permissions.includes('office_message.push')
  const messageById = useMemo(() => new Map(messages.map((message) => [message.id, message])), [messages])

  const load = useCallback(async (signal?: AbortSignal) => {
    setLoading(true)
    const [messageResult, targetResult, runResult] = await Promise.all([getOfficeMessages(client, signal), getOfficeTargets(client, signal), getOfficeRuns(client, signal)])
    if (signal?.aborted) return
    if (messageResult.ok) setMessages(messageResult.data)
    if (targetResult.ok) setTargets(targetResult.data)
    if (runResult.ok) setRuns(runResult.data)
    const failed = [messageResult, targetResult, runResult].find((result) => !result.ok)
    setNotice(failed && !failed.ok ? { text: failed.error, error: true } : null)
    setLoading(false)
  }, [client])

  useEffect(() => { const controller = new AbortController(); void load(controller.signal); return () => controller.abort() }, [load, reload])
  useEffect(() => {
    if (!runs.some((run) => run.status === 'QUEUED' || run.status === 'RUNNING')) return
    const timer = window.setInterval(() => setReload((value) => value + 1), 8000)
    return () => window.clearInterval(timer)
  }, [runs])

  function showNotice(text: string, error = false) { setNotice({ text, error }) }
  async function save() {
    if (!draft || saving) return
    if (!draft.name.trim() || !draft.messageId || !draft.receiveId.trim()) return setDraftError('请完整填写配置名称、消息和接收 ID。')
    setSaving(true); setDraftError('')
    const body = { name: draft.name.trim(), messageId: draft.messageId, receiveIdType: draft.receiveIdType, receiveId: draft.receiveId.trim(), enabled: draft.enabled, expectedLockVersion: draft.id ? draft.lockVersion : 0 }
    const result = await saveOfficeTarget(client, draft, body)
    setSaving(false)
    if (!result.ok) return setDraftError(result.error)
    setDraft(null); showNotice('飞书推送配置已保存。'); setReload((value) => value + 1)
  }
  async function remove() {
    if (!pendingDelete || saving) return
    setSaving(true)
    const result = await deleteOfficeTarget(client, pendingDelete.id, pendingDelete.lockVersion)
    setSaving(false)
    if (!result.ok) return showNotice(result.error, true)
    setPendingDelete(null); showNotice('飞书推送配置已删除。'); setReload((value) => value + 1)
  }
  async function push() {
    if (!pushTarget || saving) return
    const message = messageById.get(pushTarget.messageId)
    const missing = message?.parameters.find((parameter) => parameter.required && !pushValues[parameter.code]?.trim())
    if (missing) return setPushError(`请填写${missing.label}。`)
    setSaving(true); setPushError('')
    const result = await createOfficeRun(client, pushTarget.id, pushRequestId, pushValues)
    setSaving(false)
    if (!result.ok) return setPushError(result.error)
    setPushTarget(null); setPushValues({}); setPushRequestId(''); showNotice(`推送任务 #${result.data.id} 已进入队列。`); setReload((value) => value + 1)
  }

  return <PageCanvas>
    <PageHeader eyebrow="FEISHU DELIVERY" title="推送管理" description="维护飞书自建应用机器人的接收目标，手动发起持久化异步推送并查看运行状态。" actions={<div className={styles.actions}><button type="button" disabled={loading} onClick={() => setReload((value) => value + 1)}><RefreshCw aria-hidden="true" />刷新</button>{canManage ? <button type="button" className={styles.primary} onClick={() => { setDraftError(''); setDraft(emptyPushTarget(messages)) }}><Plus aria-hidden="true" />新增配置</button> : null}</div>} />
    {notice ? <p className={notice.error ? styles.errorNotice : styles.notice} role={notice.error ? 'alert' : 'status'}>{notice.text}</p> : null}
    <Section title="飞书推送配置" description={`共 ${targets.length} 个目标；应用凭证由服务端环境变量统一管理。`} flush>
      {loading && targets.length === 0 ? <FeedbackState kind="loading" title="正在加载推送配置" /> : targets.length === 0 ? <FeedbackState kind="empty" title="暂无推送配置" /> : <DataTable containerClassName={styles.table} minWidth={900}><thead><tr><th scope="col">配置</th><th scope="col">消息</th><th scope="col">接收方</th><th scope="col">状态</th><th scope="col">操作</th></tr></thead><tbody>{targets.map((target) => <tr key={target.id}><td><span className={styles.identity}><Send aria-hidden="true" /><span><strong>{target.name}</strong><code>#{target.id}</code></span></span></td><td>{messageById.get(target.messageId)?.name ?? `消息 #${target.messageId}`}</td><td><code>{target.receiveIdType}:{target.receiveId}</code></td><td><StatusTag tone={target.enabled ? 'success' : 'neutral'}>{target.enabled ? '启用' : '停用'}</StatusTag></td><td><div className={styles.actions}>{canPush ? <button type="button" disabled={!target.enabled || !messageById.get(target.messageId)?.enabled} onClick={() => { setPushError(''); setPushValues({}); setPushRequestId(crypto.randomUUID()); setPushTarget(target) }}><Send aria-hidden="true" />立即推送</button> : null}{canManage ? <><button type="button" onClick={() => { setDraftError(''); setDraft(targetDraftFrom(target)) }}><Pencil aria-hidden="true" />编辑</button><button type="button" onClick={() => setPendingDelete(target)}><Trash2 aria-hidden="true" />删除</button></> : null}</div></td></tr>)}</tbody></DataTable>}
    </Section>
    <Section title="最近推送记录" description="排队和运行中的记录会每 8 秒自动刷新。" flush>{loading && runs.length === 0 ? <FeedbackState kind="loading" title="正在加载推送记录" /> : runs.length === 0 ? <FeedbackState kind="empty" title="暂无推送记录" /> : <DataTable containerClassName={styles.table} minWidth={980}><thead><tr><th scope="col">任务</th><th scope="col">配置 / 消息</th><th scope="col">状态</th><th scope="col">尝试</th><th scope="col">行数</th><th scope="col">创建时间</th><th scope="col">结果</th></tr></thead><tbody>{runs.map((run) => <tr key={run.id}><td><strong>#{run.id}</strong><small className={styles.cellHint}>{run.runUuid.slice(0, 8)}</small></td><td>#{run.targetId} / #{run.messageId}</td><td><RunStatus status={run.status} /></td><td>{run.attemptCount}</td><td>{run.rowCount || '—'}</td><td>{formatTime(run.createdAt)}</td><td>{run.errorMessage || (run.finishedAt ? formatTime(run.finishedAt) : '—')}</td></tr>)}</tbody></DataTable>}</Section>
    <PushTargetDrawer draft={draft} messages={messages} saving={saving} error={draftError} onChange={setDraft} onClose={() => { if (!saving) setDraft(null) }} onSave={() => void save()} />
    <PushRunDialog target={pushTarget} message={pushTarget ? messageById.get(pushTarget.messageId) ?? null : null} values={pushValues} sending={saving} error={pushError} onValuesChange={setPushValues} onClose={() => { if (!saving) { setPushTarget(null); setPushRequestId('') } }} onConfirm={() => void push()} />
    <Dialog open={Boolean(pendingDelete)} role="alertdialog" title="删除飞书推送配置" description="已创建的历史运行记录会保留。" closeDisabled={saving} onClose={() => setPendingDelete(null)} footer={<><button type="button" disabled={saving} onClick={() => setPendingDelete(null)}>取消</button><button type="button" className={styles.danger} disabled={saving} onClick={() => void remove()}>{saving ? '删除中…' : '确认删除'}</button></>}><p className={styles.dialogCopy}>确认删除“{pendingDelete?.name}”？</p></Dialog>
  </PageCanvas>
}

function RunStatus({ status }: { status: OfficePushRun['status'] }) {
  const labels = { QUEUED: '排队中', RUNNING: '发送中', SUCCEEDED: '成功', FAILED: '失败', UNKNOWN: '状态未知' }
  const tones = { QUEUED: 'info', RUNNING: 'running', SUCCEEDED: 'success', FAILED: 'danger', UNKNOWN: 'warning' } as const
  return <StatusTag tone={tones[status]}>{labels[status]}</StatusTag>
}

function formatTime(value: string) { const date = new Date(value); return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString('zh-CN', { hour12: false }) }
