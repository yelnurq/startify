// Prints the generated colour scales and the WCAG contrast of the pairs the
// design system relies on. Run: node scripts/palette.ts
import {
  contrastRatio,
  firstAccessibleStep,
  generateScale,
  SCALE_STEPS,
} from '../src/design-system/color.ts';

const BASES = {
  primary: { color: '#00A3C4', hueShift: 6 },
  gold: { color: '#F5B800', hueShift: -18 },
  success: { color: '#16A34A', hueShift: 4 },
  warning: { color: '#F59E0B', hueShift: -14 },
  danger: { color: '#E5484D', hueShift: 0 },
  info: { color: '#3B82F6', hueShift: 6 },
};

for (const [name, { color, hueShift }] of Object.entries(BASES)) {
  const scale = generateScale(color, hueShift);
  console.log(`\n/* ${name} (${color}) */`);
  for (const step of SCALE_STEPS) {
    const hex = scale[step];
    const onWhite = contrastRatio(hex, '#ffffff').toFixed(2);
    console.log(`  --color-${name}-${step}: ${hex};  /* text on white ${onWhite} */`);
  }
  console.log(
    `  solid (white label AA): ${firstAccessibleStep(scale, '#ffffff', 'fill')}, text on white AA: ${firstAccessibleStep(scale, '#ffffff', 'text')}`,
  );
}

const neutrals: [string, string, string][] = [
  ['text #0B1220 on bg', '#0B1220', '#FAFBFC'],
  ['secondary #4A5568 on bg', '#4A5568', '#FAFBFC'],
  ['tertiary #8A94A6 on surface', '#8A94A6', '#FFFFFF'],
  ['tertiary #8A94A6 on bg', '#8A94A6', '#FAFBFC'],
  ['subtle #667085 on bg', '#667085', '#FAFBFC'],
  ['subtle #667085 on raised', '#667085', '#F4F6F9'],
  ['dark text #E8ECF3 on #0A0D14', '#E8ECF3', '#0A0D14'],
  ['dark secondary #A3ADBF on #111621', '#A3ADBF', '#111621'],
  ['dark tertiary #7C879A on #111621', '#7C879A', '#111621'],
  ['dark tertiary #8A94A6 on #111621', '#8A94A6', '#111621'],
  ['gold #F5B800 on white', '#F5B800', '#FFFFFF'],
  ['#5C4300 on gold #F5B800', '#5C4300', '#F5B800'],
];
console.log('\n/* neutral pairs */');
for (const [label, fg, bg] of neutrals) {
  console.log(`  ${label}: ${contrastRatio(fg, bg).toFixed(2)}`);
}
