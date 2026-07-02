import { FormEvent, ReactNode, useMemo, useState } from 'react'
import {
  Activity,
  Database,
  FileJson,
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
  const [active, setActive] = useState<NavKey>('sources')
  const [baseUrl, setBaseUrl] = useState('/api')
  const [token, setToken] = useState('')
  const [result, setResult] = useState<ApiResult | null>(null)
  const [loading, setLoading] = useState(false)

  const client = useMemo(() => {
    return async (path: string, body?: unknown) => {
      setLoading(true)
      try {
        const response = await fetch(`${baseUrl}${path}`, {
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
  }, [baseUrl, token])

  return (
    <main className="app-shell">
      <aside className="sidebar" aria-label="Primary">
        <div className="brand">
          <Database aria-hidden="true" />
          <div>
            <h1>Data Warehouse</h1>
            <span>Pipeline Console</span>
          </div>
        </div>
        <nav className="nav-list">
          <NavButton active={active === 'sources'} icon={<Database />} label="Sources" onClick={() => setActive('sources')} />
          <NavButton active={active === 'transform'} icon={<FileJson />} label="Transform" onClick={() => setActive('transform')} />
          <NavButton active={active === 'delivery'} icon={<Send />} label="Delivery" onClick={() => setActive('delivery')} />
          <NavButton active={active === 'runs'} icon={<Activity />} label="Runs" onClick={() => setActive('runs')} />
        </nav>
      </aside>

      <section className="workspace">
        <header className="topbar">
          <div>
            <p className="eyebrow">develop branch</p>
            <h2>{sectionTitle(active)}</h2>
          </div>
          <div className="connection-panel" aria-label="API settings">
            <Settings aria-hidden="true" />
            <label>
              Base URL
              <input value={baseUrl} onChange={(event) => setBaseUrl(event.target.value)} />
            </label>
            <label>
              JWT
              <input
                value={token}
                type="password"
                autoComplete="off"
                onChange={(event) => setToken(event.target.value)}
              />
            </label>
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
              <h3>API Result</h3>
            </div>
            {result ? (
              <pre className={result.ok ? 'result success' : 'result error'}>{JSON.stringify(result, null, 2)}</pre>
            ) : (
              <div className="empty-state">No request has been sent.</div>
            )}
          </aside>
        </div>
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
        <h3>Source Setup</h3>
      </div>
      <form className="form-grid" onSubmit={createSource}>
        <Field label="Name" name="name" defaultValue="Qimai Webhook" />
        <Field label="Code" name="code" defaultValue="qimai_order" />
        <label>
          Type
          <select name="source_type" defaultValue="webhook">
            <option value="webhook">webhook</option>
            <option value="api_poll">api_poll</option>
            <option value="database">database</option>
          </select>
        </label>
        <Field label="Source query key" name="source_query_key" defaultValue="source" />
        <label className="wide">
          Config JSON
          <textarea name="config_json" defaultValue={'{}'} rows={6} />
        </label>
        <button className="primary" disabled={loading}>
          <Plus aria-hidden="true" />
          Create
        </button>
      </form>

      <div className="inline-actions">
        <label>
          Source ID
          <input value={sourceID} onChange={(event) => setSourceID(event.target.value)} />
        </label>
        <button type="button" onClick={() => client(`/v1/sources/${sourceID}/test`)} disabled={loading}>
          <RefreshCcw aria-hidden="true" />
          Test
        </button>
        <button type="button" onClick={() => client(`/v1/sources/${sourceID}/fetch`)} disabled={loading}>
          <Play aria-hidden="true" />
          Fetch
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
        <h3>Transform Rules</h3>
      </div>
      <form className="form-grid" onSubmit={createRule}>
        <Field label="Source ID" name="source_id" defaultValue="1" />
        <Field label="Name" name="name" defaultValue="Order mapping" />
        <Field label="Order" name="order_index" defaultValue="1" />
        <label className="wide">
          Mapping config
          <textarea name="config_json" defaultValue={defaultMappingConfig} rows={10} />
        </label>
        <button className="primary" disabled={loading}>
          <Plus aria-hidden="true" />
          Save Rule
        </button>
      </form>

      <form className="form-grid separated" onSubmit={testRule}>
        <label className="wide">
          Sample raw JSON
          <textarea
            name="raw_content"
            rows={5}
            defaultValue={'{"params":{"orderNo":"ORDER-1"},"data":{"amount":"12345"}}'}
          />
        </label>
        <label className="wide">
          Config JSON
          <textarea name="config_json" rows={8} defaultValue={defaultMappingConfig} />
        </label>
        <button disabled={loading}>
          <RefreshCcw aria-hidden="true" />
          Test Rule
        </button>
      </form>

      <div className="inline-actions">
        <label>
          Raw record ID
          <input value={rawRecordID} onChange={(event) => setRawRecordID(event.target.value)} />
        </label>
        <button type="button" onClick={() => client(`/v1/raw-records/${rawRecordID}/retransform`)} disabled={loading}>
          <Play aria-hidden="true" />
          Retransform
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
        <h3>Delivery</h3>
      </div>
      <form className="form-grid" onSubmit={createDestination}>
        <Field label="Name" name="name" defaultValue="HTTP Sink" />
        <Field label="Code" name="code" defaultValue="http_sink" />
        <label>
          Type
          <select name="destination_type" defaultValue="http">
            <option value="http">http</option>
            <option value="soap">soap</option>
          </select>
        </label>
        <label className="wide">
          Config JSON
          <textarea name="config_json" rows={6} defaultValue={'{"url":"http://localhost:9000/orders","method":"POST"}'} />
        </label>
        <button className="primary" disabled={loading}>
          <Plus aria-hidden="true" />
          Create Destination
        </button>
      </form>

      <form className="form-grid separated" onSubmit={createTask}>
        <Field label="Task name" name="name" defaultValue="Push clean orders" />
        <Field label="Source ID" name="source_id" defaultValue="1" />
        <Field label="Clean table" name="clean_table" defaultValue="clean_orders" />
        <Field label="Destination ID" name="destination_id" defaultValue="1" />
        <label>
          Trigger
          <select name="trigger_type" defaultValue="manual">
            <option value="manual">manual</option>
            <option value="schedule">schedule</option>
            <option value="event">event</option>
          </select>
        </label>
        <Field label="Cron" name="cron_expr" defaultValue="@every 5m" />
        <label className="wide">
          Payload template
          <textarea name="payload_template" rows={5} defaultValue={'{"order_no":"{{order_no}}","amount":"{{actual_amount}}"}'} />
        </label>
        <button className="primary" disabled={loading}>
          <Plus aria-hidden="true" />
          Create Task
        </button>
      </form>

      <div className="inline-actions">
        <label>
          Destination ID
          <input value={destinationID} onChange={(event) => setDestinationID(event.target.value)} />
        </label>
        <button type="button" onClick={() => client(`/v1/destinations/${destinationID}/test`)} disabled={loading}>
          <RefreshCcw aria-hidden="true" />
          Test
        </button>
        <label>
          Task ID
          <input value={taskID} onChange={(event) => setTaskID(event.target.value)} />
        </label>
        <button type="button" onClick={() => client(`/v1/delivery-tasks/${taskID}/run`)} disabled={loading}>
          <Play aria-hidden="true" />
          Run
        </button>
      </div>
    </>
  )
}

function RunsPanel() {
  const stats = [
    ['Sources', 'source_definitions'],
    ['Raw', 'raw_records'],
    ['Clean', 'clean_records'],
    ['Runs', 'pipeline_runs'],
    ['Delivery', 'delivery_logs'],
  ]

  return (
    <>
      <div className="panel-heading">
        <Activity aria-hidden="true" />
        <h3>Trace Surface</h3>
      </div>
      <div className="metric-grid">
        {stats.map(([label, table]) => (
          <div className="metric" key={table}>
            <span>{label}</span>
            <strong>{table}</strong>
          </div>
        ))}
      </div>
      <div className="empty-state">Run and trace query endpoints are the next backend slice.</div>
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
      return 'Sources'
    case 'transform':
      return 'Transform Pipeline'
    case 'delivery':
      return 'Delivery Pipeline'
    case 'runs':
      return 'Runs and Traces'
  }
}

export default App
