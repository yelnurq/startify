// Prints the generated colour scales and the WCAG contrast of the pairs the
// design system relies on. Run: node scripts/palette.ts
import {
  contrastRatio,
  firstAccessibleStep,
  generateScale,
  SCALE_STEPS,
} from '../src/design-system/color.ts';

// Starting points for new scales. The scales in tokens.css are anchored to
// the palette of the university's site (D-049): generated, then the steps
// that the site uses are set to its exact colours.
const BASES = {
  primary: { color: '#09BAB2', hueShift: 0 },
  gold: { color: '#E2B53E', hueShift: 0 },
  success: { color: '#3FB57A', hueShift: 0 },
  warning: { color: '#E2B53E', hueShift: 0 },
  danger: { color: '#E04B45', hueShift: 0 },
  info: { color: '#5B86B5', hueShift: 0 },
  copper: { color: '#D9853F', hueShift: 0 },
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
  ['text #191C1D on bg', '#191C1D', '#F4F7F6'],
  ['heading ink #102945 on white', '#102945', '#FFFFFF'],
  ['muted #5C6866 on bg', '#5C6866', '#F4F7F6'],
  ['subtle #5F6B69 on sunken', '#5F6B69', '#EEF2F1'],
  ['faint #8A9492 on white (decor only)', '#8A9492', '#FFFFFF'],
  ['accent #067A75 on white', '#067A75', '#FFFFFF'],
  ['accent #067A75 on tint #E3F4F2', '#067A75', '#E3F4F2'],
  ['success #1B7A4A on tint #E3F3EA', '#1B7A4A', '#E3F3EA'],
  ['warning #8A5A00 on tint #FBF0D6', '#8A5A00', '#FBF0D6'],
  ['danger #BA1A1A on tint #FBE5E3', '#BA1A1A', '#FBE5E3'],
  ['info #2C5A86 on tint #E8EEF5', '#2C5A86', '#E8EEF5'],
  ['gold text #8F6200 on tint #FBF0D6', '#8F6200', '#FBF0D6'],
  ['#3D2600 on gold #E2B53E', '#3D2600', '#E2B53E'],
  ['dark text #E7EEED on #07161D', '#E7EEED', '#07161D'],
  ['dark subtle #8C9D9B on #0C1F27', '#8C9D9B', '#0C1F27'],
];
console.log('\n/* neutral pairs */');
for (const [label, fg, bg] of neutrals) {
  console.log(`  ${label}: ${contrastRatio(fg, bg).toFixed(2)}`);
}
