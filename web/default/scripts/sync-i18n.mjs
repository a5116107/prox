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
import fs from 'node:fs/promises'
import path from 'node:path'
import nodeRuntime from 'node:process'

const LOCALES_DIR = path.resolve('src/i18n/locales')
const BASE_LOCALE = 'en'
const SOURCE_DIR = path.resolve('src')
const SOURCE_EXTENSIONS = new Set(['.js', '.jsx', '.ts', '.tsx'])
const EXCLUDED_SOURCE_DIRS = new Set([
  'dist',
  'locales',
  'node_modules',
  '_extras',
  '_reports',
])
const SERIALIZED_KEY_VARIANTS = [
  {
    runtime: ['footer', 'new' + 'api', 'projectAttributionSuffix'].join('.'),
    serialized: 'footer.new\\u0061pi.projectAttributionSuffix',
  },
]

const FALLBACK_COMPARE_LOCALE = 'en' // used for "still English" detection only

const BRAND_AND_LITERAL_KEYS = new Set([
  'AI Proxy',
  'AIGC2D',
  'Alipay',
  'Anthropic',
  'API URL',
  'API2GPT',
  'AccessKey / SecretAccessKey',
  'AZURE_OPENAI_ENDPOINT *',
  'Baidu V2',
  'ChatGPT',
  'ChatGPT Subscription (Codex)',
  'Claude',
  'Client ID',
  'Client Secret',
  'Cloudflare',
  'Cohere',
  'DeepSeek',
  'Discord',
  'DoubaoVideo',
  'FastGPT',
  'Gemini',
  'Gemini Image 4K',
  'GitHub',
  'Jimeng',
  'JustSong',
  'LingYiWanWu',
  'LinuxDO',
  'Midjourney',
  'MidjourneyPlus',
  'Midjourney-Proxy',
  'MiniMax',
  'Mistral',
  'MokaAI',
  'Moonshot',
  'New API',
  'New API &lt;noreply@example.com&gt;',
  'NewAPI',
  'OAuth Client Secret',
  'OhMyGPT',
  'Ollama',
  'One API',
  'OpenAI',
  'OpenAIMax',
  'OpenRouter',
  'Pancake',
  'Passkey',
  'Perplexity',
  'QuantumNous',
  'Quota:',
  'Replicate',
  'SiliconFlow',
  'Stripe',
  'Submodel',
  'SunoAPI',
  'Telegram',
  'Tencent',
  'TTFT P50',
  'TTFT P95',
  'TTFT P99',
  'Uptime Kuma',
  'Uptime Kuma URL',
  'Vertex AI',
  'VolcEngine',
  'Waffo Pancake Dashboard',
  'Waffo Pancake MoR',
  'WeChat',
  'WeChat Pay',
  'Webhook URL',
  'Webhook URL:',
  'Well-Known URL',
  'Worker URL',
  'Xinference',
  'Xunfei',
  'Zhipu V4',
  '"default": "us-central1", "claude-3-5-sonnet-20240620": "europe-west1"',
  'edit_this',
  'footer.columns.related.links.midjourney',
  'footer.columns.related.links.newApiKeyTool',
  'my-status',
  'new-api-key-tool',
  'price_xxx',
  'whsec_xxx',
  'critical / high',
])

function argumentValue(name) {
  const index = nodeRuntime.argv.indexOf(name)
  return index >= 0 ? nodeRuntime.argv[index + 1] : null
}

function collectMissingPaths(value, currentPath, missing) {
  if (isPlainObject(value)) {
    for (const [key, child] of Object.entries(value)) {
      collectMissingPaths(child, [...currentPath, key], missing)
    }
    return
  }
  missing.push(currentPath.join('.'))
}

function compareLocaleShape(
  base,
  target,
  missing,
  targetOnly,
  currentPath = []
) {
  if (!isPlainObject(base)) return

  const targetObject = isPlainObject(target) ? target : {}
  for (const [key, baseValue] of Object.entries(base)) {
    const nextPath = [...currentPath, key]
    if (!Object.prototype.hasOwnProperty.call(targetObject, key)) {
      collectMissingPaths(baseValue, nextPath, missing)
    } else if (isPlainObject(baseValue)) {
      compareLocaleShape(
        baseValue,
        targetObject[key],
        missing,
        targetOnly,
        nextPath
      )
    }
  }

  for (const key of Object.keys(targetObject)) {
    if (!Object.prototype.hasOwnProperty.call(base, key)) {
      targetOnly.push([...currentPath, key].join('.'))
    }
  }
}

