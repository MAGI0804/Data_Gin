import { Play, X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { parsePipelineRunResult, pipelineRunPath, type PipelineRunResult } from './pipelineRun'
import { runSingleFlight } from './singleFlight'
import { Dialog } from './ui'
import styles from './PipelineRunPanel.module.css'

export type Pipeline = { id: number; name: string; code: string; enabled: boolean }
type ApiResult = { ok: boolean; status: number; data: unknown }
type ApiClient = (path: string, options: { method: 'POST'; showResult: false; silentLoading: true; signal?: AbortSignal }) => Promise<ApiResult>

export function PipelineRunPanel({ pipelines, client, onRunCompleted }: { pipelines: Pipeline[]; client: ApiClient; onRunCompleted: () => void }) {
  const enabledPipelines = pipelines.filter((pipeline) => pipeline.enabled)
  const [selectedID, setSelectedID] = useState(0)
  const [confirming, setConfirming] = useState(false)
  const [running, setRunning] = useState(false)
  const [error, setError] = useState('')
  const [result, setResult] = useState<PipelineRunResult | null>(null)
  const controllerRef = useRef<AbortController | null>(null)
  const runInFlightRef = useRef(false)
  const confirmRef = useRef<HTMLButtonElement>(null)
  const openerRef = useRef<HTMLElement | null>(null)
  const selected = enabledPipelines.find((pipeline) => pipeline.id === selectedID) ?? enabledPipelines[0]

  useEffect(() => () => controllerRef.current?.abort(), [])

  function openConfirm() {
    if (!selected || running) return
    openerRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null
    setError('')
    setResult(null)
    setConfirming(true)
  }

  function closeConfirm() {
    if (running) return
    setConfirming(false)
  }

  async function run() {
    if (!selected) return
    const runPromise = runSingleFlight(runInFlightRef, async () => {
      const controller = new AbortController()
      controllerRef.current = controller
      setRunning(true)
      setError('')
      try {
        const response = await client(pipelineRunPath(selected.id), { method: 'POST', showResult: false, silentLoading: true, signal: controller.signal })
        if (controller.signal.aborted) return
        if (!response.ok) {
          setError(runError(response.status))
          return
        }
        const next = parsePipelineRunResult(response.data)
        if (!next) {
          setError('流水线运行结果格式不正确，请刷新运行记录确认状态。')
          return
        }
        setResult(next)
        setConfirming(false)
        onRunCompleted()
      } catch {
        if (!controller.signal.aborted) setError('流水线运行请求未完成，请刷新运行记录确认状态。')
      } finally {
        if (controllerRef.current === controller) {
          controllerRef.current = null
          if (!controller.signal.aborted) setRunning(false)
        }
      }
    })
    if (!runPromise) return
    await runPromise
  }

  return <section className={styles.panel} aria-busy={running}>
    <div className={styles.heading}><strong>手动执行流水线</strong><span>仅可执行已启用的流水线；执行前需要确认，提交不会自动重试。</span></div>
    {enabledPipelines.length === 0 ? <p role="status">暂无可执行的已启用流水线。</p> : <div className={styles.controls}><label><span>流水线</span><select value={selected?.id ?? ''} onChange={(event) => setSelectedID(Number(event.currentTarget.value))} disabled={running}>{enabledPipelines.map((pipeline) => <option key={pipeline.id} value={pipeline.id}>{pipeline.name} · {pipeline.code}</option>)}</select></label><button className={styles.primary} type="button" onClick={openConfirm} disabled={running}><Play aria-hidden="true" />执行</button></div>}
    {result && <p className={styles.result} role="status" aria-live="polite">已创建运行 #{result.runID} · Trace ID {result.traceID} · 成功 {result.successCount} · 失败 {result.failedCount}</p>}
    {error && !confirming ? <p className={`${styles.result} ${styles.error}`} role="alert">{error}</p> : null}
    <Dialog open={confirming && Boolean(selected)} title="确认执行流水线" description="提交后不会自动重试。" closeDisabled={running} returnFocus={openerRef.current} initialFocusRef={confirmRef} onClose={closeConfirm} footer={<div className={styles.dialogFooter}><button type="button" onClick={closeConfirm} disabled={running}><X aria-hidden="true" />取消</button><button ref={confirmRef} className={styles.primary} type="button" onClick={() => void run()} disabled={running}><Play aria-hidden="true" />{running ? '执行中' : '确认执行'}</button></div>}><p className={styles.dialogCopy}>将立即执行“{selected?.name}”（{selected?.code}）的已启用步骤。</p>{error ? <p className={`${styles.result} ${styles.error}`} role="alert">{error}</p> : null}</Dialog>
  </section>
}

function runError(status: number) {
  if (status === 0) return '无法连接服务，请检查网络后重试。'
  if (status === 401) return '登录已失效，请重新登录。'
  if (status === 403) return '当前账号无权执行流水线。'
  if (status === 404) return '所选流水线不存在，请刷新后重试。'
  if (status === 409) return '流水线状态已变化，请刷新后重试。'
  if (status >= 500) return '流水线运行失败，请刷新运行记录确认状态。'
  return `流水线运行失败（HTTP ${status}）。`
}
