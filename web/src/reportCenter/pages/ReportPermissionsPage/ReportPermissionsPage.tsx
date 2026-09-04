import { useEffect, useState, type ReactNode } from 'react'
import { FolderKey, Plus, RefreshCcw, ShieldCheck, Trash2 } from 'lucide-react'
import { Button, Drawer, FeedbackState, PageCanvas, PageHeader, Section, StatusTag } from '../../../ui'
import { getReportCategoryAccess, replaceReportCategoryAccess, type ReportCenterClient } from '../../api'
import type { ReportCategoryAccess, ReportGrant } from '../../types'
import styles from './ReportPermissionsPage.module.css'

export function ReportPermissionsPage({ client, navigation }: { client: ReportCenterClient; navigation?: ReactNode }) {
  const [items, setItems] = useState<ReportCategoryAccess[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [reloadVersion, setReloadVersion] = useState(0)
  const [selected, setSelected] = useState<ReportCategoryAccess | null>(null)

  useEffect(() => {
    const controller = new AbortController()
    setLoading(true)
    setError('')
    void getReportCategoryAccess(client, controller.signal).then((result) => {
      if (controller.signal.aborted) return
      if (!result.ok) {
        setError(result.error)
        setLoading(false)
        return
      }
      setItems(result.data)
      setLoading(false)
    })
    return () => controller.abort()
  }, [client, reloadVersion])

  function reload() {
    setReloadVersion((version) => version + 1)
  }

  function handleSaved(access: ReportCategoryAccess) {
    setItems((current) => current.map((item) => item.category === access.category ? access : item))
    setSelected(null)
  }

  return <PageCanvas density="compact">
    {navigation}
    <PageHeader eyebrow="CATEGORY ACCESS" title="报表分类权限" description="按分类统一授予用户或角色查询、生成和下载报表的权限；分类策略会覆盖该分类下的逐报表旧授权。" actions={<button type="button" onClick={reload} disabled={loading}><RefreshCcw aria-hidden="true" />刷新</button>} />
    <Section title="分类授权策略" description="权限对分类内所有已上线报表生效；保存空授权可暂停整个分类的访问。" flush>
      {loading && items.length === 0 ? <FeedbackState kind="loading" title="正在读取分类权限" /> : null}
      {error ? <FeedbackState kind="error" title="分类权限加载失败" description={error} action={<button type="button" onClick={reload}>重试</button>} /> : null}
      {!loading && !error && items.length === 0 ? <FeedbackState kind="empty" title="暂无可授权分类" description="请先在报表配置中创建带分类的报表。" /> : null}
      {items.length > 0 ? <div className={styles.list}>{items.map((item) => <article className={styles.item} key={item.category}>
        <span className={styles.icon}><FolderKey aria-hidden="true" /></span>
        <span className={styles.identity}><strong>{item.category}</strong><small>{item.reportCount.toLocaleString('zh-CN')} 份报表 · {item.grants.length} 个授权主体</small></span>
        <StatusTag tone={item.configured ? item.grants.length ? 'success' : 'danger' : 'warning'}>{item.configured ? item.grants.length ? '分类策略已启用' : '已暂停访问' : '兼容旧授权'}</StatusTag>
        <button type="button" onClick={() => setSelected(item)}><ShieldCheck aria-hidden="true" />配置权限</button>
      </article>)}</div> : null}
    </Section>
    {selected ? <CategoryAccessDrawer client={client} access={selected} onSaved={handleSaved} onClose={() => setSelected(null)} /> : null}
  </PageCanvas>
}

function CategoryAccessDrawer({ client, access, onSaved, onClose }: { client: ReportCenterClient; access: ReportCategoryAccess; onSaved: (access: ReportCategoryAccess) => void; onClose: () => void }) {
  const [grants, setGrants] = useState<ReportGrant[]>(access.grants)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const validationError = validateGrants(grants)

  async function save() {
    if (saving || validationError) return
    setSaving(true)
    setError('')
    const result = await replaceReportCategoryAccess(client, { ...access, grants })
    if (!result.ok) {
      setSaving(false)
      setError(result.error)
      return
    }
    onSaved(result.data)
  }

  return <Drawer open title={`${access.category} · 分类权限`} description={`统一控制分类内 ${access.reportCount} 份报表；导出权限会自动包含查询权限。`} size="medium" closeDisabled={saving} onClose={onClose} footer={<><span className={styles.version}>{access.configured ? `策略版本 v${access.lockVersion}` : '尚未配置分类策略'}</span><button type="button" disabled={saving} onClick={onClose}>取消</button><Button variant="primary" disabled={saving || Boolean(validationError)} onClick={() => void save()}>{saving ? '保存中…' : '保存分类权限'}</Button></>}>
    <div className={styles.editorHeader}><div><strong>授权主体</strong><p>主体 ID 对应账号或角色管理中的数据库 ID。</p></div><button type="button" disabled={saving} onClick={() => setGrants((current) => [...current, { subjectType: 'ROLE', subjectId: 0, actions: ['QUERY'] }])}><Plus aria-hidden="true" />新增主体</button></div>
    {grants.length === 0 ? <FeedbackState kind="empty" title="保存后将暂停此分类访问" description="分类策略仍会保持启用，旧的逐报表授权不会再生效。" /> : <div className={styles.grants}>{grants.map((grant, index) => <GrantRow grant={grant} disabled={saving} key={index} onChange={(next) => setGrants((current) => replaceAt(current, index, next))} onDelete={() => setGrants((current) => current.filter((_, itemIndex) => itemIndex !== index))} />)}</div>}
    {validationError ? <p className={styles.error} role="alert">{validationError}</p> : null}
    {error ? <p className={styles.error} role="alert">{error}</p> : null}
  </Drawer>
}

function GrantRow({ grant, disabled, onChange, onDelete }: { grant: ReportGrant; disabled: boolean; onChange: (grant: ReportGrant) => void; onDelete: () => void }) {
  return <div className={styles.grantRow}>
    <label><span>主体类型</span><select disabled={disabled} value={grant.subjectType} onChange={(event) => onChange({ ...grant, subjectType: event.currentTarget.value as ReportGrant['subjectType'] })}><option value="ROLE">角色</option><option value="USER">用户</option></select></label>
    <label><span>主体 ID</span><input type="number" min="1" disabled={disabled} value={grant.subjectId || ''} onChange={(event) => onChange({ ...grant, subjectId: Number(event.currentTarget.value) })} /></label>
    <label className={styles.action}><input type="checkbox" disabled={disabled} checked={grant.actions.includes('QUERY')} onChange={(event) => onChange({ ...grant, actions: event.currentTarget.checked ? addAction(grant.actions, 'QUERY') : grant.actions.filter((action) => action !== 'QUERY' && action !== 'EXPORT') })} />查询</label>
    <label className={styles.action}><input type="checkbox" disabled={disabled} checked={grant.actions.includes('EXPORT')} onChange={(event) => onChange({ ...grant, actions: event.currentTarget.checked ? addAction(addAction(grant.actions, 'QUERY'), 'EXPORT') : grant.actions.filter((action) => action !== 'EXPORT') })} />生成并下载</label>
    <button className={styles.delete} type="button" aria-label="删除此分类授权" disabled={disabled} onClick={onDelete}><Trash2 aria-hidden="true" /></button>
  </div>
}

function addAction(actions: string[], action: string) { return actions.includes(action) ? actions : [...actions, action] }
function replaceAt<T>(items: T[], index: number, value: T) { return items.map((item, itemIndex) => itemIndex === index ? value : item) }
function validateGrants(grants: ReportGrant[]) {
  if (grants.some((grant) => !Number.isSafeInteger(grant.subjectId) || grant.subjectId <= 0 || grant.actions.length === 0)) return '每个主体都需要有效整数 ID，并至少选择查询权限。'
  const subjects = grants.map((grant) => `${grant.subjectType}:${grant.subjectId}`)
  if (new Set(subjects).size !== subjects.length) return '同一个用户或角色不能重复授权。'
  return ''
}
