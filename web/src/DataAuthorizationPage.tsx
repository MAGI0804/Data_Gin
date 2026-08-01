import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Check, Clipboard, KeyRound, Plus, RefreshCcw, RotateCcw, Search, ShieldCheck, ShieldOff, UserRound } from 'lucide-react'
import {
  authorizationExpiryISO,
  buildDataAuthorizationAuditQuery,
  dataAuthorizationAuditActions,
  defaultAuthorizationExpiry,
  newAuthorizationIdempotencyKey,
  parseCreatedAuthorization,
  parseDataAuthorizationAccounts,
  parseDataAuthorizationAudits,
  type CursorPagination,
  type DataAuthorizationAuditAction,
  type DataAuthorizationAccount,
  type DataAuthorizationAudit,
  type DataAuthorizationPermission,
  type DataPermissionCode,
} from './dataAuthorization'
import './DataAuthorizationPage.css'

type ApiResult = { ok: boolean; status: number; data: unknown }
type ApiClient = (path: string, options?: {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  body?: unknown
  headers?: Record<string, string>
  showResult?: boolean
  silentLoading?: boolean
  signal?: AbortSignal
}) => Promise<ApiResult>

type ActionDialog =
  | { kind: 'grant'; permission: DataPermissionCode }
  | { kind: 'revoke'; permission: DataPermissionCode }
  | { kind: 'reissue' }
  | null

type SubmittedAction = {
  kind: 'grant' | 'revoke' | 'reissue'
  targetID: number
  account: string
  permission?: DataPermissionCode
  expiresAt?: string
  reason: string
}

const emptyPagination: CursorPagination = { pageSize: 20, nextBeforeId: 0, hasMore: false }
type AuditFilters = { targetUserId: number; action: DataAuthorizationAuditAction | ''; startTime: string; endTime: string }
const emptyAuditFilters: AuditFilters = { targetUserId: 0, action: '', startTime: '', endTime: '' }
const permissionCatalog: Array<{ permission: DataPermissionCode; label: string; description: string }> = [
  { permission: 'weather.read', label: '天气数据查询', description: '可查询全部商场的天气实况、预报、预警和生活指数。' },
  { permission: 'bojun.order.read', label: 'Bojun 订单查询', description: '可按时间、商场和游标分页查询脱敏订单明细。' },
]

