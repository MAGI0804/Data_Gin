import assert from 'node:assert/strict'
import { readdirSync, readFileSync } from 'node:fs'
import test from 'node:test'
import ts from 'typescript'

import { createReportRun, deleteReportDraft, getReportAudits, getReportInputOptions, getReportInputQueries, getReportProcedureSignature, getReportProcedures, getReportResultTableSchema, getReportResultTables, parsePublication, parseReportAuditPage, parseReportCatalogPage, parseReportDatasource, parseReportDatasources, parseReportDatasourceTest, parseReportDraft, parseReportExport, parseReportExportPage, parseReportProcedurePage, parseReportProcedureSignature, parseReportResultPage, parseReportResultTablePage, parseReportResultTableSchema, parseReportRun, parseReportRunContract, parseReportVersionDiff, parseReportVersionPage, saveAndPublishReportDraft, saveReportDraft, testReportDatasourceConnection } from '../.test-dist/reportCenter/api.js'
import { applyExcelMapping, buildReportConditions, excelMappingFromColumns, initialReportConditionValues, parseExcelMappingDocument, parseReportInputSchemaDocument, parseReportInputSchemaText, reconcileReportColumnsWithResultSchema, renameExcelMappingField, reportColumnsFromResultSchema } from '../.test-dist/reportCenter/refCursorConfig.js'
import { reportParameterControls, reportParameterFlagDisabled, updateReportParameterFlag, updateReportParameterLogicalType } from '../.test-dist/reportCenter/parameterConfig.js'
import { buildNewReportRunState, canStartNewReportRun, initialReportParameterValues } from '../.test-dist/reportCenter/queryParameters.js'
import { createLatestRequestGuard } from '../.test-dist/reportCenter/components/ReportVersionDrawer/requestGuard.js'
import { normalizeDatasourceCode, validateDatasourceConnection, validateDatasourceSave } from '../.test-dist/reportCenter/datasourceValidation.js'
import { mergeReportInputOptions } from '../.test-dist/reportCenter/inputOptions.js'

test('functional state updates do not retain React DOM events', () => {
  const sourceRoot = new URL('../src/', import.meta.url)
  const violations = []
  for (const file of typescriptFiles(sourceRoot)) {
    const source = readFileSync(file, 'utf8')
    const sourceFile = ts.createSourceFile(file.pathname, source, ts.ScriptTarget.Latest, true, file.pathname.endsWith('.tsx') ? ts.ScriptKind.TSX : ts.ScriptKind.TS)
    inspect(sourceFile)

    function inspect(node) {
      if (ts.isCallExpression(node) && ts.isIdentifier(node.expression) && /^set[A-Z]/.test(node.expression.text)) {
        const updater = node.arguments[0]
        if (updater && (ts.isArrowFunction(updater) || ts.isFunctionExpression(updater))) inspectUpdater(updater.body)
      }
      ts.forEachChild(node, inspect)
    }

    function inspectUpdater(node) {
      if (ts.isPropertyAccessExpression(node) && ['currentTarget', 'target', 'nativeEvent'].includes(node.name.text)) {
        const position = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile))
        violations.push(`${file.pathname}:${position.line + 1}:${position.character + 1}`)
      }
      ts.forEachChild(node, inspectUpdater)
    }
  }
  assert.deepEqual(violations, [])
})

test('drawer keeps long parsed configuration inside its scrollable body', () => {
  const source = readFileSync(new URL('../src/ui/Drawer/Drawer.module.css', import.meta.url), 'utf8')
  const layerRule = source.match(/\.layer\s*\{([^}]*)\}/)?.[1] ?? ''
  const drawerRule = source.match(/\.drawer\s*\{([^}]*)\}/)?.[1] ?? ''
  const bodyRule = source.match(/\.body\s*\{([^}]*)\}/)?.[1] ?? ''
  assert.match(layerRule, /grid-template-rows:\s*minmax\(0,\s*1fr\)\s*;/)
  assert.match(layerRule, /overflow:\s*hidden\s*;/)
  assert.match(drawerRule, /height:\s*100%\s*;/)
  assert.match(drawerRule, /min-height:\s*0\s*;/)
  assert.match(bodyRule, /min-height:\s*0\s*;/)
  assert.match(bodyRule, /overflow-y:\s*auto\s*;/)
})

test('condition editor keeps date format out of a narrow action column', () => {
  const source = readFileSync(new URL('../src/reportCenter/components/ReportConfigDrawer/ReportInputSchemaEditor.module.css', import.meta.url), 'utf8')
  const rowRule = source.match(/\.row\s*\{([^}]*)\}/)?.[1] ?? ''
  const formatRule = source.match(/\.formatField\s*\{([^}]*)\}/)?.[1] ?? ''
  assert.match(rowRule, /grid-template-columns:\s*repeat\(4,\s*minmax\(130px,\s*1fr\)\)\s*;/)
  assert.doesNotMatch(rowRule, /38px/)
  assert.match(formatRule, /grid-column:\s*span 2\s*;/)
  assert.match(formatRule, /border-inline-start:\s*2px solid var\(--color-brand\)\s*;/)
})

test('report configuration locks its editor while save and publication are in flight', () => {
  const source = readFileSync(new URL('../src/reportCenter/components/ReportConfigDrawer/ReportConfigDrawer.tsx', import.meta.url), 'utf8')
  assert.match(source, /<fieldset[^>]*disabled=\{state\.saving\}[^>]*aria-busy=\{state\.saving \|\| undefined\}/)
})

function typescriptFiles(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const file = new URL(entry.name, directory)
    if (entry.isDirectory()) return typescriptFiles(new URL(`${entry.name}/`, directory))
    return /\.tsx?$/.test(entry.name) ? [file] : []
  })
}

test('parseReportCatalogPage reads the standard API envelope', () => {
  const page = parseReportCatalogPage({
    code: 200,
    msg: 'success',
    data: {
      items: [{
        id: 12,
        code: 'sales_report',
        name: '销售报表',
        category: '经营',
        datasourceId: 3,
        status: 'ACTIVE',
        lockVersion: 4,
        isOwner: false,
        updatedAt: '2026-08-12T10:00:00Z',
      }],
      hasMore: true,
      nextAfterId: 12,
    },
  })

  assert.equal(page.items.length, 1)
  assert.equal(page.items[0].name, '销售报表')
  assert.equal(page.items[0].status, 'ACTIVE')
  assert.equal(page.items[0].lockVersion, 4)
  assert.equal(page.items[0].isOwner, false)
  assert.equal(page.hasMore, true)
  assert.equal(page.nextAfterId, 12)
})

test('input schema rejects payloads beyond the backend 64 KiB boundary', () => {
  assert.throws(
    () => parseReportInputSchemaDocument({ condition1: { type: 'VARCHAR2', displayName: '条件', example: '测'.repeat(22_000) } }),
    /64 KiB/,
  )
})

test('input schema keeps a configured query binding on scalar selectors', () => {
	const schema = parseReportInputSchemaDocument({ store: { type: 'str', displayName: '门店', control: 'SELECT', queryName: 'stores' } })
	assert.equal(schema.store.queryName, 'stores')
	assert.throws(() => parseReportInputSchemaDocument({ stores: { type: 'list[str]', displayName: '门店', control: 'SELECT', queryName: 'stores' } }), /queryName/)
	assert.throws(() => parseReportInputSchemaDocument({ store: { type: 'str', displayName: '门店', control: 'TEXT', queryName: 'stores' } }), /queryName/)
	assert.throws(() => parseReportInputSchemaDocument({ store: { type: 'str', displayName: '门店', control: 'SELECT', queryName: 'stores', allowedValues: ['S001'] } }), /allowedValues/)
})

test('report input query clients use configured names and exact-name search', async () => {
	const requests = []
	const client = async (path, options) => {
		requests.push({ path, options })
		if (path === '/v1/report-input-queries') return { ok: true, data: { data: { items: ['stores'] } } }
		return { ok: true, data: { data: { items: [{ id: 'S001', name: '上海店' }] } } }
	}
	assert.deepEqual(await getReportInputQueries(client), { ok: true, data: ['stores'] })
	assert.deepEqual(await getReportInputOptions(client, 9, 'store_id', '上海店'), { ok: true, data: [{ id: 'S001', name: '上海店' }] })
	assert.equal(requests[1].path, '/v1/reports/9/input-options/store_id?name=%E4%B8%8A%E6%B5%B7%E5%BA%97')
	assert.equal(requests[1].options.method, 'GET')
})

