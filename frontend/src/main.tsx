import '@/design-system/styles.css';

import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import { App } from '@/app/app';
import { i18nReady } from '@/shared/i18n';

const container = document.getElementById('root');
if (!container) throw new Error('#root element is missing from index.html');

// Render only once the active language is loaded: no flash of raw keys.
void i18nReady.then(() => {
  createRoot(container).render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
});