export function DataAuthorizationPage({ client }: { client: ApiClient }) {
  const [accounts, setAccounts] = useState<DataAuthorizationAccount[]>([])
  const [pagination, setPagination] = useState(emptyPagination)
  const [selectedID, setSelectedID] = useState<number | null>(null)
  const [keyword, setKeyword] = useState('')
  const [loading, setLoading] = useState(true)
  const [mutating, setMutating] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [audits, setAudits] = useState<DataAuthorizationAudit[]>([])
  const [auditPagination, setAuditPagination] = useState(emptyPagination)
  const [auditFilters, setAuditFilters] = useState<AuditFilters>(emptyAuditFilters)
  const [appliedAuditFilters, setAppliedAuditFilters] = useState<AuditFilters>(emptyAuditFilters)
  const [auditLoading, setAuditLoading] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const [actionDialog, setActionDialog] = useState<ActionDialog>(null)
  const [pendingAction, setPendingAction] = useState<SubmittedAction | null>(null)
  const [dangerConfirmed, setDangerConfirmed] = useState(false)
  const [oneTimeToken, setOneTimeToken] = useState('')
  const [tokenCopied, setTokenCopied] = useState(false)
  const requestSequence = useRef(0)
  const accountRequestRef = useRef<AbortController | null>(null)
  const auditRequestRef = useRef<AbortController | null>(null)
  const auditRequestSequence = useRef(0)
  const mutationInFlight = useRef(false)

  const selected = accounts.find((account) => account.id === selectedID) ?? accounts[0] ?? null

  const loadAccounts = useCallback(async (options: { append?: boolean; search?: string } = {}) => {
    accountRequestRef.current?.abort()
    const controller = new AbortController()
    accountRequestRef.current = controller
    const sequence = ++requestSequence.current
    setLoading(true)
    setError('')
    const append = options.append === true
    try {
      const response = await client('/v1/data-authorizations/accounts/query', {
        method: 'POST',
        body: { keyword: options.search ?? keyword, beforeId: append ? pagination.nextBeforeId : 0, pageSize: 20 },
        showResult: false,
        silentLoading: true,
        signal: controller.signal,
      })
      if (controller.signal.aborted || sequence !== requestSequence.current) return
      if (!response.ok) {
        setError(response.status === 403 ? '当前登录账号不是可信管理员，无法管理数据授权。' : '开放接口账号加载失败，请稍后重试。')
        return
      }
      const parsed = parseDataAuthorizationAccounts(response.data)
      setAccounts((current) => append ? deduplicateAccounts([...current, ...parsed.accounts]) : parsed.accounts)
      setPagination(parsed.pagination)
      if (!append && parsed.accounts.length > 0) setSelectedID((current) => parsed.accounts.some((account) => account.id === current) ? current : parsed.accounts[0].id)
    } catch {
      if (!controller.signal.aborted && sequence === requestSequence.current) setError('开放接口账号加载失败，请检查网络后重试。')
    } finally {
      if (!controller.signal.aborted && sequence === requestSequence.current) setLoading(false)
    }
  }, [client, keyword, pagination.nextBeforeId])

  const loadAudits = useCallback(async (filters: AuditFilters, options: { append?: boolean; beforeId?: number } = {}) => {
    auditRequestRef.current?.abort()
    const controller = new AbortController()
    auditRequestRef.current = controller
    const sequence = ++auditRequestSequence.current
    const append = options.append === true
    setAuditLoading(true)
    setError('')
    try {
      const response = await client('/v1/data-authorizations/audits/query', {
        method: 'POST',
        body: buildDataAuthorizationAuditQuery({ targetUserId: filters.targetUserId, action: filters.action, startTime: filters.startTime, endTime: filters.endTime, beforeId: append ? options.beforeId : 0, pageSize: 30 }),
        showResult: false,
        silentLoading: true,
        signal: controller.signal,
      })
      if (controller.signal.aborted || sequence !== auditRequestSequence.current) return
      if (!response.ok) {
        setError(response.status === 403 ? '当前登录账号不是可信管理员，无法查看授权审计。' : '授权审计加载失败，请稍后重试。')
        return
      }
      const parsed = parseDataAuthorizationAudits(response.data)
      setAudits((current) => append ? deduplicateAudits([...current, ...parsed.audits]) : parsed.audits)
      setAuditPagination(parsed.pagination)
    } catch {
      if (!controller.signal.aborted && sequence === auditRequestSequence.current) setError('授权审计加载失败，请检查网络后重试。')
    } finally {
      if (!controller.signal.aborted && sequence === auditRequestSequence.current) setAuditLoading(false)
    }
  }, [client])

  useEffect(() => {
    void loadAccounts({ search: '' })
    return () => {
      accountRequestRef.current?.abort()
      requestSequence.current += 1
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    const nextFilters = { ...emptyAuditFilters, targetUserId: selected?.id ?? 0 }
    setAuditFilters(nextFilters)
    setAppliedAuditFilters(nextFilters)
    void loadAudits(nextFilters)
  }, [loadAudits, selected?.id])
  useEffect(() => () => auditRequestRef.current?.abort(), [])

  async function refreshAfterMutation(message: string, targetID?: number) {
    setNotice(message)
    if (targetID) setSelectedID(targetID)
    await loadAccounts({ search: keyword })
    const nextFilters = { ...appliedAuditFilters, targetUserId: targetID ?? selected?.id ?? 0 }
    setAuditFilters(nextFilters)
    setAppliedAuditFilters(nextFilters)
    await loadAudits(nextFilters)
  }

  async function createAccount(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!beginMutation()) return
    const form = new FormData(event.currentTarget)
    const expiry = authorizationExpiryISO(formString(form, 'expiresAt'))
    const permissions = permissionCatalog
      .filter((item) => form.get(item.permission) === 'on')
      .map((item) => ({ permission: item.permission, expiresAt: expiry }))
    setError('')
    let response: ApiResult
    try {
      response = await client('/v1/data-authorizations/accounts/create', {
        method: 'POST',
        headers: { 'Idempotency-Key': newAuthorizationIdempotencyKey('account-create') },
        body: { account: formString(form, 'account'), email: formString(form, 'email'), nickname: formString(form, 'nickname'), permissions, reason: formString(form, 'reason') },
        showResult: false,
        silentLoading: true,
      })
    } catch {
      setError('账号开通失败，请检查网络后重试。')
      return
    } finally {
      endMutation()
    }
    if (!response.ok) {
      setError('账号开通失败，请检查字段和授权有效期。')
      return
    }
    const created = parseCreatedAuthorization(response.data)
    setCreateOpen(false)
    if (created.token && created.oneTimeTokenAvailable) setOneTimeToken(created.token)
    else setNotice(created.replayed ? '该开户请求已处理；出于安全原因，Token 不会再次展示。' : '账号已创建。')
    await refreshAfterMutation('开放接口账号已创建。', created.account?.id)
  }

  async function submitAction(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selected || !actionDialog || mutating) return
    const form = new FormData(event.currentTarget)
    let submittedAction: SubmittedAction
    if (actionDialog.kind === 'grant') {
      submittedAction = {
        kind: 'grant', targetID: selected.id, account: selected.account, permission: actionDialog.permission,
        expiresAt: authorizationExpiryISO(formString(form, 'expiresAt')), reason: formString(form, 'reason'),
      }
    } else if (actionDialog.kind === 'revoke') {
      submittedAction = {
        kind: 'revoke', targetID: selected.id, account: selected.account, permission: actionDialog.permission, reason: formString(form, 'reason'),
      }
    } else {
      submittedAction = { kind: 'reissue', targetID: selected.id, account: selected.account, reason: formString(form, 'reason') }
    }

    if (submittedAction.kind === 'revoke' || submittedAction.kind === 'reissue') {
      setActionDialog(null)
      setPendingAction(submittedAction)
      setDangerConfirmed(false)
      return
    }
    await submitAuthorizedAction(submittedAction)
  }

  async function submitAuthorizedAction(action: SubmittedAction) {
    if (!beginMutation()) return
    let path = ''
    let body: Record<string, unknown> = { reason: action.reason }
    let scope = ''
    if (action.kind === 'grant') {
      path = `/v1/data-authorizations/accounts/${action.targetID}/permissions/grant`
      body = { ...body, permission: action.permission, expiresAt: action.expiresAt }
      scope = 'permission-grant'
    } else if (action.kind === 'revoke') {
      path = `/v1/data-authorizations/accounts/${action.targetID}/permissions/revoke`
      body = { ...body, permission: action.permission }
      scope = 'permission-revoke'
    } else {
      path = `/v1/data-authorizations/accounts/${action.targetID}/token/reissue`
      scope = 'token-reissue'
    }
    setError('')
    let response: ApiResult
    try {
      response = await client(path, {
        method: 'POST', headers: { 'Idempotency-Key': newAuthorizationIdempotencyKey(scope) }, body, showResult: false, silentLoading: true,
      })
    } catch {
      setError('数据授权操作失败，请检查网络后重试。')
      return
    } finally {
      endMutation()
    }
    if (!response.ok) {
      setError('数据授权操作失败，请刷新后重试。')
      return
    }
    if (action.kind === 'reissue') {
      const data = responseData(response.data)
      const token = typeof data?.token === 'string' ? data.token : ''
      if (token) setOneTimeToken(token)
    }
    const message = action.kind === 'grant' ? '授权已保存。' : action.kind === 'revoke' ? '权限已撤销。' : '访问 Token 已重新签发。'
    setActionDialog(null)
    setPendingAction(null)
    setDangerConfirmed(false)
    await refreshAfterMutation(message, action.targetID)
  }

  function beginMutation() {
    if (mutationInFlight.current) return false
    mutationInFlight.current = true
    setMutating(true)
    return true
  }

  function endMutation() {
    mutationInFlight.current = false
    setMutating(false)
  }

  async function copyToken() {
    try {
      await navigator.clipboard.writeText(oneTimeToken)
      setTokenCopied(true)
    } catch {
      setTokenCopied(false)
    }
  }

  function submitAuditFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (auditFilters.startTime && auditFilters.endTime && new Date(auditFilters.startTime).getTime() > new Date(auditFilters.endTime).getTime()) {
      setError('审计结束时间不能早于开始时间。')
      return
    }
    const nextFilters = { ...auditFilters }
    setAppliedAuditFilters(nextFilters)
    void loadAudits(nextFilters)
  }

  function cancelAuditLoad() {
    auditRequestRef.current?.abort()
    auditRequestRef.current = null
    auditRequestSequence.current += 1
    setAuditLoading(false)
    setNotice('已取消授权审计查询，保留最近一次成功数据。')
  }

  return (
    <div className="view-stack data-authorization-page" aria-busy={loading || mutating}>
      <section className="data-authorization-toolbar">
        <div>
          <span className="eyebrow">ADMIN CONTROLLED ACCESS</span>
          <strong>账号由管理员创建，开放用户不进入内部控制台。</strong>
        </div>
        <div className="record-actions">
          <button type="button" onClick={() => void loadAccounts({ search: keyword })} disabled={loading}><RefreshCcw aria-hidden="true" />刷新</button>
          <button className="primary" type="button" onClick={() => setCreateOpen(true)}><Plus aria-hidden="true" />开通开放 API 账号</button>
        </div>
      </section>

      {(error || notice) && <div className={error ? 'data-authorization-banner error' : 'data-authorization-banner'} role={error ? 'alert' : 'status'} aria-live="polite">{error || notice}</div>}

      <section className="data-authorization-layout">
        <aside className="workbench-panel data-authorization-account-panel" aria-label="开放接口账号列表">
          <div className="panel-title"><UserRound aria-hidden="true" /><div><h3>开放接口账号</h3><span>{accounts.length} 个已加载账号</span></div></div>
          <form className="data-authorization-search" onSubmit={(event) => { event.preventDefault(); void loadAccounts({ search: keyword }) }}>
            <label><span className="sr-only">搜索账号</span><Search aria-hidden="true" /><input value={keyword} onChange={(event) => setKeyword(event.currentTarget.value)} placeholder="账号、邮箱或名称" /></label>
            <button type="submit">查询</button>
          </form>
          {loading && accounts.length === 0 && <div className="data-authorization-skeleton" aria-label="账号加载中" />}
          {!loading && accounts.length === 0 && !error && <div className="empty-state">暂无开放接口账号，请先由管理员开通。</div>}
          <div className="data-authorization-account-list">
            {accounts.map((account) => (
              <button className={selected?.id === account.id ? 'data-authorization-account active' : 'data-authorization-account'} type="button" key={account.id} onClick={() => setSelectedID(account.id)}>
                <span><strong>{account.nickname || account.account}</strong><small>{account.account} · {account.email}</small></span>
                <em>{account.credentialStatus === 'ACTIVE' ? '凭证有效' : '凭证已撤销'}</em>
              </button>
            ))}
          </div>
          {pagination.hasMore && <button type="button" onClick={() => void loadAccounts({ append: true })} disabled={loading}>加载更多</button>}
        </aside>

        <section className="workbench-panel data-authorization-detail">
          {selected ? <>
            <div className="data-authorization-account-heading">
              <div><span className="eyebrow">ACCOUNT #{selected.id}</span><h3>{selected.nickname || selected.account}</h3><p>{selected.account} · {selected.email}</p></div>
              <div className="record-actions"><span className="status-pill">{selected.credentialStatus === 'ACTIVE' ? '凭证有效' : '凭证已撤销'}</span><button type="button" onClick={() => setActionDialog({ kind: 'reissue' })}><RotateCcw aria-hidden="true" />重签 Token</button></div>
            </div>
            <dl className="data-authorization-credential"><div><dt>Token 标识</dt><dd><code>{selected.tokenPrefix || '-'}</code></dd></div><div><dt>签发时间</dt><dd>{formatDateTime(selected.issuedAt)}</dd></div><div><dt>权限范围</dt><dd>仅开放接口，不允许登录控制台</dd></div></dl>
            <div className="data-authorization-permissions">
              {permissionCatalog.map((catalog) => {
                const permission = selected.permissions.find((item) => item.permission === catalog.permission) ?? fallbackPermission(catalog.permission, catalog.label)
                return <PermissionRow key={catalog.permission} permission={permission} description={catalog.description} onGrant={() => setActionDialog({ kind: 'grant', permission: catalog.permission })} onRevoke={() => setActionDialog({ kind: 'revoke', permission: catalog.permission })} />
              })}
            </div>
          </> : <div className="empty-state">从左侧选择一个账号查看授权详情。</div>}
        </section>
      </section>

      <section className="workbench-panel data-authorization-audits">
        <div className="panel-title"><ShieldCheck aria-hidden="true" /><div><h3>授权审计</h3><span>{appliedAuditFilters.targetUserId ? `目标账号 #${appliedAuditFilters.targetUserId}` : '最近变更'} · ID 游标分页</span></div></div>
        <form className="query-bar" onSubmit={submitAuditFilters} aria-label="授权审计筛选">
          <div className="query-fields">
            <label>目标账号
              <select value={String(auditFilters.targetUserId)} disabled={auditLoading} onChange={(event) => setAuditFilters((current) => ({ ...current, targetUserId: Number(event.currentTarget.value) || 0 }))}>
                <option value="0">全部已加载账号</option>
                {accounts.map((account) => <option value={account.id} key={account.id}>{account.account}</option>)}
              </select>
            </label>
            <label>审计动作
              <select value={auditFilters.action} disabled={auditLoading} onChange={(event) => setAuditFilters((current) => ({ ...current, action: event.currentTarget.value as DataAuthorizationAuditAction | '' }))}>
                <option value="">全部动作</option>
                {dataAuthorizationAuditActions.map((action) => <option value={action} key={action}>{auditActionLabel(action)}</option>)}
              </select>
            </label>
            <label>开始时间<input type="datetime-local" value={auditFilters.startTime} disabled={auditLoading} onChange={(event) => setAuditFilters((current) => ({ ...current, startTime: event.currentTarget.value }))} /></label>
            <label>结束时间<input type="datetime-local" value={auditFilters.endTime} disabled={auditLoading} onChange={(event) => setAuditFilters((current) => ({ ...current, endTime: event.currentTarget.value }))} /></label>
          </div>
          <div className="record-actions"><button type="submit" disabled={auditLoading}>{auditLoading ? '查询中…' : '查询审计'}</button><button type="button" onClick={cancelAuditLoad} disabled={!auditLoading}>取消</button></div>
        </form>
        <p className="query-contract-note">服务端按目标账号、动作、时间范围和 <code>beforeId</code> 游标筛选；时间边界包含在查询结果内。</p>
        {audits.length === 0 ? <div className="empty-state">{auditLoading ? '授权审计加载中…' : '暂无匹配的授权变更记录。'}</div> : <div className="data-table-wrap"><table className="data-table"><thead><tr><th>时间</th><th>动作</th><th>账号</th><th>权限</th><th>有效期变更</th><th>原因</th></tr></thead><tbody>{audits.map((audit) => <tr key={audit.id}><td>{formatDateTime(audit.createdAt)}</td><td>{auditActionLabel(audit.action)}</td><td>{audit.targetAccount || `#${audit.targetUserId}`}</td><td>{permissionLabel(audit.permission)}</td><td>{formatAuditExpiry(audit)}</td><td className="data-authorization-reason">{audit.reason}</td></tr>)}</tbody></table></div>}
        {auditPagination.hasMore && <div className="record-actions"><button type="button" disabled={auditLoading} onClick={() => void loadAudits(appliedAuditFilters, { append: true, beforeId: auditPagination.nextBeforeId })}>{auditLoading ? '加载中…' : '加载更早记录'}</button></div>}
      </section>

      {createOpen && <Modal title="开通开放 API 账号" closeDisabled={mutating} onClose={() => setCreateOpen(false)}><form className="data-authorization-form" onSubmit={createAccount}>
        <label>账号<input name="account" required minLength={3} maxLength={40} pattern="[a-z0-9][a-z0-9_\-]{2,39}" placeholder="partner_weather_01" autoFocus /></label>
        <label>邮箱<input name="email" required type="email" maxLength={80} placeholder="owner@example.com" /></label>
        <label>显示名称<input name="nickname" required maxLength={64} placeholder="合作方天气账号" /></label>
        <fieldset><legend>初始数据权限（可暂不授权）</legend>{permissionCatalog.map((item) => <label className="data-authorization-checkbox" key={item.permission}><input type="checkbox" name={item.permission} /><span><strong>{item.label}</strong><small>{item.description}</small></span></label>)}</fieldset>
        <label>统一到期时间<input name="expiresAt" type="datetime-local" defaultValue={defaultAuthorizationExpiry()} /></label>
        <label className="wide">开户原因<textarea name="reason" required maxLength={500} rows={3} placeholder="说明申请方、用途和审批依据" /></label>
        <p className="data-authorization-warning wide"><KeyRound aria-hidden="true" />账号创建后只展示一次访问 Token，请立即复制并通过安全渠道交付。</p>
        <div className="data-authorization-form-actions wide"><button type="button" onClick={() => setCreateOpen(false)} disabled={mutating}>取消</button><button className="primary" type="submit" disabled={mutating}>{mutating ? '正在开通…' : '确认开通'}</button></div>
      </form></Modal>}

      {actionDialog && selected && <Modal title={actionTitle(actionDialog)} closeDisabled={mutating} onClose={() => setActionDialog(null)}><form className="data-authorization-form single" onSubmit={submitAction}>
        {actionDialog.kind === 'grant' && <label>授权到期时间<input name="expiresAt" type="datetime-local" defaultValue={defaultAuthorizationExpiry()} required autoFocus /></label>}
        <label>操作原因<textarea name="reason" required maxLength={500} rows={4} autoFocus={actionDialog.kind !== 'grant'} placeholder="填写审批依据或撤销原因" /></label>
        <div className="data-authorization-form-actions"><button type="button" onClick={() => setActionDialog(null)} disabled={mutating}>取消</button><button className={actionDialog.kind === 'revoke' ? 'danger' : 'primary'} type="submit" disabled={mutating}>{mutating ? '正在提交…' : '继续'}</button></div>
      </form></Modal>}

      {pendingAction && <Modal title={dangerousActionTitle(pendingAction)} closeDisabled={mutating} onClose={() => { setPendingAction(null); setDangerConfirmed(false) }}><form className="data-authorization-form single" onSubmit={(event) => { event.preventDefault(); if (dangerConfirmed) void submitAuthorizedAction(pendingAction) }}>
        <p className="data-authorization-warning">{dangerousActionDescription(pendingAction)}</p>
        <label className="data-authorization-checkbox"><input type="checkbox" checked={dangerConfirmed} disabled={mutating} onChange={(event) => setDangerConfirmed(event.currentTarget.checked)} /><span><strong>我已理解此操作的影响</strong><small>确认后将立即执行，且不能撤回。</small></span></label>
        <div className="data-authorization-form-actions"><button type="button" onClick={() => { setPendingAction(null); setDangerConfirmed(false) }} disabled={mutating}>取消</button><button className="danger" type="submit" disabled={mutating || !dangerConfirmed}>{mutating ? '正在提交…' : '确认执行'}</button></div>
      </form></Modal>}

      {oneTimeToken && <Modal title="访问 Token（仅展示一次）" onClose={() => { setOneTimeToken(''); setTokenCopied(false) }}><div className="data-authorization-token" role="status"><p>关闭后无法再次查看。请现在复制，并通过安全渠道交付给接口调用方。</p><code>{oneTimeToken}</code><button className="primary" type="button" onClick={() => void copyToken()}>{tokenCopied ? <Check aria-hidden="true" /> : <Clipboard aria-hidden="true" />}{tokenCopied ? '已复制' : '复制 Token'}</button></div></Modal>}
    </div>
  )
}

