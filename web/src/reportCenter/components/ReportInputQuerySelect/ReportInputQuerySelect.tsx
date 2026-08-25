import { useEffect, useRef, useState } from 'react'
import Select from 'antd/es/select'
import { getReportInputOptions, type ReportCenterClient } from '../../api'
import { mergeReportInputOptions } from '../../inputOptions'
import type { ReportInputOption } from '../../types'
import styles from './ReportInputQuerySelect.module.css'

export function ReportInputQuerySelect({ client, reportId, conditionCode, inputId, value, required, disabled, onChange }: {
	client: ReportCenterClient
	reportId: number
	conditionCode: string
	inputId: string
	value: unknown
	required: boolean
	disabled: boolean
	onChange: (value: string) => void
}) {
	const [search, setSearch] = useState('')
	const [options, setOptions] = useState<ReportInputOption[]>([])
	const [state, setState] = useState({ loading: false, error: '' })
	const selected = useRef(new Map<string, ReportInputOption>())
	const selectedKey = value === undefined || value === null || value === '' ? undefined : String(value)

	useEffect(() => {
		selected.current.clear()
		setSearch('')
		setOptions([])
		setState({ loading: false, error: '' })
	}, [reportId, conditionCode])

	useEffect(() => {
		const current = selectedKey ? options.find((option) => option.id === selectedKey) : undefined
		if (current) selected.current.set(current.id, current)
	}, [options, selectedKey])

	useEffect(() => {
		const controller = new AbortController()
		const timer = window.setTimeout(() => {
			setState({ loading: true, error: '' })
			void getReportInputOptions(client, reportId, conditionCode, search, controller.signal).then((response) => {
				if (controller.signal.aborted) return
				if (!response.ok) {
					setState({ loading: false, error: response.error })
					return
				}
				const cached = [...selected.current.values()]
				setOptions(mergeReportInputOptions(cached, response.data))
				setState({ loading: false, error: '' })
			})
		}, 300)
		return () => {
			window.clearTimeout(timer)
			controller.abort()
		}
	}, [client, reportId, conditionCode, search])

	return <div className={styles.wrapper}>
		<Select
			aria-label={`搜索并选择${conditionCode}`}
			aria-required={required}
			allowClear={!required}
			className={styles.select}
			disabled={disabled}
			filterOption={false}
			id={inputId}
			loading={state.loading}
			notFoundContent={state.loading ? '正在查询…' : state.error || '没有匹配选项'}
			options={options.map((option) => ({ value: option.id, label: option.name }))}
			placeholder="输入名称精确查询"
			searchValue={search}
			showSearch
			value={selectedKey}
			onClear={() => onChange('')}
			onSearch={setSearch}
			onSelect={(key) => {
				const option = options.find((item) => item.id === key)
				if (option) selected.current.set(option.id, option)
				onChange(key)
				setSearch('')
			}}
		/>
		{state.error ? <small className={styles.error} role="status">{state.error}</small> : null}
	</div>
}