test('remote input options retain the selected item across later searches', () => {
	const selected = [{ id: 'S001', name: '上海店' }]
	const nextSearch = [{ id: 'B001', name: '北京店' }]
	assert.deepEqual(mergeReportInputOptions(selected, nextSearch), [...selected, ...nextSearch])
})

test('parseReportCatalogPage drops malformed rows and normalizes unsafe fields', () => {
  const page = parseReportCatalogPage({
    data: {
      items: [
        { id: 0, code: 'invalid', name: '无效报表' },
        { id: 9, code: 'inventory', name: '库存报表', status: 'UNKNOWN', datasourceId: '4' },
      ],
      nextAfterId: '9',
    },
  })

  assert.equal(page.items.length, 1)
  assert.equal(page.items[0].status, 'DRAFT')
  assert.equal(page.items[0].datasourceId, 4)
  assert.equal(page.items[0].isOwner, false)
  assert.equal(page.nextAfterId, 9)
})

test('parseReportCatalogPage rejects an incomplete or mismatched continuation cursor', () => {
  assert.throws(() => parseReportCatalogPage({ data: { items: [], hasMore: true, nextAfterId: 9 } }))
  assert.throws(() => parseReportCatalogPage({ data: { items: [{ id: 9, code: 'sales', name: '销售' }], hasMore: true, nextAfterId: 10 } }))
})

test('deleteReportDraft sends an optimistic DELETE and validates the deleted id', async () => {
  const requests = []
  const client = async (path, options) => {
    requests.push({ path, options })
    return { ok: true, data: { data: { id: 9 } } }
  }
  assert.deepEqual(await deleteReportDraft(client, 9, 4), { ok: true, data: { id: 9 } })
  assert.equal(requests[0].path, '/v1/reports/9?expectedLockVersion=4')
  assert.equal(requests[0].options.method, 'DELETE')

  const malformed = await deleteReportDraft(async () => ({ ok: true, data: { data: { id: 0 } } }), 9, 4)
  assert.deepEqual(malformed, { ok: false, error: '服务返回的数据格式不完整，请稍后重试。' })
})

test('parseReportRunContract keeps typed published parameters', () => {
  const contract = parseReportRunContract({ data: {
    definitionId: 9,
    versionId: 23,
    code: 'sales_report',
    name: '销售报表',
    parameters: [
      { code: 'runId', label: '运行编号', systemInjected: true, displayOrder: 1, position: 1, oracleType: 'VARCHAR2' },
      { code: 'storeCode', label: '门店', controlType: 'SELECT', logicalType: 'enum', required: true, allowedValues: ['S001', 'S002'], displayOrder: 2, position: 2, oracleType: 'VARCHAR2' },
    ],
  } })
  assert.equal(contract.versionId, 23)
  assert.equal(contract.executionMode, 'TABLE_SNAPSHOT')
  assert.equal(contract.jsonInput, false)
  assert.equal(contract.parameters[1].controlType, 'SELECT')
  assert.deepEqual(contract.parameters[1].allowedValues, ['S001', 'S002'])
})

test('parseReportRunContract exposes REF CURSOR condition schema for the query form', () => {
  const contract = parseReportRunContract({ data: {
    definitionId: 9, versionId: 24, code: 'sales_report', name: '销售报表', executionMode: 'REF_CURSOR', parameters: [],
    inputSchema: {
      store_id: { type: 'VARCHAR2', displayName: '门店', required: true, allowedValues: ['S001', 'S002'] },
      product_ids: { type: 'VARCHAR2', displayName: '商品', multiple: true, default: ['P001'] },
    },
  } })
  assert.equal(contract.executionMode, 'REF_CURSOR')
  assert.equal(contract.jsonInput, true)
  assert.equal(contract.inputSchema.store_id.displayName, '门店')
  assert.equal(contract.inputSchema.store_id.type, 'str')
  assert.equal(contract.inputSchema.product_ids.type, 'list[str]')
  assert.deepEqual(initialReportConditionValues(contract.inputSchema), { store_id: '', product_ids: ['P001'] })
  assert.deepEqual(buildReportConditions(contract.inputSchema, { store_id: 'S001', product_ids: '["P001","P002"]' }), { ok: true, conditions: { store_id: 'S001', product_ids: ['P001', 'P002'] } })
  assert.deepEqual(buildReportConditions(contract.inputSchema, { store_id: '' }), { ok: false, error: '门店 为必填筛选条件。' })
})

test('REF CURSOR numeric enum defaults keep their type for query selectors', () => {
  const schema = parseReportInputSchemaDocument({ status: { type: 'NUMBER', displayName: '状态', default: 1, allowedValues: [1, 2] } })
  assert.equal(schema.status.type, 'number')
  assert.deepEqual(initialReportConditionValues(schema), { status: 1 })
  assert.deepEqual(buildReportConditions(schema, { status: 1 }), { ok: true, conditions: { status: 1 } })
})

test('createReportRun sends conditions for JSON procedures and keeps parameters for legacy reports', async () => {
  const requests = []
  const client = async (path, options) => {
    requests.push({ path, options })
    return { ok: true, data: { data: { id: requests.length, runUuid: 'run', definitionId: 9, versionId: 24, status: 'QUEUED' } } }
  }
  await createReportRun(client, 9, { store_id: 'S001' }, 'REF_CURSOR')
  await createReportRun(client, 9, { storeCode: 'S001' }, 'TABLE_SNAPSHOT')
  await createReportRun(client, 9, { supplier_id: 'A001' }, 'TABLE_SNAPSHOT', true)
  assert.deepEqual(requests[0].options.body, { conditions: { store_id: 'S001' } })
  assert.deepEqual(requests[1].options.body, { parameters: { storeCode: 'S001' } })
  assert.deepEqual(requests[2].options.body, { conditions: { supplier_id: 'A001' } })
})

test('report parameter defaults use the same editable shapes as their controls', () => {
  const parameter = (code, logicalType, defaultValue, extra = {}) => ({
    code, logicalType, defaultValue, systemInjected: false, controlType: 'TEXT', ...extra,
  })
  const values = initialReportParameterValues([
    parameter('count', 'integer', 12),
    parameter('ratio', 'decimal', 12.5),
    parameter('options', 'json', { active: true }),
    parameter('jsonText', 'json', 'literal'),
    parameter('stores', 'multi_enum', ['S001', 2], { controlType: 'MULTI_SELECT' }),
    parameter('enabled', 'boolean', false, { controlType: 'CHECKBOX' }),
    parameter('name', 'string', ' 默认名称 '),
    parameter('emptyJson', 'json', null, { controlType: 'TEXTAREA' }),
    parameter('runId', 'string', 'ignored', { systemInjected: true }),
  ])

  assert.deepEqual(values, {
    count: '12', ratio: '12.5', options: '{"active":true}', jsonText: '"literal"', stores: ['S001', '2'],
    enabled: false, name: ' 默认名称 ', emptyJson: '',
  })
})

test('historical sensitive system parameters can be repaired without recreation', () => {
  const historical = { systemInjected: true, sensitive: true, normalizer: {}, valueSource: { source: 'RUN_ID' }, defaultValue: undefined }
  assert.equal(reportParameterFlagDisabled(historical, 'systemInjected'), false)
  assert.equal(reportParameterFlagDisabled(historical, 'sensitive'), false)
  assert.deepEqual(updateReportParameterFlag(historical, 'systemInjected', false), {
    ...historical, systemInjected: false, valueSource: {},
  })
  assert.deepEqual(updateReportParameterFlag(historical, 'sensitive', false), {
    ...historical, sensitive: false,
  })

  assert.equal(reportParameterFlagDisabled({ ...historical, systemInjected: false }, 'systemInjected'), true)
  assert.equal(reportParameterFlagDisabled({ ...historical, sensitive: false }, 'sensitive'), true)
})

