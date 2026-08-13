import type { ReportParameter } from './types'

export type ReportParameterFlag = 'systemInjected' | 'sensitive'

export function reportParameterFlagDisabled(parameter: ReportParameter, flag: ReportParameterFlag) {
  if (flag === 'systemInjected') return parameter.sensitive && !parameter.systemInjected
  return parameter.systemInjected && !parameter.sensitive
}

export function updateReportParameterFlag(parameter: ReportParameter, flag: ReportParameterFlag, checked: boolean): ReportParameter {
  if (flag === 'systemInjected') {
    return {
      ...parameter,
      systemInjected: checked,
      normalizer: checked ? {} : parameter.normalizer,
      valueSource: checked ? parameter.valueSource : {},
    }
  }
  return {
    ...parameter,
    sensitive: checked,
    defaultValue: checked ? undefined : parameter.defaultValue,
  }
}
