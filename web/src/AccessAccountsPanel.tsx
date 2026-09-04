import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import Select from 'antd/es/select'
import { Building2, FileSpreadsheet, KeyRound, Pencil, Plus, RefreshCcw, Search, ShieldCheck, UserRound, X } from 'lucide-react'
import { AccessAccountReportCategories } from './AccessAccountReportCategories'
import { AccessRoleCreateDrawer, type AccessRoleCreateInput } from './AccessRoleCreateDrawer'
import {
  accountRoleIDs,
  accessMallCatalogPath,
  accessMallScopeRequest,
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
import { DataTable, Dialog, Drawer, FeedbackState, FilterToolbar, Section, StatusTag } from './ui'
import styles from './AccessManagementPage.module.css'

type ApiResult = { ok: boolean; status: number; data: unknown }
type ApiClient = (path: string, options?: { method?: 'GET' | 'POST' | 'PUT'; body?: unknown; headers?: Record<string, string>; showResult?: boolean; silentLoading?: boolean; signal?: AbortSignal }) => Promise<ApiResult>
type MallCatalogStatus = 'idle' | 'loading' | 'ready' | 'error'
type AccountSection = 'overview' | 'roles' | 'malls' | 'reports' | 'security'

const headers = () => ({ 'Idempotency-Key': `access:${globalThis.crypto.randomUUID()}` })
const unavailable: ApiResult = { ok: false, status: 403, data: null }

function envelopeList<T>(payload: unknown): T[] {
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) return []
  const data = (payload as Record<string, unknown>).data
  return Array.isArray(data) ? data as T[] : []
}