test('report parameter logical types derive compatible control and collection shapes', () => {
  const parameter = {
    controlType: 'TEXTAREA', logicalType: 'string', cardinality: 'SINGLE', collectionEncoding: '',
    normalizer: { trim: true }, valueSource: { source: 'RUN_ID' },
  }
  assert.deepEqual(reportParameterControls('string'), ['TEXT', 'TEXTAREA'])
  assert.deepEqual(reportParameterControls('boolean'), ['CHECKBOX'])
  assert.deepEqual(reportParameterControls('multi_enum'), ['MULTI_SELECT'])
  assert.deepEqual(updateReportParameterLogicalType(parameter, 'multi_enum'), {
    ...parameter, logicalType: 'multi_enum', controlType: 'MULTI_SELECT', cardinality: 'MULTIPLE',
    collectionEncoding: 'JSON_CLOB', valueSource: {},
  })
  assert.deepEqual(updateReportParameterLogicalType(parameter, 'json'), {
    ...parameter, logicalType: 'json', controlType: 'TEXTAREA', cardinality: 'SINGLE',
    collectionEncoding: '', normalizer: {}, valueSource: {},
  })
})

test('new report runs are allowed only after run and export processing finish', () => {
  const runStatuses = ['QUEUED', 'RUNNING', 'CANCEL_REQUESTED', 'SUCCEEDED', 'FAILED', 'CANCELLED', 'UNKNOWN', 'RECONCILING', 'EXPORTING', 'EXPORTED', 'RESULT_PURGING', 'RESULT_PURGED', 'SUPERSEDED']
  const exportStatuses = [null, 'PENDING', 'RUNNING', 'READY', 'FAILED', 'CANCELLED', 'EXPIRED']
  const terminalRuns = new Set(['SUCCEEDED', 'FAILED', 'CANCELLED', 'EXPORTED', 'RESULT_PURGED', 'SUPERSEDED'])
  const terminalExports = new Set([null, 'READY', 'FAILED', 'CANCELLED', 'EXPIRED'])
  for (const runStatus of runStatuses) {
    for (const exportStatus of exportStatuses) {
      assert.equal(canStartNewReportRun(runStatus, exportStatus, false), terminalRuns.has(runStatus) && terminalExports.has(exportStatus), `${runStatus}/${exportStatus}`)
      assert.equal(canStartNewReportRun(runStatus, exportStatus, true), false, `${runStatus}/${exportStatus}/busy`)
    }
  }
})

test('new report run state clears the frozen snapshot and restores parameter defaults', () => {
  const state = buildNewReportRunState([{
    code: 'count', logicalType: 'integer', defaultValue: 5, systemInjected: false,
  }])
  assert.deepEqual(state, {
    values: { count: '5' }, run: null, result: null, reportExport: null,
    resultQuery: { filters: [], sort: [] }, appliedQuery: { filters: [], sort: [] },
    cursorHistory: [''], cursorIndex: 0, filtersOpen: false, parametersOpen: true,
    operation: { busy: false, error: '' },
  })
})

test('run, result and export parsers preserve cursor and large numeric strings', () => {
  const runPayload = { id: 31, runUuid: 'run-uuid', definitionId: 9, versionId: 23, status: 'SUCCEEDED', rowCount: 1, resultAvailable: true }
  const run = parseReportRun({ data: runPayload })
  assert.equal(run.status, 'SUCCEEDED')

  const page = parseReportResultPage({ data: {
    run: runPayload,
    columns: [{ fieldId: 'amount-id', code: 'amount', header: '金额', valueType: 'decimal', filterable: true, sortable: true, allowedOperators: ['EQ', 'BETWEEN'] }],
    rows: [{ key: '1', values: { amount: '9999999999999999.01' } }],
    pagination: { pageSize: 100, hasMore: true, nextCursor: 'signed.cursor' },
  } })
  assert.equal(page.rows[0].values.amount, '9999999999999999.01')
  assert.equal(page.pagination.nextCursor, 'signed.cursor')
	assert.deepEqual(page.columns[0].allowedOperators, ['EQ', 'BETWEEN'])

  const reportExport = parseReportExport({ data: { id: 41, runId: 31, exportUuid: 'export-uuid', status: 'READY', exportedRows: 1, canDownload: true } })
  assert.equal(reportExport.status, 'READY')
  assert.equal(reportExport.canDownload, true)
})

test('parseReportExportPage preserves archive and purge fields', () => {
  const page = parseReportExportPage({ data: {
    items: [{
      id: 41, runId: 31, exportUuid: 'export-uuid', reportName: '门店销售日报', status: 'READY',
      exportedRows: 120, purgedRows: 80, purgeStartedAt: '2026-08-13T09:59:00Z',
      expiresAt: '2026-08-16T10:00:00Z', canDownload: true,
    }],
    hasMore: true,
    nextAfterId: 41,
  } })

  assert.equal(page.items[0].reportName, '门店销售日报')
  assert.equal(page.items[0].purgedRows, 80)
  assert.equal(page.items[0].purgeStartedAt, '2026-08-13T09:59:00Z')
  assert.equal(page.hasMore, true)
  assert.equal(page.nextAfterId, 41)
})

test('parseReportExportPage rejects a missing cursor when more rows exist', () => {
  assert.throws(() => parseReportExportPage({ data: { items: [], hasMore: true } }))
})

test('parseReportResultPage rejects a missing signed cursor', () => {
  const run = { id: 31, runUuid: 'run-uuid', definitionId: 9, versionId: 23, status: 'SUCCEEDED', resultAvailable: true }
  assert.throws(() => parseReportResultPage({ data: { run, columns: [], rows: [], pagination: { pageSize: 100, hasMore: true } } }))
})

test('parseReportDraft preserves parameter, field and excel mappings', () => {
  const draft = parseReportDraft({ data: {
    id: 9, code: 'sales_report', name: '销售报表', datasourceId: 3, status: 'DRAFT', lockVersion: 4,
    procedure: { owner: 'BI', package: 'REPORT_PKG', name: 'SALES' },
    result: { tableOwner: 'BI', tableName: 'REPORT_RESULT' },
    callTemplate: 'BEGIN BI.REPORT_PKG.SALES({{runId}}, {{storeCode}}); END;',
    parameters: [{ code: 'runId', label: '运行编号', displayOrder: 0, controlType: 'TEXT', logicalType: 'string', procedureArgName: 'P_RUN_ID', position: 1, oracleType: 'VARCHAR2', precision: 38, scale: 0, required: true, systemInjected: true, normalizer: { trim: true }, valueSource: { source: 'run_id' }, nullPolicy: 'TYPED_NULL' }],
    columns: [{ fieldId: '11111111-1111-4111-8111-111111111111', logicalCode: 'amount', databaseColumn: 'AMOUNT', sourceOracleType: 'NUMBER', precision: 18, scale: 2, valueType: 'decimal', previewHeader: '金额', excelHeader: '含税金额', displayOrder: 2, exportOrder: 1, previewVisible: true, exportVisible: true, exportAllowed: true, dictionaryVersion: { version: 'v2' } }],
    grants: [{ subjectType: 'ROLE', subjectId: 7, actions: ['QUERY', 'EXPORT'] }],
  } })
  assert.equal(draft.parameters[0].systemInjected, true)
  assert.equal(draft.parameters[0].precision, 38)
  assert.deepEqual(draft.parameters[0].normalizer, { trim: true })
  assert.deepEqual(draft.parameters[0].valueSource, { source: 'RUN_ID' })
  assert.equal(draft.columns[0].databaseColumn, 'AMOUNT')
  assert.equal(draft.columns[0].precision, 18)
  assert.equal(draft.columns[0].displayOrder, 2)
  assert.deepEqual(draft.columns[0].dictionaryVersion, { version: 'v2' })
  assert.equal(draft.columns[0].excelHeader, '含税金额')
  assert.deepEqual(draft.grants[0].actions, ['QUERY', 'EXPORT'])
  assert.equal(draft.executionMode, 'TABLE_SNAPSHOT')
})

