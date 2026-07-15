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
import { parseExpressionAt } from 'acorn'
import { BILLING_CACHE_VAR_MAP } from './billing-expr.ts'

export const CACHE_MODE_TIMED = 'timed'
export const CACHE_MODE_GENERIC = 'generic'
export type CacheMode = typeof CACHE_MODE_TIMED | typeof CACHE_MODE_GENERIC

export type TierConditionInput = {
  var: 'p' | 'c' | 'len'
  op: '<' | '<=' | '>' | '>='
  value: number | string
}

export type VisualTier = {
  label: string
  conditions: TierConditionInput[]
  input_unit_cost: number
  output_unit_cost: number
  cache_mode: CacheMode
  cache_read_unit_cost?: number
  cache_create_unit_cost?: number
  cache_create_1h_unit_cost?: number
  image_unit_cost?: number
  image_output_unit_cost?: number
  audio_input_unit_cost?: number
  audio_output_unit_cost?: number
  [field: string]: unknown
}

export type VisualConfig = {
  tiers: VisualTier[]
}

export function getTierCacheMode(
  tier: Partial<VisualTier> | null | undefined
): CacheMode {
  if (tier?.cache_mode === CACHE_MODE_TIMED) return CACHE_MODE_TIMED
  if (tier?.cache_mode === CACHE_MODE_GENERIC) return CACHE_MODE_GENERIC
  return Number(tier?.cache_create_1h_unit_cost) > 0
    ? CACHE_MODE_TIMED
    : CACHE_MODE_GENERIC
}

export function normalizeVisualTier(
  tier: Partial<VisualTier> = {}
): VisualTier {
  return {
    label: tier.label ?? '',
    input_unit_cost: Number(tier.input_unit_cost) || 0,
    output_unit_cost: Number(tier.output_unit_cost) || 0,
    cache_mode: getTierCacheMode(tier),
    conditions: Array.isArray(tier.conditions) ? tier.conditions : [],
    ...tier,
    cache_read_unit_cost: Number(tier.cache_read_unit_cost) || 0,
    cache_create_unit_cost: Number(tier.cache_create_unit_cost) || 0,
    cache_create_1h_unit_cost: Number(tier.cache_create_1h_unit_cost) || 0,
    image_unit_cost: Number(tier.image_unit_cost) || 0,
    image_output_unit_cost: Number(tier.image_output_unit_cost) || 0,
    audio_input_unit_cost: Number(tier.audio_input_unit_cost) || 0,
    audio_output_unit_cost: Number(tier.audio_output_unit_cost) || 0,
  }
}

export function createDefaultVisualConfig(): VisualConfig {
  return {
    tiers: [
      normalizeVisualTier({
        conditions: [],
        input_unit_cost: 0,
        output_unit_cost: 0,
        label: 'base',
        cache_mode: CACHE_MODE_GENERIC,
      }),
    ],
  }
}

export function normalizeVisualConfig(
  config: VisualConfig | null | undefined
): VisualConfig {
  if (!config || !Array.isArray(config.tiers) || config.tiers.length === 0) {
    return createDefaultVisualConfig()
  }
  return {
    ...config,
    tiers: config.tiers.map((tier) => normalizeVisualTier(tier)),
  }
}

function buildConditionStr(conditions: TierConditionInput[]): string {
  if (!conditions || conditions.length === 0) return ''
  return conditions
    .filter((c) => c.var && c.op && c.value != null && c.value !== '')
    .map((c) => `${c.var} ${c.op} ${c.value}`)
    .join(' && ')
}

function buildTierBodyExpr(tier: VisualTier): string {
  const parts: string[] = []
  const ic = Number(tier.input_unit_cost) || 0
  const oc = Number(tier.output_unit_cost) || 0
  parts.push(`p * ${ic}`)
  parts.push(`c * ${oc}`)
  for (const cv of BILLING_CACHE_VAR_MAP) {
    const v = Number((tier as Record<string, unknown>)[cv.field]) || 0
    if (v !== 0) parts.push(`${cv.exprVar} * ${v}`)
  }
  return parts.join(' + ')
}

