import type { ReportInputOption } from './types'

export function mergeReportInputOptions(selected: ReportInputOption[], latest: ReportInputOption[]): ReportInputOption[] {
	const merged = new Map<string, ReportInputOption>()
	for (const option of [...selected, ...latest]) merged.set(option.id, option)
	return [...merged.values()]
}