test('REF CURSOR draft keeps JSON condition display names and automatic argument bindings', () => {
  const draft = parseReportDraft({ data: {
    id: 10, code: 'supplier_report', name: '供应商报表', datasourceId: 3, status: 'DRAFT', lockVersion: 2,
    executionMode: 'REF_CURSOR',
    procedure: { owner: 'BI', package: 'REPORT_PKG', name: 'SUPPLIER', jsonInputArgName: 'P_PAYLOAD', resultCursorArgName: 'P_RESULT' },
    inputSchema: {
      c_supplier_id: { type: 'varchar2', displayName: '供应商', control: 'multi_select', required: true, multiple: true, example: ['a', 'b'] },
      datein_begin: { type: 'date', displayName: '开始日期', control: 'date', example: '20260504' },
    },
    columns: [], grants: [], parameters: [], result: {}, callTemplate: '',
  } })
  assert.equal(draft.executionMode, 'REF_CURSOR')
  assert.equal(draft.procedure.jsonInputArgName, 'P_PAYLOAD')
  assert.equal(draft.inputSchema.c_supplier_id.displayName, '供应商')
  assert.equal(draft.inputSchema.c_supplier_id.type, 'list[str]')
  assert.equal(draft.inputSchema.c_supplier_id.control, 'SELECT')
  assert.equal(draft.inputSchema.datein_begin.type, 'str')
  assert.equal(draft.inputSchema.datein_begin.control, 'DATE')
  assert.equal(draft.inputSchema.datein_begin.format, 'YYYYMMDD')
  assert.deepEqual(draft.inputSchema.c_supplier_id.example, ['a', 'b'])
})

test('JSON result-table draft keeps one input binding and the Oracle snapshot table', async () => {
  const payload = { data: {
    id: 11, code: 'table_json_report', name: '结果表报表', datasourceId: 3, status: 'DRAFT', lockVersion: 3,
    executionMode: 'TABLE_SNAPSHOT',
    procedure: { owner: 'BI', package: 'REPORT_PKG', name: 'BUILD_RESULT', jsonInputArgName: 'P_PAYLOAD', resultCursorArgName: '' },
    inputSchema: { store_id: { type: 'VARCHAR2', displayName: '门店' } },
    result: { tableOwner: 'BI', tableName: 'REPORT_RESULT' },
    columns: [{ fieldId: '11111111-1111-4111-8111-111111111111', logicalCode: 'amount', databaseColumn: 'AMOUNT', sourceOracleType: 'NUMBER', valueType: 'decimal', previewHeader: '金额', excelHeader: '金额', displayOrder: 0, exportOrder: 0, previewVisible: true, exportVisible: true, exportAllowed: true }],
    grants: [], parameters: [], callTemplate: '',
  } }
  const draft = parseReportDraft(payload)
  assert.equal(draft.procedure.resultCursorArgName, '')
  assert.equal(draft.result.tableName, 'REPORT_RESULT')
  assert.equal(draft.inputSchema.store_id.displayName, '门店')
  draft.inputSchema.store_id = { ...draft.inputSchema.store_id, type: 'list[str]', displayName: '门店筛选', required: true, control: 'SELECT', allowedValues: ['S001', 'S002'] }
  const requests = []
  const client = async (path, options) => { requests.push({ path, options }); return { ok: true, data: payload } }
  await saveReportDraft(client, draft)
  assert.equal(requests[0].options.body.executionMode, 'TABLE_SNAPSHOT')
  assert.deepEqual(requests[0].options.body.parameters, [])
  assert.deepEqual(requests[0].options.body.inputSchema.store_id, { type: 'list[str]', displayName: '门店筛选', required: true, control: 'SELECT', allowedValues: ['S001', 'S002'] })
  assert.deepEqual(requests[0].options.body.result, { tableOwner: 'BI', tableName: 'REPORT_RESULT' })
  assert.equal(requests[0].options.body.callTemplate, '')
})

test('dirty report configuration saves the selected date format before publication', async () => {
  const hash = 'a'.repeat(64)
  const savedPayload = { data: {
    id: 11, code: 'table_json_report', name: '结果表报表', datasourceId: 3, status: 'DRAFT', lockVersion: 4,
    executionMode: 'TABLE_SNAPSHOT', procedure: { owner: 'BI', package: 'REPORT_PKG', name: 'BUILD_RESULT', jsonInputArgName: 'P_PAYLOAD', resultCursorArgName: '' },
    inputSchema: { datein_begin: { type: 'str', displayName: '开始日期', control: 'DATE', format: 'YYYYMMDD' } },
    result: { tableOwner: 'BI', tableName: 'REPORT_RESULT' },
    columns: [{ fieldId: '11111111-1111-4111-8111-111111111111', logicalCode: 'amount', databaseColumn: 'AMOUNT', sourceOracleType: 'NUMBER', valueType: 'decimal', previewHeader: '金额', excelHeader: '金额', displayOrder: 0, exportOrder: 0, previewVisible: true, exportVisible: true, exportAllowed: true }],
    grants: [], parameters: [], callTemplate: '',
  } }
  const draft = parseReportDraft(savedPayload)
  draft.lockVersion = 3
  const requests = []
  const client = async (path, options) => {
    requests.push({ path, options })
    if (path.endsWith('/publish')) return { ok: true, data: { data: { definitionId: 11, versionId: 8, version: 4, status: 'PUBLISHED', contractHash: hash, publishedAt: '2026-08-14T14:30:00Z' } } }
    return { ok: true, data: savedPayload }
  }
  const result = await saveAndPublishReportDraft(client, draft)
  assert.equal(result.ok, true)
  assert.equal(requests[0].path, '/v1/reports/11')
  assert.equal(requests[0].options.body.inputSchema.datein_begin.format, 'YYYYMMDD')
  assert.equal(requests[1].path, '/v1/reports/11/publish')
  assert.equal(requests[1].options.body.expectedLockVersion, 4)

  const saveFailureRequests = []
  const saveFailure = await saveAndPublishReportDraft(async (path, options) => {
    saveFailureRequests.push({ path, options })
    return { ok: false, error: { message: '草稿保存失败' } }
  }, draft)
  assert.deepEqual(saveFailure, { ok: false, error: '草稿保存失败' })
  assert.equal(saveFailureRequests.length, 1)

  const publishFailureRequests = []
  const publishFailure = await saveAndPublishReportDraft(async (path, options) => {
    publishFailureRequests.push({ path, options })
    if (path.endsWith('/publish')) return { ok: false, error: { message: '发布失败' } }
    return { ok: true, data: savedPayload }
  }, draft)
  assert.equal(publishFailure.ok, false)
  assert.equal(publishFailure.error, '发布失败')
  assert.equal(publishFailure.draft.lockVersion, 4)
  assert.equal(publishFailureRequests.length, 2)
})

test('condition JSON requires a filter display name and normalizes supported types', () => {
  const schema = parseReportInputSchemaDocument({ a: { type: 'varchar2', displayName: '门店', control: 'text', required: true, example: '01' } })
  assert.deepEqual(schema.a, { type: 'str', displayName: '门店', control: 'TEXT', required: true, example: '01' })
  assert.throws(() => parseReportInputSchemaDocument({ a: { type: 'VARCHAR2' } }), /筛选显示名/)
  assert.throws(() => parseReportInputSchemaDocument({ a: { type: 'VARCHAR2', displayName: '门店', unknown: true } }), /未知配置/)
})

test('condition JSON validates metadata types, date precision and exact numeric literals before save', () => {
  assert.throws(() => parseReportInputSchemaDocument({ amount: { type: 'number', displayName: '金额', default: '12.5' } }), /默认值/)
  assert.throws(() => parseReportInputSchemaDocument({ stores: { type: 'list[str]', displayName: '门店', example: ['S001', 2] } }), /示例值/)
  assert.throws(() => parseReportInputSchemaDocument({ day: { type: 'str', displayName: '日期', control: 'DATE', format: 'YYYYMMDD', default: '20260504123045' } }), /日期格式/)
  assert.throws(() => parseReportInputSchemaDocument({ day: { type: 'str', displayName: '日期', control: 'DATE', format: 'BAD' } }), /日期格式不受支持/)
  assert.throws(() => parseReportInputSchemaText('{"amount":{"type":"number","displayName":"金额","default":9007199254740993}}'), /安全数字范围/)
  assert.throws(() => parseReportInputSchemaText('{"amount":{"type":"number","displayName":"金额","default":0.10000000000000001}}'), /安全数字范围/)
})

