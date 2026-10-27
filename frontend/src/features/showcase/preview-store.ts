import { create } from 'zustand';

import type { ShowcaseCard } from './api';

/** The card the catalogue will show, live while the team edits (read by the side panel). */
export const useShowcasePreview = create<{
  card: ShowcaseCard | null;
  setCard: (card: ShowcaseCard | null) => void;
}>((set) => ({
  card: null,
  setCard: (card) => {
    set({ card });
  },
}));
