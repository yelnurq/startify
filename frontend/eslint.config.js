import js from '@eslint/js';
import prettier from 'eslint-config-prettier';
import i18next from 'eslint-plugin-i18next';
import reactHooks from 'eslint-plugin-react-hooks';
import reactRefresh from 'eslint-plugin-react-refresh';
import { defineConfig, globalIgnores } from 'eslint/config';
import globals from 'globals';
import tseslint from 'typescript-eslint';

// A string with at least two letters (Latin or Cyrillic incl. Kazakh) is text.
const LETTERS = '/[A-Za-zА-яЁёӘәҒғҚқҢңӨөҰұҮүҺһІі]{2}/';

export default defineConfig([
  globalIgnores([
    'dist',
    'coverage',
    'playwright-report',
    'test-results',
    'src/routeTree.gen.ts',
    'src/shared/api/schema.gen.ts',
  ]),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.strictTypeChecked,
      tseslint.configs.stylisticTypeChecked,
      reactHooks.configs.flat['recommended-latest'],
    ],
    languageOptions: {
      ecmaVersion: 2023,
      globals: globals.browser,
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    plugins: { 'react-refresh': reactRefresh },
    rules: {
      'react-refresh/only-export-components': [
        'error',
        { allowConstantExport: true, allowExportNames: ['Route', 'NavLink'] },
      ],
      '@typescript-eslint/no-explicit-any': 'error',
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_', ignoreRestSiblings: true },
      ],
      '@typescript-eslint/consistent-type-imports': 'error',
      '@typescript-eslint/restrict-template-expressions': ['error', { allowNumber: true }],
      '@typescript-eslint/no-misused-promises': [
        'error',
        { checksVoidReturn: { attributes: false } },
      ],
      eqeqeq: ['error', 'always'],
      'no-console': ['error', { allow: ['warn', 'error'] }],
    },
  },
  {
    // Every user-visible string goes through i18next: no literals in JSX.
    files: ['src/**/*.tsx'],
    ignores: ['src/**/*.test.tsx'],
    plugins: { i18next },
    rules: {
      // Text between tags.
      'i18next/no-literal-string': [
        'error',
        {
          mode: 'jsx-text-only',
          words: {
            exclude: ['[0-9!-/:-@[-`{-~]+', '[A-Z_-]+', '[·—→↑↓↵⌘∞]', 'Esc', 'Ctrl'],
          },
        },
      ],
      // Visible attributes and literal children such as {'Сохранить'}; strings
      // used as keys, classes or values elsewhere in JSX stay allowed.
      'no-restricted-syntax': [
        'error',
        {
          selector: `JSXAttribute[name.name=/^(alt|aria-label|aria-description|title|placeholder|label)$/] > Literal[value=${LETTERS}]`,
          message: 'User-visible attribute text must come from t().',
        },
        {
          selector: `:matches(JSXElement, JSXFragment) > JSXExpressionContainer > Literal[value=${LETTERS}]`,
          message: 'User-visible text must come from t().',
        },
      ],
    },
  },
  {
    // The design system exports components together with their variants and
    // hooks (buttonVariants, useFieldControl, toast); fast refresh then falls
    // back to a full reload for these files, which is acceptable.
    files: ['src/design-system/**/*.tsx'],
    rules: { 'react-refresh/only-export-components': 'off' },
  },
  {
    files: ['*.{js,ts}', 'scripts/**/*.{mjs,ts}', 'e2e/**/*.ts'],
    languageOptions: { globals: globals.node },
  },
  {
    // Command-line scripts report to the terminal.
    files: ['scripts/**/*.{mjs,ts}'],
    rules: { 'no-console': 'off' },
  },
  {
    files: ['**/*.{js,mjs}'],
    extends: [tseslint.configs.disableTypeChecked],
  },
  prettier,
]);
