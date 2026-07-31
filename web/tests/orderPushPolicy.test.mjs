import assert from 'node:assert/strict'
import test from 'node:test'

import { runSingleFlight, validateOrderPushSkipPolicy } from '../.test-dist/orderPushPolicy.js'

test('rejects order push skip policies that the backend does not accept', () => {
  assert.equal(validateOrderPushSkipPolicy([{ cycle: 5, skip: 1 }]), '')
  assert.match(validateOrderPushSkipPolicy([{ cycle: 5, skip: 5 }]), /小于循环总单数/)
  assert.match(validateOrderPushSkipPolicy([{ cycle: 0, skip: 1 }]), /循环为 0/)
})

test('runs only one order push policy save until the current save settles', async () => {
  const lock = { current: false }
  let calls = 0
  let release
  const pending = new Promise((resolve) => { release = resolve })
  const first = runSingleFlight(lock, async () => {
    calls += 1
    await pending
    return 'saved'
  })

  assert.ok(first)
  assert.equal(runSingleFlight(lock, async () => { calls += 1; return 'duplicate' }), null)
  assert.equal(calls, 1)
  release()
  assert.equal(await first, 'saved')
  assert.equal(lock.current, false)
  assert.equal(await runSingleFlight(lock, async () => { calls += 1; return 'saved again' }), 'saved again')
  assert.equal(calls, 2)
})

test('releases the order push policy save lock when a save fails', async () => {
  const lock = { current: false }
  await assert.rejects(runSingleFlight(lock, async () => { throw new Error('save failed') }), /save failed/)
  assert.equal(lock.current, false)
  assert.equal(await runSingleFlight(lock, async () => 'retry saved'), 'retry saved')
})