test('condition JSON sends only configured canonical values and formats date controls', () => {
  const schema = parseReportInputSchemaDocument({
    supplier: { type: 'str', displayName: '供应商' },
    amount: { type: 'number', displayName: '金额' },
    enabled: { type: 'bool', displayName: '启用' },
    stores: { type: 'list[str]', displayName: '门店' },
    levels: { type: 'list[number]', displayName: '等级' },
    flags: { type: 'list[bool]', displayName: '标记' },
    options: { type: 'json', displayName: '扩展条件' },
    dayCompact: { type: 'str', displayName: '业务日期', control: 'DATE', format: 'YYYYMMDD' },
    dayDashed: { type: 'str', displayName: '结束日期', control: 'DATE', format: 'YYYY-MM-DD' },
    timeCompact: { type: 'str', displayName: '执行时间', control: 'DATETIME', format: 'YYYYMMDDHHmmss' },
    timeReadable: { type: 'str', displayName: '完成时间', control: 'DATETIME', format: 'YYYY-MM-DD HH:mm:ss' },
    timeISO: { type: 'str', displayName: '同步时间', control: 'DATETIME', format: 'ISO8601' },
  })
  assert.deepEqual(buildReportConditions(schema, {
    supplier: 'A001', amount: '12.5', enabled: true, stores: '["S001","S002"]', levels: '[1,2]', flags: '[true,false]', options: '{"active":true}',
    dayCompact: '2026-05-04', dayDashed: '2026-05-05', timeCompact: '2026-05-04T13:25', timeReadable: '2026-05-04T13:25:06', timeISO: '2026-05-04T13:25',
    report_id: 99, run_id: 100,
  }), { ok: true, conditions: {
    supplier: 'A001', amount: 12.5, enabled: true, stores: ['S001', 'S002'], levels: [1, 2], flags: [true, false], options: { active: true },
    dayCompact: '20260504', dayDashed: '2026-05-05', timeCompact: '20260504132500', timeReadable: '2026-05-04 13:25:06', timeISO: '2026-05-04T13:25:00',
  } })
  assert.deepEqual(buildReportConditions(parseReportInputSchemaDocument({ day: { type: 'str', displayName: '日期', control: 'DATE' } }), { day: '2026-05-04' }), { ok: true, conditions: { day: '20260504' } })
  assert.deepEqual(buildReportConditions(schema, { levels: '["1"]' }), { ok: false, error: '等级 与 list[number] 类型不匹配。' })
  assert.deepEqual(buildReportConditions(schema, { dayCompact: '2026-02-30' }), { ok: false, error: '业务日期 必须填写有效日期。' })
})

test('optional conditions keep typed empty values instead of omitting fields', () => {
  const schema = parseReportInputSchemaDocument({
    supplier_ids: { type: 'list[str]', displayName: '供应商' },
    store_id: { type: 'str', displayName: '门店' },
    report_date: { type: 'str', displayName: '报表日期', control: 'DATE', format: 'YYYYMMDD' },
    minimum: { type: 'number', displayName: '最小值' },
    enabled: { type: 'bool', displayName: '启用' },
    options: { type: 'json', displayName: '扩展条件' },
    default_store: { type: 'str', displayName: '默认门店', default: 'S001' },
  })
  assert.deepEqual(buildReportConditions(schema, {}), { ok: true, conditions: {
    supplier_ids: [], store_id: '', report_date: '', minimum: null, enabled: null, options: null, default_store: 'S001',
  } })
})

test('number conditions reject values that JavaScript cannot preserve safely', () => {
  const schema = parseReportInputSchemaDocument({
    amount: { type: 'number', displayName: '金额' },
    levels: { type: 'list[number]', displayName: '等级' },
  })
  const unsafeMessage = ' 超出 JavaScript 安全数字范围或无法无损表示，请改用 str 类型。'

  assert.deepEqual(buildReportConditions(schema, { amount: '9007199254740991' }), { ok: true, conditions: { amount: 9007199254740991, levels: [] } })
  assert.deepEqual(buildReportConditions(schema, { amount: '1.25e3' }), { ok: true, conditions: { amount: 1250, levels: [] } })
  assert.deepEqual(buildReportConditions(schema, { amount: '9007199254740993' }), { ok: false, error: `金额${unsafeMessage}` })
  assert.deepEqual(buildReportConditions(schema, { amount: '0.10000000000000001' }), { ok: false, error: `金额${unsafeMessage}` })
  assert.deepEqual(buildReportConditions(schema, { levels: '[1,9007199254740993]' }), { ok: false, error: `等级${unsafeMessage}` })
  assert.deepEqual(buildReportConditions(schema, { levels: '[0.1,0.10000000000000001]' }), { ok: false, error: `等级${unsafeMessage}` })
  assert.deepEqual(buildReportConditions(schema, { levels: '[1e309]' }), { ok: false, error: `等级${unsafeMessage}` })
})

test('formatted date defaults use HTML input values and convert back before submission', () => {
  const schema = parseReportInputSchemaDocument({
    day: { type: 'str', displayName: '日期', control: 'DATE', format: 'YYYYMMDD', default: '20260504', allowedValues: ['20260504', '20260505'] },
    compactTime: { type: 'str', displayName: '紧凑时间', control: 'DATETIME', format: 'YYYYMMDDHHmmss', default: '20260504123045' },
    readableTime: { type: 'str', displayName: '可读时间', control: 'DATETIME', format: 'YYYY-MM-DD HH:mm:ss', default: '2026-05-04 12:30:45' },
    isoTime: { type: 'str', displayName: 'ISO 时间', control: 'DATETIME', format: 'ISO8601', default: '2026-05-04T12:30:45' },
  })
  const values = initialReportConditionValues(schema)
  assert.deepEqual(values, { day: '2026-05-04', compactTime: '2026-05-04T12:30:45', readableTime: '2026-05-04T12:30:45', isoTime: '2026-05-04T12:30:45' })
  assert.deepEqual(buildReportConditions(schema, values), { ok: true, conditions: { day: '20260504', compactTime: '20260504123045', readableTime: '2026-05-04 12:30:45', isoTime: '2026-05-04T12:30:45' } })
  assert.deepEqual(buildReportConditions(schema, { ...values, day: '2026-05-05' }), { ok: true, conditions: { day: '20260505', compactTime: '20260504123045', readableTime: '2026-05-04 12:30:45', isoTime: '2026-05-04T12:30:45' } })
})

test('Excel JSON mapping and table edits share the existing columns contract', () => {
  const mapping = parseExcelMappingDocument({ a: 'id', amount: '金额' })
  const columns = applyExcelMapping([], mapping, (() => { let index = 0; return () => `00000000-0000-4000-8000-${String(++index).padStart(12, '0')}` })())
  assert.equal(columns[0].databaseColumn, 'a')
  assert.equal(columns[0].excelHeader, 'id')
  assert.equal(columns[1].exportOrder, 1)
  assert.deepEqual(excelMappingFromColumns(columns), mapping)
  const edited = applyExcelMapping(columns, { a: '编号' }, () => { throw new Error('existing field must keep its stable id') })
  assert.equal(edited[0].fieldId, columns[0].fieldId)
  assert.equal(edited[0].excelHeader, '编号')
  const renamed = renameExcelMappingField(columns, 'a', 'supplier_id')
  assert.equal(renamed[0].fieldId, columns[0].fieldId)
  assert.equal(renamed[0].databaseColumn, 'supplier_id')
})

test('result table schema maps every field and keeps edited headers', () => {
  let index = 0
  const createFieldId = () => `00000000-0000-4000-8000-${String(++index).padStart(12, '0')}`
  const schema = [
    { name: 'RUN_ID', position: 1, oracleType: 'VARCHAR2', dataLength: 36, precision: null, scale: null, nullable: false },
    { name: 'ID', position: 2, oracleType: 'NUMBER', dataLength: 22, precision: 18, scale: 0, nullable: false },
    { name: 'SUPPLIER_ID', position: 3, oracleType: 'VARCHAR2', dataLength: 64, precision: null, scale: null, nullable: true },
  ]
  const generated = reportColumnsFromResultSchema(schema, createFieldId)
  assert.deepEqual(generated.map((column) => column.databaseColumn), ['RUN_ID', 'ID', 'SUPPLIER_ID'])
  assert.equal(generated[0].sourceOracleType, 'VARCHAR2')
  assert.equal(generated[1].sourceOracleType, 'NUMBER')
  const customized = generated.map((column) => column.databaseColumn === 'SUPPLIER_ID'
    ? { ...column, excelHeader: '供应商编码', previewHeader: '供应商编码' }
    : column)
  const reconciled = reconcileReportColumnsWithResultSchema(schema, customized, () => { throw new Error('existing field must keep its stable id') })
  const supplier = reconciled.find((column) => column.databaseColumn === 'SUPPLIER_ID')
  assert.equal(supplier.fieldId, generated[2].fieldId)
  assert.equal(supplier.excelHeader, '供应商编码')
})

