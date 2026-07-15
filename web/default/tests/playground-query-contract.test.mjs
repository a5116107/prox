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
import ts from 'typescript'

const playgroundFile = fileURLToPath(
  new URL('../src/features/playground/index.tsx', import.meta.url)
)

function walk(node, visitor) {
  visitor(node)
  ts.forEachChild(node, (child) => walk(child, visitor))
}

function getObjectProperty(objectLiteral, name) {
  return objectLiteral.properties.find(
    (property) =>
      ts.isPropertyAssignment(property) &&
      property.name.getText(objectLiteral.getSourceFile()) === name
  )
}

test('playground queries surface request failures through React Query', () => {
  const sourceFile = ts.createSourceFile(
    'index.tsx',
    fs.readFileSync(playgroundFile, 'utf8'),
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TSX
  )
  const queryFunctions = new Map()

  walk(sourceFile, (node) => {
    if (
      !ts.isCallExpression(node) ||
      !ts.isIdentifier(node.expression) ||
      node.expression.text !== 'useQuery' ||
      !node.arguments[0] ||
      !ts.isObjectLiteralExpression(node.arguments[0])
    ) {
      return
    }

    const queryOptions = node.arguments[0]
    const queryKey = getObjectProperty(queryOptions, 'queryKey')
    const queryFn = getObjectProperty(queryOptions, 'queryFn')
    if (
      !queryKey ||
      !ts.isArrayLiteralExpression(queryKey.initializer) ||
      !queryKey.initializer.elements[0] ||
      !ts.isStringLiteral(queryKey.initializer.elements[0]) ||
      !queryFn
    ) {
      return
    }

    queryFunctions.set(
      queryKey.initializer.elements[0].text,
      queryFn.initializer.getText(sourceFile)
    )
  })

  assert.deepEqual(
    Object.fromEntries(queryFunctions),
    {
      'playground-models': 'getUserModels',
      'playground-groups': 'getUserGroups',
    },
    'query functions must reject on request failure instead of returning an empty successful result'
  )
})