export function generateExprFromVisualConfig(
  config: VisualConfig | null | undefined
): string {
  if (!config || !config.tiers || config.tiers.length === 0) {
    return 'p * 0 + c * 0'
  }
  const tiers = config.tiers

  if (tiers.length === 1) {
    const tier = tiers[0]
    const label = tier.label || 'default'
    const body = `tier("${label}", ${buildTierBodyExpr(tier)})`
    const cond = buildConditionStr(tier.conditions)
    if (cond) {
      return `${cond} ? ${body} : p * 0 + c * 0`
    }
    return body
  }

  const parts: string[] = []
  for (let i = 0; i < tiers.length; i++) {
    const tier = tiers[i]
    const label = tier.label || `tier_${i + 1}`
    const body = `tier("${label}", ${buildTierBodyExpr(tier)})`
    const cond = buildConditionStr(tier.conditions)

    if (i < tiers.length - 1 && cond) {
      parts.push(`${cond} ? ${body}`)
    } else {
      parts.push(body)
    }
  }
  return parts.join(' : ')
}

type VisualTierCaptureIndexes = {
  condition?: number
  label: number
  inputCost: number
  outputCost: number
  firstCacheCost: number
}

const SINGLE_VISUAL_TIER_CAPTURES: VisualTierCaptureIndexes = {
  label: 1,
  inputCost: 2,
  outputCost: 3,
  firstCacheCost: 4,
}

const TIERED_VISUAL_TIER_CAPTURES: VisualTierCaptureIndexes = {
  condition: 1,
  label: 2,
  inputCost: 3,
  outputCost: 4,
  firstCacheCost: 5,
}

function stripVisualConfigVersion(expression: string): string {
  return expression.match(/^v\d+:([\s\S]*)$/)?.[1] ?? expression
}

function buildVisualTierBodyPattern(): string {
  const optionalCacheCosts = BILLING_CACHE_VAR_MAP.map(
    ({ exprVar }) => `(?:\\s*\\+\\s*${exprVar}\\s*\\*\\s*([\\d.eE+-]+))?`
  ).join('')
  return `p\\s*\\*\\s*([\\d.eE+-]+)\\s*\\+\\s*c\\s*\\*\\s*([\\d.eE+-]+)${optionalCacheCosts}`
}

function parseTierConditions(source?: string): TierConditionInput[] {
  if (!source) return []
  return source.split(/\s*&&\s*/).flatMap((part) => {
    const match = part.trim().match(/^(p|c|len)\s*(<|<=|>|>=)\s*([\d.eE+]+)$/)
    if (!match) return []
    return [
      {
        var: match[1] as TierConditionInput['var'],
        op: match[2] as TierConditionInput['op'],
        value: Number(match[3]),
      },
    ]
  })
}

function applyCapturedCacheCosts(
  tier: Record<string, unknown>,
  match: RegExpMatchArray,
  firstCapture: number
): void {
  BILLING_CACHE_VAR_MAP.forEach(({ field }, index) => {
    const value = match[firstCapture + index]
    if (value != null) tier[field] = Number(value)
  })
}

function createVisualTierFromMatch(
  match: RegExpMatchArray,
  captures: VisualTierCaptureIndexes
): VisualTier {
  const tier: Record<string, unknown> = {
    conditions: parseTierConditions(
      captures.condition === undefined ? undefined : match[captures.condition]
    ),
    input_unit_cost: Number(match[captures.inputCost]),
    output_unit_cost: Number(match[captures.outputCost]),
    label: match[captures.label],
  }
  applyCapturedCacheCosts(tier, match, captures.firstCacheCost)
  return normalizeVisualTier(tier as Partial<VisualTier>)
}

