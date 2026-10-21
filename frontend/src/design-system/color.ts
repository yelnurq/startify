// Colour science for the design system: 50–950 scales generated in OKLCH and
// WCAG 2.x contrast checks. Used to build the static palette (scripts/palette.ts)
// and, at runtime, tenant brand palettes from a single colour.
import {
  clampChroma,
  formatHex,
  modeLrgb,
  modeOklch,
  modeRgb,
  parse,
  useMode as registerMode,
  wcagContrast,
  type Oklch,
} from 'culori/fn';

const toOklch = registerMode(modeOklch);
registerMode(modeRgb);
registerMode(modeLrgb);

export const SCALE_STEPS = [50, 100, 200, 300, 400, 500, 600, 700, 800, 900, 950] as const;
export type ScaleStep = (typeof SCALE_STEPS)[number];
export type Scale = Record<ScaleStep, string>;

// Perceptual lightness per step. 500 sits close to a typical brand colour.
const LIGHTNESS: Record<ScaleStep, number> = {
  50: 0.978,
  100: 0.952,
  200: 0.906,
  300: 0.842,
  400: 0.762,
  500: 0.678,
  600: 0.598,
  700: 0.522,
  800: 0.448,
  900: 0.384,
  950: 0.29,
};

// Share of the base chroma per step: tints and shades are calmer than the core.
const CHROMA: Record<ScaleStep, number> = {
  50: 0.16,
  100: 0.3,
  200: 0.52,
  300: 0.76,
  400: 0.94,
  500: 1,
  600: 1,
  700: 0.93,
  800: 0.82,
  900: 0.7,
  950: 0.56,
};

function parseColor(input: string): Oklch {
  const parsed = parse(input);
  if (!parsed) throw new Error(`invalid colour: ${input}`);
  const color = toOklch(parsed);
  return { ...color, h: color.h ?? 0 };
}

/** Builds an 11-step scale from one colour, kept inside the sRGB gamut. */
export function generateScale(base: string, hueShift = 0): Scale {
  const { c, h = 0 } = parseColor(base);
  const scale = {} as Scale;
  for (const step of SCALE_STEPS) {
    // Darker steps drift slightly in hue, as natural pigments do.
    const drift = ((step - 500) / 450) * hueShift;
    const color = clampChroma(
      { mode: 'oklch', l: LIGHTNESS[step], c: c * CHROMA[step], h: h + drift },
      'oklch',
    );
    scale[step] = formatHex(color);
  }
  return scale;
}

/** WCAG 2.x contrast ratio between two colours (1–21). */
export function contrastRatio(a: string, b: string): number {
  return wcagContrast(a, b);
}

/** WCAG 2.2 AA for normal text (4.5:1) or large text and UI parts (3:1). */
export function meetsAA(foreground: string, background: string, large = false): boolean {
  return contrastRatio(foreground, background) >= (large ? 3 : 4.5);
}

/**
 * Picks the lightest step that still carries `text` at AA contrast, e.g. the
 * fill of a primary button with white labels.
 */
export function firstAccessibleStep(
  scale: Scale,
  text: string,
  against: 'fill' | 'text',
): ScaleStep {
  for (const step of SCALE_STEPS) {
    const ok = against === 'fill' ? meetsAA(text, scale[step]) : meetsAA(scale[step], text);
    if (ok) return step;
  }
  return 950;
}

export interface BrandPalette {
  scale: Scale;
  /** Step used for filled controls with white text. */
  solidStep: ScaleStep;
  /** Step used for links and coloured text on white. */
  textStep: ScaleStep;
}

/** Derives the accessible brand palette of a tenant from its primary colour. */
export function brandPalette(primary: string): BrandPalette {
  const scale = generateScale(primary);
  return {
    scale,
    solidStep: firstAccessibleStep(scale, '#ffffff', 'fill'),
    textStep: firstAccessibleStep(scale, '#ffffff', 'text'),
  };
}
