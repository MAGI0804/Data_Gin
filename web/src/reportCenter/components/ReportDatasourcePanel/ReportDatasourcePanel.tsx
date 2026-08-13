import { useCallback, useEffect, useRef, useState } from 'react'
import { Database, Pencil, Plus, RefreshCw, Unplug } from 'lucide-react'
import { Button, DataTable, Drawer, FeedbackState, StatusTag } from '../../../ui'
import {
  createReportDatasource,
  testReportDatasource,
  updateReportDatasource,
  type ReportCenterClient,
} from '../../api'
import type { ReportDatasource, ReportDatasourceInput } from '../../types'
import styles from './ReportDatasourcePanel.module.css'

type EditorState = { open: false } | { open: true; datasource: ReportDatasource | null }

export function ReportDatasourcePanel({
  client,
  datasources,
  loading,
  error,
  onReload,
}: {
  client: ReportCenterClient
  datasources: ReportDatasource[]
  loading: boolean
  error: string
  onReload: () => void
}) {
  const [editor, setEditor] = useState<EditorState>({ open: false })
  const [testingId, setTestingId] = useState(0)
  const [testNotice, setTestNotice] = useState('')

  async function testConnection(datasource: ReportDatasource) {
    setTestingId(datasource.id)
    setTestNotice('')
    const response = await testReportDatasource(client, datasource.id)
    setTestingId(0)
    if (!response.ok) {
      setTestNotice(response.error)
      return
    }
    setTestNotice(response.data.message)
    onReload()
  }

  return (
    <div className={styles.root}>
      <div className={styles.toolbar}>
        <div>
          <strong>Oracle 数据源</strong>
          <span>连接参数和加密凭据保存在 MySQL，Oracle 仅用于执行和取数。</span>
        </div>
        <button type="button" onClick={() => setEditor({ open: true, datasource: null })}>
          <Plus aria-hidden="true" />新增数据源
        </button>
      </div>
      {testNotice ? <p className={styles.notice} role="status">{testNotice}</p> : null}
      {loading ? <FeedbackState kind="loading" title="正在读取 Oracle 数据源" /> : null}
      {error ? <FeedbackState kind="error" title="数据源列表不可用" description={error} action={<button type="button" onClick={onReload}>重试</button>} /> : null}
      {!loading && !error && datasources.length === 0 ? <FeedbackState kind="empty" title="暂无 Oracle 数据源" description="请先建立数据源，再绑定报表配置。" /> : null}
      {datasources.length > 0 ? (
        <DataTable density="compact" minWidth={980} scrollLabel="Oracle 数据源列表">
          <thead><tr><th scope="col">数据源</th><th scope="col">连接地址</th><th scope="col">用户</th><th scope="col">状态</th><th scope="col">最近测试</th><th scope="col">操作</th></tr></thead>
          <tbody>{datasources.map((item) => (
            <tr key={item.id}>
              <td><span className={styles.identity}><Database aria-hidden="true" /><span><strong>{item.name}</strong><code>{item.code}</code></span></span></td>
              <td><code>{item.host}:{item.port}/{item.serviceName || item.sid}</code><small>{item.serviceName ? 'Service Name' : 'SID'}</small></td>
              <td><code>{item.username}</code></td>
              <td><StatusTag tone={item.enabled ? 'success' : 'neutral'}>{item.enabled ? '启用' : '停用'}</StatusTag></td>
              <td><TestStatus datasource={item} /></td>
              <td><span className={styles.actions}>
                <button type="button" onClick={() => setEditor({ open: true, datasource: item })}><Pencil aria-hidden="true" />编辑</button>
                <button type="button" disabled={testingId !== 0} onClick={() => void testConnection(item)}><RefreshCw aria-hidden="true" />{testingId === item.id ? '测试中…' : '连接测试'}</button>
              </span></td>
            </tr>
          ))}</tbody>
        </DataTable>
      ) : null}
      {editor.open ? (
        <ReportDatasourceDrawer
          client={client}
          datasource={editor.datasource}
          onClose={() => setEditor({ open: false })}
          onSaved={() => { setEditor({ open: false }); onReload() }}
        />
      ) : null}
    </div>
  )
}

function TestStatus({ datasource }: { datasource: ReportDatasource }) {
  if (datasource.lastTestStatus === 'NOT_TESTED') return <span className={styles.muted}>尚未测试</span>
  const success = datasource.lastTestStatus === 'SUCCESS'
  return <span className={styles.testStatus}><StatusTag tone={success ? 'success' : 'danger'}>{success ? '通过' : '失败'}</StatusTag><small>{formatDate(datasource.lastTestedAt)}{!success && datasource.lastTestError ? ` · ${datasource.lastTestError}` : ''}</small></span>
}