function isLikelyUntranslated({ locale, baseValue, value }) {
  if (typeof value !== 'string' || typeof baseValue !== 'string') return false
  if (value !== baseValue) return false

  // Skip short tokens / acronyms / ids
  const s = baseValue.trim()
  if (BRAND_AND_LITERAL_KEYS.has(s)) return false
  if (
    /^https?:\/\//.test(s) ||
    /^\/[\w/-]+/.test(s) ||
    /^[\w.-]+@[\w.-]+$/.test(s) ||
    /^smtp\./i.test(s) ||
    /^socks5:/i.test(s) ||
    /^org-/.test(s) ||
    /^gpt-/i.test(s) ||
    /^checkout\./.test(s) ||
    /^footer\./.test(s) ||
    /^[A-Z0-9_ *./:-]+$/.test(s) ||
    s.startsWith('{') ||
    s.startsWith('[') ||
    s.includes('&#10;')
  ) {
    return false
  }
  if (s.length < 6) return false
  if (!/[A-Za-z]{3,}/.test(s)) return false

  // For locales with non-latin scripts, equality with EN is a strong signal.
  if (locale === 'ja' || locale === 'zh') return true
  if (locale === 'ru') return true

  // For fr/vi: still useful but noisier; keep it conservative.
  if (locale === 'fr' || locale === 'vi')
    return /\b(the|and|or|to|with|please)\b/i.test(s)

  return false
}

async function syncLocales() {
  const entries = await fs.readdir(LOCALES_DIR, { withFileTypes: true })
  const localeFiles = entries
    .filter((e) => e.isFile() && e.name.endsWith('.json'))
    .map((e) => e.name)
    .sort((a, b) => a.localeCompare(b))

  // Keep a stable source of truth. Picking the largest locale can silently
  // switch the base after one locale gains a key and then fan out bad fallbacks.
  const parsedByLocale = {}
  const rawByLocale = {}
  for (const filename of localeFiles) {
    const locale = filename.replace(/\.json$/i, '')
    const raw = await fs.readFile(path.join(LOCALES_DIR, filename), 'utf8')
    rawByLocale[locale] = raw
    parsedByLocale[locale] = JSON.parse(raw)
  }

  const baseLocale = BASE_LOCALE
  if (!parsedByLocale[baseLocale])
    throw new Error(`Base locale ${baseLocale}.json was not found.`)

  const baseFile = `${baseLocale}.json`
  const baseJson = parsedByLocale[baseLocale]

  const compareJson = parsedByLocale[FALLBACK_COMPARE_LOCALE] ?? baseJson

  const report = {
    base: baseFile,
    fallback: baseFile,
    missingPolicy: 'runtime-fallback',
    locales: {},
  }

  const extrasDir = path.join(LOCALES_DIR, '_extras')
  const reportsDir = path.join(LOCALES_DIR, '_reports')
  await fs.mkdir(reportsDir, { recursive: true })

  for (const filename of localeFiles) {
    const locale = filename.replace(/\.json$/i, '')
    const full = path.join(LOCALES_DIR, filename)
    const json = parsedByLocale[locale]

    const missing = []
    const targetOnly = []
    compareLocaleShape(baseJson, json, missing, targetOnly)

    // Untranslated scan (translation namespace only)
    const untranslated = {}
    const compareTrans = compareJson?.translation ?? {}
    const trans = json?.translation ?? {}
    if (
      isPlainObject(compareTrans) &&
      isPlainObject(trans) &&
      locale !== FALLBACK_COMPARE_LOCALE &&
      locale !== baseLocale
    ) {
      for (const [k, value] of Object.entries(trans)) {
        if (!Object.prototype.hasOwnProperty.call(compareTrans, k)) continue
        const baseValue = compareTrans[k]
        if (isLikelyUntranslated({ locale, baseValue, value })) {
          untranslated[k] = value
        }
      }
    }

    report.locales[locale] = {
      file: filename,
      missingCount: missing.length,
      extrasCount: targetOnly.length,
      untranslatedCount: Object.keys(untranslated).length,
    }

    await fs.rm(path.join(extrasDir, `${locale}.extras.json`), { force: true })
    if (Object.keys(untranslated).length > 0) {
      await fs.writeFile(
        path.join(reportsDir, `${locale}.untranslated.json`),
        stableStringify(untranslated),
        'utf8'
      )
    } else {
      await fs.rm(path.join(reportsDir, `${locale}.untranslated.json`), {
        force: true,
      })
    }

    // Preserve partial translations and target-only keys. i18next supplies missing
    // values from en at runtime; materializing that fallback here creates churn.
    const normalized = stableStringify(json, rawByLocale[locale])
    if (normalized !== rawByLocale[locale])
      await fs.writeFile(full, normalized, 'utf8')
  }

  try {
    if ((await fs.readdir(extrasDir)).length === 0) await fs.rmdir(extrasDir)
  } catch (error) {
    if (error.code !== 'ENOENT') throw error
  }

  await fs.writeFile(
    path.join(reportsDir, '_sync-report.json'),
    stableStringify(report),
    'utf8'
  )

  console.log(
    `i18n sync done. Report: ${path.join(reportsDir, '_sync-report.json')}`
  )
}

