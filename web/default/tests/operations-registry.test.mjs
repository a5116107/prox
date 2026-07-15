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
import fs from 'node:fs'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'

const webRoot = fileURLToPath(new URL('..', import.meta.url))
const operationsDir = path.join(
  webRoot,
  'src',
  'features',
  'system-settings',
  'operations'
)

const registryFiles = [
  'ops-registry-contract.ts',
  'ops-registry-normalizers.ts',
  'ops-registry-queries.ts',
  'ops-registry-mutations.ts',
  'use-ops-registry.ts',
]

const expectedApiCalls = [
  'get:/api/ops/access-policy/${encodeURIComponent(siteId)}/explain-user/${encodeURIComponent(String(userId))}',
  'get:/api/ops/audits/${encodeURIComponent(siteId)}',
  'get:/api/ops/community-gate/${encodeURIComponent(siteId)}',
  'get:/api/ops/control-plane/site/${encodeURIComponent(siteId)}',
  'get:/api/ops/fund/${encodeURIComponent(siteId)}',
  'get:/api/ops/group-capabilities/${encodeURIComponent(siteId)}',
  'get:/api/ops/groups',
  'get:/api/ops/invite-journey/${encodeURIComponent(siteId)}',
  'get:/api/ops/releases/${encodeURIComponent(siteId)}/impact-preview',
  'get:/api/ops/releases/${encodeURIComponent(siteId)}',
  'post:/api/ops/groups',
  'post:/api/ops/groups/${encodeURIComponent(String(id))}/clone',
  'post:/api/ops/groups/bulk',
  'post:/api/ops/releases/${encodeURIComponent(siteId)}/publish',
  'post:/api/ops/releases/${encodeURIComponent(siteId)}/rollback',
  'put:/api/ops/groups/${encodeURIComponent(String(id))}/chatops',
  'put:/api/ops/groups/${encodeURIComponent(String(id))}/games',
  'put:/api/ops/groups/${encodeURIComponent(String(id))}',
].sort()

const expectedQueryKeys = [
  'ops-registry-${name}|siteId',
  'ops-registry-audits|siteId',
  'ops-registry-community-gate|siteId',
  'ops-registry-control-plane|siteId',
  'ops-registry-fund|siteId',
  'ops-registry-group-capabilities|siteId',
  'ops-registry-groups|siteId',
  'ops-registry-invite-journey|siteId',
  'ops-registry-release-impact|siteId',
  'ops-registry-releases|siteId',
].sort()

const expectedScopeKeys = [
  'audits',
  'communityGate',
  'controlPlane',
  'groupCapabilities',
  'groups',
  'inviteJourney',
  'releaseImpact',
  'releases',
  'rewardFund',
].sort()

const expectedPublicTypes = [
  'OpsAccessExplainRequest',
  'OpsAccessExplainResult',
  'OpsAuditOverview',
  'OpsCommunityGateOverview',
  'OpsControlPlaneField',
  'OpsControlPlaneSnapshot',
  'OpsControlPlaneSource',
  'OpsGroupActions',
  'OpsGroupBulkSaveFailure',
  'OpsGroupBulkSavePayload',
  'OpsGroupBulkSaveResult',
  'OpsGroupCapabilityMatrixItem',
  'OpsGroupCapabilityMatrixOverview',
  'OpsGroupChatOpsSavePayload',
  'OpsGroupGamesSavePayload',
  'OpsGroupGameSaveItem',
  'OpsGroupSavePayload',
  'OpsInviteJourneyOverview',
  'OpsRegistryGroup',
  'OpsRegistryScope',
  'OpsReleaseActions',
  'OpsReleaseImpactDiffBucket',
  'OpsReleaseImpactPreview',
  'OpsReleaseImpactState',
  'OpsReleaseOverview',
  'OpsReleasePublishPayload',
  'OpsReleaseRecord',
  'OpsReleaseRollbackPayload',
  'OpsRewardFundOverview',
  'OpsUnifiedAuditEvent',
].sort()

function readOperationsFile(fileName) {
  return fs.readFileSync(path.join(operationsDir, fileName), 'utf8')
}

function parseOperationsFile(fileName) {
  return ts.createSourceFile(
    fileName,
    readOperationsFile(fileName),
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TS
  )
}

function walk(node, visitor) {
  visitor(node)
  ts.forEachChild(node, (child) => walk(child, visitor))
}

