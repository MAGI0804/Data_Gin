import assert from 'node:assert/strict'
import test from 'node:test'
import { renderToStaticMarkup } from 'react-dom/server'
import { createElement } from 'react'

import { Brand } from '../.test-dist/Brand.js'

test('renders the exact Allblu brand name and accessible logo', () => {
  const markup = renderToStaticMarkup(createElement(Brand, { size: 'large' }))

  assert.match(markup, />Allblu<\/span>/)
  assert.match(markup, /src="\/logo\.jpg"/)
  assert.match(markup, /alt="Allblu Logo"/)
})
