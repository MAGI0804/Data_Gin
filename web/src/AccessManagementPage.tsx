import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import Select from 'antd/es/select'
import { KeyRound, Plus, RefreshCcw, ShieldCheck } from 'lucide-react'
import { AccessRoleCreateDrawer, type AccessRoleCreateInput } from './AccessRoleCreateDrawer'
import { DataAuthorizationPage } from './DataAuthorizationPage'
import {
  accountRoleIDs,
  accessMallCatalogPath,
  accessMallScopeRequest,
  accessManagementCapabilities,
  canReplaceAccountRoles,
  mergeAccessMalls,
  parseAccessAccounts,
  parseAccessMallCatalog,
  parseCreatedAccessRole,
  updateRoleSelection,
  type AccessAccount as Account,
  type AccessMall,
  type AccessPermission as Permission,
  type AccessRole as Role,
  type RoleCatalogStatus,
} from './accessManagement'
import { DataTable, Dialog, Drawer, FeedbackState, FilterToolbar, MasterDetail, PageCanvas, PageHeader, Section, StatusTag } from './ui'
import styles from './AccessManagementPage.module.css'

type ApiResult = { ok: boolean; status: number; data: unknown }
type ApiClient = (path: string, options?: { method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'; body?: unknown; headers?: Record<string, string>; showResult?: boolean; silentLoading?: boolean; signal?: AbortSignal }) => Promise<ApiResult>
type Tab = 'accounts' | 'roles' | 'open-api' | 'audits'
type Audit = { id: number; actorUserId: number; action: string; targetType: string; targetId: number; reason: string; createdAt: string }
type MallCatalogStatus = 'idle' | 'loading' | 'ready' | 'error'

const headers = () => ({ 'Idempotency-Key': `access:${globalThis.crypto.randomUUID()}` })
const tabLabels: Record<Tab, string> = { accounts: '控制台账号', roles: '角色管理', 'open-api': '开放 API', audits: '权限审计' }

function envelopeValue(payload: unknown): unknown {
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) return null
  return (payload as Record<string, unknown>).data
}

function list<T>(payload: unknown, key?: string): T[] {
  const data = envelopeValue(payload)
  const result = key && data && typeof data === 'object' && !Array.isArray(data) ? (data as Record<string, unknown>)[key] : data
  return Array.isArray(result) ? result as T[] : []
}

export function AccessManagementPage({ client, permissions }: { client: ApiClient; permissions: string[] }) {
  const { canAccountManage, canAccountRead, canAuditRead, canRoleManage, canRoleRead } = accessManagementCapabilities(permissions)
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

  if (availableTabs.length === 0) {
    return <PageCanvas className={styles.page}>
      <PageHeader eyebrow="ACCESS CONTROL" title="账号与权限" description="在一个工作画布中管理控制台账号、角色权限矩阵、开放 API 与变更审计。" />
      <FeedbackState kind="error" title="当前账号无权查看账号与权限" description="请联系管理员补充系统账号、角色或审计查看权限。" />
    </PageCanvas>
  }

  return <PageCanvas className={styles.page}>
    <PageHeader eyebrow="ACCESS CONTROL" title="账号与权限" description="在一个工作画布中管理控制台账号、角色权限矩阵、开放 API 与变更审计。" />
    <nav className={styles.tabs} aria-label="账号与权限模块">
      {availableTabs.map((item) => <button type="button" key={item} className={tab === item ? styles.activeTab : undefined} aria-current={tab === item ? 'page' : undefined} onClick={() => setTab(item)}>{tabLabels[item]}</button>)}
    </nav>
    {tab === 'accounts' && <AccountPanel client={client} canManage={canAccountManage} canManageRoles={canRoleManage} canReadRoles={canRoleRead} grantablePermissionCodes={permissions} />}
    {tab === 'roles' && <RolePanel client={client} canManage={canRoleManage} grantablePermissionCodes={permissions} />}
    {tab === 'open-api' && <DataAuthorizationPage client={client} />}
    {tab === 'audits' && <AuditPanel client={client} />}
  </PageCanvas>
}

function AccountPanel({ client, canManage, canManageRoles, canReadRoles, grantablePermissionCodes }: { client: ApiClient; canManage: boolean; canManageRoles: boolean; canReadRoles: boolean; grantablePermissionCodes: string[] }) {
  const [accounts, setAccounts] = useState<Account[]>([])
  const [roles, setRoles] = useState<Role[]>([])
  const [catalog, setCatalog] = useState<Permission[]>([])
  const [malls, setMalls] = useState<AccessMall[]>([])
  const [mallCatalogStatus, setMallCatalogStatus] = useState<MallCatalogStatus>('idle')
  const [roleCatalogStatus, setRoleCatalogStatus] = useState<RoleCatalogStatus>('idle')
  const [createRoleIDs, setCreateRoleIDs] = useState<number[]>([])
  const [editRoleIDs, setEditRoleIDs] = useState<number[]>([])
  const [createMallScopeMode, setCreateMallScopeMode] = useState<'ALL' | 'SELECTED'>('ALL')
  const [createMallIDs, setCreateMallIDs] = useState<number[]>([])
  const [editMallScopeMode, setEditMallScopeMode] = useState<'ALL' | 'SELECTED'>('ALL')
  const [editMallIDs, setEditMallIDs] = useState<number[]>([])
  const [roleCreatorTarget, setRoleCreatorTarget] = useState<'create' | 'edit' | null>(null)
  const [roleCreatorError, setRoleCreatorError] = useState('')
  const [selectedID, setSelectedID] = useState(0)
  const [keyword, setKeyword] = useState('')
  const [creating, setCreating] = useState(false)
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')
  const selectedIDRef = useRef(0)
  const selected = accounts.find((item) => item.id === selectedID) ?? accounts[0]
  const activeRoles = useMemo(() => roles.filter((role) => role.status === 'ACTIVE'), [roles])
  const grantableCatalog = useMemo(() => {
    const grantable = new Set(grantablePermissionCodes)
    return catalog.filter((permission) => grantable.has(permission.code))
  }, [catalog, grantablePermissionCodes])

  const load = useCallback(async () => {
    setBusy(true); setMessage('')
    if (canReadRoles) setRoleCatalogStatus((current) => current === 'ready' ? current : 'loading')
    const unavailable: ApiResult = { ok: false, status: 403, data: null }
    const [accountResponse, roleResponse, permissionResponse] = await Promise.all([
      client('/v1/access/accounts/query', { method: 'POST', body: { keyword, pageSize: 100 }, showResult: false, silentLoading: true }),
      canReadRoles ? client('/v1/access/roles', { method: 'GET', showResult: false, silentLoading: true }) : Promise.resolve(unavailable),
      canManageRoles ? client('/v1/access/permissions', { method: 'GET', showResult: false, silentLoading: true }) : Promise.resolve(unavailable),
    ])
    const nextAccounts = accountResponse.ok ? parseAccessAccounts(accountResponse.data) : null
    const nextRoles = roleResponse.ok ? list<Role>(roleResponse.data) : null
    if (nextAccounts) {
      const nextSelectedID = nextAccounts.some((item) => item.id === selectedIDRef.current) ? selectedIDRef.current : nextAccounts[0]?.id ?? 0
      const nextSelected = nextAccounts.find((item) => item.id === nextSelectedID)
      selectedIDRef.current = nextSelectedID
      setAccounts(nextAccounts)
      setSelectedID(nextSelectedID)
      if (nextSelected) {
        setEditMallScopeMode(nextSelected.mallScopeMode === 'SELECTED' ? 'SELECTED' : 'ALL')
        setEditMallIDs(nextSelected.mallIds)
      }
      if (nextRoles) setEditRoleIDs(accountRoleIDs(nextSelected, nextRoles.filter((role) => role.status === 'ACTIVE')))
    } else setMessage('控制台账号加载失败。')
    if (nextRoles) { setRoles(nextRoles); setRoleCatalogStatus('ready') }
    else if (canReadRoles) { setRoleCatalogStatus('error'); setMessage('角色目录加载失败，请刷新后重试。') }
    if (permissionResponse.ok) setCatalog(list<Permission>(permissionResponse.data))
    else if (canManageRoles) setMessage('权限目录加载失败，暂时无法创建自定义角色。')
    setBusy(false)
  }, [canManageRoles, canReadRoles, client, keyword])
  useEffect(() => { void load() }, [load])

  const loadMalls = useCallback(async () => {
    if (!canManage) return
    setMallCatalogStatus('loading')
    let afterID = 0
    let loaded: AccessMall[] = []
    for (;;) {
      const response = await client(accessMallCatalogPath(afterID), { method: 'GET', showResult: false, silentLoading: true })
      const page = response.ok ? parseAccessMallCatalog(response.data) : null
      if (!page) {
        setMalls(loaded)
        setMallCatalogStatus('error')
        return
      }
      loaded = mergeAccessMalls(loaded, page.items)
      if (page.items.length < 200 || page.nextAfterId === 0) break
      if (page.nextAfterId <= afterID) {
        setMalls(loaded)
        setMallCatalogStatus('error')
        return
      }
      afterID = page.nextAfterId
    }
    setMalls(loaded)
    setMallCatalogStatus('ready')
  }, [canManage, client])
  useEffect(() => { void loadMalls() }, [loadMalls])

  async function mutate(path: string, body: unknown) {
    setBusy(true); setMessage('')
    const response = await client(path, { method: 'PUT', headers: headers(), body, showResult: false, silentLoading: true })
    setMessage(response.ok ? '账号设置已保存。' : '账号设置保存失败，请检查权限和输入。')
    if (response.ok) await load()
    setBusy(false)
  }

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!canReplaceAccountRoles(roleCatalogStatus)) {
      setMessage('角色目录尚未加载完成，暂时无法创建账号。')
      return
    }
    const form = new FormData(event.currentTarget)
    const mallScope = accessMallScopeRequest(createMallScopeMode, createMallIDs)
    if (!mallScope) {
      setMessage('选择指定商场时，请至少选择一个商场。')
      return
    }
    setBusy(true); setMessage('')
    const response = await client('/v1/access/accounts', { method: 'POST', headers: headers(), body: {
      account: value(form, 'account'), phone: value(form, 'phone'), nickname: value(form, 'nickname'), password: value(form, 'password'),
      roleIds: createRoleIDs, ...mallScope, reason: value(form, 'reason'),
    }, showResult: false, silentLoading: true })
    setMessage(response.ok ? '控制台账号已创建。' : '账号创建失败，请检查手机号、角色和商场范围。')
    if (response.ok) { setCreating(false); setCreateRoleIDs([]); await load() }
    setBusy(false)
  }

  function updateMallScope(event: FormEvent<HTMLFormElement>, targetID: number) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const mallScope = accessMallScopeRequest(editMallScopeMode, editMallIDs)
    if (!mallScope) {
      setMessage('选择指定商场时，请至少选择一个商场。')
      return
    }
    void mutate(`/v1/access/accounts/${targetID}/mall-scope`, { ...mallScope, reason: value(form, 'reason') })
  }

  async function createRole(input: AccessRoleCreateInput) {
    setBusy(true); setMessage(''); setRoleCreatorError('')
    const response = await client('/v1/access/roles', { method: 'POST', headers: headers(), body: input, showResult: false, silentLoading: true })
    const created = response.ok ? parseCreatedAccessRole(response.data) : null
    if (!created) {
      setRoleCreatorError('自定义角色创建失败，请检查角色代码、权限范围和当前权限。')
      setBusy(false)
      return
    }
    setRoles((current) => [...current.filter((role) => role.id !== created.id), created])
    if (roleCreatorTarget === 'create') setCreateRoleIDs((current) => updateRoleSelection(current, created.id, true))
    if (roleCreatorTarget === 'edit') setEditRoleIDs((current) => updateRoleSelection(current, created.id, true))
    setRoleCreatorTarget(null)
    setRoleCreatorError('')
    setMessage(`自定义角色“${created.name}”已创建并选中。`)
    setBusy(false)
  }

  function toggleRole(roleID: number, checked: boolean, target: 'create' | 'edit') {
    if (target === 'create') setCreateRoleIDs((current) => updateRoleSelection(current, roleID, checked))
    else setEditRoleIDs((current) => updateRoleSelection(current, roleID, checked))
  }

  function selectAccount(account: Account) {
    selectedIDRef.current = account.id
    setSelectedID(account.id)
    setEditRoleIDs(accountRoleIDs(account, activeRoles))
    setEditMallScopeMode(account.mallScopeMode === 'SELECTED' ? 'SELECTED' : 'ALL')
    setEditMallIDs(account.mallIds)
  }

  return <Section title="控制台账号" description="查询账号并在右侧维护基础信息、角色、商场范围和登录凭证。" flush>
    <MasterDetail className={styles.masterDetail} masterWidth={312} masterLabel="控制台账号列表" detailLabel="控制台账号详情"
      master={<div className={styles.masterPane}>
        <form className={styles.search} onSubmit={(event) => { event.preventDefault(); void load() }}><label><span>搜索账号</span><input name="accountKeyword" autoComplete="off" value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder="账号、昵称或手机号" /></label><button type="submit" disabled={busy}>查询</button></form>
        {canManage && <button className={styles.fullPrimary} type="button" disabled={busy || !canReplaceAccountRoles(roleCatalogStatus)} onClick={() => { setMessage(''); setCreateRoleIDs([]); setCreateMallScopeMode('ALL'); setCreateMallIDs([]); setCreating(true) }}><Plus aria-hidden="true" />创建子账号</button>}
        {canManage && roleCatalogStatus !== 'ready' && <p className={styles.message} role="status">{roleCatalogStatus === 'error' ? '角色目录加载失败，账号角色暂不可编辑。' : '正在加载角色目录…'}</p>}
        <div className={styles.list}>{accounts.map((account) => <button type="button" className={selected?.id === account.id ? styles.selectedItem : undefined} key={account.id} onClick={() => selectAccount(account)}><span><strong>{account.nickname}</strong><small>{account.account} · {account.phone || '未绑定'}</small></span><StatusTag tone={account.status === 'ACTIVE' ? 'success' : 'neutral'}>{account.status === 'ACTIVE' ? '启用' : '停用'}</StatusTag></button>)}</div>
      </div>}
      detail={<div className={styles.detailPane}>
        {message && <p className={styles.message} role="status">{message}</p>}
        {!selected ? <FeedbackState kind={busy ? 'loading' : 'empty'} title={busy ? '正在加载控制台账号' : '暂无控制台账号'} /> : <>
          <header className={styles.detailHeader}><div><span>控制台账号</span><h3>{selected.nickname}</h3><p>{selected.account} · {selected.phone || '未绑定手机号'}</p></div>{canManage && <button type="button" disabled={busy} onClick={() => void mutate(`/v1/access/accounts/${selected.id}/status`, { status: selected.status === 'ACTIVE' ? 'DISABLED' : 'ACTIVE', reason: selected.status === 'ACTIVE' ? '管理员停用账号' : '管理员启用账号' })}>{selected.status === 'ACTIVE' ? '停用账号' : '启用账号'}</button>}</header>
          <dl className={styles.summary}><div><dt>状态</dt><dd>{selected.status}</dd></div><div><dt>角色</dt><dd>{selected.roles.map((role) => role.name).join('、') || '未分配'}</dd></div><div><dt>商场范围</dt><dd>{selected.mallScopeMode === 'ALL' ? '全部商场' : selected.mallIds.join(', ') || '未指定'}</dd></div></dl>
          {canManage && <div className={styles.forms}>
            <form onSubmit={(event) => { event.preventDefault(); const form = new FormData(event.currentTarget); void mutate(`/v1/access/accounts/${selected.id}`, { phone: value(form, 'phone'), nickname: value(form, 'nickname'), reason: value(form, 'reason') }) }}><h4>基础信息</h4><Field label="手机号"><input name="phone" required pattern="1[3-9][0-9]{9}" placeholder="输入完整手机号" /></Field><Field label="昵称"><input name="nickname" defaultValue={selected.nickname} required /></Field><Field label="变更原因"><input name="reason" required /></Field><button type="submit" disabled={busy}>保存信息</button></form>
            {canReplaceAccountRoles(roleCatalogStatus)
              ? <form onSubmit={(event) => { event.preventDefault(); const form = new FormData(event.currentTarget); void mutate(`/v1/access/accounts/${selected.id}/roles`, { roleIds: editRoleIDs, reason: value(form, 'reason') }) }}><div className={styles.roleSelectionHeader}><h4>角色全量替换</h4>{canManageRoles && grantableCatalog.length > 0 && <button type="button" disabled={busy} onClick={() => { setRoleCreatorError(''); setRoleCreatorTarget('edit') }}><Plus aria-hidden="true" />自定义角色与权限</button>}</div>{activeRoles.map((role) => <label className={styles.check} key={role.id}><input type="checkbox" name="roleIds" value={role.id} checked={editRoleIDs.includes(role.id)} onChange={(event) => toggleRole(role.id, event.currentTarget.checked, 'edit')} />{role.name}<small>{role.permissions.length} 项权限</small></label>)}<Field label="变更原因"><input name="reason" required /></Field><button type="submit" disabled={busy}>保存角色</button></form>
              : <div className={styles.roleUnavailable} role="status"><h4>角色全量替换</h4><p>{roleCatalogStatus === 'error' ? '角色目录加载失败，为避免清空现有角色，暂时禁止保存。' : '角色目录加载完成后即可编辑。'}</p></div>}
            <form onSubmit={(event) => updateMallScope(event, selected.id)}><h4>商场范围</h4><MallScopeFields mode={editMallScopeMode} mallIDs={editMallIDs} malls={malls} catalogStatus={mallCatalogStatus} disabled={busy} onModeChange={setEditMallScopeMode} onMallIDsChange={setEditMallIDs} onRetry={loadMalls} /><Field label="变更原因"><input name="reason" required /></Field><button type="submit" disabled={busy || editMallScopeMode === 'SELECTED' && mallCatalogStatus !== 'ready'}>保存范围</button></form>
            <form onSubmit={(event) => { event.preventDefault(); const form = new FormData(event.currentTarget); void mutate(`/v1/access/accounts/${selected.id}/password`, { password: value(form, 'password'), reason: value(form, 'reason') }) }}><h4>重置密码</h4><Field label="新密码"><input type="password" name="password" minLength={10} maxLength={72} required autoComplete="new-password" placeholder="10–72 位" /></Field><Field label="重置原因"><input name="reason" required /></Field><button type="submit" disabled={busy}><KeyRound aria-hidden="true" />重置密码</button></form>
          </div>}
        </>}
      </div>}
    />
    <Drawer open={creating} title="创建控制台账号" description="创建可登录内部控制台的子账号。" size="medium" closeDisabled={busy} onClose={() => setCreating(false)}><form className={styles.drawerForm} onSubmit={create}>
      {message && <p className={styles.message} role="status">{message}</p>}
      <Field label="账号"><input name="account" required minLength={3} maxLength={40} pattern="[A-Za-z0-9][A-Za-z0-9_-]{2,39}" autoComplete="username" /></Field><Field label="手机号"><input name="phone" required pattern="1[3-9][0-9]{9}" autoComplete="tel" /></Field><Field label="昵称"><input name="nickname" required autoComplete="off" /></Field><Field label="初始密码"><input name="password" required type="password" minLength={10} maxLength={72} autoComplete="new-password" /></Field>
      <div className={styles.roleSelectionHeader}><strong>角色</strong>{canManageRoles && roleCatalogStatus === 'ready' && grantableCatalog.length > 0 && <button type="button" disabled={busy} onClick={() => { setRoleCreatorError(''); setRoleCreatorTarget('create') }}><Plus aria-hidden="true" />自定义角色与权限</button>}</div>
      <fieldset><legend className={styles.srOnly}>选择角色</legend>{activeRoles.map((role) => <label className={styles.check} key={role.id}><input type="checkbox" name="roleIds" value={role.id} checked={createRoleIDs.includes(role.id)} onChange={(event) => toggleRole(role.id, event.currentTarget.checked, 'create')} />{role.name}<small>{role.permissions.length} 项权限</small></label>)}</fieldset>
      <MallScopeFields mode={createMallScopeMode} mallIDs={createMallIDs} malls={malls} catalogStatus={mallCatalogStatus} disabled={busy} onModeChange={setCreateMallScopeMode} onMallIDsChange={setCreateMallIDs} onRetry={loadMalls} /><Field label="创建原因"><textarea name="reason" required rows={4} /></Field><button className={styles.primary} type="submit" disabled={busy || !canReplaceAccountRoles(roleCatalogStatus) || createMallScopeMode === 'SELECTED' && mallCatalogStatus !== 'ready'}>创建账号</button>
    </form></Drawer>
    <AccessRoleCreateDrawer open={roleCreatorTarget !== null} busy={busy} catalog={grantableCatalog} error={roleCreatorError} onClose={() => { setRoleCreatorError(''); setRoleCreatorTarget(null) }} onSubmit={createRole} />
  </Section>
}