function visitRegistryNodes(visitor) {
  registryFiles.forEach((fileName) => {
    const parsedFile = parseOperationsFile(fileName)
    walk(parsedFile, (node) => visitor(node, parsedFile, fileName))
  })
}

function serializeExpression(node, sourceFile) {
  if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) {
    return node.text
  }
  if (ts.isTemplateExpression(node)) {
    return node.templateSpans.reduce(
      (value, span) =>
        `${value}\${${span.expression
          .getText(sourceFile)
          .replace(/\s+/g, '')}}${span.literal.text}`,
      node.head.text
    )
  }
  return node.getText(sourceFile).replace(/\s+/g, '')
}

function collectApiCalls() {
  const calls = []
  visitRegistryNodes((node, sourceFile, fileName) => {
    if (
      !ts.isCallExpression(node) ||
      !ts.isPropertyAccessExpression(node.expression) ||
      !ts.isIdentifier(node.expression.expression) ||
      node.expression.expression.text !== 'api'
    ) {
      return
    }
    assert.ok(node.arguments[0], `API path missing in ${fileName}`)
    calls.push(
      `${node.expression.name.text}:${serializeExpression(
        node.arguments[0],
        sourceFile
      )}`
    )
  })
  return calls.sort()
}

function collectQueryKeys() {
  const keys = []
  visitRegistryNodes((node, sourceFile) => {
    if (
      !ts.isPropertyAssignment(node) ||
      node.name.getText(sourceFile) !== 'queryKey' ||
      !ts.isArrayLiteralExpression(node.initializer)
    ) {
      return
    }
    keys.push(
      node.initializer.elements
        .map((element) => serializeExpression(element, sourceFile))
        .join('|')
    )
  })
  return keys.sort()
}

function collectScopeKeys() {
  const sourceFile = parseOperationsFile('ops-registry-contract.ts')
  for (const statement of sourceFile.statements) {
    if (!ts.isVariableStatement(statement)) continue
    for (const declaration of statement.declarationList.declarations) {
      if (
        !ts.isIdentifier(declaration.name) ||
        declaration.name.text !== 'OPS_REGISTRY_DEFAULT_SCOPE' ||
        !declaration.initializer ||
        !ts.isObjectLiteralExpression(declaration.initializer)
      ) {
        continue
      }
      return declaration.initializer.properties
        .map((property) => property.name?.getText(sourceFile))
        .filter(Boolean)
        .sort()
    }
  }
  assert.fail('OPS_REGISTRY_DEFAULT_SCOPE is missing')
}

function collectFacadeTypeExports() {
  const sourceFile = parseOperationsFile('use-ops-registry.ts')
  return sourceFile.statements
    .filter(
      (statement) =>
        ts.isExportDeclaration(statement) &&
        statement.isTypeOnly &&
        statement.exportClause &&
        ts.isNamedExports(statement.exportClause)
    )
    .flatMap((statement) =>
      statement.exportClause.elements.map((element) => element.name.text)
    )
    .sort()
}

async function loadNormalizers() {
  const source = readOperationsFile('ops-registry-normalizers.ts')
  const result = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
    },
    fileName: 'ops-registry-normalizers.ts',
    reportDiagnostics: true,
  })
  const errors = (result.diagnostics ?? []).filter(
    (diagnostic) => diagnostic.category === ts.DiagnosticCategory.Error
  )
  assert.deepEqual(errors, [])
  const sourceUrl = `data:text/javascript;base64,${Buffer.from(
    result.outputText
  ).toString('base64')}`
  return import(sourceUrl)
}

test('keeps the registry API, query keys, scope, and facade exports stable', () => {
  assert.deepEqual(collectApiCalls(), expectedApiCalls)
  assert.deepEqual(collectQueryKeys(), expectedQueryKeys)
  assert.deepEqual(collectScopeKeys(), expectedScopeKeys)
  assert.deepEqual(collectFacadeTypeExports(), expectedPublicTypes)
})

