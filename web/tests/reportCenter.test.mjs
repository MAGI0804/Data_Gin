import assert from 'node:assert/strict'
import test from 'node:test'

import { getReportAudits, parseReportAuditPage, parseReportCatalogPage, parseReportDatasource, parseReportDatasources, parseReportDatasourceTest, parseReportDraft, parseReportExport, parseReportExportPage, parseReportResultPage, parseReportRun, parseReportRunContract } from '../.test-dist/reportCenter/api.js'

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
    parameters: [{ code: 'runId', label: '运行编号', displayOrder: 0, controlType: 'TEXT', logicalType: 'string', procedureArgName: 'P_RUN_ID', position: 1, oracleType: 'VARCHAR2', required: true, systemInjected: true, nullPolicy: 'TYPED_NULL' }],
    columns: [{ fieldId: '11111111-1111-4111-8111-111111111111', logicalCode: 'amount', databaseColumn: 'AMOUNT', sourceOracleType: 'NUMBER', valueType: 'decimal', previewHeader: '金额', excelHeader: '含税金额', previewVisible: true, exportVisible: true, exportAllowed: true }],
    grants: [{ subjectType: 'ROLE', subjectId: 7, actions: ['QUERY', 'EXPORT'] }],
  } })
  assert.equal(draft.parameters[0].systemInjected, true)
  assert.equal(draft.columns[0].databaseColumn, 'AMOUNT')
  assert.equal(draft.columns[0].excelHeader, '含税金额')
  assert.deepEqual(draft.grants[0].actions, ['QUERY', 'EXPORT'])
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

test('report audit parser preserves safe cursor records and structured detail', () => {
  const page = parseReportAuditPage({ data: {
    items: [{
      id: 88, actorUserId: 7, action: 'REPORT_RUN_RESULT_READ', targetType: 'REPORT_RUN', targetId: 31,
      requestId: 'request-uuid', detail: { rowCount: 100, cursor: 'redacted' }, createdAt: '2026-08-13T08:00:00Z',
    }],
    hasMore: true,
    nextAfterId: 88,
  } })
  assert.equal(page.items[0].action, 'REPORT_RUN_RESULT_READ')
  assert.deepEqual(page.items[0].detail, { rowCount: 100, cursor: 'redacted' })
  assert.equal(page.nextAfterId, 88)
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
