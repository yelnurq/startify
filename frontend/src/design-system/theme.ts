import { create } from 'zustand';

export type ResolvedTheme = 'light' | 'dark';

/**
 * Qadam is light, as the university's site (D-052): public/theme-init.js marks
 * the page light before paint, and nothing switches it. The dark tokens stay
 * in tokens.css for a later dark theme.
 */
export const useTheme = create<{ resolved: ResolvedTheme }>()(() => ({ resolved: 'light' }));