test('result table metadata parsers and requests enforce the Oracle table contract', async () => {
  const table = { owner: 'REPORT', name: 'SALES_RESULT', qualifiedName: 'REPORT.SALES_RESULT', columnCount: 3 }
  const columns = [
    { name: 'RUN_ID', position: 1, oracleType: 'VARCHAR2', dataLength: 36, precision: null, scale: null, nullable: false },
    { name: 'ID', position: 2, oracleType: 'NUMBER', dataLength: 22, precision: 18, scale: 0, nullable: false },
    { name: 'AMOUNT', position: 3, oracleType: 'NUMBER', dataLength: 22, precision: 18, scale: 2, nullable: true },
  ]
  assert.equal(parseReportResultTablePage({ data: { items: [table], hasMore: true, nextAfter: 'opaque' } }).items[0].qualifiedName, table.qualifiedName)
  assert.equal(parseReportResultTableSchema({ data: { table, columns } }).columns[2].precision, 18)
  assert.throws(() => parseReportResultTableSchema({ data: { table, columns: [columns[1], columns[0], columns[2]] } }), /column order/)

  const requests = []
  const client = async (path, options) => {
    requests.push({ path, options })
    return path.includes('result-table-schema')
      ? { ok: true, data: { data: { table, columns } } }
      : { ok: true, data: { data: { items: [table], hasMore: false, nextAfter: '' } } }
  }
  await getReportResultTables(client, 3, { owner: ' REPORT ', search: ' sales ', after: 'opaque', limit: 200 })
  await getReportResultTableSchema(client, 3, { owner: 'REPORT', name: 'SALES_RESULT' })
  assert.equal(requests[0].path, '/v1/report-datasources/3/result-tables?owner=REPORT&search=sales&after=opaque&limit=100')
  assert.equal(requests[1].path, '/v1/report-datasources/3/result-table-schema?owner=REPORT&name=SALES_RESULT')
})

test('procedure catalog and signature parsers enforce the JSON cursor protocol contract', () => {
  const page = parseReportProcedurePage({ data: { items: [{ owner: 'REPORT', package: 'PKG', name: 'BUILD', overload: '1', argumentCount: 2, qualifiedName: 'REPORT.PKG.BUILD #1' }], hasMore: true, nextAfter: 'cursor' } })
  assert.equal(page.items[0].qualifiedName, 'REPORT.PKG.BUILD #1')
  const signature = parseReportProcedureSignature({ data: {
    procedure: page.items[0], allSupported: true, protocolReady: true, inputArgName: 'P_PAYLOAD', outputArgName: 'P_RESULT', callTemplate: 'BEGIN ... END;', blockingReasons: [],
    arguments: [
      { name: 'P_PAYLOAD', position: 1, sequence: 1, direction: 'IN', oracleType: 'CLOB', dataLength: 4000, precision: null, scale: null, typeOwner: '', typeName: '', defaulted: false, supported: true, suggestedCode: 'payload', suggestedLogicalType: 'json', suggestedControlType: 'TEXTAREA', suggestedSystemValue: '', role: 'JSON_INPUT' },
      { name: 'P_RESULT', position: 2, sequence: 2, direction: 'OUT', oracleType: 'REF CURSOR', dataLength: null, precision: null, scale: null, typeOwner: '', typeName: '', defaulted: false, supported: true, suggestedCode: 'result', suggestedLogicalType: 'cursor', suggestedControlType: '', suggestedSystemValue: '', role: 'RESULT_CURSOR' },
    ],
  } })
  assert.equal(signature.protocolReady, true)
  assert.equal(signature.outputArgName, 'P_RESULT')
  assert.throws(() => parseReportProcedureSignature({ data: { ...signature, blockingReasons: ['blocked'] } }))
})

test('procedure signature accepts one JSON input without an output cursor', () => {
  const signature = parseReportProcedureSignature({ data: {
    procedure: { owner: 'REPORT', package: 'PKG', name: 'BUILD', overload: '', argumentCount: 1, qualifiedName: 'REPORT.PKG.BUILD' },
    allSupported: true, protocolReady: true, inputArgName: 'P_PAYLOAD', outputArgName: '', callTemplate: 'BEGIN REPORT.PKG.BUILD(P_PAYLOAD => :payload); END;', blockingReasons: [],
    arguments: [{ name: 'P_PAYLOAD', position: 1, sequence: 1, direction: 'IN', oracleType: 'CLOB', dataLength: null, precision: null, scale: null, typeOwner: '', typeName: '', defaulted: false, supported: true, suggestedCode: 'payload', suggestedLogicalType: 'json', suggestedControlType: 'TEXTAREA', suggestedSystemValue: '', role: 'JSON_INPUT' }],
  } })
  assert.equal(signature.protocolReady, true)
  assert.equal(signature.inputArgName, 'P_PAYLOAD')
  assert.equal(signature.outputArgName, '')
})

test('procedure API encodes discovery filters and REF CURSOR draft save omits legacy contracts', async () => {
  const requests = []
  const client = async (path, options) => {
    requests.push({ path, options })
    if (path.includes('procedure-signature')) return { ok: true, data: { data: {
      procedure: { owner: 'REPORT', package: 'PKG', name: 'BUILD', overload: '', argumentCount: 0, qualifiedName: 'REPORT.PKG.BUILD' },
      arguments: [], allSupported: false, protocolReady: false, inputArgName: '', outputArgName: '', callTemplate: '', blockingReasons: ['缺少参数'],
    } } }
    if (path.includes('/procedures?')) return { ok: true, data: { data: { items: [], hasMore: false, nextAfter: '' } } }
    return { ok: true, data: { data: {
      id: 12, code: 'json_report', name: 'JSON 报表', datasourceId: 3, status: 'DRAFT', lockVersion: 1, executionMode: 'REF_CURSOR',
      procedure: { owner: 'REPORT', package: 'PKG', name: 'BUILD', jsonInputArgName: 'P_PAYLOAD', resultCursorArgName: 'P_RESULT' },
      inputSchema: { store_id: { type: 'VARCHAR2', displayName: '门店' } }, parameters: [], columns: [], grants: [], result: {}, callTemplate: '',
    } } }
  }
  await getReportProcedures(client, 3, { owner: ' REPORT ', search: ' daily ', limit: 200 })
  await getReportProcedureSignature(client, 3, { owner: 'REPORT', package: 'PKG', name: 'BUILD', overload: '' })
  const draft = parseReportDraft({ data: { id: 12, code: 'json_report', name: 'JSON 报表', datasourceId: 3, status: 'DRAFT', lockVersion: 1, executionMode: 'REF_CURSOR', procedure: { owner: 'REPORT', package: 'PKG', name: 'BUILD', jsonInputArgName: 'P_PAYLOAD', resultCursorArgName: 'P_RESULT' }, inputSchema: { store_id: { type: 'VARCHAR2', displayName: '门店' } }, parameters: [], columns: [], grants: [], result: {}, callTemplate: '' } })
  await saveReportDraft(client, draft)
  assert.equal(requests[0].path, '/v1/report-datasources/3/procedures?owner=REPORT&search=daily&limit=100')
  assert.equal(requests[1].path, '/v1/report-datasources/3/procedure-signature?owner=REPORT&name=BUILD&package=PKG')
  assert.equal(requests[2].options.body.executionMode, 'REF_CURSOR')
  assert.deepEqual(requests[2].options.body.parameters, [])
  assert.deepEqual(requests[2].options.body.result, {})
  assert.equal(requests[2].options.body.callTemplate, '')
  assert.equal(requests[2].options.body.inputSchema.store_id.displayName, '门店')
})

