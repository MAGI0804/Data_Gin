import { FormEvent, ReactNode, useMemo, useState } from 'react'
import {
  Activity,
  Database,
  FileJson,
  LockKeyhole,
  LogOut,
  Play,
  Plus,
  RefreshCcw,
  Send,
  Settings,
  ShieldCheck,
} from 'lucide-react'
import './App.css'

type ApiResult = {
  ok: boolean
  status: number
  data: unknown
}

type NavKey = 'sources' | 'transform' | 'delivery' | 'runs'

const defaultMappingConfig = JSON.stringify(
  {
    table_name: 'clean_orders',
    business_key_field: 'order_no',
    fields: [
      { name: 'order_no', source_path: '$.params.orderNo', type: 'string', required: true },
      { name: 'actual_amount', source_path: '$.data.amount', type: 'decimal', transform: 'divide:100' },
    ],
  },
  null,
  2,
)

function App() {
  const [authenticated, setAuthenticated] = useState(() => sessionStorage.getItem('warehouse-auth') === 'ok')
  const [active, setActive] = useState<NavKey>('sources')
  const [token, setToken] = useState('')
  const [result, setResult] = useState<ApiResult | null>(null)
  const [loading, setLoading] = useState(false)

  const client = useMemo(() => {
    return async (path: string, body?: unknown) => {
      setLoading(true)
      try {
        const response = await fetch(`/api${path}`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            ...(token ? { Authorization: `Bearer ${token}` } : {}),
          },
          body: body === undefined ? undefined : JSON.stringify(body),
        })
        const data = await response.json().catch(() => ({}))
        setResult({ ok: response.ok, status: response.status, data })
      } catch (error) {
        setResult({
          ok: false,
          status: 0,
          data: error instanceof Error ? error.message : String(error),
        })
      } finally {
        setLoading(false)
      }
    }
  }, [token])

  function handleLoginSuccess() {
    sessionStorage.setItem('warehouse-auth', 'ok')
    setAuthenticated(true)
  }

  function handleLogout() {
    sessionStorage.removeItem('warehouse-auth')
    setAuthenticated(false)
    setResult(null)
  }

  if (!authenticated) {
    return <LoginScreen onLogin={handleLoginSuccess} />
  }

  return (
    <main className="app-shell">
      <aside className="sidebar" aria-label="主导航">
        <div className="brand">
          <Database aria-hidden="true" />
          <div>
            <h1>数据仓库</h1>
            <span>流水线管理台</span>
          </div>
        </div>
        <nav className="nav-list">
          <NavButton active={active === 'sources'} icon={<Database />} label="数据源" onClick={() => setActive('sources')} />
          <NavButton active={active === 'transform'} icon={<FileJson />} label="清洗规则" onClick={() => setActive('transform')} />
          <NavButton active={active === 'delivery'} icon={<Send />} label="推送任务" onClick={() => setActive('delivery')} />
          <NavButton active={active === 'runs'} icon={<Activity />} label="运行记录" onClick={() => setActive('runs')} />
        </nav>
      </aside>

      <section className="workspace">
        <header className="topbar">
          <div>
            <p className="eyebrow">develop 分支</p>
            <h2>{sectionTitle(active)}</h2>
          </div>
          <div className="connection-panel" aria-label="接口设置">
            <Settings aria-hidden="true" />
            <label>
              JWT
              <input
                value={token}
                type="password"
                autoComplete="off"
                onChange={(event) => setToken(event.target.value)}
              />
            </label>
            <button type="button" onClick={handleLogout}>
              <LogOut aria-hidden="true" />
              退出
            </button>
          </div>
        </header>

        <div className="content-grid">
          <section className="panel">
            {active === 'sources' && <SourcesPanel client={client} loading={loading} />}
            {active === 'transform' && <TransformPanel client={client} loading={loading} />}
            {active === 'delivery' && <DeliveryPanel client={client} loading={loading} />}
            {active === 'runs' && <RunsPanel />}
          </section>

          <aside className="result-panel" aria-live="polite">
            <div className="panel-heading">
              <ShieldCheck aria-hidden="true" />
              <h3>接口返回</h3>
            </div>
            {result ? (
              <pre className={result.ok ? 'result success' : 'result error'}>{JSON.stringify(result, null, 2)}</pre>
            ) : (
              <div className="empty-state">还没有发送请求。</div>
            )}
          </aside>
        </div>
      </section>
    </main>
  )
}

