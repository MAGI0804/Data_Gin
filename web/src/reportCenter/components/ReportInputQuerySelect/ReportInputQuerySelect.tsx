import { useEffect, useMemo, useRef, useState } from 'react'
import Select from 'antd/es/select'
import { RefreshCw } from 'lucide-react'
import { getReportInputOptions, type ReportCenterClient } from '../../api'
import { mergeReportInputOptions, reportInputOptionMatches, reportInputSelectionKeys, reportInputSelectionValue } from '../../inputOptions'
import type { ReportInputOption } from '../../types'
import styles from './ReportInputQuerySelect.module.css'

export function ReportInputQuerySelect({ client, reportId, conditionCode, inputId, value, required, disabled, multiple, numeric, onChange }: {
	client: ReportCenterClient
	reportId: number
	conditionCode: string
	inputId: string
	value: unknown
	required: boolean
	disabled: boolean
	multiple: boolean
	numeric: boolean
	onChange: (value: unknown) => void
}) {
	const [options, setOptions] = useState<ReportInputOption[]>([])
	const [state, setState] = useState({ loading: false, error: '' })
	const requestAbort = useRef<AbortController | null>(null)
	const selectedKeys = useMemo(
		() => reportInputSelectionKeys(value, multiple),
		[value, multiple],
	)

	useEffect(() => {
		requestAbort.current?.abort()
		const controller = new AbortController()
		requestAbort.current = controller
		setOptions([])
		setState({ loading: true, error: '' })
		void getReportInputOptions(client, reportId, conditionCode, controller.signal).then((response) => {
			if (controller.signal.aborted) return
			if (!response.ok) {
				setState({ loading: false, error: response.error })
				return
			}
			const normalized = mergeReportInputOptions([], response.data)
			setOptions(normalized)
			setState({ loading: false, error: '' })
		})
		return () => requestAbort.current?.abort()
	}, [client, conditionCode, reportId])

	async function refreshOptions() {
		requestAbort.current?.abort()
		const controller = new AbortController()
		requestAbort.current = controller
		setState({ loading: true, error: '' })
		const response = await getReportInputOptions(client, reportId, conditionCode, controller.signal)
		if (controller.signal.aborted) return
		if (!response.ok) {
			setState({ loading: false, error: options.length ? `${response.error} 当前继续使用上次缓存。` : response.error })
			return
		}
		const normalized = mergeReportInputOptions([], response.data)
		setOptions(normalized)
		setState({ loading: false, error: '' })
	}

	return <div className={styles.wrapper}>
		<div className={styles.controlRow}><Select
			aria-label={`搜索并选择${conditionCode}`}
			aria-required={required}
			allowClear={!required}
			className={styles.select}
			disabled={disabled}
			filterOption={(search, option) => reportInputOptionMatches({ id: String(option?.value ?? ''), name: String(option?.label ?? '') }, search)}
			id={inputId}
			loading={state.loading}
			mode={multiple ? 'multiple' : undefined}
			notFoundContent={state.loading ? '正在查询…' : state.error || '没有匹配选项'}
			options={options.map((option) => ({ value: option.id, label: option.name }))}
			placeholder="输入名称或 ID 模糊搜索"
			showSearch
			value={multiple ? selectedKeys : selectedKeys[0]}
			onChange={(next) => onChange(reportInputSelectionValue(next, multiple, numeric))}
		/>
		<button className={styles.refreshButton} type="button" aria-label={`刷新${conditionCode}本地选项`} disabled={disabled || state.loading} onClick={() => void refreshOptions()}><RefreshCw aria-hidden="true" /></button></div>
		<small className={state.error ? styles.cacheWarning : styles.cacheStatus}>{state.loading ? '正在执行查询语句…' : state.error ? options.length ? `当前页面保留 ${options.length} 条可用选项` : '当前页面暂无可用选项' : options.length >= 500 ? '当前页面已缓存前 500 条，可直接模糊搜索' : `当前页面已缓存 ${options.length} 条，可直接模糊搜索`}</small>
		{state.error ? <small className={styles.error} role="status">{state.error}</small> : null}
	</div>
}
