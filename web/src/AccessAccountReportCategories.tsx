import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { FileSpreadsheet, RefreshCcw, Search, ShieldCheck } from 'lucide-react'
import {
  accessAccountReportCategoriesPath,
  parseAccessAccountReportCategories,
  type AccessAccountReportCategory,
  type AccessReportAction,
} from './accessManagement'
import { Dialog, FeedbackState, StatusTag } from './ui'
import styles from './AccessManagementPage.module.css'

type ApiResult = { ok: boolean; status: number; data: unknown }
type ApiClient = (path: string, options?: { method?: 'GET' | 'PUT'; body?: unknown; headers?: Record<string, string>; showResult?: boolean; silentLoading?: boolean; signal?: AbortSignal }) => Promise<ApiResult>

export function AccessAccountReportCategories({ accountID, accountName, client }: {
  accountID: number
  accountName: string
  client: ApiClient
}) {
  const [items, setItems] = useState<AccessAccountReportCategory[]>([])
  const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading')
  const [message, setMessage] = useState('')
  const [search, setSearch] = useState('')
  const [editing, setEditing] = useState<AccessAccountReportCategory | null>(null)
  const [actions, setActions] = useState<AccessReportAction[]>([])
  const [saving, setSaving] = useState(false)
  const requestRef = useRef(0)

  const load = useCallback(async (signal?: AbortSignal) => {
    const requestID = ++requestRef.current
    setStatus('loading')
    const response = await client(accessAccountReportCategoriesPath(accountID), { method: 'GET', showResult: false, silentLoading: true, signal })
    if (signal?.aborted || requestID !== requestRef.current) return
    const parsed = response.ok ? parseAccessAccountReportCategories(response.data) : null
    if (!parsed) {
      setStatus('error')
      setMessage('报表分类权限加载失败，请稍后重试。')
      return
    }
    setItems(parsed)
    setStatus('ready')
  }, [accountID, client])

  useEffect(() => {
    const controller = new AbortController()
    setItems([])
    setMessage('')
    setSearch('')
    setEditing(null)
    setActions([])
    setSaving(false)
    void load(controller.signal)
    return () => controller.abort()
  }, [load])

  const filtered = useMemo(() => {
    const keyword = search.trim().toLocaleLowerCase('zh-CN')
    return keyword ? items.filter((item) => item.category.toLocaleLowerCase('zh-CN').includes(keyword)) : items
  }, [items, search])

  function openEditor(item: AccessAccountReportCategory) {
    setMessage('')
    setEditing(item)
    setActions(item.directActions)
  }

  function toggleAction(action: AccessReportAction, checked: boolean) {
    setActions((current) => {
      if (action === 'QUERY' && !checked) return []
      if (action === 'EXPORT' && checked) return ['QUERY', 'EXPORT']
      if (checked) return current.includes(action) ? current : [...current, action]
      return current.filter((item) => item !== action)
    })
  }

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!editing) return
    const form = new FormData(event.currentTarget)
    const reason = String(form.get('reason') ?? '').trim()
    setSaving(true)
    setMessage('')
    const response = await client(accessAccountReportCategoriesPath(accountID), {
      method: 'PUT', headers: { 'Idempotency-Key': `access-report-category:${globalThis.crypto.randomUUID()}` }, showResult: false, silentLoading: true,
      body: { category: editing.category, expectedLockVersion: editing.lockVersion, actions, reason },
    })
    setSaving(false)
    if (!response.ok) {
      if (response.status === 409) {
        setEditing(null)
        await load()
        setMessage('该分类权限已被其他管理员更新，已刷新最新状态。')
      } else setMessage('报表分类权限保存失败，请检查权限和输入。')
      return
    }
    setEditing(null)
    await load()
    setMessage('报表分类权限已保存；该账号重新登录后会显示对应的报表入口。')
  }

  if (status === 'loading' && items.length === 0) return <FeedbackState kind="loading" title="正在加载报表分类" />
  if (status === 'error' && items.length === 0) return <FeedbackState kind="error" title="报表分类加载失败" description={message} action={<button type="button" onClick={() => void load()}><RefreshCcw aria-hidden="true" />重新加载</button>} />

  return <div className={styles.reportAccess}>
    <div className={styles.reportAccessIntro}>
      <ShieldCheck aria-hidden="true" />
      <div><strong>按分类控制可查询、可下载范围</strong><p>直接授权只影响 {accountName}；角色继承权限会单独标记且不可在这里移除。</p></div>
    </div>
    {message && <p className={styles.message} role="status">{message}</p>}
    <div className={styles.reportAccessToolbar}>
      <label><Search aria-hidden="true" /><span className={styles.srOnly}>搜索报表分类</span><input value={search} onChange={(event) => setSearch(event.currentTarget.value)} placeholder="搜索分类名称" /></label>
      <span>{filtered.length} 个分类</span>
    </div>
    {filtered.length === 0 ? <FeedbackState kind="empty" title={items.length === 0 ? '暂无可授权的报表分类' : '没有匹配的报表分类'} /> : <div className={styles.categoryList}>
      {filtered.map((item) => <article key={item.category}>
        <div className={styles.categoryIdentity}><FileSpreadsheet aria-hidden="true" /><span><strong>{item.category}</strong><small>{item.reportCount} 份报表</small></span></div>
        <div className={styles.categoryStatus}>
          <StatusTag tone={item.configured ? 'success' : 'neutral'}>{item.configured ? '分类管控已启用' : '待启用'}</StatusTag>
          <span>直接：{actionLabel(item.directActions)}</span>
          {item.inheritedActions.length > 0 && <span>角色继承：{actionLabel(item.inheritedActions)}</span>}
        </div>
        <button type="button" onClick={() => openEditor(item)}>配置</button>
      </article>)}
    </div>}
    <Dialog className={styles.categoryDialog} open={Boolean(editing)} title={editing ? `配置“${editing.category}”` : '配置报表分类'} description={`为 ${accountName} 设置直接授权；角色继承权限仍然有效。`} closeDisabled={saving} onClose={() => setEditing(null)}>
      {editing && <form className={styles.drawerForm} onSubmit={save}>
        {!editing.configured && <p className={styles.permissionNotice}>首次保存授权会启用该分类的统一权限策略，并接管该分类下所有已上线报表。</p>}
        <label className={styles.permissionChoice}><input type="checkbox" checked={actions.includes('QUERY')} onChange={(event) => toggleAction('QUERY', event.currentTarget.checked)} /><span><strong>允许查询和生成</strong><small>可选择该分类的报表并提交运行。</small></span></label>
        <label className={styles.permissionChoice}><input type="checkbox" checked={actions.includes('EXPORT')} onChange={(event) => toggleAction('EXPORT', event.currentTarget.checked)} /><span><strong>允许下载 Excel</strong><small>启用下载会同时启用查询和生成。</small></span></label>
        {editing.inheritedActions.length > 0 && <p className={styles.inheritedNotice}>角色继承权限：{actionLabel(editing.inheritedActions)}。即使清空直接授权，这部分权限仍会保留。</p>}
        <label className={styles.field}><span>变更原因</span><textarea name="reason" required maxLength={500} rows={4} placeholder="说明授权用途或审批依据" /></label>
        <button className={styles.primary} type="submit" disabled={saving}>{saving ? '保存中…' : '保存分类权限'}</button>
      </form>}
    </Dialog>
  </div>
}

function actionLabel(actions: readonly AccessReportAction[]) {
  if (actions.includes('EXPORT')) return '查询、下载'
  if (actions.includes('QUERY')) return '仅查询'
  return '未授权'
}