function LoginScreen({ onLogin }: { onLogin: () => void }) {
  const [error, setError] = useState('')

  function submitLogin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const username = String(form.get('username') ?? '')
    const password = String(form.get('password') ?? '')

    if (username === 'admin' && password === 'youlan123') {
      setError('')
      onLogin()
      return
    }

    setError('账号或密码不正确')
  }

  return (
    <main className="login-shell">
      <section className="login-panel" aria-label="登录">
        <div className="login-title">
          <LockKeyhole aria-hidden="true" />
          <div>
            <h1>数据仓库管理台</h1>
            <p>请输入管理员账号后继续操作。</p>
          </div>
        </div>
        <form className="login-form" onSubmit={submitLogin}>
          <label>
            账号
            <input name="username" autoComplete="username" autoFocus />
          </label>
          <label>
            密码
            <input name="password" type="password" autoComplete="current-password" />
          </label>
          {error && <div className="login-error">{error}</div>}
          <button className="primary" type="submit">
            <ShieldCheck aria-hidden="true" />
            登录
          </button>
        </form>
      </section>
    </main>
  )
}

function SourcesPanel({ client, loading }: { client: (path: string, body?: unknown) => Promise<void>; loading: boolean }) {
  const [sourceID, setSourceID] = useState('1')

  function createSource(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    client('/v1/sources', {
      name: form.get('name'),
      code: form.get('code'),
      source_type: form.get('source_type'),
      source_query_key: form.get('source_query_key'),
      config_json: form.get('config_json'),
      enabled: true,
    })
  }

  return (
    <>
      <div className="panel-heading">
        <Plus aria-hidden="true" />
        <h3>数据源配置</h3>
      </div>
      <form className="form-grid" onSubmit={createSource}>
        <Field label="名称" name="name" defaultValue="企迈 Webhook" />
        <Field label="编码" name="code" defaultValue="qimai_order" />
        <label>
          类型
          <select name="source_type" defaultValue="webhook">
            <option value="webhook">webhook</option>
            <option value="api_poll">api_poll</option>
            <option value="database">database</option>
          </select>
        </label>
        <Field label="来源参数名" name="source_query_key" defaultValue="source" />
        <label className="wide">
          配置 JSON
          <textarea name="config_json" defaultValue={'{}'} rows={6} />
        </label>
        <button className="primary" disabled={loading}>
          <Plus aria-hidden="true" />
          创建
        </button>
      </form>

      <div className="inline-actions">
        <label>
          数据源 ID
          <input value={sourceID} onChange={(event) => setSourceID(event.target.value)} />
        </label>
        <button type="button" onClick={() => client(`/v1/sources/${sourceID}/test`)} disabled={loading}>
          <RefreshCcw aria-hidden="true" />
          测试
        </button>
        <button type="button" onClick={() => client(`/v1/sources/${sourceID}/fetch`)} disabled={loading}>
          <Play aria-hidden="true" />
          拉取
        </button>
      </div>
    </>
  )
}

function TransformPanel({ client, loading }: { client: (path: string, body?: unknown) => Promise<void>; loading: boolean }) {
  const [rawRecordID, setRawRecordID] = useState('1')

  function createRule(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    client('/v1/transform-rules', {
      source_id: Number(form.get('source_id')),
      name: form.get('name'),
      rule_type: 'mapping',
      order_index: Number(form.get('order_index')),
      config_json: form.get('config_json'),
      enabled: true,
    })
  }

  function testRule(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    client('/v1/transform-rules/test', {
      raw_content: JSON.parse(String(form.get('raw_content'))),
      config_json: form.get('config_json'),
    })
  }

  return (
    <>
      <div className="panel-heading">
        <FileJson aria-hidden="true" />
        <h3>清洗规则</h3>
      </div>
      <form className="form-grid" onSubmit={createRule}>
        <Field label="数据源 ID" name="source_id" defaultValue="1" />
        <Field label="规则名称" name="name" defaultValue="订单字段映射" />
        <Field label="排序" name="order_index" defaultValue="1" />
        <label className="wide">
          映射配置
          <textarea name="config_json" defaultValue={defaultMappingConfig} rows={10} />
        </label>
        <button className="primary" disabled={loading}>
          <Plus aria-hidden="true" />
          保存规则
        </button>
      </form>

      <form className="form-grid separated" onSubmit={testRule}>
        <label className="wide">
          原始样例 JSON
          <textarea
            name="raw_content"
            rows={5}
            defaultValue={'{"params":{"orderNo":"ORDER-1"},"data":{"amount":"12345"}}'}
          />
        </label>
        <label className="wide">
          配置 JSON
          <textarea name="config_json" rows={8} defaultValue={defaultMappingConfig} />
        </label>
        <button disabled={loading}>
          <RefreshCcw aria-hidden="true" />
          测试规则
        </button>
      </form>

      <div className="inline-actions">
        <label>
          原始记录 ID
          <input value={rawRecordID} onChange={(event) => setRawRecordID(event.target.value)} />
        </label>
        <button type="button" onClick={() => client(`/v1/raw-records/${rawRecordID}/retransform`)} disabled={loading}>
          <Play aria-hidden="true" />
          重新清洗
        </button>
      </div>
    </>
  )
}