function PermissionRow({ permission, description, onGrant, onRevoke }: { permission: DataAuthorizationPermission; description: string; onGrant: () => void; onRevoke: () => void }) {
  const active = permission.status === 'ACTIVE'
  return <article className="data-authorization-permission"><div className="data-authorization-permission-icon">{active ? <ShieldCheck aria-hidden="true" /> : <ShieldOff aria-hidden="true" />}</div><div><div className="data-authorization-permission-title"><strong>{permission.label}</strong><span className={`status-pill ${permission.status.toLowerCase()}`}>{permissionStatusLabel(permission.status)}</span></div><p>{description}</p><small>{permission.scope} · 到期时间：{formatDateTime(permission.expiresAt)}</small></div><div className="record-actions"><button type="button" onClick={onGrant}>{active ? '续期' : '授权'}</button>{permission.status !== 'NOT_GRANTED' && <button className="danger" type="button" onClick={onRevoke}>撤销</button>}</div></article>
}

function Modal({ title, children, onClose, closeDisabled = false }: { title: string; children: React.ReactNode; onClose: () => void; closeDisabled?: boolean }) {
  const panelRef = useRef<HTMLElement>(null)
  const returnFocusRef = useRef<HTMLElement | null>(document.activeElement instanceof HTMLElement ? document.activeElement : null)
  const titleID = useMemo(() => `data-authorization-modal-${Math.random().toString(36).slice(2)}`, [])
  const onCloseRef = useRef(onClose)
  const closeDisabledRef = useRef(closeDisabled)
  onCloseRef.current = onClose
  closeDisabledRef.current = closeDisabled

  useEffect(() => {
    const previousOverflow = document.body.style.overflow
    const returnFocus = returnFocusRef.current
    const focusableSelector = 'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        if (!closeDisabledRef.current) onCloseRef.current()
        return
      }
      if (event.key !== 'Tab') return
      const focusable = Array.from(panelRef.current?.querySelectorAll<HTMLElement>(focusableSelector) ?? [])
      if (focusable.length === 0) {
        event.preventDefault()
        panelRef.current?.focus()
        return
      }
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      }
    }
    document.body.style.overflow = 'hidden'
    window.addEventListener('keydown', handleKeyDown)
    const initialFocus = panelRef.current?.querySelector<HTMLElement>(focusableSelector)
    initialFocus?.focus()
    return () => {
      document.body.style.overflow = previousOverflow
      window.removeEventListener('keydown', handleKeyDown)
      returnFocus?.focus()
    }
  }, [])

  return <div className="modal-backdrop" role="presentation"><section ref={panelRef} className="modal-panel data-authorization-modal" role="dialog" aria-modal="true" aria-labelledby={titleID} tabIndex={-1}><div className="modal-title"><h3 id={titleID}>{title}</h3><button type="button" onClick={onClose} disabled={closeDisabled} aria-label={`关闭${title}`}>关闭</button></div><div className="modal-body">{children}</div></section></div>
}

