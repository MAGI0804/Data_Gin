import assert from 'node:assert/strict'
import test from 'node:test'

import {
  mallWeatherDataNavigationItems,
  navigateMallWeatherSection,
} from '../.test-dist/mallWeatherNavigation.js'

test('defines unique weather navigation targets in the intended information order', () => {
  assert.deepEqual(
    mallWeatherDataNavigationItems.map((item) => [item.targetID, item.label, item.requiresActor]),
    [
      ['mall-weather-overview', '当前实况', false],
      ['mall-weather-minutely', '约 1 km 分钟降水', false],
      ['mall-weather-hourly', '未来逐小时预报', false],
      ['mall-weather-daily', '15 天逐日预报', false],
      ['mall-weather-alerts', '气象预警', false],
      ['mall-weather-life-indices', '15 天生活指数', false],
      ['mall-weather-export', '导出 Excel', true],
      ['mall-weather-management', '管理操作', true],
    ],
  )
  assert.equal(new Set(mallWeatherDataNavigationItems.map((item) => item.targetID)).size, mallWeatherDataNavigationItems.length)
})

test('opens folded forecast details and respects reduced motion before focusing', () => {
  const events = []
  const target = {
    tagName: 'DETAILS',
    _open: false,
    get open() { return this._open },
    set open(value) {
      this._open = value
      events.push(['open', value])
    },
    scrollIntoView(options) { events.push(['scroll', options]) },
    focus(options) { events.push(['focus', options]) },
  }
  const documentRef = { getElementById: (id) => id === 'mall-weather-daily' ? target : null }

  assert.equal(navigateMallWeatherSection(documentRef, 'mall-weather-daily', true), true)
  assert.equal(target.open, true)
  assert.deepEqual(events, [
    ['open', true],
    ['scroll', { behavior: 'auto', block: 'start' }],
    ['focus', { preventScroll: true }],
  ])
})

test('uses smooth scrolling for available sections and rejects unknown targets', () => {
  const events = []
  const target = {
    tagName: 'SECTION',
    scrollIntoView(options) { events.push(['scroll', options]) },
    focus(options) { events.push(['focus', options]) },
  }
  const documentRef = { getElementById: () => target }

  assert.equal(navigateMallWeatherSection(documentRef, 'mall-weather-hourly'), true)
  assert.deepEqual(events, [
    ['scroll', { behavior: 'smooth', block: 'start' }],
    ['focus', { preventScroll: true }],
  ])
  assert.equal(navigateMallWeatherSection(documentRef, 'unknown-target'), false)
  assert.equal(navigateMallWeatherSection({ getElementById: () => null }, 'mall-weather-overview'), false)
})
