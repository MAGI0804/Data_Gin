import assert from 'node:assert/strict'
import test from 'node:test'

import { createSafeBrowserStorage } from '../.test-dist/browserStorage.js'

test('falls back to memory when the browser storage property is unavailable', () => {
  const storage = createSafeBrowserStorage(() => { throw new DOMException('blocked', 'SecurityError') })
  storage.setItem('session', 'active')
  assert.equal(storage.getItem('session'), 'active')
  storage.removeItem('session')
  assert.equal(storage.getItem('session'), null)
})

test('keeps the current page usable when individual storage operations fail', () => {
  const storage = createSafeBrowserStorage(() => ({
    getItem() { throw new DOMException('blocked', 'SecurityError') },
    setItem() { throw new DOMException('blocked', 'SecurityError') },
    removeItem() { throw new DOMException('blocked', 'SecurityError') },
  }))
  storage.setItem('pending', 'request')
  assert.equal(storage.getItem('pending'), 'request')
  storage.removeItem('pending')
  assert.equal(storage.getItem('pending'), null)
})

test('mirrors successful browser values into the fallback copy', () => {
  const values = new Map([['token', 'persisted']])
  let blocked = false
  const storage = createSafeBrowserStorage(() => ({
    getItem(key) { if (blocked) throw new Error('blocked'); return values.get(key) ?? null },
    setItem(key, value) { if (blocked) throw new Error('blocked'); values.set(key, value) },
    removeItem(key) { if (blocked) throw new Error('blocked'); values.delete(key) },
  }))
  assert.equal(storage.getItem('token'), 'persisted')
  blocked = true
  assert.equal(storage.getItem('token'), 'persisted')
})