function parseSingleVisualTier(
  body: string,
  bodyPattern: string
): VisualConfig | undefined {
  const match = body.match(
    new RegExp(`^tier\\("([^"]*)",\\s*${bodyPattern}\\)$`)
  )
  if (!match) return undefined
  return normalizeVisualConfig({
    tiers: [createVisualTierFromMatch(match, SINGLE_VISUAL_TIER_CAPTURES)],
  })
}

function parseTieredVisualConfig(
  body: string,
  bodyPattern: string
): VisualConfig | null {
  const conditionPattern =
    `((?:(?:p|c|len)\\s*(?:<|<=|>|>=)\\s*[\\d.eE+]+)` +
    `(?:\\s*&&\\s*(?:p|c|len)\\s*(?:<|<=|>|>=)\\s*[\\d.eE+]+)*)`
  const tierPattern = new RegExp(
    `(?:${conditionPattern}\\s*\\?\\s*)?tier\\("([^"]*)",\\s*${bodyPattern}\\)`,
    'g'
  )
  const tiers: VisualTier[] = []
  let match: RegExpExecArray | null
  while ((match = tierPattern.exec(body)) !== null) {
    tiers.push(createVisualTierFromMatch(match, TIERED_VISUAL_TIER_CAPTURES))
  }
  if (tiers.length === 0) return null

  const config = normalizeVisualConfig({ tiers })
  const regenerated = generateExprFromVisualConfig(config)
  return regenerated.replace(/\s+/g, '') === body.replace(/\s+/g, '')
    ? config
    : null
}

export function tryParseVisualConfig(
  exprStr: string | null | undefined
): VisualConfig | null {
  if (!exprStr) return null
  try {
    const body = stripVisualConfigVersion(exprStr)
    const bodyPattern = buildVisualTierBodyPattern()
    return (
      parseSingleVisualTier(body, bodyPattern) ??
      parseTieredVisualConfig(body, bodyPattern)
    )
  } catch {
    return null
  }
}

// ---------------------------------------------------------------------------
// Local cost evaluator (for the estimator preview)
// ---------------------------------------------------------------------------

const ESTIMATOR_VARS = [
  { var: 'cr', stateKey: 'cacheReadTokens' },
  { var: 'cc', stateKey: 'cacheCreateTokens' },
  { var: 'cc1h', stateKey: 'cacheCreate1hTokens' },
  { var: 'img', stateKey: 'imageTokens' },
  { var: 'img_o', stateKey: 'imageOutputTokens' },
  { var: 'ai', stateKey: 'audioInputTokens' },
  { var: 'ao', stateKey: 'audioOutputTokens' },
] as const

export type ExtraTokenValues = Record<
  (typeof ESTIMATOR_VARS)[number]['stateKey'],
  number
>

export type EvalResult = {
  cost: number
  matchedTier: string
  error: string | null
}

type BillingExpressionValue = number | string | boolean

type BillingExpressionNode = {
  ['type']: string
  start: number
  end: number
  value?: BillingExpressionValue | null | RegExp | bigint
  name?: string
  operator?: string
  argument?: BillingExpressionNode
  left?: BillingExpressionNode
  right?: BillingExpressionNode
  test?: BillingExpressionNode
  consequent?: BillingExpressionNode
  alternate?: BillingExpressionNode
  callee?: BillingExpressionNode
  arguments?: BillingExpressionNode[]
}

type BillingExpressionContext = {
  variables: Readonly<Record<string, number>>
  onTier: (name: string, value: number) => number
}

const MAX_BILLING_EXPRESSION_LENGTH = 8_192
const MAX_BILLING_EXPRESSION_NODES = 512
const MAX_BILLING_EXPRESSION_DEPTH = 64

type BillingBinaryEvaluator = (
  left: number,
  right: number
) => BillingExpressionValue

type BillingUnaryEvaluator = (
  operand: BillingExpressionValue
) => BillingExpressionValue