function MallScopeFields({ mode, mallIDs, malls, catalogStatus, disabled, onModeChange, onMallIDsChange, onRetry }: {
  mode: 'ALL' | 'SELECTED'
  mallIDs: number[]
  malls: AccessMall[]
  catalogStatus: MallCatalogStatus
  disabled: boolean
  onModeChange: (mode: 'ALL' | 'SELECTED') => void
  onMallIDsChange: (ids: number[]) => void
  onRetry: () => Promise<void>
}) {
  const selected = mode === 'SELECTED'
  return <>
    <Field label="商场范围"><select value={mode} disabled={disabled} onChange={(event) => { const next = event.currentTarget.value === 'SELECTED' ? 'SELECTED' : 'ALL'; onModeChange(next); if (next === 'ALL') onMallIDsChange([]) }}><option value="ALL">全部商场</option><option value="SELECTED">指定商场</option></select></Field>
    {selected && <Field label="选择商场"><Select className={styles.mallSelect} mode="multiple" showSearch optionFilterProp="label" maxTagCount="responsive" aria-label="搜索并选择商场" aria-required="true" disabled={disabled || catalogStatus !== 'ready'} loading={catalogStatus === 'loading'} value={mallIDs} options={malls.map((mall) => ({ value: mall.id, label: `${mall.nameCn} · ${mall.mallCode}` }))} placeholder="搜索商场名称或编码" notFoundContent={catalogStatus === 'loading' ? '正在加载商场…' : catalogStatus === 'error' ? '商场目录加载失败' : '没有匹配的商场'} onChange={onMallIDsChange} />{catalogStatus === 'ready' && <small className={styles.fieldHint}>已加载 {malls.length} 个可授权商场，当前选择 {mallIDs.length} 个。</small>}{catalogStatus === 'error' && <span className={styles.inlineError} role="status">商场目录加载失败。<button type="button" disabled={disabled} onClick={() => void onRetry()}>重新加载</button></span>}</Field>}
  </>
}

