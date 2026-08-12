import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react'
import { KeyRound, Plus, RefreshCcw, ShieldCheck } from 'lucide-react'
import { DataAuthorizationPage } from './DataAuthorizationPage'
import { parseAccessAccounts, type AccessAccount as Account } from './accessManagement'
import './AccessManagementPage.css'

type ApiResult = { ok: boolean; status: number; data: unknown }
type ApiClient = (path: string, options?: { method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'; body?: unknown; headers?: Record<string, string>; showResult?: boolean; silentLoading?: boolean; signal?: AbortSignal }) => Promise<ApiResult>
type Tab = 'accounts' | 'roles' | 'open-api' | 'audits'
type Role = { id: number; code: string; name: string; description: string; status: string; isSystem: boolean; isSuper: boolean; permissions: string[] }
type Permission = { code: string; name: string; module: string; description: string; riskLevel: string }
type Audit = { id: number; actorUserId: number; action: string; targetType: string; targetId: number; reason: string; createdAt: string }

const headers = () => ({ 'Idempotency-Key': `access:${globalThis.crypto.randomUUID()}` })

function envelopeValue(payload: unknown): unknown {
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) return null
  return (payload as Record<string, unknown>).data
}

function list<T>(payload: unknown, key?: string): T[] {
  const data = envelopeValue(payload)
  const value = key && data && typeof data === 'object' && !Array.isArray(data) ? (data as Record<string, unknown>)[key] : data
  return Array.isArray(value) ? value as T[] : []
}

function ids(raw: string) {
  return [...new Set(raw.split(',').map((value) => Number(value.trim())).filter((value) => Number.isSafeInteger(value) && value > 0))]
}

export function AccessManagementPage({ client, permissions }: { client: ApiClient; permissions: string[] }) {
  const canAccountRead = permissions.includes('system.account.read') || permissions.includes('system.account.manage')
  const canAccountManage = permissions.includes('system.account.manage')
  const canRoleRead = permissions.includes('system.role.read') || permissions.includes('system.role.manage')
  const canRoleManage = permissions.includes('system.role.manage')
  const canAuditRead = permissions.includes('system.audit.read')
  const availableTabs = useMemo(() => [
    ...(canAccountRead ? ['accounts' as const] : []),
    ...(canRoleRead ? ['roles' as const] : []),
    ...(canAccountManage ? ['open-api' as const] : []),
    ...(canAuditRead ? ['audits' as const] : []),
  ], [canAccountManage, canAccountRead, canAuditRead, canRoleRead])
  const [tab, setTab] = useState<Tab>(availableTabs[0] ?? 'accounts')

  useEffect(() => {
    if (!availableTabs.includes(tab)) setTab(availableTabs[0] ?? 'accounts')
  }, [availableTabs, tab])

  if (availableTabs.length === 0) return <div className="empty-state">当前账号没有账号与权限查看权限。</div>
  return <div className="access-page view-stack">
    <nav className="access-tabs" aria-label="账号与权限模块">
      {availableTabs.map((item) => <button type="button" key={item} className={tab === item ? 'active' : ''} aria-current={tab === item ? 'page' : undefined} onClick={() => setTab(item)}>{item === 'accounts' ? '控制台账号' : item === 'roles' ? '角色管理' : item === 'open-api' ? '开放 API' : '权限审计'}</button>)}
    </nav>
    {tab === 'accounts' && <AccountPanel client={client} canManage={canAccountManage} />}
    {tab === 'roles' && <RolePanel client={client} canManage={canRoleManage} />}
    {tab === 'open-api' && <DataAuthorizationPage client={client} />}
    {tab === 'audits' && <AuditPanel client={client} />}
  </div>
}

function AccountPanel({ client, canManage }: { client: ApiClient; canManage: boolean }) {
  const [accounts, setAccounts] = useState<Account[]>([])
  const [roles, setRoles] = useState<Role[]>([])
  const [selectedID, setSelectedID] = useState(0)
  const [keyword, setKeyword] = useState('')
  const [creating, setCreating] = useState(false)
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')
  const selected = accounts.find((item) => item.id === selectedID) ?? accounts[0]

  const load = useCallback(async () => {
    setBusy(true); setMessage('')
    const [accountResponse, roleResponse] = await Promise.all([
      client('/v1/access/accounts/query', { method: 'POST', body: { keyword, pageSize: 100 }, showResult: false, silentLoading: true }),
      client('/v1/access/roles', { method: 'GET', showResult: false, silentLoading: true }),
    ])
    if (accountResponse.ok) {
      const next = parseAccessAccounts(accountResponse.data)
      setAccounts(next)
      setSelectedID((current) => next.some((item) => item.id === current) ? current : next[0]?.id ?? 0)
    } else setMessage('控制台账号加载失败。')
    if (roleResponse.ok) setRoles(list<Role>(roleResponse.data))
    setBusy(false)
  }, [client, keyword])
  useEffect(() => { void load() }, [load])

  async function mutate(path: string, body: unknown) {
    setBusy(true); setMessage('')
    const response = await client(path, { method: 'PUT', headers: headers(), body, showResult: false, silentLoading: true })
    setMessage(response.ok ? '账号设置已保存。' : '账号设置保存失败，请检查权限和输入。')
    if (response.ok) await load()
    setBusy(false)
  }

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setMessage('')
    const form = new FormData(event.currentTarget)
    const response = await client('/v1/access/accounts', { method: 'POST', headers: headers(), body: {
      account: value(form, 'account'), phone: value(form, 'phone'), nickname: value(form, 'nickname'), password: value(form, 'password'),
      roleIds: form.getAll('roleIds').map(Number), mallScopeMode: value(form, 'mallScopeMode'), mallIds: ids(value(form, 'mallIds')), reason: value(form, 'reason'),
    }, showResult: false, silentLoading: true })
    setMessage(response.ok ? '控制台账号已创建。' : '账号创建失败，请检查手机号、角色和商场范围。')
    if (response.ok) { setCreating(false); await load() }
    setBusy(false)
  }

  return <section className="access-layout">
    <aside className="panel access-list-panel">
      <form className="access-search" onSubmit={(event) => { event.preventDefault(); void load() }}><input aria-label="搜索控制台账号" value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder="账号、昵称或手机号" /><button type="submit">查询</button></form>
      {canManage && <button className="primary" type="button" onClick={() => setCreating(true)}><Plus />创建子账号</button>}
      <div className="access-list">{accounts.map((account) => <button type="button" className={selected?.id === account.id ? 'active' : ''} key={account.id} onClick={() => setSelectedID(account.id)}><span><strong>{account.nickname}</strong><small>{account.account} · {account.phone || '未绑定'}</small></span><em>{account.status === 'ACTIVE' ? '启用' : '停用'}</em></button>)}</div>
    </aside>
    <div className="panel access-detail">
      {message && <p className="access-message" role="status">{message}</p>}
      {!selected ? <div className="empty-state">暂无控制台账号。</div> : <>
        <header><div><span>控制台账号</span><h3>{selected.nickname}</h3><p>{selected.account} · {selected.phone}</p></div>{canManage && <button type="button" disabled={busy} onClick={() => void mutate(`/v1/access/accounts/${selected.id}/status`, { status: selected.status === 'ACTIVE' ? 'DISABLED' : 'ACTIVE', reason: selected.status === 'ACTIVE' ? '管理员停用账号' : '管理员启用账号' })}>{selected.status === 'ACTIVE' ? '停用账号' : '启用账号'}</button>}</header>
        <dl className="access-summary"><div><dt>状态</dt><dd>{selected.status}</dd></div><div><dt>角色</dt><dd>{selected.roles.map((role) => role.name).join('、') || '未分配'}</dd></div><div><dt>商场范围</dt><dd>{selected.mallScopeMode === 'ALL' ? '全部商场' : selected.mallIds.join(', ')}</dd></div></dl>
        {canManage && <div className="access-forms">
          <form onSubmit={(event) => { event.preventDefault(); const form = new FormData(event.currentTarget); void mutate(`/v1/access/accounts/${selected.id}`, { phone: value(form, 'phone'), nickname: value(form, 'nickname'), reason: value(form, 'reason') }) }}><h4>基础信息</h4><input name="phone" required pattern="1[3-9][0-9]{9}" placeholder="输入完整手机号" /><input name="nickname" defaultValue={selected.nickname} required placeholder="昵称" /><input name="reason" required placeholder="变更原因" /><button type="submit" disabled={busy}>保存信息</button></form>
          <form onSubmit={(event) => { event.preventDefault(); const form = new FormData(event.currentTarget); void mutate(`/v1/access/accounts/${selected.id}/roles`, { roleIds: form.getAll('roleIds').map(Number), reason: value(form, 'reason') }) }}><h4>角色全量替换</h4>{roles.filter((role) => role.status === 'ACTIVE').map((role) => <label className="access-check" key={role.id}><input type="checkbox" name="roleIds" value={role.id} defaultChecked={selected.roles.some((item) => item.code === role.code)} />{role.name}</label>)}<input name="reason" required placeholder="变更原因" /><button type="submit" disabled={busy}>保存角色</button></form>
          <form onSubmit={(event) => { event.preventDefault(); const form = new FormData(event.currentTarget); void mutate(`/v1/access/accounts/${selected.id}/mall-scope`, { mallScopeMode: value(form, 'mallScopeMode'), mallIds: ids(value(form, 'mallIds')), reason: value(form, 'reason') }) }}><h4>商场范围</h4><select name="mallScopeMode" defaultValue={selected.mallScopeMode}><option value="ALL">全部商场</option><option value="SELECTED">指定商场</option></select><input name="mallIds" defaultValue={selected.mallIds.join(',')} placeholder="商场 ID，逗号分隔" /><input name="reason" required placeholder="变更原因" /><button type="submit" disabled={busy}>保存范围</button></form>
          <form onSubmit={(event) => { event.preventDefault(); const form = new FormData(event.currentTarget); void mutate(`/v1/access/accounts/${selected.id}/password`, { password: value(form, 'password'), reason: value(form, 'reason') }) }}><h4>重置密码</h4><input type="password" name="password" minLength={10} maxLength={72} required autoComplete="new-password" placeholder="10–72 位新密码" /><input name="reason" required placeholder="重置原因" /><button type="submit" disabled={busy}><KeyRound />重置密码</button></form>
        </div>}
      </>}
    </div>
    {creating && <div className="modal-backdrop" role="presentation"><section className="modal access-modal" role="dialog" aria-modal="true" aria-labelledby="create-account-title"><header><h3 id="create-account-title">创建控制台账号</h3><button type="button" onClick={() => setCreating(false)}>关闭</button></header><form onSubmit={create}><input name="account" required minLength={3} maxLength={40} placeholder="账号" /><input name="phone" required pattern="1[3-9][0-9]{9}" placeholder="手机号" /><input name="nickname" required placeholder="昵称" /><input name="password" required type="password" minLength={10} maxLength={72} autoComplete="new-password" placeholder="初始密码" /><fieldset><legend>角色</legend>{roles.filter((role) => role.status === 'ACTIVE').map((role) => <label className="access-check" key={role.id}><input type="checkbox" name="roleIds" value={role.id} />{role.name}</label>)}</fieldset><select name="mallScopeMode" defaultValue="SELECTED"><option value="SELECTED">指定商场</option><option value="ALL">全部商场</option></select><input name="mallIds" placeholder="商场 ID，逗号分隔" /><textarea name="reason" required placeholder="创建原因" /><button className="primary" type="submit" disabled={busy}>创建账号</button></form></section></div>}
  </section>
}