test('publication parser keeps only the safe Oracle validation summary', () => {
  const hash = 'a'.repeat(64)
  const publication = parsePublication({ data: {
    definitionId: 9, versionId: 23, version: 3, status: 'PUBLISHED', contractHash: hash, publishedAt: '2026-08-13T08:00:01Z',
    validation: {
      validatedAt: '2026-08-13T08:00:00Z',
      procedure: { owner: 'REPORT', package: 'PKG_SALES', name: 'BUILD_REPORT', overload: '', argumentCount: 2, signatureHash: hash, password: 'must-not-pass' },
      result: { tableOwner: 'REPORT', tableName: 'SALES_RESULT', columnCount: 12, schemaHash: hash, dsn: 'must-not-pass' },
      snapshot: { resultTableValidated: true },
      export: { exportableColumnCount: 8, schemaHash: hash },
    },
  } })
  assert.equal(publication.validation.procedure.argumentCount, 2)
  assert.equal(publication.validation.result.columnCount, 12)
  assert.equal(publication.validation.snapshot.resultTableValidated, true)
  assert.equal(Object.hasOwn(publication.validation.procedure, 'password'), false)
  assert.equal(Object.hasOwn(publication.validation.result, 'dsn'), false)
  assert.equal(parsePublication({ data: { definitionId: 9, versionId: 23, version: 3, status: 'PUBLISHED', contractHash: hash } }).validation, null)
  assert.throws(() => parsePublication({ data: { definitionId: 9, versionId: 23, version: 3, contractHash: hash, validation: {} } }))
  for (const validation of [
    { ...publication.validation, procedure: { ...publication.validation.procedure, name: '' } },
    { ...publication.validation, result: { ...publication.validation.result, columnCount: 0 } },
    { ...publication.validation, export: { ...publication.validation.export, schemaHash: 'a'.repeat(63) } },
    { ...publication.validation, snapshot: { ...publication.validation.snapshot, resultTableValidated: false } },
  ]) assert.throws(() => parsePublication({ data: { definitionId: 9, versionId: 23, version: 3, status: 'PUBLISHED', contractHash: hash, validation } }))
})

test('version parsers enforce cursor and structured summary differences', () => {
  const version = (id, number) => ({ id, version: number, status: 'PUBLISHED', contractFingerprint: 'a'.repeat(12), parameterCount: 2, columnCount: 4, grantCount: 1 })
  const page = parseReportVersionPage({ data: { items: [version(23, 2), version(11, 1)], hasMore: true, nextAfterId: 11 } })
  assert.equal(page.items[0].version, 2)
  assert.throws(() => parseReportVersionPage({ data: { items: [version(23, 2)], hasMore: true, nextAfterId: 99 } }))
  const sections = [
    { key: 'procedure', label: '存储过程', changes: [] },
    { key: 'parameters', label: '{{形参}}', changes: [{ kind: 'CHANGED', key: 'parameterCount', label: '参数数量', before: 2, after: 3 }] },
    { key: 'results', label: '结果字段与 Excel', changes: [] },
    { key: 'excel', label: 'Excel 契约', changes: [{ kind: 'CHANGED', key: 'exportSchemaHash', label: 'Excel Schema', before: 'b'.repeat(12), after: 'c'.repeat(12) }] },
    { key: 'permissions', label: '权限', changes: [] },
  ]
  const diff = parseReportVersionDiff({ data: { base: version(11, 1), target: version(23, 2), sections } })
  assert.equal(diff.sections[1].label, '筛选条件')
  assert.equal(diff.sections[1].changes[0].after, 3)
  assert.equal(diff.sections[3].changes[0].before, 'b'.repeat(12))
  assert.throws(() => parseReportVersionDiff({ data: { base: version(11, 1), target: version(23, 2), sections: sections.map((section) => section.key === 'excel' ? { ...section, changes: [{ ...section.changes[0], before: { leaked: true } }] } : section) } }))
  assert.throws(() => parseReportVersionPage({ data: { items: [{ ...version(23, 2), contractFingerprint: '' }], hasMore: false, nextAfterId: 23 } }))
  assert.throws(() => parseReportVersionPage({ data: { items: [{ ...version(23, 2), contractFingerprint: 'a'.repeat(64) }], hasMore: false, nextAfterId: 23 } }))
  assert.throws(() => parseReportVersionPage({ data: { items: [{ ...version(23, 2), contractFingerprint: 'a'.repeat(13) }], hasMore: false, nextAfterId: 23 } }))
  assert.throws(() => parseReportVersionPage({ data: { items: [version(11, 1), version(23, 2)], hasMore: false, nextAfterId: 23 } }))
})

test('version request guard invalidates superseded and cancelled responses', () => {
  const guard = createLatestRequestGuard()
  const first = guard.begin()
  const second = guard.begin()
  assert.equal(first.signal.aborted, true)
  assert.equal(first.isCurrent(), false)
  assert.equal(second.isCurrent(), true)
  guard.cancel()
  assert.equal(second.signal.aborted, true)
  assert.equal(second.isCurrent(), false)
})

test('Oracle datasource parser exposes only the public management contract', () => {
  const datasource = parseReportDatasource({ data: {
    id: 7, code: 'report_oracle', name: '经营报表库', driver: 'ORACLE', host: 'oracle.internal', port: 1521,
    serviceName: 'REPORT', sid: '', username: 'report_user', hasPassword: true, enabled: true,
    sessionTimezone: 'Asia/Shanghai', connectTimeoutSeconds: 5, queryTimeoutSeconds: 300,
    maxOpenConnections: 10, maxIdleConnections: 2, prefetchRows: 1000, arraySize: 1000,
    lastTestStatus: 'SUCCESS', lastTestedAt: '2026-08-13T08:00:00Z',
  } })
  assert.equal(datasource.id, 7)
  assert.equal(datasource.serviceName, 'REPORT')
  assert.equal(datasource.hasPassword, true)
  assert.equal(Object.hasOwn(datasource, 'password'), false)
  assert.equal(Object.hasOwn(datasource, 'passwordCiphertext'), false)
  assert.equal(Object.hasOwn(datasource, 'credentialKeyVersion'), false)
  assert.equal(Object.hasOwn(datasource, 'sessionInitJSON'), false)
  for (const forbidden of ['password', 'passwordCiphertext', 'credentialKeyVersion', 'sessionInitJSON', 'dsn']) {
    assert.throws(() => parseReportDatasource({ data: {
      id: 7, code: 'report_oracle', name: '经营报表库', driver: 'ORACLE', host: 'oracle.internal', port: 1521,
      serviceName: 'REPORT', sid: '', username: 'report_user', hasPassword: true, enabled: true, [forbidden]: 'must-not-pass',
    } }))
  }
})

test('Oracle datasource parser enforces a single Service Name or SID', () => {
  const base = { id: 7, code: 'report_oracle', name: '经营报表库', driver: 'ORACLE', host: 'oracle.internal', port: 1521, username: 'report_user', hasPassword: true, enabled: true }
  assert.throws(() => parseReportDatasource({ data: { ...base, serviceName: '', sid: '' } }))
  assert.throws(() => parseReportDatasource({ data: { ...base, serviceName: 'REPORT', sid: 'ORCL' } }))
  assert.deepEqual(parseReportDatasources({ data: { items: [{ ...base, serviceName: '', sid: 'ORCL' }] } }).map((item) => item.id), [7])
})

test('Oracle datasource test parser accepts only safe stable fields', () => {
  const result = parseReportDatasourceTest({ data: { status: 'FAILED', testedAt: '2026-08-13T08:00:00Z', latencyMs: 12, errorCode: 'AUTHENTICATION_FAILED', message: 'Oracle 用户名或密码无效', rawError: 'password=secret' } })
  assert.equal(result.errorCode, 'AUTHENTICATION_FAILED')
  assert.equal(Object.hasOwn(result, 'rawError'), false)
})