function DeliveryPanel({ client, loading }: { client: (path: string, body?: unknown) => Promise<void>; loading: boolean }) {
  const [destinationID, setDestinationID] = useState('1')
  const [taskID, setTaskID] = useState('1')

  function createDestination(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    client('/v1/destinations', {
      name: form.get('name'),
      code: form.get('code'),
      destination_type: form.get('destination_type'),
      config_json: form.get('config_json'),
      enabled: true,
    })
  }

  function createTask(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    client('/v1/delivery-tasks', {
      name: form.get('name'),
      source_id: Number(form.get('source_id')),
      clean_table: form.get('clean_table'),
      destination_id: Number(form.get('destination_id')),
      trigger_type: form.get('trigger_type'),
      cron_expr: form.get('cron_expr'),
      payload_template: form.get('payload_template'),
      enabled: true,
    })
  }

  return (
    <>
      <div className="panel-heading">
        <Send aria-hidden="true" />
        <h3>推送任务</h3>
      </div>
      <form className="form-grid" onSubmit={createDestination}>
        <Field label="目标名称" name="name" defaultValue="HTTP 推送目标" />
        <Field label="目标编码" name="code" defaultValue="http_sink" />
        <label>
          类型
          <select name="destination_type" defaultValue="http">
            <option value="http">http</option>
            <option value="soap">soap</option>
          </select>
        </label>
        <label className="wide">
          配置 JSON
          <textarea name="config_json" rows={6} defaultValue={'{"url":"http://localhost:9000/orders","method":"POST"}'} />
        </label>
        <button className="primary" disabled={loading}>
          <Plus aria-hidden="true" />
          创建目标
        </button>
      </form>

      <form className="form-grid separated" onSubmit={createTask}>
        <Field label="任务名称" name="name" defaultValue="推送清洗订单" />
        <Field label="数据源 ID" name="source_id" defaultValue="1" />
        <Field label="清洗表" name="clean_table" defaultValue="clean_orders" />
        <Field label="目标 ID" name="destination_id" defaultValue="1" />
        <label>
          触发方式
          <select name="trigger_type" defaultValue="manual">
            <option value="manual">manual</option>
            <option value="schedule">schedule</option>
            <option value="event">event</option>
          </select>
        </label>
        <Field label="Cron" name="cron_expr" defaultValue="@every 5m" />
        <label className="wide">
          报文模板
          <textarea name="payload_template" rows={5} defaultValue={'{"order_no":"{{order_no}}","amount":"{{actual_amount}}"}'} />
        </label>
        <button className="primary" disabled={loading}>
          <Plus aria-hidden="true" />
          创建任务
        </button>
      </form>

      <div className="inline-actions">
        <label>
          目标 ID
          <input value={destinationID} onChange={(event) => setDestinationID(event.target.value)} />
        </label>
        <button type="button" onClick={() => client(`/v1/destinations/${destinationID}/test`)} disabled={loading}>
          <RefreshCcw aria-hidden="true" />
          测试
        </button>
        <label>
          任务 ID
          <input value={taskID} onChange={(event) => setTaskID(event.target.value)} />
        </label>
        <button type="button" onClick={() => client(`/v1/delivery-tasks/${taskID}/run`)} disabled={loading}>
          <Play aria-hidden="true" />
          执行
        </button>
      </div>
    </>
  )
}

function RunsPanel() {
  const stats = [
    ['数据源', 'source_definitions'],
    ['原始记录', 'raw_records'],
    ['清洗结果', 'clean_records'],
    ['运行记录', 'pipeline_runs'],
    ['推送日志', 'delivery_logs'],
  ]

  return (
    <>
      <div className="panel-heading">
        <Activity aria-hidden="true" />
        <h3>运行追踪</h3>
      </div>
      <div className="metric-grid">
        {stats.map(([label, table]) => (
          <div className="metric" key={table}>
            <span>{label}</span>
            <strong>{table}</strong>
          </div>
        ))}
      </div>
      <div className="empty-state">运行记录和链路查询接口会在下一步后端切片中补齐。</div>
    </>
  )
}

function Field({ label, name, defaultValue }: { label: string; name: string; defaultValue: string }) {
  return (
    <label>
      {label}
      <input name={name} defaultValue={defaultValue} />
    </label>
  )
}

function NavButton({
  active,
  icon,
  label,
  onClick,
}: {
  active: boolean
  icon: ReactNode
  label: string
  onClick: () => void
}) {
  return (
    <button className={active ? 'nav-button active' : 'nav-button'} onClick={onClick} type="button">
      {icon}
      <span>{label}</span>
    </button>
  )
}

function sectionTitle(key: NavKey) {
  switch (key) {
    case 'sources':
      return '数据源'
    case 'transform':
      return '清洗流水线'
    case 'delivery':
      return '推送流水线'
    case 'runs':
      return '运行记录'
  }
}

export default App