function ReportDatasourceDrawer({ client, datasource, onClose, onSaved }: { client: ReportCenterClient; datasource: ReportDatasource | null; onClose: () => void; onSaved: () => void }) {
  const [input, setInput] = useState<ReportDatasourceInput>(() => datasourceInput(datasource))
  const [serviceMode, setServiceMode] = useState<'SERVICE_NAME' | 'SID'>(() => datasource?.sid ? 'SID' : 'SERVICE_NAME')
  const serviceNameDraft = useRef(datasource?.serviceName ?? '')
  const sidDraft = useRef(datasource?.sid ?? '')
  const [state, setState] = useState({ saving: false, error: '' })
  const set = useCallback(<K extends keyof ReportDatasourceInput>(key: K, value: ReportDatasourceInput[K]) => setInput((current) => ({ ...current, [key]: value })), [])

  useEffect(() => {
    setInput(datasourceInput(datasource))
    setServiceMode(datasource?.sid ? 'SID' : 'SERVICE_NAME')
    serviceNameDraft.current = datasource?.serviceName ?? ''
    sidDraft.current = datasource?.sid ?? ''
  }, [datasource])

  async function save() {
    const validationError = validateDatasource(input, Boolean(datasource))
    if (validationError) { setState({ saving: false, error: validationError }); return }
    setState({ saving: true, error: '' })
    const response = datasource
      ? await updateReportDatasource(client, datasource.id, input)
      : await createReportDatasource(client, input)
    if (!response.ok) { setState({ saving: false, error: response.error }); return }
    onSaved()
  }

  function changeServiceMode(nextMode: 'SERVICE_NAME' | 'SID') {
    if (serviceMode === 'SERVICE_NAME') serviceNameDraft.current = input.serviceName
    else sidDraft.current = input.sid
    setServiceMode(nextMode)
    setInput((current) => ({
      ...current,
      serviceName: nextMode === 'SERVICE_NAME' ? serviceNameDraft.current : '',
      sid: nextMode === 'SID' ? sidDraft.current : '',
    }))
  }
  const footer = <><button type="button" disabled={state.saving} onClick={onClose}>取消</button><Button variant="primary" disabled={state.saving} onClick={() => void save()}>{state.saving ? '保存中…' : '保存数据源'}</Button></>
  return (
    <Drawer open title={datasource ? '编辑 Oracle 数据源' : '新增 Oracle 数据源'} description="凭据由服务端使用当前密钥版本加密，页面不会读取或回显密文。" size="medium" closeDisabled={state.saving} onClose={onClose} footer={footer}>
      <form className={styles.form} onSubmit={(event) => { event.preventDefault(); void save() }}>
        <Field label="数据源名称"><input required disabled={state.saving} maxLength={128} value={input.name} onChange={(event) => set('name', event.currentTarget.value)} /></Field>
        <Field label="数据源编码"><input required disabled={state.saving} className={styles.mono} maxLength={64} pattern="[a-z][a-z0-9_]{2,63}" value={input.code} onChange={(event) => set('code', event.currentTarget.value)} placeholder="report_oracle" /></Field>
        <Field label="主机地址"><input required disabled={state.saving} className={styles.mono} maxLength={255} value={input.host} onChange={(event) => set('host', event.currentTarget.value)} placeholder="oracle.internal" /></Field>
        <Field label="端口"><input required disabled={state.saving} type="number" min="1" max="65535" value={input.port} onChange={(event) => set('port', Number(event.currentTarget.value))} /></Field>
        <Field label="连接标识类型"><select disabled={state.saving} value={serviceMode} onChange={(event) => changeServiceMode(event.currentTarget.value as 'SERVICE_NAME' | 'SID')}><option value="SERVICE_NAME">Service Name</option><option value="SID">SID</option></select></Field>
        <Field label={serviceMode === 'SID' ? 'SID' : 'Service Name'}><input required disabled={state.saving} className={styles.mono} maxLength={128} value={serviceMode === 'SID' ? input.sid : input.serviceName} onChange={(event) => { if (serviceMode === 'SID') { sidDraft.current = event.currentTarget.value; set('sid', event.currentTarget.value) } else { serviceNameDraft.current = event.currentTarget.value; set('serviceName', event.currentTarget.value) } }} /></Field>
        <Field label="Oracle 用户名"><input required disabled={state.saving} className={styles.mono} maxLength={128} autoComplete="username" value={input.username} onChange={(event) => set('username', event.currentTarget.value)} /></Field>
        <Field label={datasource ? '新密码（可选）' : '密码'} hint={datasource ? '留空将保留现有密码，不会从服务端回显。' : '创建时必填；保存后不会回显。'}><input required={!datasource} disabled={state.saving} maxLength={1024} type="password" autoComplete="new-password" value={input.password} onChange={(event) => set('password', event.currentTarget.value)} /></Field>
        <Field label="会话时区"><input disabled={state.saving} maxLength={64} value={input.sessionTimezone} onChange={(event) => set('sessionTimezone', event.currentTarget.value)} /></Field>
        <Field label="连接超时（秒）"><input disabled={state.saving} type="number" min="1" max="60" value={input.connectTimeoutSeconds} onChange={(event) => set('connectTimeoutSeconds', Number(event.currentTarget.value))} /></Field>
        <Field label="查询超时（秒）"><input disabled={state.saving} type="number" min="1" max="86400" value={input.queryTimeoutSeconds} onChange={(event) => set('queryTimeoutSeconds', Number(event.currentTarget.value))} /></Field>
        <Field label="最大连接数"><input disabled={state.saving} type="number" min="1" max="100" value={input.maxOpenConnections} onChange={(event) => set('maxOpenConnections', Number(event.currentTarget.value))} /></Field>
        <Field label="最大空闲连接"><input disabled={state.saving} type="number" min="0" max={input.maxOpenConnections} value={input.maxIdleConnections} onChange={(event) => set('maxIdleConnections', Number(event.currentTarget.value))} /></Field>
        <Field label="预取行数"><input disabled={state.saving} type="number" min="1" max="10000" value={input.prefetchRows} onChange={(event) => set('prefetchRows', Number(event.currentTarget.value))} /></Field>
        <Field label="批量数组大小"><input disabled={state.saving} type="number" min="1" max="10000" value={input.arraySize} onChange={(event) => set('arraySize', Number(event.currentTarget.value))} /></Field>
        <label className={styles.switch}><input disabled={state.saving} type="checkbox" checked={input.enabled} onChange={(event) => set('enabled', event.currentTarget.checked)} /><span>允许新报表绑定和运行</span></label>
      </form>
      {state.error ? <div className={styles.error} role="alert"><Unplug aria-hidden="true" />{state.error}</div> : null}
    </Drawer>
  )
}

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return <label className={styles.field}><span>{label}</span>{children}{hint ? <small>{hint}</small> : null}</label>
}

