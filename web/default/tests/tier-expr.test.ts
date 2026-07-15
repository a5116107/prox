/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import {
  evalExprLocally,
  generateExprFromVisualConfig,
  tryParseVisualConfig,
  type ExtraTokenValues,
} from '../src/features/pricing/lib/tier-expr.ts'

const EMPTY_EXTRAS: ExtraTokenValues = {
  cacheReadTokens: 0,
  cacheCreateTokens: 0,
  cacheCreate1hTokens: 0,
  imageTokens: 0,
  imageOutputTokens: 0,
  audioInputTokens: 0,
  audioOutputTokens: 0,
}

describe('tryParseVisualConfig', () => {
  test('parses a versioned single tier with cache pricing', () => {
    const config = tryParseVisualConfig(
      'v2:tier("base", p * 1.5 + c * 2 + cr * 0.25)'
    )

    assert.equal(config?.tiers.length, 1)
    assert.equal(config?.tiers[0].label, 'base')
    assert.equal(config?.tiers[0].input_unit_cost, 1.5)
    assert.equal(config?.tiers[0].output_unit_cost, 2)
    assert.equal(config?.tiers[0].cache_read_unit_cost, 0.25)
  })

  test('round-trips ordered tiers and their conditions', () => {
    const expression =
      'len <= 100 && c > 0 ? tier("short", p * 1 + c * 2) : tier("long", p * 3 + c * 4)'
    const config = tryParseVisualConfig(expression)

    assert.equal(config?.tiers.length, 2)
    assert.deepEqual(config?.tiers[0].conditions, [
      { var: 'len', op: '<=', value: 100 },
      { var: 'c', op: '>', value: 0 },
    ])
    assert.equal(config?.tiers[1].label, 'long')
    assert.equal(generateExprFromVisualConfig(config), expression)
  })

  test('rejects expressions outside the visual editor grammar', () => {
    assert.equal(tryParseVisualConfig('tier("base", p + c)'), null)
    assert.equal(
      tryParseVisualConfig(
        'len < 100 ? tier("short", p * 1 + c * 1) : arbitrary(p)'
      ),
      null
    )
  })
})

describe('evalExprLocally', () => {
  test('evaluates supported arithmetic and records the selected tier', () => {
    const result = evalExprLocally(
      'tier("base", p * 2 + c * 4)',
      100,
      10,
      EMPTY_EXTRAS
    )

    assert.deepEqual(result, {
      cost: 240,
      matchedTier: 'base',
      error: null,
    })
  })

  test('evaluates conditional tiers and extra token variables', () => {
    const result = evalExprLocally(
      'len <= 200 ? tier("short", p + cr * 0.5) : tier("long", p * 2)',
      100,
      0,
      { ...EMPTY_EXTRAS, cacheReadTokens: 20 }
    )

    assert.deepEqual(result, {
      cost: 110,
      matchedTier: 'short',
      error: null,
    })
  })

  test('supports the documented numeric helper functions', () => {
    const result = evalExprLocally(
      'tier("helpers", max(abs(-3), min(8, ceil(2.2) + floor(1.9))))',
      0,
      0,
      EMPTY_EXTRAS
    )

    assert.deepEqual(result, {
      cost: 4,
      matchedTier: 'helpers',
      error: null,
    })
  })

  test('short-circuits conditions so only the selected tier is recorded', () => {
    const result = evalExprLocally(
      'len < 100 && c < 10 ? tier("small", 1) : tier("large", 2)',
      150,
      1,
      EMPTY_EXTRAS
    )

    assert.deepEqual(result, {
      cost: 2,
      matchedTier: 'large',
      error: null,
    })
  })

  test('rejects access to the JavaScript runtime without causing side effects', () => {
    const probeKey = '__billingEvalProbe'
    Reflect.deleteProperty(globalThis, probeKey)

    const result = evalExprLocally(
      `globalThis.${probeKey} = 1`,
      0,
      0,
      EMPTY_EXTRAS
    )

    assert.notEqual(result.error, null)
    assert.equal(Reflect.has(globalThis, probeKey), false)
  })

  const rejectedExpressions = [
    ['unknown variable', 'tier("bad", process)'],
    ['member access', 'tier("bad", globalThis.process)'],
    ['inherited function name', 'constructor(1)'],
    ['assignment', 'p = 1'],
    ['trailing syntax', 'tier("base", p); tier("other", c)'],
    ['non-finite result', 'tier("bad", p / 0)'],
    ['too few helper arguments', 'max(p)'],
    ['too many helper arguments', 'abs(p, c)'],
    [
      'forbidden syntax in an unselected branch',
      'p > 0 ? tier("ok", 1) : globalThis.process',
    ],
  ] as const

  for (const [name, expression] of rejectedExpressions) {
    test(`rejects ${name}`, () => {
      const result = evalExprLocally(expression, 10, 5, EMPTY_EXTRAS)

      assert.equal(result.cost, 0)
      assert.equal(result.matchedTier, '')
      assert.notEqual(result.error, null)
    })
  }
})
