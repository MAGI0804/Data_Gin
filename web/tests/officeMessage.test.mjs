import assert from 'node:assert/strict'
import test from 'node:test'

import {
  buildOfficeMessagePayload,
  emptyOfficeMessageDraft,
  mappingsFromColumns,
  parseOfficeFeishuBots,
  parseOfficeMessages,
  parseOfficeTargets,
} from '../.test-dist/officeMessage/contracts.js'

test('office Feishu bot contract projects public App ID metadata', () => {
  const [bot] = parseOfficeFeishuBots({ data: { items: [{ id: 'cli_office', name: '办公消息机器人', source: 'ENVIRONMENT' }] } })
  assert.deepEqual(bot, { id: 'cli_office', name: '办公消息机器人', source: 'ENVIRONMENT' })

  const [target] = parseOfficeTargets({ data: { items: [{
    id: 3, name: '销售日报推送', messageId: 8, channel: 'FEISHU', botAppId: 'cli_office',
    receiveIdType: 'chat_id', receiveId: 'oc_sales', enabled: true, lockVersion: 1, updatedAt: '2026-09-01T08:00:00Z',
  }] } })
  assert.equal(target.botAppId, 'cli_office')
})

test('office message contract keeps yyyyMMdd query parameters and column mappings', () => {
  const [message] = parseOfficeMessages({ data: { items: [{
    id: 8,
    name: '销售日报',
    sourceType: 'ORACLE_QUERY',
    content: '',
    procedureOwner: '',
    packageName: '',
    procedureName: '',
    procedureOverload: '',
    resultTableOwner: '',
    resultTableName: '',
    selectSql: 'SELECT ORDER_NO FROM SALES WHERE BILL_DATE = :bill_date',
    parameters: [{ code: 'bill_date', label: '业务日期', valueType: 'date', format: 'yyyyMMdd', required: true }],
    columnMapping: [{ sourceColumn: 'ORDER_NO', header: '单号', valueType: 'string', order: 0, width: 18 }],
    enabled: true,
    lockVersion: 2,
    updatedAt: '2026-09-01T08:00:00Z',
  }] } })

  assert.equal(message.parameters[0].format, 'yyyyMMdd')
  assert.equal(message.columnMapping[0].header, '单号')
})

test('query payload requires configured names to match SELECT binds', () => {
  const draft = {
    ...emptyOfficeMessageDraft(),
    name: '销售日报',
    sourceType: 'ORACLE_QUERY',
    selectSql: 'SELECT ORDER_NO FROM SALES WHERE BILL_DATE = :bill_date',
    parameters: [{ code: 'wrong_date', label: '业务日期', valueType: 'date', format: 'yyyyMMdd', required: true }],
    columnMapping: [{ sourceColumn: 'ORDER_NO', header: '单号', valueType: 'string', order: 0, width: 18 }],
  }
  assert.throws(() => buildOfficeMessagePayload(draft), /命名绑定/)
})

test('query payload ignores colons inside Oracle string literals', () => {
  const draft = {
    ...emptyOfficeMessageDraft(),
    name: '销售日报',
    sourceType: 'ORACLE_QUERY',
    selectSql: "SELECT ':display_text' AS LABEL, ORDER_NO FROM SALES WHERE BILL_DATE = :bill_date",
    parameters: [{ code: 'bill_date', label: '业务日期', valueType: 'date', format: 'yyyyMMdd', required: true }],
    columnMapping: [{ sourceColumn: 'ORDER_NO', header: '单号', valueType: 'string', order: 0, width: 18 }],
  }
  assert.equal(buildOfficeMessagePayload(draft).parameters.length, 1)
})

test('Oracle column metadata creates ordered Excel mappings', () => {
  assert.deepEqual(mappingsFromColumns([
    { name: 'AMOUNT', dataType: 'NUMBER' },
    { name: 'CREATED_AT', dataType: 'TIMESTAMP' },
  ]).map(({ sourceColumn, valueType, order }) => ({ sourceColumn, valueType, order })), [
    { sourceColumn: 'AMOUNT', valueType: 'decimal', order: 0 },
    { sourceColumn: 'CREATED_AT', valueType: 'datetime', order: 1 },
  ])
})
