import { lazy, Suspense, useEffect, useState, type ReactNode } from 'react';
import { create } from 'zustand';

export interface PaletteCommand {
  id: string;
  /** Group heading, already translated. */
  group: string;
  label: string;
  /** Extra search words, e.g. the same label in other languages. */
  keywords?: string[];
  icon?: ReactNode;
  hint?: string;
  perform: () => void;
}

interface PaletteState {
  open: boolean;
  commands: Record<string, PaletteCommand>;
  setOpen: (open: boolean) => void;
  register: (commands: PaletteCommand[]) => () => void;
}

export const useCommandPalette = create<PaletteState>()((set) => ({
  open: false,
  commands: {},
  setOpen: (open) => {
    set({ open });
  },
  register: (commands) => {
    set((state) => ({
      commands: { ...state.commands, ...Object.fromEntries(commands.map((c) => [c.id, c])) },
    }));
    return () => {
      set((state) => {
        const next = { ...state.commands };
        for (const c of commands) Reflect.deleteProperty(next, c.id);
        return { commands: next };
      });
    };
  },
}));

/** Registers commands while the calling component is mounted. */
export function useRegisterCommands(commands: PaletteCommand[]) {
  const register = useCommandPalette((s) => s.register);
  useEffect(() => register(commands), [register, commands]);
}

// cmdk and the dialog load on the first ⌘K, not with every page (BACKLOG B-010).
const PaletteDialog = lazy(() =>
  import('./command-palette-dialog').then((m) => ({ default: m.CommandPaletteDialog })),
);

/** ⌘K / Ctrl+K command palette: search sections, projects and actions. */
export function CommandPalette() {
  const open = useCommandPalette((s) => s.open);
  const setOpen = useCommandPalette((s) => s.setOpen);
  const [loaded, setLoaded] = useState(open);
  if (open && !loaded) setLoaded(true);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() === 'k' && (event.metaKey || event.ctrlKey)) {
        event.preventDefault();
        setOpen(!useCommandPalette.getState().open);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => {
      window.removeEventListener('keydown', onKeyDown);
    };
  }, [setOpen]);

  if (!loaded) return null;
  return (
    <Suspense fallback={null}>
      <PaletteDialog />
    </Suspense>
  );
}