test('Oracle connection validation does not require save-only name and code fields', () => {
  const input = {
    code: '', name: '', host: 'oracle.internal', port: 1521, serviceName: 'REPORT', sid: '',
    username: 'report_user', password: 'draft-password', sessionTimezone: 'Asia/Shanghai', connectTimeoutSeconds: 5,
    queryTimeoutSeconds: 300, maxOpenConnections: 10, maxIdleConnections: 2, prefetchRows: 1000, arraySize: 1000, enabled: true,
  }
  assert.equal(validateDatasourceConnection(input, false), '')
  assert.equal(validateDatasourceSave(input, false), '请填写数据源名称。')
})

test('Oracle datasource code normalizes uppercase input and reports exact missing fields', () => {
  assert.equal(normalizeDatasourceCode('  REPORT_Oracle  '), 'report_oracle')
  const input = {
    code: 'REPORT_Oracle', name: 'Oracle 报表库', host: '', port: 1521, serviceName: 'REPORT', sid: '',
    username: 'report_user', password: 'draft-password', sessionTimezone: 'Asia/Shanghai', connectTimeoutSeconds: 5,
    queryTimeoutSeconds: 300, maxOpenConnections: 10, maxIdleConnections: 2, prefetchRows: 1000, arraySize: 1000, enabled: true,
  }
  assert.equal(validateDatasourceSave(input, false), '请填写主机地址。')
})

test('Oracle datasource validation distinguishes invalid code and saved-password reuse', () => {
  const input = {
    code: 'report-oracle', name: 'Oracle 报表库', host: 'oracle.internal', port: 1521, serviceName: 'REPORT', sid: '',
    username: 'report_user', password: '', sessionTimezone: 'Asia/Shanghai', connectTimeoutSeconds: 5,
    queryTimeoutSeconds: 300, maxOpenConnections: 10, maxIdleConnections: 2, prefetchRows: 1000, arraySize: 1000, enabled: true,
  }
  assert.match(validateDatasourceSave(input, false), /数据源编码/)
  input.code = 'report_oracle'
  assert.equal(validateDatasourceConnection(input, false), '创建数据源时必须填写密码。')
  assert.equal(validateDatasourceConnection(input, true), '')
})

test('Oracle datasource draft test sends connection fields without saving metadata', async () => {
  let captured = null
  const client = async (path, options) => {
    captured = { path, options }
    return { ok: true, status: 200, data: { data: { status: 'SUCCESS', testedAt: '2026-08-13T08:00:00Z', latencyMs: 18, message: 'Oracle 连接测试成功' } } }
  }
  const result = await testReportDatasourceConnection(client, {
    code: 'draft_oracle', name: '草稿连接', host: 'oracle.internal', port: 1521, serviceName: 'REPORT', sid: '',
    username: 'report_user', password: 'draft-password', sessionTimezone: 'Asia/Shanghai', connectTimeoutSeconds: 5,
    queryTimeoutSeconds: 300, maxOpenConnections: 10, maxIdleConnections: 2, prefetchRows: 1000, arraySize: 1000, enabled: true,
  }, 7)
  assert.equal(result.ok, true)
  assert.equal(captured.path, '/v1/report-datasource-connection-tests')
  assert.equal(captured.options.method, 'POST')
  assert.equal(captured.options.body.datasourceId, 7)
  assert.equal(captured.options.body.password, 'draft-password')
  assert.equal(Object.hasOwn(captured.options.body, 'code'), false)
  assert.equal(Object.hasOwn(captured.options.body, 'name'), false)
  assert.equal(Object.hasOwn(captured.options.body, 'enabled'), false)
})

test('Oracle datasource draft test reuses a saved password without sending an empty password', async () => {
  let captured = null
  const client = async (path, options) => {
    captured = { path, options }
    return { ok: true, status: 200, data: { data: { status: 'SUCCESS', testedAt: '2026-08-13T08:00:00Z', latencyMs: 18, message: 'Oracle 连接测试成功' } } }
  }
  await testReportDatasourceConnection(client, {
    code: 'saved_oracle', name: '已保存连接', host: 'oracle.internal', port: 1521, serviceName: 'REPORT', sid: '',
    username: 'report_user', password: '', sessionTimezone: 'Asia/Shanghai', connectTimeoutSeconds: 5,
    queryTimeoutSeconds: 300, maxOpenConnections: 10, maxIdleConnections: 2, prefetchRows: 1000, arraySize: 1000, enabled: true,
  }, 7)
  assert.equal(captured.options.body.datasourceId, 7)
  assert.equal(Object.hasOwn(captured.options.body, 'password'), false)
})

test('report audit parser preserves safe cursor records and structured detail', () => {
  const page = parseReportAuditPage({ data: {
    items: [{
      id: 88, actorType: 'USER', actorUserId: 7, action: 'REPORT_RUN_RESULT_READ', targetType: 'REPORT_RUN', targetId: 31,
      requestId: 'request-uuid', detail: { rowCount: 100, cursor: 'redacted' }, createdAt: '2026-08-13T08:00:00Z',
    }],
    hasMore: true,
    nextAfterId: 88,
  } })
  assert.equal(page.items[0].action, 'REPORT_RUN_RESULT_READ')
  assert.equal(page.items[0].actorType, 'USER')
  assert.deepEqual(page.items[0].detail, { rowCount: 100, cursor: 'redacted' })
  assert.equal(page.nextAfterId, 88)
})

test('report audit parser accepts a system actor and rejects inconsistent actors', () => {
  const base = { id: 89, action: 'REPORT_RUN_SUCCEEDED', targetType: 'REPORT_RUN', targetId: 31, requestId: 'system-request', detail: {}, createdAt: '2026-08-13T08:00:00Z' }
  const page = parseReportAuditPage({ data: { items: [{ ...base, actorType: 'SYSTEM', actorUserId: 0 }], hasMore: false } })
  assert.equal(page.items[0].actorType, 'SYSTEM')
  assert.equal(page.items[0].actorUserId, 0)
  assert.throws(() => parseReportAuditPage({ data: { items: [{ ...base, actorType: 'USER', actorUserId: 0 }], hasMore: false } }))
  assert.throws(() => parseReportAuditPage({ data: { items: [{ ...base, actorType: 'SYSTEM', actorUserId: 7 }], hasMore: false } }))
})

test('report audit parser rejects incomplete records and missing cursors', () => {
  assert.throws(() => parseReportAuditPage({ data: { items: [{ id: 1 }], hasMore: false } }))
  assert.throws(() => parseReportAuditPage({ data: { items: [], hasMore: true } }))
  assert.throws(() => parseReportAuditPage({ data: {} }))
  assert.throws(() => parseReportAuditPage({ data: { items: 'invalid', hasMore: false } }))
  assert.throws(() => parseReportAuditPage({ data: { items: [], hasMore: 'invalid' } }))
  assert.throws(() => parseReportAuditPage({ data: { items: [], hasMore: false, nextAfterId: 1 } }))
  const audit = { actorUserId: 7, action: 'REPORT_RESULT_QUERY_SUCCESS', targetType: 'REPORT_RUN', targetId: 31, requestId: 'request-uuid', detail: {}, createdAt: '2026-08-13T08:00:00Z' }
  assert.throws(() => parseReportAuditPage({ data: { items: [{ ...audit, id: 88 }], hasMore: true, nextAfterId: 99 } }))
  assert.throws(() => parseReportAuditPage({ data: { items: [{ ...audit, id: 88 }, { ...audit, id: 88 }], hasMore: true, nextAfterId: 88 } }))
  assert.throws(() => parseReportAuditPage({ data: { items: [{ ...audit, id: 87 }, { ...audit, id: 88 }], hasMore: true, nextAfterId: 88 } }))
})

test('report audit request builds bounded encoded cursor filters', async () => {
  let request
  const client = async (path, options) => {
    request = { path, options }
    return { ok: true, data: { data: { items: [], hasMore: false } } }
  }
  const result = await getReportAudits(client, {
    action: ' REPORT_RESULT_QUERY_SUCCESS ', targetType: 'REPORT_RUN', targetId: 31, afterId: 88, limit: 1000,
  })
  assert.equal(result.ok, true)
  assert.equal(request.path, '/v1/report-audits?action=REPORT_RESULT_QUERY_SUCCESS&targetType=REPORT_RUN&targetId=31&afterId=88&limit=100')
  assert.deepEqual(request.options, { method: 'GET', signal: undefined, showResult: false, silentLoading: true, acceptSafeErrorMessage: true })
})