export function AccessAccountsPanel({ client, canManage, canManageRoles, canReadRoles, canManageReportCategories, grantablePermissionCodes }: {
  client: ApiClient
  canManage: boolean
  canManageRoles: boolean
  canReadRoles: boolean
  canManageReportCategories: boolean
  grantablePermissionCodes: string[]
}) {
  const [accounts, setAccounts] = useState<Account[]>([])
  const [roles, setRoles] = useState<Role[]>([])
  const [catalog, setCatalog] = useState<Permission[]>([])
  const [malls, setMalls] = useState<AccessMall[]>([])
  const [mallCatalogStatus, setMallCatalogStatus] = useState<MallCatalogStatus>('idle')
  const [roleCatalogStatus, setRoleCatalogStatus] = useState<RoleCatalogStatus>('idle')
  const [draftKeyword, setDraftKeyword] = useState('')
  const [draftStatus, setDraftStatus] = useState('')
  const [filters, setFilters] = useState({ keyword: '', status: '' })
  const [reloadVersion, setReloadVersion] = useState(0)
  const [loading, setLoading] = useState(true)
  const [mutating, setMutating] = useState(false)
  const [message, setMessage] = useState('')
  const [selectedID, setSelectedID] = useState(0)
  const [accountSection, setAccountSection] = useState<AccountSection>('overview')
  const [profilePhone, setProfilePhone] = useState('')
  const [profileNickname, setProfileNickname] = useState('')
  const [editRoleIDs, setEditRoleIDs] = useState<number[]>([])
  const [editMallScopeMode, setEditMallScopeMode] = useState<'ALL' | 'SELECTED'>('ALL')
  const [editMallIDs, setEditMallIDs] = useState<number[]>([])
  const [creating, setCreating] = useState(false)
  const [createRoleIDs, setCreateRoleIDs] = useState<number[]>([])
  const [createMallScopeMode, setCreateMallScopeMode] = useState<'ALL' | 'SELECTED'>('ALL')
  const [createMallIDs, setCreateMallIDs] = useState<number[]>([])
  const [roleCreatorTarget, setRoleCreatorTarget] = useState<'create' | 'edit' | null>(null)
  const [roleCreatorError, setRoleCreatorError] = useState('')
  const [pendingStatus, setPendingStatus] = useState<Account | null>(null)
  const selectedIDRef = useRef(0)
  const accountRequestRef = useRef(0)
  const selected = accounts.find((item) => item.id === selectedID) ?? null
  const activeRoles = useMemo(() => roles.filter((role) => role.status === 'ACTIVE'), [roles])
  const grantableCatalog = useMemo(() => {
    const grantable = new Set(grantablePermissionCodes)
    return catalog.filter((permission) => grantable.has(permission.code))
  }, [catalog, grantablePermissionCodes])

  const hydrateEditor = useCallback((account: Account) => {
    setProfilePhone('')
    setProfileNickname(account.nickname)
    setEditRoleIDs(accountRoleIDs(account, activeRoles))
    setEditMallScopeMode(account.mallScopeMode === 'SELECTED' ? 'SELECTED' : 'ALL')
    setEditMallIDs(account.mallIds)
  }, [activeRoles])

  const loadAccounts = useCallback(async (signal?: AbortSignal) => {
    const requestID = ++accountRequestRef.current
    setLoading(true)
    const response = await client('/v1/access/accounts/query', {
      method: 'POST', body: { keyword: filters.keyword, status: filters.status, pageSize: 100 },
      showResult: false, silentLoading: true, signal,
    })
    if (signal?.aborted || requestID !== accountRequestRef.current) return
    if (!response.ok) {
      setLoading(false)
      setMessage('账号目录加载失败，请刷新后重试。')
      return
    }
    const nextAccounts = parseAccessAccounts(response.data)
    setAccounts(nextAccounts)
    const current = nextAccounts.find((account) => account.id === selectedIDRef.current)
    if (current) hydrateEditor(current)
    else {
      selectedIDRef.current = 0
      setSelectedID(0)
    }
    setLoading(false)
  }, [client, filters.keyword, filters.status, hydrateEditor])

  useEffect(() => {
    const controller = new AbortController()
    void loadAccounts(controller.signal)
    return () => controller.abort()
  }, [loadAccounts, reloadVersion])

  useEffect(() => {
    const controller = new AbortController()
    async function loadRoleCatalog() {
      if (!canReadRoles) return
      setRoleCatalogStatus('loading')
      const [roleResponse, permissionResponse] = await Promise.all([
        client('/v1/access/roles', { method: 'GET', showResult: false, silentLoading: true, signal: controller.signal }),
        canManageRoles ? client('/v1/access/permissions', { method: 'GET', showResult: false, silentLoading: true, signal: controller.signal }) : Promise.resolve(unavailable),
      ])
      if (controller.signal.aborted) return
      if (!roleResponse.ok) {
        setRoleCatalogStatus('error')
        setMessage('角色目录加载失败，账号角色暂不可编辑。')
        return
      }
      setRoles(envelopeList<Role>(roleResponse.data))
      setRoleCatalogStatus('ready')
      if (permissionResponse.ok) setCatalog(envelopeList<Permission>(permissionResponse.data))
    }
    void loadRoleCatalog()
    return () => controller.abort()
  }, [canManageRoles, canReadRoles, client])

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

  function openAccount(account: Account) {
    if (mutating) return
    selectedIDRef.current = account.id
    setSelectedID(account.id)
    setAccountSection('overview')
    setMessage('')
    hydrateEditor(account)
  }

  function closeAccount() {
    if (mutating) return
    selectedIDRef.current = 0
    setSelectedID(0)
    setMessage('')
  }

  async function mutate(path: string, body: unknown, success: string) {
    if (!selected || mutating) return false
    const targetID = selected.id
    setMutating(true)
    setMessage('')
    const response = await client(path, { method: 'PUT', headers: headers(), body, showResult: false, silentLoading: true })
    if (!response.ok) {
      setMessage('账号设置保存失败，请检查权限和输入。')
      setMutating(false)
      return false
    }
    await loadAccounts()
    if (selectedIDRef.current === targetID) setMessage(success)
    setMutating(false)
    return true
  }

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!canReplaceAccountRoles(roleCatalogStatus) || mutating) {
      setMessage('角色目录尚未加载完成，暂时无法创建账号。')
      return
    }
    const form = new FormData(event.currentTarget)
    const mallScope = accessMallScopeRequest(createMallScopeMode, createMallIDs)
    if (!mallScope) {
      setMessage('选择指定商场时，请至少选择一个商场。')
      return
    }
    setMutating(true)
    setMessage('')
    const response = await client('/v1/access/accounts', { method: 'POST', headers: headers(), body: {
      account: value(form, 'account'), phone: value(form, 'phone'), nickname: value(form, 'nickname'), password: value(form, 'password'),
      roleIds: createRoleIDs, ...mallScope, reason: value(form, 'reason'),
    }, showResult: false, silentLoading: true })
    if (!response.ok) {
      setMessage('账号创建失败，请检查手机号、角色和商场范围。')
      setMutating(false)
      return
    }
    setCreating(false)
    setCreateRoleIDs([])
    await loadAccounts()
    setMessage('控制台账号已创建。')
    setMutating(false)
  }

  async function createRole(input: AccessRoleCreateInput) {
    setMutating(true)
    setRoleCreatorError('')
    const response = await client('/v1/access/roles', { method: 'POST', headers: headers(), body: input, showResult: false, silentLoading: true })
    const created = response.ok ? parseCreatedAccessRole(response.data) : null
    if (!created) {
      setRoleCreatorError('自定义角色创建失败，请检查角色代码、权限范围和当前权限。')
      setMutating(false)
      return
    }
    setRoles((current) => [...current.filter((role) => role.id !== created.id), created])
    if (roleCreatorTarget === 'create') setCreateRoleIDs((current) => updateRoleSelection(current, created.id, true))
    if (roleCreatorTarget === 'edit') setEditRoleIDs((current) => updateRoleSelection(current, created.id, true))
    setRoleCreatorTarget(null)
    setMutating(false)
  }

  const dialogTabs = [
    { key: 'overview' as const, label: '基本资料', icon: UserRound, visible: true },
    { key: 'roles' as const, label: '角色权限', icon: ShieldCheck, visible: canManage },
    { key: 'malls' as const, label: '商场范围', icon: Building2, visible: canManage },
    { key: 'reports' as const, label: '报表权限', icon: FileSpreadsheet, visible: canManageReportCategories },
    { key: 'security' as const, label: '登录安全', icon: KeyRound, visible: canManage },
  ].filter((item) => item.visible)

  return <Section title="账号目录" description="通过搜索定位账号；账号资料和权限在独立窗口中编辑。" flush>
    <FilterToolbar summary={`共加载 ${accounts.length} 个账号`}>
      <form className={styles.accountFilters} onSubmit={(event) => { event.preventDefault(); setFilters({ keyword: draftKeyword.trim(), status: draftStatus }) }}>
        <label className={styles.keywordField}><span className={styles.srOnly}>搜索账号</span><Search aria-hidden="true" /><input type="search" value={draftKeyword} onChange={(event) => setDraftKeyword(event.currentTarget.value)} placeholder="搜索账号、昵称或手机号" /></label>
        <label><span className={styles.srOnly}>账号状态</span><select value={draftStatus} onChange={(event) => setDraftStatus(event.currentTarget.value)}><option value="">全部状态</option><option value="ACTIVE">已启用</option><option value="DISABLED">已停用</option></select></label>
        <button type="submit" disabled={loading}>查询</button>
        {(draftKeyword || draftStatus) && <button type="button" aria-label="清空筛选" onClick={() => { setDraftKeyword(''); setDraftStatus(''); setFilters({ keyword: '', status: '' }) }}><X aria-hidden="true" /></button>}
      </form>
      <div className={styles.accountToolbar}>
        <button type="button" onClick={() => setReloadVersion((value) => value + 1)} disabled={loading}><RefreshCcw aria-hidden="true" />刷新</button>
        {canManage && <button className={styles.primary} type="button" disabled={mutating || !canReplaceAccountRoles(roleCatalogStatus)} onClick={() => { setMessage(''); setCreateRoleIDs([]); setCreateMallScopeMode('ALL'); setCreateMallIDs([]); setCreating(true) }}><Plus aria-hidden="true" />创建账号</button>}
      </div>
    </FilterToolbar>
    {message && !selected && <p className={styles.message} role="status">{message}</p>}
    {loading && accounts.length === 0 ? <FeedbackState kind="loading" title="正在加载账号目录" /> : null}
    {!loading && accounts.length === 0 ? <FeedbackState kind="empty" title="暂无匹配账号" description="调整搜索条件后重试。" /> : null}
    {accounts.length > 0 ? <DataTable containerClassName={styles.accountTable} minWidth={900} scrollLabel="账号目录表格"><thead><tr><th scope="col">账号</th><th scope="col">联系方式</th><th scope="col">角色</th><th scope="col">商场范围</th><th scope="col">状态</th><th scope="col">操作</th></tr></thead><tbody>{accounts.map((account) => <tr key={account.id}><td><span className={styles.accountIdentity}><UserRound aria-hidden="true" /><span><strong>{account.nickname}</strong><code>{account.account}</code></span></span></td><td>{account.phone || '未绑定'}</td><td>{account.roles.map((role) => role.name).join('、') || '未分配'}</td><td>{account.mallScopeMode === 'ALL' ? '全部商场' : `${account.mallIds.length} 个商场`}</td><td><StatusTag tone={account.status === 'ACTIVE' ? 'success' : 'neutral'}>{account.status === 'ACTIVE' ? '启用' : '停用'}</StatusTag></td><td><button type="button" disabled={mutating} onClick={() => openAccount(account)}><Pencil aria-hidden="true" />管理</button></td></tr>)}</tbody></DataTable> : null}

    <Drawer open={Boolean(selected)} size="wide" title={selected?.nickname ?? '账号管理'} description={selected ? `${selected.account} · ${selected.phone || '未绑定手机号'}` : undefined} closeLabel="关闭账号管理" closeDisabled={mutating} onClose={closeAccount}>
      {selected && <div className={styles.accountEditor} key={selected.id}>
        <div className={styles.editorStatus}><StatusTag tone={selected.status === 'ACTIVE' ? 'success' : 'neutral'}>{selected.status === 'ACTIVE' ? '账号已启用' : '账号已停用'}</StatusTag>{canManage && <button type="button" disabled={mutating} onClick={() => setPendingStatus(selected)}>{selected.status === 'ACTIVE' ? '停用账号' : '启用账号'}</button>}</div>
        <nav className={styles.editorTabs} aria-label={`${selected.nickname} 账号设置`}>{dialogTabs.map((item) => { const Icon = item.icon; return <button type="button" key={item.key} className={accountSection === item.key ? styles.activeEditorTab : undefined} aria-current={accountSection === item.key ? 'page' : undefined} onClick={() => { setMessage(''); setAccountSection(item.key) }}><Icon aria-hidden="true" />{item.label}</button> })}</nav>
        {message && <p className={styles.message} role="status">{message}</p>}
        <div className={styles.editorBody}>
          {accountSection === 'overview' && <><dl className={styles.accountSummary}><div><dt>账号状态</dt><dd>{selected.status === 'ACTIVE' ? '启用' : '停用'}</dd></div><div><dt>已分配角色</dt><dd>{selected.roles.map((role) => role.name).join('、') || '未分配'}</dd></div><div><dt>数据范围</dt><dd>{selected.mallScopeMode === 'ALL' ? '全部商场' : `${selected.mallIds.length} 个指定商场`}</dd></div></dl>{canManage && <form className={styles.singleForm} onSubmit={(event) => { event.preventDefault(); const form = new FormData(event.currentTarget); void mutate(`/v1/access/accounts/${selected.id}`, { phone: profilePhone, nickname: profileNickname, reason: value(form, 'reason') }, '基础信息已保存。') }}><h3>基本资料</h3><p>手机号需要重新输入完整号码；列表中的号码已脱敏。</p><Field label="完整手机号"><input value={profilePhone} onChange={(event) => setProfilePhone(event.currentTarget.value)} required pattern="1[3-9][0-9]{9}" placeholder="输入 11 位手机号" /></Field><Field label="昵称"><input value={profileNickname} onChange={(event) => setProfileNickname(event.currentTarget.value)} required /></Field><Field label="变更原因"><input name="reason" required /></Field><button className={styles.primary} type="submit" disabled={mutating}>保存基本资料</button></form>}</>}
          {accountSection === 'roles' && (canReplaceAccountRoles(roleCatalogStatus) ? <form className={styles.singleForm} onSubmit={(event) => { event.preventDefault(); const form = new FormData(event.currentTarget); void mutate(`/v1/access/accounts/${selected.id}/roles`, { roleIds: editRoleIDs, reason: value(form, 'reason') }, '角色权限已保存。') }}><div className={styles.roleSelectionHeader}><div><h3>角色权限</h3><p>角色决定账号可访问的业务模块。</p></div>{canManageRoles && grantableCatalog.length > 0 && <button type="button" disabled={mutating} onClick={() => { setRoleCreatorError(''); setRoleCreatorTarget('edit') }}><Plus aria-hidden="true" />自定义角色</button>}</div><div className={styles.roleChoices}>{activeRoles.map((role) => <label className={styles.check} key={role.id}><input type="checkbox" checked={editRoleIDs.includes(role.id)} onChange={(event) => { const checked = event.currentTarget.checked; setEditRoleIDs((current) => updateRoleSelection(current, role.id, checked)) }} />{role.name}<small>{role.permissions.length} 项权限</small></label>)}</div><Field label="变更原因"><input name="reason" required /></Field><button className={styles.primary} type="submit" disabled={mutating}>保存角色权限</button></form> : <FeedbackState kind={roleCatalogStatus === 'error' ? 'error' : 'loading'} title={roleCatalogStatus === 'error' ? '角色目录加载失败' : '正在加载角色目录'} />)}
          {accountSection === 'malls' && <form className={styles.singleForm} onSubmit={(event) => { event.preventDefault(); const form = new FormData(event.currentTarget); const scope = accessMallScopeRequest(editMallScopeMode, editMallIDs); if (!scope) { setMessage('选择指定商场时，请至少选择一个商场。'); return } void mutate(`/v1/access/accounts/${selected.id}/mall-scope`, { ...scope, reason: value(form, 'reason') }, '商场范围已保存。') }}><h3>商场范围</h3><p>限制该账号可查看和操作的商场数据。</p><MallScopeFields mode={editMallScopeMode} mallIDs={editMallIDs} malls={malls} catalogStatus={mallCatalogStatus} disabled={mutating} onModeChange={setEditMallScopeMode} onMallIDsChange={setEditMallIDs} onRetry={loadMalls} /><Field label="变更原因"><input name="reason" required /></Field><button className={styles.primary} type="submit" disabled={mutating || editMallScopeMode === 'SELECTED' && mallCatalogStatus !== 'ready'}>保存商场范围</button></form>}
          {accountSection === 'reports' && canManageReportCategories && <AccessAccountReportCategories key={selected.id} accountID={selected.id} accountName={selected.nickname} client={client} />}
          {accountSection === 'security' && <form className={styles.singleForm} onSubmit={(event) => { event.preventDefault(); const form = new FormData(event.currentTarget); void mutate(`/v1/access/accounts/${selected.id}/password`, { password: value(form, 'password'), reason: value(form, 'reason') }, '密码已重置，账号旧会话已失效。') }}><h3>登录安全</h3><p>重置密码后，该账号的所有旧会话会立即失效。</p><Field label="新密码"><input type="password" name="password" minLength={10} maxLength={72} required autoComplete="new-password" placeholder="10–72 位" /></Field><Field label="重置原因"><input name="reason" required /></Field><button className={styles.primary} type="submit" disabled={mutating}><KeyRound aria-hidden="true" />重置密码</button></form>}
        </div>
      </div>}
    </Drawer>

    <Dialog className={styles.createAccountDialog} open={creating} title="创建控制台账号" description="创建后即可通过角色和分类权限控制可用功能。" closeDisabled={mutating} onClose={() => setCreating(false)}><form className={styles.drawerForm} onSubmit={create}>
      {message && <p className={styles.message} role="status">{message}</p>}
      <Field label="账号"><input name="account" required minLength={3} maxLength={40} pattern="[A-Za-z0-9][A-Za-z0-9_-]{2,39}" autoComplete="username" /></Field><Field label="手机号"><input name="phone" required pattern="1[3-9][0-9]{9}" autoComplete="tel" /></Field><Field label="昵称"><input name="nickname" required autoComplete="off" /></Field><Field label="初始密码"><input name="password" required type="password" minLength={10} maxLength={72} autoComplete="new-password" /></Field>
      <div className={styles.roleSelectionHeader}><div><strong>初始角色</strong></div>{canManageRoles && grantableCatalog.length > 0 && <button type="button" disabled={mutating} onClick={() => { setRoleCreatorError(''); setRoleCreatorTarget('create') }}><Plus aria-hidden="true" />新建角色</button>}</div>
      <div className={styles.roleChoices}>{activeRoles.map((role) => <label className={styles.check} key={role.id}><input type="checkbox" checked={createRoleIDs.includes(role.id)} onChange={(event) => { const checked = event.currentTarget.checked; setCreateRoleIDs((current) => updateRoleSelection(current, role.id, checked)) }} />{role.name}<small>{role.permissions.length} 项权限</small></label>)}</div>
      <MallScopeFields mode={createMallScopeMode} mallIDs={createMallIDs} malls={malls} catalogStatus={mallCatalogStatus} disabled={mutating} onModeChange={setCreateMallScopeMode} onMallIDsChange={setCreateMallIDs} onRetry={loadMalls} />
      <Field label="创建原因"><input name="reason" required /></Field><div className={styles.dialogActions}><button type="button" disabled={mutating} onClick={() => setCreating(false)}>取消</button><button className={styles.primary} type="submit" disabled={mutating}>{mutating ? '创建中…' : '创建账号'}</button></div>
    </form></Dialog>

    <Dialog open={Boolean(pendingStatus)} role="alertdialog" title={pendingStatus?.status === 'ACTIVE' ? '停用账号' : '启用账号'} description={pendingStatus ? `即将变更“${pendingStatus.nickname}”的登录状态。` : undefined} closeDisabled={mutating} onClose={() => setPendingStatus(null)} footer={<><button type="button" disabled={mutating} onClick={() => setPendingStatus(null)}>取消</button><button className={pendingStatus?.status === 'ACTIVE' ? styles.danger : styles.primary} type="button" disabled={mutating} onClick={() => { if (!pendingStatus) return; void mutate(`/v1/access/accounts/${pendingStatus.id}/status`, { status: pendingStatus.status === 'ACTIVE' ? 'DISABLED' : 'ACTIVE', reason: pendingStatus.status === 'ACTIVE' ? '管理员停用账号' : '管理员启用账号' }, pendingStatus.status === 'ACTIVE' ? '账号已停用。' : '账号已启用。').then((ok) => { if (ok) setPendingStatus(null) }) }}>确认{pendingStatus?.status === 'ACTIVE' ? '停用' : '启用'}</button></>}><p className={styles.dangerNotice}>{pendingStatus?.status === 'ACTIVE' ? '停用后，该账号现有会话会失效且无法继续登录。' : '启用后，该账号可以重新登录。'}</p></Dialog>

    <AccessRoleCreateDrawer open={roleCreatorTarget !== null} catalog={grantableCatalog} busy={mutating} error={roleCreatorError} onClose={() => { if (!mutating) { setRoleCreatorTarget(null); setRoleCreatorError('') } }} onSubmit={createRole} />
  </Section>
}

