import assert from 'node:assert/strict'
import test from 'node:test'

import { mallWeatherCapacityPlanPath, parseMallWeatherCapacityPlan } from '../.test-dist/mallWeatherCapacityPlan.js'

const input = { mallCount: '5', providerQps: '2.5', hourlySteps: '360', dailySteps: '15', lifeIndexDays: '15', alertsPerMall: '3', feishuBatchRows: '200' }

test('builds the exact bounded capacity-plan query', () => {
  assert.equal(mallWeatherCapacityPlanPath(input), '/v1/mall-weather/capacity-plan?mallCount=5&providerQps=2.5&hourlySteps=360&dailySteps=15&lifeIndexDays=15&alertsPerMall=3&feishuBatchRows=200')
  assert.equal(mallWeatherCapacityPlanPath({ ...input, mallCount: '100000', providerQps: '0.0001', hourlySteps: '1', dailySteps: '1', lifeIndexDays: '1', alertsPerMall: '0', feishuBatchRows: '1' }), '/v1/mall-weather/capacity-plan?mallCount=100000&providerQps=0.0001&hourlySteps=1&dailySteps=1&lifeIndexDays=1&alertsPerMall=0&feishuBatchRows=1')
  for (const invalid of [
    { mallCount: '0' }, { mallCount: '100001' }, { providerQps: '0' }, { providerQps: '10001' },
    { hourlySteps: '0' }, { hourlySteps: '361' }, { dailySteps: '0' }, { dailySteps: '16' },
    { lifeIndexDays: '0' }, { lifeIndexDays: '16' }, { alertsPerMall: '-1' }, { alertsPerMall: '257' },
    { feishuBatchRows: '0' }, { feishuBatchRows: '501' },
  ]) assert.throws(() => mallWeatherCapacityPlanPath({ ...input, ...invalid }), /invalid capacity plan input/)
})

test('parses only complete safe capacity plan responses', () => {
  const payload = { code: 0, data: {
    mallCount: 5, providerQps: 2.5, weatherV26ProviderRequestsPerDay: 840, lifeIndexV3ProviderRequestsPerDay: 0,
    providerRequests: 840, providerDrainSeconds: 336, minimumQpsForOneHourDrain: 840 / 3600,
    totalDatabaseRows: 2570, totalDatabaseBatches: 35, feishuBatchRows: 200, totalFeishuBatches: 16,
    datasets: [{ kind: 'hourly', rows: 1800, databaseBatches: 10, feishuBatches: 9 }],
  } }
  assert.deepEqual(parseMallWeatherCapacityPlan(payload), payload.data)
  assert.equal(parseMallWeatherCapacityPlan({ ...payload, data: { ...payload.data, datasets: [{ ...payload.data.datasets[0], rows: -1 }] } }), null)
  assert.equal(parseMallWeatherCapacityPlan({ ...payload, data: { ...payload.data, providerQps: 0 } }), null)
  assert.equal(parseMallWeatherCapacityPlan({ ...payload, code: 200 }), null)
  assert.equal(parseMallWeatherCapacityPlan({ ...payload, data: { ...payload.data, totalFeishuBatches: undefined } }), null)
  assert.equal(parseMallWeatherCapacityPlan({ ...payload, data: { ...payload.data, providerDrainSeconds: Number.POSITIVE_INFINITY } }), null)
  assert.equal(parseMallWeatherCapacityPlan({ ...payload, data: { ...payload.data, datasets: [{ kind: 'hourly', rows: 1, databaseBatches: 1 }] } }), null)
})