function RolePanel({ client, canManage, grantablePermissionCodes }: { client: ApiClient; canManage: boolean; grantablePermissionCodes: string[] }) {
  const [roles, setRoles] = useState<Role[]>([])
  const [catalog, setCatalog] = useState<Permission[]>([])
  const [selectedID, setSelectedID] = useState(0)
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState('')
  const [deletePending, setDeletePending] = useState(false)
  const selected = roles.find((role) => role.id === selectedID) ?? roles[0]
  const load = useCallback(async () => { setBusy(true); const [rolesResponse, permissionResponse] = await Promise.all([client('/v1/access/roles', { method: 'GET', showResult: false, silentLoading: true }), client('/v1/access/permissions', { method: 'GET', showResult: false, silentLoading: true })]); if (rolesResponse.ok) { const next = list<Role>(rolesResponse.data); setRoles(next); setSelectedID((current) => next.some((role) => role.id === current) ? current : next[0]?.id ?? 0) }; if (permissionResponse.ok) setCatalog(list<Permission>(permissionResponse.data)); setBusy(false) }, [client])
  useEffect(() => { void load() }, [load])
  const groups = useMemo(() => { const grouped: Record<string, Permission[]> = {}; for (const permission of catalog) (grouped[permission.module || '其他'] ??= []).push(permission); return Object.entries(grouped) }, [catalog])
  const grantableCatalog = useMemo(() => { const grantable = new Set(grantablePermissionCodes); return catalog.filter((permission) => grantable.has(permission.code)) }, [catalog, grantablePermissionCodes])
  async function savePermissions(event: FormEvent<HTMLFormElement>) { event.preventDefault(); if (!selected) return; const form = new FormData(event.currentTarget); setBusy(true); const response = await client(`/v1/access/roles/${selected.id}/permissions`, { method: 'PUT', headers: headers(), body: { permissions: form.getAll('permissions'), reason: value(form, 'reason') }, showResult: false, silentLoading: true }); setMessage(response.ok ? '角色权限已保存，相关账号旧会话已失效。' : '角色权限保存失败。'); if (response.ok) await load(); setBusy(false) }
  async function roleMutation(path: string, method: 'POST' | 'PUT' | 'DELETE', body: unknown, success: string) { setBusy(true); const response = await client(path, { method, headers: headers(), body, showResult: false, silentLoading: true }); setMessage(response.ok ? success : '角色操作失败，请检查系统角色保护和当前权限。'); if (response.ok) await load(); setBusy(false); return response.ok }
  async function createRole(input: AccessRoleCreateInput) { setCreateError(''); const success = await roleMutation('/v1/access/roles', 'POST', input, '自定义角色已创建。'); if (success) setCreating(false); else setCreateError('角色创建失败，请检查角色代码、权限范围和当前权限。') }

  return <Section title="角色管理" description="维护自定义角色状态和全量权限矩阵；系统角色保持只读。" flush>
    <MasterDetail className={styles.masterDetail} masterWidth={312} masterLabel="角色目录" detailLabel="角色权限详情" master={<div className={styles.masterPane}><div className={styles.masterTitle}><ShieldCheck aria-hidden="true" /><strong>角色目录</strong></div>{canManage && <button className={styles.fullPrimary} type="button" onClick={() => { setCreateError(''); setCreating(true) }}><Plus aria-hidden="true" />创建角色</button>}<div className={styles.list}>{roles.map((role) => <button type="button" className={selected?.id === role.id ? styles.selectedItem : undefined} key={role.id} onClick={() => setSelectedID(role.id)}><span><strong>{role.name}</strong><small>{role.code}</small></span><StatusTag tone={role.isSystem ? 'neutral' : 'success'}>{role.isSystem ? '系统' : '自定义'}</StatusTag></button>)}</div></div>}
      detail={<div className={styles.detailPane}>{message && <p className={styles.message} role="status">{message}</p>}{!selected ? <FeedbackState kind={busy ? 'loading' : 'empty'} title={busy ? '正在加载角色' : '暂无角色'} /> : <><header className={styles.detailHeader}><div><span>角色</span><h3>{selected.name}</h3><p>{selected.description || selected.code}</p></div>{canManage && !selected.isSystem ? <div className={styles.actions}><button type="button" disabled={busy} onClick={() => void roleMutation(`/v1/access/roles/${selected.id}/status`, 'PUT', { status: selected.status === 'ACTIVE' ? 'DISABLED' : 'ACTIVE', reason: '管理员调整角色状态' }, '角色状态已更新。')}>{selected.status === 'ACTIVE' ? '停用' : '启用'}</button><button className={styles.danger} type="button" disabled={busy} onClick={() => setDeletePending(true)}>删除</button></div> : <StatusTag tone="neutral">{selected.status}</StatusTag>}</header><form key={selected.id} className={styles.permissionMatrix} onSubmit={savePermissions}>{groups.map(([module, items]) => <fieldset key={module}><legend>{module}</legend>{items.map((permission) => <label key={permission.code}><input type="checkbox" name="permissions" value={permission.code} defaultChecked={selected.permissions.includes(permission.code)} disabled={!canManage || selected.isSystem} /><span><strong>{permission.name}</strong><small>{permission.code} · {permission.description}</small></span><em>{permission.riskLevel}</em></label>)}</fieldset>)}{canManage && !selected.isSystem && <div className={styles.saveRow}><Field label="权限变更原因"><input name="reason" required /></Field><button className={styles.primary} type="submit" disabled={busy}>保存权限矩阵</button></div>}</form></>}</div>} />
    <AccessRoleCreateDrawer open={creating} busy={busy} catalog={grantableCatalog} error={createError} onClose={() => { setCreateError(''); setCreating(false) }} onSubmit={createRole} />
    <Dialog open={deletePending && Boolean(selected)} role="alertdialog" title="确认删除角色" description={selected ? `确认删除未使用角色“${selected.name}”？删除后无法恢复。` : undefined} closeDisabled={busy} onClose={() => setDeletePending(false)} footer={<><button type="button" disabled={busy} onClick={() => setDeletePending(false)}>取消</button><button className={styles.danger} type="button" disabled={busy} onClick={() => { if (selected) void roleMutation(`/v1/access/roles/${selected.id}`, 'DELETE', { reason: '管理员删除未使用角色' }, '角色已删除。').then((success) => { if (success) setDeletePending(false) }) }}>确认删除</button></>}><p className={styles.dangerNotice}>仅未被账号使用的自定义角色可以删除，系统角色始终受保护。</p></Dialog>
  </Section>
}