function RolePanel({ client, canManage }: { client: ApiClient; canManage: boolean }) {
  const [roles, setRoles] = useState<Role[]>([]); const [catalog, setCatalog] = useState<Permission[]>([]); const [selectedID, setSelectedID] = useState(0); const [message, setMessage] = useState(''); const [busy, setBusy] = useState(false); const [creating, setCreating] = useState(false)
  const selected = roles.find((role) => role.id === selectedID) ?? roles[0]
  const load = useCallback(async () => { setBusy(true); const [rolesResponse, permissionResponse] = await Promise.all([client('/v1/access/roles', { method: 'GET', showResult: false, silentLoading: true }), client('/v1/access/permissions', { method: 'GET', showResult: false, silentLoading: true })]); if (rolesResponse.ok) { const next = list<Role>(rolesResponse.data); setRoles(next); setSelectedID((current) => next.some((role) => role.id === current) ? current : next[0]?.id ?? 0) }; if (permissionResponse.ok) setCatalog(list<Permission>(permissionResponse.data)); setBusy(false) }, [client])
  useEffect(() => { void load() }, [load])
  const groups = useMemo(() => {
    const grouped: Record<string, Permission[]> = {}
    for (const permission of catalog) (grouped[permission.module || '其他'] ??= []).push(permission)
    return Object.entries(grouped)
  }, [catalog])
  async function savePermissions(event: FormEvent<HTMLFormElement>) { event.preventDefault(); if (!selected) return; const form = new FormData(event.currentTarget); setBusy(true); const response = await client(`/v1/access/roles/${selected.id}/permissions`, { method: 'PUT', headers: headers(), body: { permissions: form.getAll('permissions'), reason: value(form, 'reason') }, showResult: false, silentLoading: true }); setMessage(response.ok ? '角色权限已保存，相关账号旧会话已失效。' : '角色权限保存失败。'); if (response.ok) await load(); setBusy(false) }
  async function roleMutation(path: string, method: 'POST' | 'PUT' | 'DELETE', body: unknown, success: string) { setBusy(true); const response = await client(path, { method, headers: headers(), body, showResult: false, silentLoading: true }); setMessage(response.ok ? success : '角色操作失败，请检查系统角色保护和当前权限。'); if (response.ok) await load(); setBusy(false) }
  async function createRole(event: FormEvent<HTMLFormElement>) { event.preventDefault(); const form = new FormData(event.currentTarget); await roleMutation('/v1/access/roles', 'POST', { code: value(form, 'code'), name: value(form, 'name'), description: value(form, 'description'), permissions: form.getAll('permissions'), reason: value(form, 'reason') }, '自定义角色已创建。'); setCreating(false) }
  return <section className="access-layout"><aside className="panel access-list-panel"><div className="access-panel-title"><ShieldCheck /><strong>角色目录</strong></div>{canManage && <button className="primary" type="button" onClick={() => setCreating(true)}><Plus />创建角色</button>}<div className="access-list">{roles.map((role) => <button type="button" className={selected?.id === role.id ? 'active' : ''} key={role.id} onClick={() => setSelectedID(role.id)}><span><strong>{role.name}</strong><small>{role.code}</small></span><em>{role.isSystem ? '系统' : '自定义'}</em></button>)}</div></aside><div className="panel access-detail">{message && <p className="access-message" role="status">{message}</p>}{!selected ? <div className="empty-state">暂无角色。</div> : <><header><div><span>角色</span><h3>{selected.name}</h3><p>{selected.description || selected.code}</p></div>{canManage && !selected.isSystem ? <div className="access-role-actions"><button type="button" onClick={() => void roleMutation(`/v1/access/roles/${selected.id}/status`, 'PUT', { status: selected.status === 'ACTIVE' ? 'DISABLED' : 'ACTIVE', reason: '管理员调整角色状态' }, '角色状态已更新。')}>{selected.status === 'ACTIVE' ? '停用' : '启用'}</button><button className="danger" type="button" onClick={() => void roleMutation(`/v1/access/roles/${selected.id}`, 'DELETE', { reason: '管理员删除未使用角色' }, '角色已删除。')}>删除</button></div> : <span>{selected.status}</span>}</header><form key={selected.id} className="permission-matrix" onSubmit={savePermissions}>{groups.map(([module, items]) => <fieldset key={module}><legend>{module}</legend>{items.map((permission) => <label key={permission.code}><input type="checkbox" name="permissions" value={permission.code} defaultChecked={selected.permissions.includes(permission.code)} disabled={!canManage || selected.isSystem} /><span><strong>{permission.name}</strong><small>{permission.code} · {permission.description}</small></span><em>{permission.riskLevel}</em></label>)}</fieldset>)}{canManage && !selected.isSystem && <div className="access-save-row"><input name="reason" required placeholder="权限变更原因" /><button className="primary" type="submit" disabled={busy}>保存权限矩阵</button></div>}</form></>}</div>{creating && <div className="modal-backdrop" role="presentation"><section className="modal access-modal" role="dialog" aria-modal="true" aria-labelledby="create-role-title"><header><h3 id="create-role-title">创建自定义角色</h3><button type="button" onClick={() => setCreating(false)}>关闭</button></header><form onSubmit={createRole}><input name="code" required pattern="[a-z][a-z0-9_]{2,63}" placeholder="角色代码" /><input name="name" required placeholder="角色名称" /><textarea name="description" placeholder="角色说明" /><fieldset><legend>初始权限</legend>{catalog.map((permission) => <label className="access-check" key={permission.code}><input type="checkbox" name="permissions" value={permission.code} />{permission.name}</label>)}</fieldset><textarea name="reason" required placeholder="创建原因" /><button className="primary" type="submit" disabled={busy}>创建角色</button></form></section></div>}</section>
}

