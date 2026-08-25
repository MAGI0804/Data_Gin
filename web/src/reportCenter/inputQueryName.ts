export const reportInputQueryNamePatternSource = String.raw`\p{L}[\p{L}\p{N}_-]{0,63}`

const reportInputQueryNamePattern = new RegExp(`^(?:${reportInputQueryNamePatternSource})$`, 'u')

export function isReportInputQueryName(value: string) {
	return reportInputQueryNamePattern.test(value)
}