test('keeps registry modules inside their ownership boundaries', () => {
  const boundaries = {
    'ops-registry-contract.ts': {
      maxLines: 450,
      imports: [],
    },
    'ops-registry-normalizers.ts': {
      maxLines: 600,
      imports: ['./ops-registry-contract'],
    },
    'ops-registry-queries.ts': {
      maxLines: 360,
      imports: [
        './ops-registry-contract',
        './ops-registry-normalizers',
        '@/lib/api',
        '@tanstack/react-query',
      ],
    },
    'ops-registry-mutations.ts': {
      maxLines: 380,
      imports: [
        './ops-registry-contract',
        './ops-registry-normalizers',
        '@/lib/api',
        '@tanstack/react-query',
      ],
    },
    'use-ops-registry.ts': {
      maxLines: 160,
      imports: [
        './ops-registry-contract',
        './ops-registry-mutations',
        './ops-registry-normalizers',
        './ops-registry-queries',
        '@tanstack/react-query',
        'react',
      ],
    },
  }

  Object.entries(boundaries).forEach(([fileName, boundary]) => {
    const source = readOperationsFile(fileName)
    const sourceFile = parseOperationsFile(fileName)
    const imports = sourceFile.statements
      .filter(ts.isImportDeclaration)
      .map((statement) => statement.moduleSpecifier.text)
      .sort()

    assert.ok(
      source.split(/\r?\n/).length <= boundary.maxLines,
      `${fileName} exceeds ${boundary.maxLines} lines`
    )
    assert.deepEqual(imports, boundary.imports.sort(), fileName)
  })

  const facade = parseOperationsFile('use-ops-registry.ts')
  const facadeFunctions = facade.statements
    .filter(ts.isFunctionDeclaration)
    .map((declaration) => declaration.name?.text)
    .filter(Boolean)
    .sort()
  assert.deepEqual(facadeFunctions, [
    'buildOpsRegistrySnapshot',
    'useOpsRegistry',
  ])
  assert.equal(
    facade.statements.some(
      (statement) =>
        ts.isInterfaceDeclaration(statement) ||
        ts.isTypeAliasDeclaration(statement) ||
        ts.isEnumDeclaration(statement)
    ),
    false
  )

  const forbiddenCalls = []
  walk(facade, (node) => {
    if (
      ts.isCallExpression(node) &&
      ts.isIdentifier(node.expression) &&
      ['useMutation', 'useQuery'].includes(node.expression.text)
    ) {
      forbiddenCalls.push(node.expression.text)
    }
  })
  assert.deepEqual(forbiddenCalls, [])
})

test('normalizes optional audit numbers without changing legacy semantics', async () => {
  const { normalizeAuditPayload } = await loadNormalizers()
  const result = normalizeAuditPayload({
    site_id: 'prox',
    summary: { total: 4 },
    events: [
      { id: 'undefined' },
      { id: 'zero', actor_user_id: 0, user_id: 0, at: 0 },
      { id: 'null', actor_user_id: null, user_id: null, at: null },
      {
        id: 42,
        domain: 'membership',
        event_type: 'invite_reward',
        title: 'Invite reward',
        subject: 7,
        status: 'completed',
        severity: 'info',
        reason_code: 'rewarded',
        reason_message: 'Reward issued',
        actor: 'system',
        actor_user_id: '42',
        user_id: '7',
        room_id: 10001,
        provider_slug: 'qq',
        access_level: 'member',
        at: '1720000000',
        raw: { trace_id: 'audit-42' },
      },
    ],
  })

  assert.equal(result.site_id, 'prox')
  assert.deepEqual(result.summary, { total: 4 })
  assert.deepEqual(
    result.events.map(({ id, actor_user_id, user_id, at }) => ({
      id,
      actor_user_id,
      user_id,
      at,
    })),
    [
      {
        id: 'undefined',
        actor_user_id: undefined,
        user_id: undefined,
        at: undefined,
      },
      { id: 'zero', actor_user_id: 0, user_id: 0, at: 0 },
      { id: 'null', actor_user_id: 0, user_id: 0, at: 0 },
      { id: '42', actor_user_id: 42, user_id: 7, at: 1720000000 },
    ]
  )
  assert.deepEqual(result.events[3], {
    id: '42',
    domain: 'membership',
    event_type: 'invite_reward',
    title: 'Invite reward',
    subject: '7',
    status: 'completed',
    severity: 'info',
    reason_code: 'rewarded',
    reason_message: 'Reward issued',
    actor: 'system',
    actor_user_id: 42,
    user_id: 7,
    room_id: '10001',
    provider_slug: 'qq',
    access_level: 'member',
    at: 1720000000,
    raw: { trace_id: 'audit-42' },
  })
})