type BillingFunctionSpec = {
  minimumArguments: number
  maximumArguments: number
  validateArguments: (argumentsList: BillingExpressionNode[]) => void
  evaluateArguments: (
    values: BillingExpressionValue[],
    context: BillingExpressionContext
  ) => BillingExpressionValue
}

type BillingNodeValidator = (
  node: BillingExpressionNode,
  context: BillingExpressionContext,
  state: { nodes: number },
  depth: number
) => void

type BillingNodeEvaluator = (
  node: BillingExpressionNode,
  context: BillingExpressionContext
) => BillingExpressionValue

function assertBillingExpression(
  condition: boolean,
  message: string
): asserts condition {
  if (!condition) throw new Error(message)
}

function requireBillingNode(
  node: BillingExpressionNode | undefined,
  parentType: string
): BillingExpressionNode {
  assertBillingExpression(
    node !== undefined,
    `Invalid ${parentType} billing expression`
  )
  return node
}

function requireBillingNodeName(
  node: BillingExpressionNode,
  context: string
): string {
  assertBillingExpression(
    node.name !== undefined,
    `Invalid ${context} billing expression name`
  )
  assertBillingExpression(
    node.name.length > 0,
    `Invalid ${context} billing expression name`
  )
  return node.name
}

function requireBillingOperator(node: BillingExpressionNode): string {
  assertBillingExpression(
    node.operator !== undefined,
    `Invalid ${node.type} billing expression operator`
  )
  return node.operator
}

function requireBillingArguments(
  node: BillingExpressionNode
): BillingExpressionNode[] {
  assertBillingExpression(
    node.arguments !== undefined,
    'Invalid billing function arguments'
  )
  return node.arguments
}

function requireBillingNumber(
  value: BillingExpressionValue,
  context: string
): number {
  assertBillingExpression(
    typeof value !== 'string',
    `${context} must produce a finite number`
  )
  assertBillingExpression(
    typeof value !== 'boolean',
    `${context} must produce a finite number`
  )
  assertBillingExpression(
    Number.isFinite(value),
    `${context} must produce a finite number`
  )
  return value
}

function requireBillingString(
  value: BillingExpressionValue,
  context: string
): string {
  assertBillingExpression(
    typeof value !== 'number',
    `${context} must produce a string`
  )
  assertBillingExpression(
    typeof value !== 'boolean',
    `${context} must produce a string`
  )
  return value
}

function requireBillingEntry<T>(
  entries: Readonly<Record<string, T>>,
  key: string,
  context: string
): T {
  assertBillingExpression(
    Object.prototype.hasOwnProperty.call(entries, key),
    `Unsupported ${context}: ${key}`
  )
  return entries[key] as T
}

const BILLING_BINARY_EVALUATORS: Record<string, BillingBinaryEvaluator> = {
  '+': (left, right) => requireBillingNumber(left + right, 'Addition output'),
  '-': (left, right) =>
    requireBillingNumber(left - right, 'Subtraction output'),
  '*': (left, right) =>
    requireBillingNumber(left * right, 'Multiplication output'),
  '/': (left, right) => requireBillingNumber(left / right, 'Division output'),
  '%': (left, right) => requireBillingNumber(left % right, 'Remainder output'),
  '**': (left, right) =>
    requireBillingNumber(left ** right, 'Exponentiation output'),
  '<': (left, right) => left < right,
  '<=': (left, right) => left <= right,
  '>': (left, right) => left > right,
  '>=': (left, right) => left >= right,
  '==': (left, right) => left === right,
  '===': (left, right) => left === right,
  '!=': (left, right) => left !== right,
  '!==': (left, right) => left !== right,
}

const BILLING_UNARY_EVALUATORS: Record<string, BillingUnaryEvaluator> = {
  '!': (operand) => !operand,
  '+': (operand) => requireBillingNumber(operand, 'Unary operand'),
  '-': (operand) => -requireBillingNumber(operand, 'Unary operand'),
}

