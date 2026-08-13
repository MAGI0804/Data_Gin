import assert from 'node:assert/strict'
import test from 'node:test'

import {
  clearStoredToken,
  defaultTokenLifetimeMs,
  loadStoredToken,
  saveStoredToken,
  storedTokenExpiresAt,
  tokenActorID,
  tokenExpiresAt,
} from '../.test-dist/authStorage.js'

class MemoryStorage {
  #values = new Map()

  getItem(key) {
    return this.#values.get(key) ?? null
  }

  setItem(key, value) {
    this.#values.set(key, String(value))
  }

  removeItem(key) {
    this.#values.delete(key)
  }
}

function encodeToken(value) {
  return Buffer.from(value, 'utf8').toString('base64url')
}

test('reads expiry from full and compact backend token formats', () => {
  assert.equal(tokenExpiresAt(encodeToken('42:r:2000000000:1900000000:17:abcdef12')), 2_000_000_000_000)
  assert.equal(tokenExpiresAt(encodeToken('42:2000000000:abcdef12')), 2_000_000_000_000)
})

test('reads a stable actor id only from supported backend token formats', () => {
  assert.equal(tokenActorID(encodeToken('42:r:2000000000:1900000000:17:abcdef12')), '42')
  assert.equal(tokenActorID(encodeToken('7:2000000000:abcdef12')), '7')
  assert.equal(tokenActorID(encodeToken('0:2000000000:abcdef12')), null)
  assert.equal(tokenActorID(encodeToken('42:r:2000000000:1900000000:abcdef12')), null)
  assert.equal(tokenActorID(encodeToken('42:r:2000000000:abcdef12')), null)
  assert.equal(tokenActorID('not-a-valid-token'), null)
})

test('stores a malformed token for the default one-day lifetime', () => {
  const storage = new MemoryStorage()
  const now = 1_900_000_000_000

  assert.equal(saveStoredToken('not-a-valid-token', storage, now), now + defaultTokenLifetimeMs)
  assert.equal(loadStoredToken(storage, now + 1), 'not-a-valid-token')
  assert.equal(storedTokenExpiresAt(storage), now + defaultTokenLifetimeMs)
})

test('removes token state at or after its expiry time', () => {
  const storage = new MemoryStorage()
  const expirySeconds = 2_000_000_000
  const token = encodeToken(`42:${expirySeconds}:abcdef12`)

  saveStoredToken(token, storage, 1_900_000_000_000)
  assert.equal(loadStoredToken(storage, expirySeconds * 1000 - 1), token)
  assert.equal(loadStoredToken(storage, expirySeconds * 1000), '')
  assert.equal(storedTokenExpiresAt(storage), null)
})

test('clearStoredToken removes both the token and expiry metadata', () => {
  const storage = new MemoryStorage()
  saveStoredToken('token', storage, 1_900_000_000_000)
  storage.setItem('warehouse-session-user', '{"id":1}')

  clearStoredToken(storage)

  assert.equal(loadStoredToken(storage, 1_900_000_000_001), '')
  assert.equal(storedTokenExpiresAt(storage), null)
  assert.equal(storage.getItem('warehouse-session-user'), null)
})