function AuditPanel({ client }: { client: ApiClient }) {
  const [audits, setAudits] = useState<Audit[]>([]); const [action, setAction] = useState(''); const [targetType, setTargetType] = useState(''); const [busy, setBusy] = useState(false)
  const load = useCallback(async () => { setBusy(true); const query = new URLSearchParams({ pageSize: '100' }); if (action) query.set('action', action); if (targetType) query.set('targetType', targetType); const response = await client(`/v1/access/audits?${query}`, { method: 'GET', showResult: false, silentLoading: true }); if (response.ok) setAudits(list<Audit>(response.data, 'audits')); setBusy(false) }, [action, client, targetType])
  useEffect(() => { void load() }, [load])
  return <section className="panel"><form className="access-audit-filter" onSubmit={(event) => { event.preventDefault(); void load() }}><input value={action} onChange={(event) => setAction(event.target.value)} placeholder="动作，例如 ACCOUNT_STATUS" /><select value={targetType} onChange={(event) => setTargetType(event.target.value)}><option value="">全部对象</option><option value="ACCOUNT">账号</option><option value="role">角色</option></select><button type="submit" disabled={busy}><RefreshCcw />查询</button></form><div className="data-table-wrap"><table className="data-table"><thead><tr><th>时间</th><th>操作者</th><th>动作</th><th>对象</th><th>原因</th></tr></thead><tbody>{audits.map((audit) => <tr key={audit.id}><td>{new Date(audit.createdAt).toLocaleString('zh-CN')}</td><td>#{audit.actorUserId}</td><td>{audit.action}</td><td>{audit.targetType} #{audit.targetId}</td><td>{audit.reason}</td></tr>)}</tbody></table></div>{audits.length === 0 && <div className="empty-state">暂无符合条件的权限审计。</div>}</section>
}

function value(form: FormData, key: string) { const item = form.get(key); return typeof item === 'string' ? item.trim() : '' }
