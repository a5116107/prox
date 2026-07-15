import type { KnipConfig } from 'knip'

const config: KnipConfig = {
  entry: [
    'src/main.tsx',
    'src/**/*.d.ts',
    'src/i18n/static-keys.ts',
    'src/features/system-settings/operations/use-ops-registry.ts',
  ],
  project: ['src/**/*.{ts,tsx,css}'],
  ignore: ['src/components/ui/**'],
  // Generated Shadcn components are retained as a local UI toolkit.
  ignoreDependencies: [
    'embla-carousel-react',
    'react-resizable-panels',
    'recharts',
  ],
}

export default config
