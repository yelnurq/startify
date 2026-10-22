import { formatNumber } from '@/shared/i18n/format';
import type { Locale } from '@/shared/i18n';

/** Largest amount the server accepts. */
export const MAX_NUMBER = 1e15;

/**
 * Reads a number typed by a person: spaces (also non-breaking ones used as
 * thousand separators), a comma or a dot as the decimal separator. Blank is
 * null; anything else unreadable is undefined.
 */
export function parseNumberInput(text: string): number | null | undefined {
  const compact = text.replace(/[\s'’]/g, '').replace(/₸|тг|шт\.?/gi, '');
  if (compact === '') return null;
  if (!/^-?\d*(?:[.,]\d+)?$/.test(compact) || compact === '-') return undefined;
  const value = Number(compact.replace(',', '.'));
  return Number.isFinite(value) ? value : undefined;
}

/** Grouped digits in the interface language: 1 500 000. */
export function formatNumberInput(value: number | null, locale: Locale): string {
  if (value === null) return '';
  return formatNumber(value, locale, { maximumFractionDigits: 2 });
}

export interface UnitEconomics {
  margin: number | null;
  ltv: number | null;
  ratio: number | null;
}

/** Margin per unit, lifetime value and LTV/CAC from the unit economics step. */
export function unitEconomics(values: {
  price: number | null;
  variableCost: number | null;
  cac: number | null;
  purchases: number | null;
}): UnitEconomics {
  const margin =
    values.price !== null && values.variableCost !== null
      ? values.price - values.variableCost
      : null;
  const ltv = margin !== null && values.purchases !== null ? margin * values.purchases : null;
  const ratio = ltv !== null && values.cac !== null && values.cac > 0 ? ltv / values.cac : null;
  return { margin, ltv, ratio };
}