const BILLING_FUNCTION_SPECS: Record<string, BillingFunctionSpec> = {
  tier: {
    minimumArguments: 2,
    maximumArguments: 2,
    validateArguments: (argumentsList) => {
      const nameNode = requireBillingNode(argumentsList[0], 'tier argument')
      assertBillingExpression(
        nameNode['type'] === 'Literal' && typeof nameNode.value === 'string',
        'tier requires a literal name and a numeric value'
      )
    },
    evaluateArguments: (values, context) =>
      context.onTier(
        requireBillingString(values[0], 'tier name'),
        requireBillingNumber(values[1], 'tier value')
      ),
  },
  max: {
    minimumArguments: 2,
    maximumArguments: 2,
    validateArguments: () => {},
    evaluateArguments: (values) =>
      requireBillingNumber(
        Math.max(
          ...values.map((value) => requireBillingNumber(value, 'max argument'))
        ),
        'max output'
      ),
  },
  min: {
    minimumArguments: 2,
    maximumArguments: 2,
    validateArguments: () => {},
    evaluateArguments: (values) =>
      requireBillingNumber(
        Math.min(
          ...values.map((value) => requireBillingNumber(value, 'min argument'))
        ),
        'min output'
      ),
  },
  abs: {
    minimumArguments: 1,
    maximumArguments: 1,
    validateArguments: () => {},
    evaluateArguments: (values) =>
      Math.abs(requireBillingNumber(values[0], 'abs argument')),
  },
  ceil: {
    minimumArguments: 1,
    maximumArguments: 1,
    validateArguments: () => {},
    evaluateArguments: (values) =>
      Math.ceil(requireBillingNumber(values[0], 'ceil argument')),
  },
  floor: {
    minimumArguments: 1,
    maximumArguments: 1,
    validateArguments: () => {},
    evaluateArguments: (values) =>
      Math.floor(requireBillingNumber(values[0], 'floor argument')),
  },
}

function validateBillingCall(
  node: BillingExpressionNode,
  context: BillingExpressionContext,
  state: { nodes: number },
  depth: number
): void {
  const callee = requireBillingNode(node.callee, node.type)
  assertBillingExpression(
    callee['type'] === 'Identifier',
    'Billing functions must be called by name'
  )

  const functionName = requireBillingNodeName(callee, 'function')
  const functionSpec = requireBillingEntry(
    BILLING_FUNCTION_SPECS,
    functionName,
    'billing function'
  )
  const argumentsList = requireBillingArguments(node)
  assertBillingExpression(
    argumentsList.length >= functionSpec.minimumArguments,
    `${functionName} received too few arguments`
  )
  assertBillingExpression(
    argumentsList.length <= functionSpec.maximumArguments,
    `${functionName} received too many arguments`
  )
  functionSpec.validateArguments(argumentsList)
  argumentsList.forEach((argument) =>
    validateBillingNode(argument, context, state, depth + 1)
  )
}

const BILLING_LOGICAL_OPERATORS = new Set(['&&', '||'])

function validateBillingOperands(
  node: BillingExpressionNode,
  context: BillingExpressionContext,
  state: { nodes: number },
  depth: number
): void {
  validateBillingNode(
    requireBillingNode(node.left, node.type),
    context,
    state,
    depth + 1
  )
  validateBillingNode(
    requireBillingNode(node.right, node.type),
    context,
    state,
    depth + 1
  )
}

