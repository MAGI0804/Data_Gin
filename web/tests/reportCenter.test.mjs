import assert from 'node:assert/strict'
import test from 'node:test'

import { getReportAudits, parsePublication, parseReportAuditPage, parseReportCatalogPage, parseReportDatasource, parseReportDatasources, parseReportDatasourceTest, parseReportDraft, parseReportExport, parseReportExportPage, parseReportResultPage, parseReportRun, parseReportRunContract, parseReportVersionDiff, parseReportVersionPage, testReportDatasourceConnection } from '../.test-dist/reportCenter/api.js'
import { reportParameterControls, reportParameterFlagDisabled, updateReportParameterFlag, updateReportParameterLogicalType } from '../.test-dist/reportCenter/parameterConfig.js'
import { buildNewReportRunState, canStartNewReportRun, initialReportParameterValues } from '../.test-dist/reportCenter/queryParameters.js'
import { createLatestRequestGuard } from '../.test-dist/reportCenter/components/ReportVersionDrawer/requestGuard.js'
import { normalizeDatasourceCode, validateDatasourceConnection, validateDatasourceSave } from '../.test-dist/reportCenter/datasourceValidation.js'

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
  assert.equal(contract.parameters[1].controlType, 'SELECT')
  assert.deepEqual(contract.parameters[1].allowedValues, ['S001', 'S002'])
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
  const runStatuses = ['QUEUED', 'RUNNING', 'CANCEL_REQUESTED', 'SUCCEEDED', 'FAILED', 'CANCELLED', 'UNKNOWN', 'RECONCILING', 'EXPORTING', 'EXPORTED', 'RESULT_PURGING', 'RESULT_PURGED']
  const exportStatuses = [null, 'PENDING', 'RUNNING', 'READY', 'FAILED', 'CANCELLED', 'EXPIRED']
  const terminalRuns = new Set(['SUCCEEDED', 'FAILED', 'CANCELLED', 'EXPORTED', 'RESULT_PURGED'])
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
    result: { tableOwner: 'BI', tableName: 'REPORT_RESULT', runIdColumn: 'RUN_ID', rowIdColumn: 'ROW_NO' },
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
})

test('publication parser keeps only the safe Oracle validation summary', () => {
  const hash = 'a'.repeat(64)
  const publication = parsePublication({ data: {
    definitionId: 9, versionId: 23, version: 3, status: 'PUBLISHED', contractHash: hash, publishedAt: '2026-08-13T08:00:01Z',
    validation: {
      validatedAt: '2026-08-13T08:00:00Z',
      procedure: { owner: 'REPORT', package: 'PKG_SALES', name: 'BUILD_REPORT', overload: '', argumentCount: 2, signatureHash: hash, password: 'must-not-pass' },
      result: { tableOwner: 'REPORT', tableName: 'SALES_RESULT', columnCount: 12, schemaHash: hash, dsn: 'must-not-pass' },
      snapshot: { runIdColumn: 'RUN_ID', rowIdColumn: 'ROW_NO', uniqueKeyValidated: true },
      export: { exportableColumnCount: 8, schemaHash: hash },
    },
  } })
  assert.equal(publication.validation.procedure.argumentCount, 2)
  assert.equal(publication.validation.result.columnCount, 12)
  assert.equal(publication.validation.snapshot.uniqueKeyValidated, true)
  assert.equal(Object.hasOwn(publication.validation.procedure, 'password'), false)
  assert.equal(Object.hasOwn(publication.validation.result, 'dsn'), false)
  assert.equal(parsePublication({ data: { definitionId: 9, versionId: 23, version: 3, status: 'PUBLISHED', contractHash: hash } }).validation, null)
  assert.throws(() => parsePublication({ data: { definitionId: 9, versionId: 23, version: 3, contractHash: hash, validation: {} } }))
  for (const validation of [
    { ...publication.validation, procedure: { ...publication.validation.procedure, name: '' } },
    { ...publication.validation, result: { ...publication.validation.result, columnCount: 0 } },
    { ...publication.validation, export: { ...publication.validation.export, schemaHash: 'a'.repeat(63) } },
    { ...publication.validation, snapshot: { ...publication.validation.snapshot, uniqueKeyValidated: false } },
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
  assert.deepEqual(request.options, { method: 'GET', signal: undefined, showResult: false, silentLoading: true })
})