function AuditPanel({ client }: { client: ApiClient }) {
  const [audits, setAudits] = useState<Audit[]>([])
  const [action, setAction] = useState('')
  const [targetType, setTargetType] = useState('')
  const [busy, setBusy] = useState(false)
  const load = useCallback(async () => { setBusy(true); const query = new URLSearchParams({ pageSize: '100' }); if (action) query.set('action', action); if (targetType) query.set('targetType', targetType); const response = await client(`/v1/access/audits?${query}`, { method: 'GET', showResult: false, silentLoading: true }); if (response.ok) setAudits(list<Audit>(response.data, 'audits')); setBusy(false) }, [action, client, targetType])
  useEffect(() => { void load() }, [load])
  return <Section title="权限审计" description="按动作与对象查询最近 100 条账号和角色变更。" flush><FilterToolbar summary={`${audits.length} 条记录`}><form className={styles.auditFilter} onSubmit={(event) => { event.preventDefault(); void load() }}><label>审计动作<input value={action} onChange={(event) => setAction(event.target.value)} placeholder="例如 ACCOUNT_STATUS" /></label><label>对象类型<select value={targetType} onChange={(event) => setTargetType(event.target.value)}><option value="">全部对象</option><option value="ACCOUNT">账号</option><option value="role">角色</option></select></label><button type="submit" disabled={busy}><RefreshCcw aria-hidden="true" />{busy ? '查询中' : '查询'}</button></form></FilterToolbar>{audits.length === 0 ? <FeedbackState kind={busy ? 'loading' : 'empty'} title={busy ? '正在加载权限审计' : '暂无符合条件的权限审计'} /> : <DataTable scrollLabel="权限审计记录"><thead><tr><th scope="col">时间</th><th scope="col">操作者</th><th scope="col">动作</th><th scope="col">对象</th><th scope="col">原因</th></tr></thead><tbody>{audits.map((audit) => <tr key={audit.id}><td>{new Date(audit.createdAt).toLocaleString('zh-CN')}</td><td>#{audit.actorUserId}</td><td>{audit.action}</td><td>{audit.targetType} #{audit.targetId}</td><td>{audit.reason}</td></tr>)}</tbody></DataTable>}</Section>
}

function Field({ label, children }: { label: string; children: React.ReactNode }) { return <label className={styles.field}><span>{label}</span>{children}</label> }
function value(form: FormData, key: string) { const result = form.get(key); return typeof result === 'string' ? result.trim() : '' }
