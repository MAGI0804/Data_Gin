import { ListChecks, Save } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { FeedbackState, MetricStrip, PageCanvas, PageHeader, Section, StatusTag } from '../../ui'
import { buildOrderPushPolicyDraft, buildOrderPushPolicyPayload, parseOrderPushPolicyEnvelope, parseSavedOrderPushPolicy, policyDeliveryRatio, policyEnabled, type OrderPushPolicyDraft, type OrderPushTargetOption } from '../orderPushPolicyContracts'
import type { ConfigurationClient } from '../types'
import styles from './PushPolicyPage.module.css'

export function PushPolicyPage({ client, canManage, refreshVersion }: { client: ConfigurationClient; canManage: boolean; refreshVersion: number }) {
  const [options, setOptions] = useState<OrderPushTargetOption[]>([])
  const [draft, setDraft] = useState<OrderPushPolicyDraft[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [dirty, setDirty] = useState(false)
  const [remoteChanged, setRemoteChanged] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const requestRef = useRef<AbortController | null>(null)
  const discardRequestRef = useRef<AbortController | null>(null)
  const saveInFlightRef = useRef(false)
  const dirtyRef = useRef(false)

  useEffect(() => { dirtyRef.current = dirty }, [dirty])
  useEffect(() => () => discardRequestRef.current?.abort(), [])
  useEffect(() => {
    requestRef.current?.abort()
    const controller = new AbortController()
    requestRef.current = controller
    async function load() {
      setLoading(true); setError('')
      try {
        const response = await client('/v1/order-push-skip-config', { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true })
        if (controller.signal.aborted) return
        const parsed = response.ok ? parseOrderPushPolicyEnvelope(response.data) : null
        if (!parsed) { setError(response.error?.message || '订单少推送策略暂时不可用。'); return }
        if (dirtyRef.current) { setRemoteChanged(true); setNotice('服务端策略或目标列表已变化，当前草稿已保留但不能继续保存；请点击“放弃修改”同步最新配置。'); return }
        setOptions(parsed.options); setDraft(buildOrderPushPolicyDraft(parsed.config, parsed.options)); setDirty(false); setRemoteChanged(false)
      } catch { if (!controller.signal.aborted) setError('订单少推送策略暂时不可用。') } finally { if (!controller.signal.aborted) setLoading(false) }
    }
    void load()
    return () => controller.abort()
  }, [client, refreshVersion])

  async function reloadDiscardingDraft() {
    if (saving) return
    discardRequestRef.current?.abort()
    const controller = new AbortController()
    discardRequestRef.current = controller
    setDirty(false); dirtyRef.current = false; setRemoteChanged(false); setNotice(''); setError(''); setLoading(true)
    try {
      const response = await client('/v1/order-push-skip-config', { method: 'GET', signal: controller.signal, showResult: false, silentLoading: true })
      if (controller.signal.aborted || discardRequestRef.current !== controller) return
      const parsed = response.ok ? parseOrderPushPolicyEnvelope(response.data) : null
      if (!parsed) { setError(response.error?.message || '订单少推送策略暂时不可用。'); return }
      setOptions(parsed.options); setDraft(buildOrderPushPolicyDraft(parsed.config, parsed.options)); setRemoteChanged(false)
    } catch { if (!controller.signal.aborted) setError('订单少推送策略暂时不可用。') } finally { if (discardRequestRef.current === controller) { discardRequestRef.current = null; setLoading(false) } }
  }

  async function save() {
    if (!canManage || saving || saveInFlightRef.current || remoteChanged) return
    const validation = buildOrderPushPolicyPayload(draft, options)
    if (!validation.ok) { setError(validation.error); return }
    requestRef.current?.abort(); requestRef.current = null
    saveInFlightRef.current = true; setSaving(true); setError(''); setNotice('')
    try {
      const response = await client('/v1/order-push-skip-config', { method: 'PUT', body: validation.payload, showResult: false, silentLoading: true })
      const saved = response.ok ? parseSavedOrderPushPolicy(response.data) : null
      if (!saved) { setError(response.error?.message || '订单少推送策略保存未完成。'); return }
      setDraft(buildOrderPushPolicyDraft(saved, options)); dirtyRef.current = false; setDirty(false); setRemoteChanged(false); setNotice('订单少推送策略已保存。')
    } catch { setError('订单少推送策略保存未完成。') } finally { saveInFlightRef.current = false; setSaving(false) }
  }

  function update(index: number, key: 'cycle' | 'skip', value: string) {
    if (!canManage || saving || !/^\d*$/.test(value)) return
    setDraft((current) => current.map((item, itemIndex) => itemIndex === index ? { ...item, [key]: value } : item))
    dirtyRef.current = true; setDirty(true); setNotice('')
  }

  const enabledCount = draft.filter(policyEnabled).length
  const pageBusy = loading || saving
  return <PageCanvas>
    <PageHeader eyebrow="DELIVERY POLICY" title="推送策略" description="按真实推送目标配置周期少推规则；任务自身的策略 JSON 优先于这里的全局目标策略。" actions={canManage ? <button className={styles.primary} type="button" disabled={saving || loading || !dirty || remoteChanged} onClick={() => void save()}><Save aria-hidden="true" />{saving ? '保存中…' : remoteChanged ? '请先同步' : '保存策略'}</button> : <StatusTag tone="neutral">只读权限</StatusTag>} />
    {notice ? <p className={styles.notice} role="status" aria-live="polite">{notice}</p> : null}
    <MetricStrip label="少推策略概览" items={[{ key: 'targets', label: '可配置目标', value: options.length }, { key: 'enabled', label: '已启用少推', value: enabledCount }, { key: 'scope', label: '计数范围', value: '单次任务', detail: '每次最多 100 条并重新计数' }]} />
    {error ? <FeedbackState kind="error" title="推送策略提示" description={error} /> : null}
    <Section title="订单少推配置" description="每个周期跳过最后 N 单；0 / 0 表示不启用。保存会以当前目标列表完整覆盖全局策略。" actions={dirty ? <button type="button" disabled={saving || loading} onClick={() => void reloadDiscardingDraft()}>放弃修改</button> : undefined}>
      {loading && draft.length === 0 ? <FeedbackState kind="loading" title="正在加载推送策略" /> : options.length === 0 ? <FeedbackState kind="empty" title="暂无可配置推送目标" /> : <fieldset className={styles.policyList} disabled={!canManage || pageBusy}>{draft.map((item, index) => <div className={styles.policyRow} key={item.code}><div className={styles.identity}><ListChecks aria-hidden="true" /><span><strong>{item.name}</strong><code>{item.code}</code></span></div><label>循环总单数<input inputMode="numeric" value={item.cycle} onChange={(event) => update(index, 'cycle', event.currentTarget.value)} /></label><label>少推单数<input inputMode="numeric" value={item.skip} onChange={(event) => update(index, 'skip', event.currentTarget.value)} /></label><div className={styles.ratio}><StatusTag tone={policyEnabled(item) ? 'warning' : 'neutral'}>{policyEnabled(item) ? '已启用' : '不启用'}</StatusTag><span>预计推送 {policyDeliveryRatio(item)}</span></div></div>)}</fieldset>}
      <p className={styles.contractNote}>规则只对匹配目标编码的数据生效；每次任务运行从第 1 条重新计数，任务策略 JSON 中的少推配置优先。</p>
    </Section>
  </PageCanvas>
}
