import type { ReportInputOption } from './types'

export function mergeReportInputOptions(selected: ReportInputOption[], latest: ReportInputOption[]): ReportInputOption[] {
	const merged = new Map<string, ReportInputOption>()
	for (const option of [...selected, ...latest]) merged.set(option.id, option)
	return [...merged.values()]
}

export function reportInputSelectionKeys(value: unknown, multiple: boolean): string[] {
	if (multiple) return Array.isArray(value) ? value.map(String) : []
	return value === undefined || value === null || value === '' ? [] : [String(value)]
}

export function reportInputSelectionValue(value: string | string[], multiple: boolean, numeric: boolean): unknown {
	if (!multiple) return Array.isArray(value) ? value[0] ?? '' : value
	const keys = Array.isArray(value) ? value : value ? [value] : []
	return numeric ? keys.map(Number) : keys
}