function responseData(payload: unknown) { return payload && typeof payload === 'object' && !Array.isArray(payload) && (payload as { data?: unknown }).data && typeof (payload as { data?: unknown }).data === 'object' ? (payload as { data: Record<string, unknown> }).data : null }
function formString(form: FormData, key: string) { const value = form.get(key); return typeof value === 'string' ? value.trim() : '' }
function deduplicateAccounts(accounts: DataAuthorizationAccount[]) { return [...new Map(accounts.map((account) => [account.id, account])).values()] }
function deduplicateAudits(audits: DataAuthorizationAudit[]) { return [...new Map(audits.map((audit) => [audit.id, audit])).values()] }
function fallbackPermission(permission: DataPermissionCode, label: string): DataAuthorizationPermission { return { permission, label, scope: '全模块数据', status: 'NOT_GRANTED', expiresAt: null } }
function permissionLabel(permission: string) { return permission === 'weather.read' ? '天气数据查询' : permission === 'bojun.order.read' ? 'Bojun 订单查询' : permission === 'open_api.account' ? '开放接口账号' : permission === 'open_api.credential' ? '访问凭证' : permission }
function permissionStatusLabel(status: string) { return status === 'ACTIVE' ? '生效中' : status === 'EXPIRED' ? '已过期' : '未授权' }
function auditActionLabel(action: string) { return ({ ACCOUNT_CREATE: '开通账号', GRANT: '授权', RENEW: '续期', REVOKE: '撤销', TOKEN_REISSUE: '重签 Token' } as Record<string, string>)[action] ?? action }
function actionTitle(action: NonNullable<ActionDialog>) { return action.kind === 'grant' ? `授权 / 续期 · ${permissionLabel(action.permission)}` : action.kind === 'revoke' ? `撤销 · ${permissionLabel(action.permission)}` : '重新签发访问 Token' }
function dangerousActionTitle(action: SubmittedAction) { return action.kind === 'revoke' ? '确认撤销数据权限' : '确认重新签发访问 Token' }
function dangerousActionDescription(action: SubmittedAction) { return action.kind === 'revoke' ? `确认撤销 ${action.account} 的 ${permissionLabel(action.permission ?? '')}？撤销后，该权限的下一次开放接口请求将立即失效。` : `确认重新签发 ${action.account} 的访问 Token？旧 Token 将立即失效。` }
function formatDateTime(value: string | null | undefined) { if (!value) return '-'; const date = new Date(value); return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat('zh-CN', { timeZone: 'Asia/Shanghai', dateStyle: 'medium', timeStyle: 'short' }).format(date) }
function formatAuditExpiry(audit: DataAuthorizationAudit) { if (!audit.oldExpiresAt && !audit.newExpiresAt) return '-'; return `${formatDateTime(audit.oldExpiresAt)} → ${formatDateTime(audit.newExpiresAt)}` }
