#!/usr/bin/env node

import { access, mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(scriptDir, '..', '..');
const placeholder = '<!doctype html><title>test embed placeholder</title>\n';

for (const frontend of ['default', 'classic']) {
  const indexPath = path.join(repoRoot, 'web', frontend, 'dist', 'index.html');
  try {
    await access(indexPath);
  } catch {
    await mkdir(path.dirname(indexPath), { recursive: true });
    await writeFile(indexPath, placeholder, 'utf8');
  }
}
