import type { ReportParameter } from './types'

export type ReportParameterFlag = 'systemInjected' | 'sensitive'

type ReportParameterLogicalType = ReportParameter['logicalType']

export function reportParameterControls(logicalType: ReportParameterLogicalType): ReportParameter['controlType'][] {
  switch (logicalType) {
    case 'string': return ['TEXT', 'TEXTAREA']
    case 'integer':
    case 'decimal': return ['NUMBER']
    case 'boolean': return ['CHECKBOX']
    case 'date': return ['DATE']
    case 'datetime': return ['DATETIME']
    case 'enum': return ['SELECT']
    case 'multi_enum': return ['MULTI_SELECT']
    case 'json': return ['TEXTAREA']
  }
}

export function updateReportParameterLogicalType(parameter: ReportParameter, logicalType: ReportParameterLogicalType): ReportParameter {
  const source = parameter.valueSource.source
  const compatibleSource = (source === 'RUN_ID' && logicalType === 'string') || (source === 'ACTOR_ID' && logicalType === 'integer')
  const supportsNormalizer = logicalType === 'string' || logicalType === 'enum' || logicalType === 'multi_enum'
  const controls = reportParameterControls(logicalType)
  return {
    ...parameter,
    logicalType,
    controlType: controls.includes(parameter.controlType) ? parameter.controlType : controls[0],
    cardinality: logicalType === 'multi_enum' ? 'MULTIPLE' : 'SINGLE',
    collectionEncoding: logicalType === 'multi_enum' ? 'JSON_CLOB' : '',
    normalizer: supportsNormalizer ? parameter.normalizer : {},
    valueSource: compatibleSource ? parameter.valueSource : {},
  }
}

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
