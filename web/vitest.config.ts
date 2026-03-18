import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'lcov'],
      reportsDirectory: './coverage',
      include: ['src/**/*.{ts,tsx}'],
      exclude: [
        'src/test/**',
        'src/main.tsx',
        'src/vite-env.d.ts',
        'src/svg.d.ts',
        'src/icons/**',
        'src/pages/Blank.tsx',
        'src/pages/Charts/**',
        'src/pages/Forms/**',
        'src/pages/Tables/**',
        'src/pages/UiElements/**',
        'src/pages/UserProfiles.tsx',
        'src/pages/Issues/index.tsx',
        'src/pages/Dashboard/Home.tsx',
        'src/components/symphony/LogViewer.tsx',
      ],
    },
  },
});
