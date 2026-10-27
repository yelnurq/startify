import { converter } from 'culori';
import { describe, expect, it } from 'vitest';

import {
  brandPalette,
  contrastRatio,
  firstAccessibleStep,
  generateScale,
  meetsAA,
  SCALE_STEPS,
} from './color';

const oklch = converter('oklch');

describe('generateScale', () => {
  it('builds 11 steps that get darker monotonically', () => {
    const scale = generateScale('#00A3C4');
    expect(Object.keys(scale)).toHaveLength(SCALE_STEPS.length);
    const lightness = SCALE_STEPS.map((step) => oklch(scale[step])?.l ?? 0);
    for (let i = 1; i < lightness.length; i++) {
      expect(lightness[i]).toBeLessThan(lightness[i - 1] ?? 1);
    }
  });

  it('keeps every step a valid sRGB hex colour', () => {
    for (const hex of Object.values(generateScale('#F5B800', -18))) {
      expect(hex).toMatch(/^#[0-9a-f]{6}$/);
    }
  });

  it('reproduces the shipped primary scale', () => {
    const scale = generateScale('#00A3C4', 6);
    expect(scale[500]).toBe('#14a8c9');
    expect(scale[700]).toBe('#007591');
  });
});

describe('contrast', () => {
  it('computes WCAG ratios', () => {
    expect(contrastRatio('#000000', '#ffffff')).toBeCloseTo(21, 5);
    expect(contrastRatio('#ffffff', '#ffffff')).toBeCloseTo(1, 5);
  });

  it('checks AA thresholds for normal and large text', () => {
    expect(meetsAA('#007591', '#ffffff')).toBe(true);
    expect(meetsAA('#008ead', '#ffffff')).toBe(false);
    expect(meetsAA('#008ead', '#ffffff', true)).toBe(true);
  });
});

describe('brandPalette', () => {
  it('picks accessible steps for the KazUTB sky blue', () => {
    const palette = brandPalette('#00A3C4');
    expect(palette.solidStep).toBe(700);
    expect(palette.textStep).toBe(700);
    expect(contrastRatio('#ffffff', palette.scale[palette.solidStep])).toBeGreaterThanOrEqual(4.5);
  });

  it('adapts to dark brand colours', () => {
    const palette = brandPalette('#1E3A8A');
    expect(firstAccessibleStep(palette.scale, '#ffffff', 'fill')).toBeLessThanOrEqual(700);
  });

  it('rejects invalid colours', () => {
    expect(() => brandPalette('not-a-colour')).toThrow(/invalid colour/);
  });
});