function datasourceInput(datasource: ReportDatasource | null): ReportDatasourceInput {
  return datasource ? {
    code: datasource.code, name: datasource.name, host: datasource.host, port: datasource.port,
    serviceName: datasource.serviceName, sid: datasource.sid, username: datasource.username, password: '',
    sessionTimezone: datasource.sessionTimezone, connectTimeoutSeconds: datasource.connectTimeoutSeconds,
    queryTimeoutSeconds: datasource.queryTimeoutSeconds, maxOpenConnections: datasource.maxOpenConnections,
    maxIdleConnections: datasource.maxIdleConnections, prefetchRows: datasource.prefetchRows,
    arraySize: datasource.arraySize, enabled: datasource.enabled,
  } : { code: '', name: '', host: '', port: 1521, serviceName: '', sid: '', username: '', password: '', sessionTimezone: 'Asia/Shanghai', connectTimeoutSeconds: 5, queryTimeoutSeconds: 300, maxOpenConnections: 10, maxIdleConnections: 2, prefetchRows: 1000, arraySize: 1000, enabled: true }
}

function validateDatasource(input: ReportDatasourceInput, editing: boolean) {
  if (!input.name.trim() || !/^[a-z][a-z0-9_]{2,63}$/.test(input.code.trim()) || !input.host.trim() || !input.username.trim()) return '请完整填写名称、合法编码、主机和用户名。'
  if (Boolean(input.serviceName.trim()) === Boolean(input.sid.trim())) return 'Service Name 与 SID 必须且只能填写一个。'
  if (!editing && !input.password) return '创建数据源时必须填写密码。'
  if (input.maxIdleConnections > input.maxOpenConnections) return '最大空闲连接不能超过最大连接数。'
  return ''
}

function formatDate(value: string | null) {
  return value ? new Intl.DateTimeFormat('zh-CN', { dateStyle: 'short', timeStyle: 'short' }).format(new Date(value)) : '-'
}