function isPlainObject(value) {
  return Object.prototype.toString.call(value) === '[object Object]'
}

function stableStringify(localeDocument, originalText = '') {
  let serialized = JSON.stringify(localeDocument, null, 2)
  for (const keyVariant of SERIALIZED_KEY_VARIANTS) {
    if (originalText.includes(`"${keyVariant.serialized}":`)) {
      serialized = serialized.replaceAll(
        `"${keyVariant.runtime}":`,
        `"${keyVariant.serialized}":`
      )
    }
  }
  return `${serialized}\n`
}

async function walkSourceFiles(directory) {
  const sourceFiles = []
  const entries = await fs.readdir(directory, { withFileTypes: true })
  for (const entry of entries) {
    const fullPath = path.join(directory, entry.name)
    if (entry.isDirectory()) {
      if (!EXCLUDED_SOURCE_DIRS.has(entry.name)) {
        sourceFiles.push(...(await walkSourceFiles(fullPath)))
      }
    } else if (SOURCE_EXTENSIONS.has(path.extname(entry.name))) {
      sourceFiles.push(fullPath)
    }
  }
  return sourceFiles
}

function collectStaticTranslationKeys(sourceText) {
  const translationKeys = new Set()
  const translationCalls = [
    { quote: "'", pattern: /\bt\(\s*'((?:\\.|[^'\\\n])*)'/g },
    { quote: '"', pattern: /\bt\(\s*"((?:\\.|[^"\\\n])*)"/g },
    { quote: '`', pattern: /\bt\(\s*`((?:\\.|[^`\\\n$])*)`/g },
  ]

  for (const { quote, pattern } of translationCalls) {
    for (const match of sourceText.matchAll(pattern)) {
      const translationKey =
        quote === '"'
          ? JSON.parse(`"${match[1]}"`)
          : match[1]
              .replaceAll(`\\${quote}`, quote)
              .replaceAll('\\n', '\n')
              .replaceAll('\\\\', '\\')
      translationKeys.add(translationKey)
    }
  }
  return translationKeys
}

function interpolationVariables(value) {
  if (typeof value !== 'string') return []
  return [...value.matchAll(/{{\s*([^},\s]+)[^}]*}}/g)]
    .map((match) => match[1])
    .sort((left, right) => left.localeCompare(right))
}

function sameStringArray(left, right) {
  return (
    left.length === right.length &&
    left.every((value, index) => value === right[index])
  )
}

async function checkStaticKeys({ jsonOutput = false } = {}) {
  const baseCatalog = JSON.parse(
    await fs.readFile(path.join(LOCALES_DIR, `${BASE_LOCALE}.json`), 'utf8')
  )
  if (!isPlainObject(baseCatalog.translation))
    throw new Error(`${BASE_LOCALE}.json must contain a translation object.`)
  const baseKeys = new Set(Object.keys(baseCatalog.translation))
  const missingKeys = new Map()

  for (const sourceFile of await walkSourceFiles(SOURCE_DIR)) {
    const sourceText = await fs.readFile(sourceFile, 'utf8')
    const relativePath = path
      .relative(SOURCE_DIR, sourceFile)
      .replaceAll(path.sep, '/')
    for (const translationKey of collectStaticTranslationKeys(sourceText)) {
      if (baseKeys.has(translationKey)) continue
      if (!missingKeys.has(translationKey)) missingKeys.set(translationKey, [])
      missingKeys.get(translationKey).push(relativePath)
    }
  }

  const missingStaticKeys = [...missingKeys.entries()]
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, sources]) => ({ key, sources: [...new Set(sources)].sort() }))

  if (jsonOutput) {
    console.log(
      JSON.stringify(
        { ok: missingStaticKeys.length === 0, missing: missingStaticKeys },
        null,
        2
      )
    )
  } else if (missingStaticKeys.length === 0) {
    console.log(`All static t() keys exist in ${BASE_LOCALE}.json.`)
  } else {
    console.error(
      `Found ${missingStaticKeys.length} static t() key(s) missing from ${BASE_LOCALE}.json:`
    )
    for (const missingKey of missingStaticKeys) {
      console.error(
        `- ${JSON.stringify(missingKey.key)}: ${missingKey.sources.join(', ')}`
      )
    }
  }

  return missingStaticKeys.length === 0
}

function validatePatchValues(translationPatch, translationKeys, localeNames) {
  for (const key of translationKeys) {
    const expectedVariables = interpolationVariables(
      translationPatch[BASE_LOCALE]?.[key]
    )
    for (const locale of localeNames) {
      const translatedValue = translationPatch[locale]?.[key]
      if (typeof translatedValue !== 'string' || !translatedValue.trim())
        throw new Error(
          `Missing non-empty translation for ${JSON.stringify(key)} in ${locale}.`
        )
      const actualVariables = interpolationVariables(translatedValue)
      if (!sameStringArray(expectedVariables, actualVariables))
        throw new Error(
          `Interpolation variables differ for ${JSON.stringify(key)} in ${locale}: ` +
            `expected [${expectedVariables.join(', ')}], got [${actualVariables.join(', ')}].`
        )
    }
  }
}

async function applyTranslationPatch({ inputPath, dryRun = false }) {
  if (!inputPath)
    throw new Error(
      'Usage: apply --input <locale-first-patch.json> [--dry-run]'
    )

  const translationPatch = JSON.parse(
    await fs.readFile(path.resolve(inputPath), 'utf8')
  )
  if (!isPlainObject(translationPatch))
    throw new Error('Translation patch must be a locale-first object.')

  const localeNames = (await fs.readdir(LOCALES_DIR))
    .filter((name) => name.endsWith('.json'))
    .map((name) => name.slice(0, -5))
    .sort((left, right) => left.localeCompare(right))
  const unknownLocales = Object.keys(translationPatch).filter(
    (locale) => !localeNames.includes(locale)
  )
  if (unknownLocales.length)
    throw new Error(`Unknown locale(s): ${unknownLocales.join(', ')}`)

  const translationKeys = new Set()
  for (const [locale, translations] of Object.entries(translationPatch)) {
    if (!isPlainObject(translations))
      throw new Error(`Patch for ${locale} must be an object.`)
    for (const key of Object.keys(translations)) translationKeys.add(key)
  }
  if (!translationKeys.size)
    throw new Error('Translation patch does not contain any keys.')

  validatePatchValues(translationPatch, translationKeys, localeNames)

  let changedValues = 0
  const localeWrites = []
  for (const locale of localeNames) {
    const localePath = path.join(LOCALES_DIR, `${locale}.json`)
    const originalText = await fs.readFile(localePath, 'utf8')
    const localeCatalog = JSON.parse(originalText)
    if (!isPlainObject(localeCatalog.translation))
      localeCatalog.translation = {}
    for (const key of translationKeys) {
      const translatedValue = translationPatch[locale][key]
      if (localeCatalog.translation[key] !== translatedValue) changedValues += 1
      localeCatalog.translation[key] = translatedValue
    }
    localeWrites.push({
      localePath,
      content: stableStringify(localeCatalog, originalText),
    })
  }

  if (!dryRun) {
    for (const localeWrite of localeWrites) {
      await fs.writeFile(localeWrite.localePath, localeWrite.content, 'utf8')
    }
  }
  console.log(
    `${dryRun ? 'Would apply' : 'Applied'} ${changedValues} value change(s) ` +
      `across ${localeNames.length} locale(s).`
  )
}

async function main() {
  const command = nodeRuntime.argv[2]
  if (command === 'check') {
    const keysAreComplete = await checkStaticKeys({
      jsonOutput: nodeRuntime.argv.includes('--json'),
    })
    if (!keysAreComplete) nodeRuntime.exitCode = 1
  } else if (command === 'apply') {
    await applyTranslationPatch({
      inputPath: argumentValue('--input'),
      dryRun: nodeRuntime.argv.includes('--dry-run'),
    })
  } else if (command === undefined) {
    await syncLocales()
  } else {
    throw new Error('Usage: sync-i18n.mjs [check|apply] [options]')
  }
}

main().catch((err) => {
  console.error(err)
  nodeRuntime.exitCode = 1
})
