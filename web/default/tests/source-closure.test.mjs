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
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const communityOpsFile = fileURLToPath(
  new URL(
    '../src/features/system-settings/operations/community-ops-section.tsx',
    import.meta.url
  )
)
const promptInputFile = fileURLToPath(
  new URL('../src/components/ai-elements/prompt-input.tsx', import.meta.url)
)
const modelDetailsFiles = [
  'model-details.tsx',
  'model-details-drawer.tsx',
  'model-details-overview.tsx',
  'model-details-page.tsx',
  'model-details-section-title.tsx',
].map((file) =>
  fileURLToPath(
    new URL(`../src/features/pricing/components/${file}`, import.meta.url)
  )
)
const pricingRouteFile = fileURLToPath(
  new URL('../src/routes/pricing/$modelId/index.tsx', import.meta.url)
)
const pricingComponentsIndexFile = fileURLToPath(
  new URL('../src/features/pricing/components/index.ts', import.meta.url)
)
const opsI18nFile = fileURLToPath(
  new URL(
    '../src/features/system-settings/operations/ops-i18n.ts',
    import.meta.url
  )
)
const usersTableFile = fileURLToPath(
  new URL('../src/features/users/components/users-table.tsx', import.meta.url)
)
const userRowActionsFile = fileURLToPath(
  new URL(
    '../src/features/users/components/data-table-row-actions.tsx',
    import.meta.url
  )
)
const userBulkActionsFile = fileURLToPath(
  new URL(
    '../src/features/users/components/data-table-bulk-actions.tsx',
    import.meta.url
  )
)
const apiKeysIndexFile = fileURLToPath(
  new URL('../src/features/keys/index.tsx', import.meta.url)
)
const apiKeysDialogsFile = fileURLToPath(
  new URL(
    '../src/features/keys/components/api-keys-dialogs.tsx',
    import.meta.url
  )
)
const apiKeysPrimaryButtonsFile = fileURLToPath(
  new URL(
    '../src/features/keys/components/api-keys-primary-buttons.tsx',
    import.meta.url
  )
)
const apiKeysRowActionsFile = fileURLToPath(
  new URL(
    '../src/features/keys/components/data-table-row-actions.tsx',
    import.meta.url
  )
)
const riskControlSectionFile = fileURLToPath(
  new URL(
    '../src/features/system-settings/operations/risk-control-section.tsx',
    import.meta.url
  )
)
const riskControlDetailFile = fileURLToPath(
  new URL(
    '../src/features/system-settings/operations/risk-control-detail-sheet.tsx',
    import.meta.url
  )
)

