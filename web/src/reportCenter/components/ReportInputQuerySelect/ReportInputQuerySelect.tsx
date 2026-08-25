import { useEffect, useMemo, useRef, useState } from 'react'
import Select from 'antd/es/select'
import { getReportInputOptions, type ReportCenterClient } from '../../api'
import { mergeReportInputOptions, reportInputSelectionKeys, reportInputSelectionValue } from '../../inputOptions'
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
	const [search, setSearch] = useState('')
	const [options, setOptions] = useState<ReportInputOption[]>([])
	const [state, setState] = useState({ loading: false, error: '' })
	const selected = useRef(new Map<string, ReportInputOption>())
	const selectedKeys = useMemo(
		() => reportInputSelectionKeys(value, multiple),
		[value, multiple],
	)

	useEffect(() => {
		selected.current.clear()
		setSearch('')
		setOptions([])
		setState({ loading: false, error: '' })
	}, [reportId, conditionCode])

	useEffect(() => {
		const active = new Set(selectedKeys)
		for (const key of selected.current.keys()) if (!active.has(key)) selected.current.delete(key)
		for (const key of selectedKeys) {
			const current = options.find((option) => option.id === key)
			if (current) selected.current.set(current.id, current)
		}
	}, [options, selectedKeys])

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
			mode={multiple ? 'multiple' : undefined}
			notFoundContent={state.loading ? '正在查询…' : state.error || '没有匹配选项'}
			options={options.map((option) => ({ value: option.id, label: option.name }))}
			placeholder="输入名称精确查询"
			searchValue={search}
			showSearch
			value={multiple ? selectedKeys : selectedKeys[0]}
			onChange={(next) => onChange(reportInputSelectionValue(next, multiple, numeric))}
			onSearch={setSearch}
			onSelect={(key) => {
				const option = options.find((item) => item.id === key)
				if (option) selected.current.set(option.id, option)
				setSearch('')
			}}
		/>
		{state.error ? <small className={styles.error} role="status">{state.error}</small> : null}
	</div>
}
