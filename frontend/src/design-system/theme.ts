import { create } from 'zustand';

export type ThemePreference = 'light' | 'dark' | 'system';
export type ResolvedTheme = 'light' | 'dark';

// Keep in sync with public/theme-init.js, which applies the theme before paint.
const STORAGE_KEY = 'qadam.theme';
const DARK_QUERY = '(prefers-color-scheme: dark)';

function systemTheme(): ResolvedTheme {
  return window.matchMedia(DARK_QUERY).matches ? 'dark' : 'light';
}

function readPreference(): ThemePreference {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved === 'light' || saved === 'dark' || saved === 'system') return saved;
  } catch {
    // Storage unavailable: follow the system.
  }
  return 'system';
}

export function resolveTheme(preference: ThemePreference): ResolvedTheme {
  return preference === 'system' ? systemTheme() : preference;
}

/**
 * Switches the theme attribute, then runs `onApplied`, so that subscribers
 * re-render only once the new CSS variables are in effect.
 */
function applyTheme(theme: ResolvedTheme, onApplied: () => void) {
  const root = document.documentElement;
  if (root.dataset.theme === theme) {
    onApplied();
    return;
  }
  const swap = () => {
    // Colours switch at once: no element should animate between themes.
    root.classList.add('theme-switching');
    root.dataset.theme = theme;
    onApplied();
    requestAnimationFrame(() => {
      root.classList.remove('theme-switching');
    });
  };
  const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (!reduceMotion && typeof document.startViewTransition === 'function') {
    document.startViewTransition(swap);
  } else {
    swap();
  }
}

interface ThemeState {
  preference: ThemePreference;
  resolved: ResolvedTheme;
  setPreference: (preference: ThemePreference) => void;
}

const initial = readPreference();

export const useTheme = create<ThemeState>()((set) => ({
  preference: initial,
  resolved: resolveTheme(initial),
  setPreference: (preference) => {
    try {
      localStorage.setItem(STORAGE_KEY, preference);
    } catch {
      // The choice lasts for this session only.
    }
    const resolved = resolveTheme(preference);
    applyTheme(resolved, () => {
      set({ preference, resolved });
    });
  },
}));

window.matchMedia(DARK_QUERY).addEventListener('change', () => {
  const { preference } = useTheme.getState();
  if (preference !== 'system') return;
  const resolved = systemTheme();
  applyTheme(resolved, () => {
    useTheme.setState({ resolved });
  });
});