function MallScopeFields({ mode, mallIDs, malls, catalogStatus, disabled, onModeChange, onMallIDsChange, onRetry }: {
  mode: 'ALL' | 'SELECTED'; mallIDs: number[]; malls: AccessMall[]; catalogStatus: MallCatalogStatus; disabled: boolean
  onModeChange: (mode: 'ALL' | 'SELECTED') => void; onMallIDsChange: (ids: number[]) => void; onRetry: () => void
}) {
  const options = malls.map((mall) => ({ value: mall.id, label: `${mall.nameCn}（${mall.mallCode}）` }))
  return <><Field label="商场范围"><select value={mode} disabled={disabled} onChange={(event) => onModeChange(event.currentTarget.value as 'ALL' | 'SELECTED')}><option value="ALL">全部商场</option><option value="SELECTED">指定商场</option></select></Field>{mode === 'SELECTED' && <Field label="选择商场"><Select className={styles.mallSelect} mode="multiple" showSearch optionFilterProp="label" maxTagCount="responsive" placeholder={catalogStatus === 'loading' ? '正在加载商场…' : '搜索并选择商场'} value={mallIDs} options={options} loading={catalogStatus === 'loading'} disabled={disabled || catalogStatus !== 'ready'} onChange={(values) => onMallIDsChange(values)} /><small className={styles.fieldHint}>已选择 {mallIDs.length} 个商场。</small>{catalogStatus === 'error' && <span className={styles.inlineError}>商场目录加载失败。<button type="button" onClick={onRetry}>重试</button></span>}</Field>}</>
}

function Field({ label, children }: { label: string; children: React.ReactNode }) { return <label className={styles.field}><span>{label}</span>{children}</label> }
function value(form: FormData, key: string) { const result = form.get(key); return typeof result === 'string' ? result.trim() : '' }
