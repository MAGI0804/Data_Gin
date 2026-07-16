export type ExcelMatchFilterConfig = {
  column: string
  op: string
  value: string
}

export type ExcelMatchStepConfig = {
  name: string
  filters: ExcelMatchFilterConfig[]
  tableName: string
  matchExcelColumn: string
  dbMatchField: string
  dbValueField: string
  outputColumnName: string
}

type ExcelMatchSchemeSource = {
  filters?: Array<Partial<ExcelMatchFilterConfig>>
  matchExcelColumn?: string
  dbMatchField?: string
  dbValueField?: string
  tableName?: string
  outputColumnName?: string
  steps?: Array<Partial<ExcelMatchStepConfig>>
}

type ExcelExportConfigInput = {
  sheetName: string
  steps: ExcelMatchStepConfig[]
  exportColumnFormats: Array<{ column: string; format: string }>
  batchSize: number
}

function cloneFilters(filters: Array<Partial<ExcelMatchFilterConfig>> | undefined) {
  if (!Array.isArray(filters)) return []
  return filters.map((filter) => ({
    column: filter.column ?? '',
    op: filter.op ?? 'eq',
    value: filter.value ?? '',
  }))
}

export function cloneExcelMatchSteps(steps: ExcelMatchStepConfig[]) {
  return steps.map((step) => ({
    ...step,
    filters: cloneFilters(step.filters),
  }))
}

export function migrateExcelMatchSteps(config: ExcelMatchSchemeSource, fallbackStep: ExcelMatchStepConfig) {
  const configuredSteps = Array.isArray(config.steps) && config.steps.length > 0
    ? config.steps.map((step, index): ExcelMatchStepConfig => ({
        name: step.name ?? `步骤 ${index + 1}`,
        filters: cloneFilters(step.filters),
        tableName: step.tableName ?? '',
        matchExcelColumn: step.matchExcelColumn ?? '',
        dbMatchField: step.dbMatchField ?? '',
        dbValueField: step.dbValueField ?? '',
        outputColumnName: step.outputColumnName ?? '',
      }))
    : [{
        name: '兼容旧方案',
        filters: [],
        tableName: config.tableName || fallbackStep.tableName,
        matchExcelColumn: config.matchExcelColumn || fallbackStep.matchExcelColumn,
        dbMatchField: config.dbMatchField || fallbackStep.dbMatchField,
        dbValueField: config.dbValueField || fallbackStep.dbValueField,
        outputColumnName: config.outputColumnName || fallbackStep.outputColumnName,
      }]
  const legacyFilters = cloneFilters(config.filters)
  if (legacyFilters.length > 0) {
    configuredSteps[0].filters = [...legacyFilters, ...configuredSteps[0].filters]
  }
  return configuredSteps
}

export function buildExcelExportConfig(input: ExcelExportConfigInput) {
  return {
    operation: 'export_match' as const,
    sheetName: input.sheetName.trim() || 'Sheet1',
    steps: input.steps.map((step) => ({
      name: step.name.trim(),
      filters: step.filters
        .map((filter) => {
          const op = filter.op.trim().toLowerCase() || 'eq'
          return {
            column: filter.column.trim(),
            op,
            value: op === 'empty' || op === 'not_empty' ? '' : filter.value.trim(),
          }
        })
        .filter((filter) => filter.column),
      tableName: step.tableName.trim(),
      matchExcelColumn: step.matchExcelColumn.trim(),
      dbMatchField: step.dbMatchField.trim(),
      dbValueField: step.dbValueField.trim(),
      outputColumnName: step.outputColumnName.trim(),
    })),
    exportColumnFormats: input.exportColumnFormats,
    batchSize: input.batchSize,
  }
}
