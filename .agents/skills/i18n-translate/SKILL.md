---
name: i18n-translate
description: >-
  Complete and maintain frontend translations for this project. Use when UI
  copy changes, locale keys are missing, or translations need normalization.
---

# Frontend i18n Translation Workflow

## Scope

- App: `web/default`
- Locale files: `src/i18n/locales/*.json`
- Base locale: `en.json`
- Translation namespace: `translation`
- Supported commands are committed under `web/default/scripts`; do not create
  one-off translation scripts.

## Workflow

Run all commands from `web/default`.

1. Normalize locale files and refresh the report:

   ```bash
   bun run i18n:sync
   ```

2. Check that every static `t()` key used by source code exists in `en.json`:

   ```bash
   bun run i18n:check
   ```

3. Review `src/i18n/locales/_reports/_sync-report.json`. `en.json` is the fixed
   base. Partial locales may have a non-zero `missingCount`; i18next resolves
   those entries through its configured English runtime fallback. The sync
   command must report those gaps without copying English text into the locale.

4. For new or corrected keys, prepare a locale-first JSON patch. Every key must
   have a non-empty value for every locale file:

   ```json
   {
     "en": { "Save changes": "Save changes" },
     "fr": { "Save changes": "Enregistrer les modifications" },
     "ja": { "Save changes": "変更を保存" },
     "ru": { "Save changes": "Сохранить изменения" },
     "vi": { "Save changes": "Lưu thay đổi" },
     "zh": { "Save changes": "保存更改" }
   }
   ```

5. Validate the patch without writing, then apply it:

   ```bash
   bun run i18n:apply -- --input translations.patch.json --dry-run
   bun run i18n:apply -- --input translations.patch.json
   ```

6. Delete the temporary patch and rerun the full checks:

   ```bash
   bun run i18n:sync
   bun run i18n:check
   bun run typecheck
   ```

## Guardrails

- Every static source key must exist in `en.json`.
- New product keys should be translated for all six locales in one patch.
- Existing partial locales may rely on the configured English runtime fallback.
- Never materialize fallback values or delete locale-only keys during sync.
- Preserve interpolation variables such as `{{count}}` exactly across locales.
- Keep product names, URLs, API identifiers, and protocol terms unchanged when
  translation would make them inaccurate.
- Do not edit generated `_reports` or `_extras` files by hand.
- Do not keep translation patch files in the repository.

## Acceptance

- `bun run i18n:check` exits with code 0.
- The sync report names `en.json` as both base and fallback, and repeated syncs
  do not change locale files after the first run.
- Untranslated findings only cover values actually present in the target locale.
- `bun run typecheck` passes.
- `git diff --check` reports no whitespace errors.