test('community operations mounts the editable access-control section', () => {
  const source = fs.readFileSync(communityOpsFile, 'utf8')

  assert.match(
    source,
    /import \{ AccessControlSection \} from ['"]\.\/access-control-section['"]/
  )
  assert.match(source, /id=['"]community-access-control['"]/)
  assert.match(
    source,
    /<AccessControlSection\s+defaultValues=\{defaultValues\}\s+embedded\s*\/>/
  )
})

test('operations translation callback remains stable across state updates', () => {
  const source = fs.readFileSync(opsI18nFile, 'utf8')

  assert.match(source, /import \{ useCallback \} from ['"]react['"]/)
  assert.match(source, /return useCallback\([\s\S]*?\[t\]\s*\)/)
})

test('admin users page keeps membership operations and override controls', () => {
  const tableSource = fs.readFileSync(usersTableFile, 'utf8')
  const actionsSource = fs.readFileSync(userRowActionsFile, 'utf8')
  const bulkActionsSource = fs.readFileSync(userBulkActionsFile, 'utf8')

  assert.match(tableSource, /getAdminUserOpsGrid/)
  assert.match(tableSource, /['"]admin-users-ops-grid['"]/)
  assert.match(actionsSource, /<UserOpsProfileDialog/)
  assert.match(actionsSource, /<UserAccessOverrideDialog/)
  assert.match(actionsSource, /!user\.can_restore/)
  assert.match(bulkActionsSource, /user\.has_active_risk_control/)
})

test('API key page keeps binding gates and risk activation entrypoints', () => {
  const indexSource = fs.readFileSync(apiKeysIndexFile, 'utf8')
  const dialogsSource = fs.readFileSync(apiKeysDialogsFile, 'utf8')
  const primaryButtonsSource = fs.readFileSync(
    apiKeysPrimaryButtonsFile,
    'utf8'
  )
  const rowActionsSource = fs.readFileSync(apiKeysRowActionsFile, 'utf8')

  assert.match(indexSource, /<CommunityGateBanner\s*\/>/)
  assert.match(dialogsSource, /<CommunityGateDialog/)
  assert.match(primaryButtonsSource, /getMyAccessControlStatus/)
  assert.match(primaryButtonsSource, /<AccessControlDialog/)
  assert.match(rowActionsSource, /<RiskActivationDialog/)
  assert.match(rowActionsSource, /regenerateRiskActivationCode/)
  assert.match(rowActionsSource, /onRegenerate=/)

  const mutateDrawerSource = fs.readFileSync(
    fileURLToPath(
      new URL(
        '../src/features/keys/components/api-keys-mutate-drawer.tsx',
        import.meta.url
      )
    ),
    'utf8'
  )
  assert.match(mutateDrawerSource, /activationResults/)
  assert.match(mutateDrawerSource, /result\.data\.risk_activation_bind_code/)
  assert.match(mutateDrawerSource, /<RiskActivationBatchDialog/)
})

test('risk audit queue exposes active reviews and follows the server action matrix', () => {
  const sectionSource = fs.readFileSync(riskControlSectionFile, 'utf8')
  const detailSource = fs.readFileSync(riskControlDetailFile, 'utf8')

  assert.match(sectionSource, /status: ['"]active['"]/)
  assert.match(sectionSource, /item === ['"]active['"]/)
  assert.match(detailSource, /canDisableKeys = \['open', 'reviewing'\]/)
  assert.match(detailSource, /canRestoreKeys = \['open', 'reviewing'\]/)
  assert.match(detailSource, /matched_user_controls/)
  assert.match(
    detailSource,
    /Restore is limited to keys and account restrictions owned by this audit/
  )
})

test('knip follows runtime, declaration, localization, and stylesheet roots', async () => {
  const { default: config } = await import('../knip.config.ts')

  assert.deepEqual(config.entry, [
    'src/main.tsx',
    'src/**/*.d.ts',
    'src/i18n/static-keys.ts',
    'src/features/system-settings/operations/use-ops-registry.ts',
  ])
  assert.deepEqual(config.project, ['src/**/*.{ts,tsx,css}'])
  assert.deepEqual(config.ignore, ['src/components/ui/**'])
  assert.deepEqual(config.ignoreDependencies, [
    'embla-carousel-react',
    'react-resizable-panels',
    'recharts',
  ])
})

test('prompt input has no unreachable provider, speech, or attachment branches', () => {
  const source = fs.readFileSync(promptInputFile, 'utf8')

  assert.doesNotMatch(
    source,
    /PromptInputController|ProviderAttachmentsContext/
  )
  assert.doesNotMatch(source, /SpeechRecognition/)
  assert.doesNotMatch(
    source,
    /FileUIPart|LocalAttachmentsContext|createObjectURL|convertBlobUrlToDataUrl/
  )
  assert.match(source, /<form[\s\S]*onSubmit=\{handleSubmit\}/)
})

test('model details modules keep bounded dependencies and dedicated entrypoints', () => {
  for (const file of modelDetailsFiles) {
    const source = fs.readFileSync(file, 'utf8')
    const importCount = source.match(/^import\b/gm)?.length ?? 0

    assert.ok(
      importCount <= 18,
      `${file} has ${importCount} direct imports; split the module before adding more`
    )
  }

  assert.match(
    fs.readFileSync(pricingRouteFile, 'utf8'),
    /from ['"]@\/features\/pricing\/components\/model-details-page['"]/
  )
  assert.match(
    fs.readFileSync(pricingComponentsIndexFile, 'utf8'),
    /from ['"]\.\/model-details-drawer['"]/
  )
})