const BILLING_NODE_VALIDATORS: Record<string, BillingNodeValidator> = {
  Literal: (node) => {
    const literal = node.value
    assertBillingExpression(
      literal !== undefined,
      'Unsupported billing expression literal'
    )
    assertBillingExpression(
      literal !== null,
      'Unsupported billing expression literal'
    )
    assertBillingExpression(
      typeof literal !== 'object',
      'Unsupported billing expression literal'
    )
    assertBillingExpression(
      typeof literal !== 'bigint',
      'Unsupported billing expression literal'
    )
  },
  Identifier: (node, context) => {
    const name = requireBillingNodeName(node, 'variable')
    assertBillingExpression(
      Object.prototype.hasOwnProperty.call(context.variables, name),
      `Unknown billing variable: ${name}`
    )
  },
  UnaryExpression: (node, context, state, depth) => {
    requireBillingEntry(
      BILLING_UNARY_EVALUATORS,
      requireBillingOperator(node),
      'unary operator'
    )
    validateBillingNode(
      requireBillingNode(node.argument, node.type),
      context,
      state,
      depth + 1
    )
  },
  BinaryExpression: (node, context, state, depth) => {
    requireBillingEntry(
      BILLING_BINARY_EVALUATORS,
      requireBillingOperator(node),
      'billing operator'
    )
    validateBillingOperands(node, context, state, depth)
  },
  LogicalExpression: (node, context, state, depth) => {
    assertBillingExpression(
      BILLING_LOGICAL_OPERATORS.has(requireBillingOperator(node)),
      `Unsupported logical operator: ${node.operator}`
    )
    validateBillingOperands(node, context, state, depth)
  },
  ConditionalExpression: (node, context, state, depth) => {
    validateBillingNode(
      requireBillingNode(node.test, node.type),
      context,
      state,
      depth + 1
    )
    validateBillingNode(
      requireBillingNode(node.consequent, node.type),
      context,
      state,
      depth + 1
    )
    validateBillingNode(
      requireBillingNode(node.alternate, node.type),
      context,
      state,
      depth + 1
    )
  },
  CallExpression: validateBillingCall,
}

function validateBillingNode(
  node: BillingExpressionNode,
  context: BillingExpressionContext,
  state: { nodes: number },
  depth: number
): void {
  state.nodes += 1
  assertBillingExpression(
    state.nodes <= MAX_BILLING_EXPRESSION_NODES,
    'Billing expression is too complex'
  )
  assertBillingExpression(
    depth <= MAX_BILLING_EXPRESSION_DEPTH,
    'Billing expression nesting is too deep'
  )
  requireBillingEntry(
    BILLING_NODE_VALIDATORS,
    node.type,
    'billing expression syntax'
  )(node, context, state, depth)
}

function evaluateBillingCall(
  node: BillingExpressionNode,
  context: BillingExpressionContext
): BillingExpressionValue {
  const callee = requireBillingNode(node.callee, node.type)
  const functionName = requireBillingNodeName(callee, 'function')
  const functionSpec = requireBillingEntry(
    BILLING_FUNCTION_SPECS,
    functionName,
    'billing function'
  )
  const values = requireBillingArguments(node).map((argument) =>
    evaluateBillingNode(argument, context)
  )
  return functionSpec.evaluateArguments(values, context)
}

function evaluateBillingBinary(
  operator: string,
  leftValue: BillingExpressionValue,
  rightValue: BillingExpressionValue
): BillingExpressionValue {
  const left = requireBillingNumber(leftValue, `Left operand of ${operator}`)
  const right = requireBillingNumber(rightValue, `Right operand of ${operator}`)
  return requireBillingEntry(
    BILLING_BINARY_EVALUATORS,
    operator,
    'billing operator'
  )(left, right)
}

type BillingLogicalEvaluator = (
  left: BillingExpressionNode,
  right: BillingExpressionNode,
  context: BillingExpressionContext
) => BillingExpressionValue

const BILLING_LOGICAL_EVALUATORS: Record<string, BillingLogicalEvaluator> = {
  '&&': (leftNode, rightNode, context) => {
    const left = evaluateBillingNode(leftNode, context)
    return left ? evaluateBillingNode(rightNode, context) : left
  },
  '||': (leftNode, rightNode, context) => {
    const left = evaluateBillingNode(leftNode, context)
    return left ? left : evaluateBillingNode(rightNode, context)
  },
}

