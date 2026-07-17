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

export type ExcelMatchModelField = {
  name: string
  modelField: string
  columnName: string
  dataType: string
  description: string
  mapping: string
  nullable: boolean
}

export type ExcelMatchModel = {
  name: string
  modelName: string
  tableName: string
  description: string
  mapping: string
  fields: ExcelMatchModelField[]
}

export type ExcelCatalogSelectOption = {
  value: string
  label: string
  currentOnly?: boolean
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

export function excelModelSelectOptions(models: ExcelMatchModel[], currentValue: string): ExcelCatalogSelectOption[] {
  const options: ExcelCatalogSelectOption[] = models.map((model) => ({
    value: model.tableName,
    label: `${model.name}（${model.modelName} → ${model.tableName}）`,
  }))
  const current = currentValue.trim()
  if (current && !models.some((model) => model.tableName === current)) {
    options.unshift({
      value: current,
      label: `当前配置：${current}（目录中不存在）`,
      currentOnly: true,
    })
  }
  return options
}

export function excelFieldSelectOptions(fields: ExcelMatchModelField[], currentValue: string): ExcelCatalogSelectOption[] {
  const options: ExcelCatalogSelectOption[] = fields.map((field) => ({
    value: field.columnName,
    label: `${field.name}（${field.modelField} → ${field.columnName}）`,
  }))
  const current = currentValue.trim()
  if (current && !fields.some((field) => field.columnName === current)) {
    options.unshift({
      value: current,
      label: `当前配置：${current}（模型中不存在）`,
      currentOnly: true,
    })
  }
  return options
}

export function selectExcelMatchStepModel(step: ExcelMatchStepConfig, tableName: string, models: ExcelMatchModel[]): ExcelMatchStepConfig {
  const model = models.find((item) => item.tableName === tableName)
  if (!model) {
    return { ...step, tableName }
  }
  const columns = new Set(model.fields.map((field) => field.columnName))
  return {
    ...step,
    tableName,
    dbMatchField: columns.has(step.dbMatchField) ? step.dbMatchField : '',
    dbValueField: columns.has(step.dbValueField) ? step.dbValueField : '',
  }
}

export function excelJobDetailHash(jobID: number) {
  const normalizedID = Math.trunc(jobID)
  return Number.isSafeInteger(normalizedID) && normalizedID > 0
    ? `excel_jobs?job=${normalizedID}`
    : 'excel_jobs'
}

export function excelJobIDFromHash(hash: string) {
  const normalizedHash = hash.replace(/^#\/?/, '')
  const [route, query = ''] = normalizedHash.split('?', 2)
  if (route !== 'excel_jobs') return null
  const jobID = Number(new URLSearchParams(query).get('job'))
  return Number.isSafeInteger(jobID) && jobID > 0 ? jobID : null
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