const BILLING_NODE_EVALUATORS: Record<string, BillingNodeEvaluator> = {
  Literal: (node) => node.value as BillingExpressionValue,
  Identifier: (node, context) =>
    requireBillingNumber(
      context.variables[requireBillingNodeName(node, 'variable')],
      `Billing variable ${node.name}`
    ),
  UnaryExpression: (node, context) =>
    requireBillingEntry(
      BILLING_UNARY_EVALUATORS,
      requireBillingOperator(node),
      'unary operator'
    )(
      evaluateBillingNode(requireBillingNode(node.argument, node.type), context)
    ),
  BinaryExpression: (node, context) =>
    evaluateBillingBinary(
      requireBillingOperator(node),
      evaluateBillingNode(requireBillingNode(node.left, node.type), context),
      evaluateBillingNode(requireBillingNode(node.right, node.type), context)
    ),
  LogicalExpression: (node, context) =>
    requireBillingEntry(
      BILLING_LOGICAL_EVALUATORS,
      requireBillingOperator(node),
      'logical operator'
    )(
      requireBillingNode(node.left, node.type),
      requireBillingNode(node.right, node.type),
      context
    ),
  ConditionalExpression: (node, context) =>
    evaluateBillingNode(requireBillingNode(node.test, node.type), context)
      ? evaluateBillingNode(
          requireBillingNode(node.consequent, node.type),
          context
        )
      : evaluateBillingNode(
          requireBillingNode(node.alternate, node.type),
          context
        ),
  CallExpression: evaluateBillingCall,
}

function evaluateBillingNode(
  node: BillingExpressionNode,
  context: BillingExpressionContext
): BillingExpressionValue {
  return requireBillingEntry(
    BILLING_NODE_EVALUATORS,
    node.type,
    'billing expression syntax'
  )(node, context)
}

function evaluateBillingExpression(
  expression: string,
  context: BillingExpressionContext
): number {
  const source = expression.trim()
  if (source.length > MAX_BILLING_EXPRESSION_LENGTH) {
    throw new Error('Billing expression is too long')
  }

  const node = parseExpressionAt(source, 0, {
    ecmaVersion: 'latest',
  }) as unknown as BillingExpressionNode
  if (source.slice(node.end).trim()) {
    throw new Error('Billing expression contains trailing syntax')
  }

  validateBillingNode(node, context, { nodes: 0 }, 0)
  return requireBillingNumber(
    evaluateBillingNode(node, context),
    'Billing expression'
  )
}

export function evalExprLocally(
  exprStr: string,
  promptTokens: number,
  completionTokens: number,
  extraTokenValues: ExtraTokenValues
): EvalResult {
  try {
    if (!exprStr || !exprStr.trim()) {
      return { cost: 0, matchedTier: '', error: null }
    }
    let matchedTier = ''
    const tierFn = (name: string, value: number) => {
      matchedTier = name
      return value
    }
    const cacheReadTokens = extraTokenValues.cacheReadTokens || 0
    const cacheCreateTokens = extraTokenValues.cacheCreateTokens || 0
    const cacheCreate1hTokens = extraTokenValues.cacheCreate1hTokens || 0
    const len =
      promptTokens + cacheReadTokens + cacheCreateTokens + cacheCreate1hTokens
    const variables: Record<string, number> = {
      p: promptTokens,
      c: completionTokens,
      len,
    }
    for (const field of ESTIMATOR_VARS) {
      variables[field.var] = extraTokenValues[field.stateKey] || 0
    }
    const cost = evaluateBillingExpression(exprStr, {
      variables,
      onTier: tierFn,
    })
    return { cost, matchedTier, error: null }
  } catch (e) {
    const message = e instanceof Error ? e.message : String(e)
    return { cost: 0, matchedTier: '', error: message }
  }
}

export function exprUsesExtraVars(exprStr: string): boolean {
  if (!exprStr) return false
  const varNames = ESTIMATOR_VARS.map((f) => f.var).join('|')
  return new RegExp(`\\b(${varNames})\\b`).test(exprStr)
}
