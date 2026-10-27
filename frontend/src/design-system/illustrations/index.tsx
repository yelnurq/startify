/*
 * Qadam illustration set for empty states, onboarding and the stories of the
 * pages. One visual grammar: a soft background blob, objects drawn with a
 * 2.5px ink outline, teal and gold accents of the university's palette, small
 * four-point sparkles. Colours come from the
 * .illustration palette in base.css, so every drawing adapts to the theme.
 */
import type { ReactNode } from 'react';

import { cn } from '../lib/cn';

const INK = 'var(--il-ink)';
const LINE = 'var(--il-line)';
const PAPER = 'var(--il-paper)';
const TINT = 'var(--il-tint)';
const TINT_SOFT = 'var(--il-tint-soft)';
const BRAND = 'var(--il-brand)';
const GOLD = 'var(--il-gold)';
const GOLD_SOFT = 'var(--il-gold-soft)';
const SOFT = 'var(--il-soft)';
const SUCCESS = 'var(--il-success)';

const stroke = {
  stroke: INK,
  strokeWidth: 2.5,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
};

interface IllustrationProps {
  className?: string;
}

function Frame({ className, children }: IllustrationProps & { children: ReactNode }) {
  return (
    <svg
      viewBox="0 0 240 180"
      fill="none"
      aria-hidden
      className={cn('illustration h-auto w-full max-w-60', className)}
    >
      {children}
    </svg>
  );
}

const BLOBS = [
  'M44 118C26 92 36 52 76 40c28-8 48 6 76 2c34-5 60 16 56 52c-4 34-32 54-72 58c-46 4-78-6-92-34Z',
  'M38 104c-6-34 22-62 58-66c26-3 40 10 66 8c32-2 50 22 44 52c-6 32-34 50-72 52c-50 2-90-12-96-46Z',
];

function Blob({ variant = 0 }: { variant?: 0 | 1 }) {
  return <path d={BLOBS[variant]} fill={TINT} />;
}

function Ground({ cx = 120, cy = 160, rx = 54 }: { cx?: number; cy?: number; rx?: number }) {
  return <ellipse cx={cx} cy={cy} rx={rx} ry={5} fill={SOFT} />;
}

function Sparkle({
  x,
  y,
  s = 1,
  fill = GOLD,
}: {
  x: number;
  y: number;
  s?: number;
  fill?: string;
}) {
  const k = (n: number) => (n * s).toFixed(2);
  return (
    <path
      d={`M${x} ${y - 6 * s}l${k(1.7)} ${k(4.3)} ${k(4.3)} ${k(1.7)} ${k(-4.3)} ${k(1.7)} ${k(-1.7)} ${k(4.3)} ${k(-1.7)} ${k(-4.3)} ${k(-4.3)} ${k(-1.7)} ${k(4.3)} ${k(-1.7)}Z`}
      fill={fill}
    />
  );
}

/** Start of a project: the idea lights up. */
export function IdeaIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob />
      <circle cx="120" cy="74" r="46" fill={GOLD_SOFT} />
      <Ground cy={152} rx={40} />
      <path
        d="M120 38c-18.8 0-33 14.2-33 32.4c0 11 5.2 19.4 12 25.4c3.8 3.4 6 7.6 6 12.4V114h30v-5.8c0-4.8 2.2-9 6-12.4c6.8-6 12-14.4 12-25.4C153 52.2 138.8 38 120 38Z"
        fill={PAPER}
        {...stroke}
      />
      <path
        d="M104 64c1.6-8 7.4-13.4 15-14.6"
        stroke={BRAND}
        strokeWidth="3"
        strokeLinecap="round"
        opacity="0.7"
      />
      <path
        d="M110 100c0-6 2.6-9.4 5-9.4s3 3.4 5 3.4s2.6-5.6 5-5.6s5 4 5 11.6"
        stroke={BRAND}
        strokeWidth="2.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <rect x="105" y="114" width="30" height="8" rx="3" fill={SOFT} {...stroke} />
      <rect x="107" y="122" width="26" height="8" rx="3" fill={SOFT} {...stroke} />
      <path d="M113 130h14v2.5a7 7 0 0 1-14 0V130Z" fill={INK} />
      <g stroke={GOLD} strokeWidth="3.5" strokeLinecap="round">
        <path d="M78 48l-9-6" />
        <path d="M92 28l-5-9" />
        <path d="M120 20V10" />
        <path d="M148 28l5-9" />
        <path d="M162 48l9-6" />
      </g>
      <Sparkle x={58} y={98} s={0.9} />
      <Sparkle x={184} y={92} s={0.7} fill={BRAND} />
      <Sparkle x={182} y={24} s={0.6} />
    </Frame>
  );
}

/** No projects yet: a project card and a rocket taking off. */
export function ProjectsIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob variant={1} />
      <Ground cx={104} cy={158} rx={58} />
      <rect x="46" y="70" width="108" height="82" rx="12" fill={PAPER} {...stroke} />
      <rect x="60" y="86" width="46" height="8" rx="4" fill={BRAND} />
      <rect x="60" y="102" width="78" height="5" rx="2.5" fill={LINE} />
      <rect x="60" y="113" width="60" height="5" rx="2.5" fill={LINE} />
      <rect x="60" y="130" width="78" height="7" rx="3.5" fill={SOFT} />
      <rect x="60" y="130" width="34" height="7" rx="3.5" fill={GOLD} />
      <path
        d="M144 82c9-5 16-11 22-18"
        stroke={LINE}
        strokeWidth="3"
        strokeLinecap="round"
        strokeDasharray="1 7"
      />
      <g transform="translate(180 50) rotate(40)">
        <path d="M0-32c11 9 15 25 12 44H-12c-3-19 1-35 12-44Z" fill={PAPER} {...stroke} />
        <circle cy="-9" r="6" fill={BRAND} {...stroke} />
        <path d="M-12 2l-10 14 11-1.6" fill={BRAND} {...stroke} />
        <path d="M12 2l10 14-11-1.6" fill={BRAND} {...stroke} />
        <path
          d="M-6.5 12c1 9 4 15 6.5 19c2.5-4 5.5-10 6.5-19Z"
          fill={GOLD}
          {...stroke}
          strokeWidth={2}
        />
      </g>
      <Sparkle x={206} y={26} s={0.8} />
      <Sparkle x={60} y={50} s={0.7} fill={BRAND} />
      <Sparkle x={212} y={112} s={0.6} />
    </Frame>
  );
}

/** Nothing found: a magnifier over documents. */
export function SearchIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob />
      <Ground cx={116} cy={160} rx={56} />
      <rect
        x="56"
        y="52"
        width="90"
        height="100"
        rx="12"
        fill={TINT_SOFT}
        stroke={LINE}
        strokeWidth="2.5"
        transform="rotate(-9 101 102)"
      />
      <rect x="72" y="46" width="90" height="102" rx="12" fill={PAPER} {...stroke} />
      <rect x="86" y="62" width="44" height="7" rx="3.5" fill={BRAND} />
      <rect x="86" y="78" width="62" height="5" rx="2.5" fill={LINE} />
      <rect x="86" y="89" width="50" height="5" rx="2.5" fill={LINE} />
      <path d="M168 130l17 17" stroke={INK} strokeWidth="10" strokeLinecap="round" />
      <path d="M170 132l13 13" stroke={GOLD} strokeWidth="4" strokeLinecap="round" />
      <circle cx="150" cy="112" r="27" fill={PAPER} stroke={INK} strokeWidth="3" />
      <circle cx="150" cy="112" r="20" fill={TINT_SOFT} />
      <path
        d="M143 106c0-4 3.2-7 7-7s7 3 7 7c0 5.2-7 5.4-7 10.6"
        stroke={BRAND}
        strokeWidth="3"
        strokeLinecap="round"
      />
      <circle cx="150" cy="123.5" r="2.2" fill={BRAND} />
      <Sparkle x={196} y={70} s={0.8} />
      <Sparkle x={48} y={44} s={0.6} fill={BRAND} />
    </Frame>
  );
}

/** All caught up: a calm bell with a check. */
export function InboxIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob variant={1} />
      <Ground cy={150} rx={46} />
      <path
        d="M120 46c-17 0-28 13-28 29v17c0 7-4 13-9 17h74c-5-4-9-10-9-17V75c0-16-11-29-28-29Z"
        fill={PAPER}
        {...stroke}
      />
      <path d="M109 110a11 11 0 0 0 22 0" fill={GOLD} {...stroke} />
      <circle cx="120" cy="41" r="5" fill={BRAND} {...stroke} />
      <path
        d="M104 72c1-7 5-12 11-14"
        stroke={BRAND}
        strokeWidth="3"
        strokeLinecap="round"
        opacity="0.7"
      />
      <circle cx="158" cy="58" r="15" fill={SUCCESS} stroke={PAPER} strokeWidth="3" />
      <path
        d="M151 58l5 5 9-10"
        stroke="#fff"
        strokeWidth="3"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <g stroke={LINE} strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
        <path d="M70 54h9l-9 10h9" />
        <path d="M58 76h6l-6 7h6" />
      </g>
      <Sparkle x={186} y={96} s={0.7} />
    </Frame>
  );
}

function Person({ cx, cy, fill }: { cx: number; cy: number; fill: string }) {
  return (
    <g>
      <circle cx={cx} cy={cy} r="24" fill={fill} {...stroke} />
      <circle cx={cx} cy={cy - 6} r="8" fill={PAPER} {...stroke} />
      <path d={`M${cx - 13} ${cy + 17}c1-9 6-13 13-13s12 4 13 13`} fill={PAPER} {...stroke} />
    </g>
  );
}

/** Looking for teammates: people linked into a team. */
export function TeamIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob />
      <g stroke={LINE} strokeWidth="2.5" strokeLinecap="round" strokeDasharray="2 7">
        <path d="M100 98l10 18" />
        <path d="M140 98l-10 18" />
        <path d="M104 84h32" />
      </g>
      <Person cx={82} cy={84} fill={TINT_SOFT} />
      <Person cx={158} cy={84} fill={GOLD_SOFT} />
      <Person cx={120} cy={132} fill={TINT_SOFT} />
      <circle cx="120" cy="50" r="14" fill={BRAND} stroke={PAPER} strokeWidth="3" />
      <path d="M120 44v12M114 50h12" stroke="#fff" strokeWidth="3" strokeLinecap="round" />
      <Sparkle x={196} y={128} s={0.8} />
      <Sparkle x={44} y={124} s={0.6} fill={BRAND} />
    </Frame>
  );
}

/** Competitions: a trophy on a podium with confetti. */
export function TrophyIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob variant={1} />
      <Ground cy={156} rx={50} />
      <rect x="84" y="126" width="72" height="26" rx="6" fill={PAPER} {...stroke} />
      <rect x="110" y="133" width="20" height="5" rx="2.5" fill={GOLD} />
      <path d="M98 52H86c0 13 5 20 13 22" {...stroke} />
      <path d="M142 52h12c0 13-5 20-13 22" {...stroke} />
      <path d="M98 46h44v22c0 14-10 26-22 26s-22-12-22-26V46Z" fill={GOLD} {...stroke} />
      <path d="M107 56v12" stroke="#fff" strokeWidth="3" strokeLinecap="round" opacity="0.7" />
      <path d="M114 94h12v14h-12Z" fill={GOLD} {...stroke} />
      <rect x="102" y="108" width="36" height="10" rx="3" fill={GOLD} {...stroke} />
      <g strokeWidth="3" strokeLinecap="round">
        <path d="M64 50l6 5" stroke={BRAND} />
        <path d="M176 44l-5 7" stroke={GOLD} />
        <path d="M58 92l8-2" stroke={GOLD} />
        <path d="M184 90l-7-3" stroke={BRAND} />
      </g>
      <circle cx="76" cy="70" r="3" fill={BRAND} />
      <circle cx="168" cy="68" r="3" fill={GOLD} />
      <Sparkle x={120} y={26} s={0.9} />
      <Sparkle x={196} y={120} s={0.6} fill={BRAND} />
    </Frame>
  );
}

/** Analytics without data yet: a chart card with a rising trend. */
export function ChartIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob />
      <rect x="48" y="44" width="144" height="108" rx="12" fill={PAPER} {...stroke} />
      <rect x="62" y="58" width="40" height="7" rx="3.5" fill={LINE} />
      <path d="M64 136h112" stroke={LINE} strokeWidth="2.5" strokeLinecap="round" />
      <rect x="70" y="114" width="18" height="22" rx="4" fill={SOFT} />
      <rect x="96" y="100" width="18" height="36" rx="4" fill={TINT} />
      <rect x="122" y="106" width="18" height="30" rx="4" fill={TINT} />
      <rect x="148" y="84" width="18" height="52" rx="4" fill={BRAND} />
      <path
        d="M72 104c12-6 20-10 30-16s20-2 30-14s18-12 26-18"
        stroke={GOLD}
        strokeWidth="3"
        strokeLinecap="round"
      />
      <path
        d="M150 54l9.5-2.4-1.5 9.6"
        stroke={GOLD}
        strokeWidth="3"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <Sparkle x={206} y={40} s={0.8} />
      <Sparkle x={36} y={120} s={0.6} fill={BRAND} />
    </Frame>
  );
}

/** Mentorship: a conversation between a student and a mentor. */
export function MentorIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob variant={1} />
      <path
        d="M50 56a12 12 0 0 1 12-12h72a12 12 0 0 1 12 12v30a12 12 0 0 1-12 12H84l-16 12V98h-6a12 12 0 0 1-12-12Z"
        fill={PAPER}
        {...stroke}
      />
      <rect x="64" y="60" width="62" height="6" rx="3" fill={LINE} />
      <rect x="64" y="74" width="42" height="6" rx="3" fill={LINE} />
      <path
        d="M108 104a12 12 0 0 1 12-12h58a12 12 0 0 1 12 12v24a12 12 0 0 1-12 12h-4v12l-16-12h-38a12 12 0 0 1-12-12Z"
        fill={TINT}
        {...stroke}
      />
      <circle cx="136" cy="116" r="4" fill={BRAND} />
      <circle cx="150" cy="116" r="4" fill={BRAND} opacity="0.7" />
      <circle cx="164" cy="116" r="4" fill={BRAND} opacity="0.4" />
      <Sparkle x={182} y={56} s={0.9} />
      <Sparkle x={52} y={128} s={0.6} fill={BRAND} />
    </Frame>
  );
}

/** Something went wrong: a plug pulled out of its socket. */
export function ErrorIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob />
      <path d="M26 128c22 0 32-18 50-18" stroke={INK} strokeWidth="3" strokeLinecap="round" />
      <rect x="76" y="96" width="30" height="28" rx="7" fill={PAPER} {...stroke} />
      <path d="M106 104h10M106 116h10" stroke={INK} strokeWidth="3.5" strokeLinecap="round" />
      <rect x="138" y="96" width="30" height="28" rx="7" fill={TINT_SOFT} {...stroke} />
      <path d="M138 104h-5M138 116h-5" stroke={INK} strokeWidth="3.5" strokeLinecap="round" />
      <path d="M168 110c18 0 28 18 48 18" stroke={INK} strokeWidth="3" strokeLinecap="round" />
      <path
        d="M125 82l-5 10h7l-5 10"
        stroke={GOLD}
        strokeWidth="3"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <g stroke={LINE} strokeWidth="2.5" strokeLinecap="round">
        <path d="M112 80l-4-6" />
        <path d="M134 78l4-6" />
        <path d="M124 136v7" />
      </g>
      <Sparkle x={70} y={58} s={0.7} fill={BRAND} />
      <Sparkle x={184} y={64} s={0.8} />
    </Frame>
  );
}

/** Certificates: a document with a verification code and a seal. */
export function CertificateIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob variant={1} />
      <rect x="64" y="34" width="104" height="122" rx="10" fill={PAPER} {...stroke} />
      <rect x="80" y="52" width="72" height="8" rx="4" fill={BRAND} />
      <rect x="80" y="70" width="72" height="5" rx="2.5" fill={LINE} />
      <rect x="80" y="81" width="52" height="5" rx="2.5" fill={LINE} />
      <rect x="80" y="106" width="28" height="28" rx="4" stroke={INK} strokeWidth="2" />
      <rect x="85" y="111" width="7" height="7" rx="1" fill={INK} />
      <rect x="96" y="111" width="7" height="7" rx="1" fill={INK} />
      <rect x="85" y="122" width="7" height="7" rx="1" fill={INK} />
      <rect x="97" y="123" width="4" height="4" rx="1" fill={BRAND} />
      <path d="M143 140l-6 20 9.5-5 6.5 7 3-19" fill={BRAND} {...stroke} strokeWidth={2} />
      <circle cx="152" cy="128" r="17" fill={GOLD} {...stroke} />
      <Sparkle x={152} y={128} s={1} fill="#fff" />
      <Sparkle x={196} y={50} s={0.8} />
      <Sparkle x={46} y={110} s={0.6} fill={BRAND} />
    </Frame>
  );
}

/** Launch: a rocket lifts off above the clouds. */
export function RocketIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob />
      <g stroke={LINE} strokeWidth="2.5" strokeLinecap="round">
        <path d="M86 44v14" />
        <path d="M154 34v12" />
        <path d="M74 80v8" />
        <path d="M168 72v10" />
      </g>
      <g transform="translate(120 70)">
        <path d="M0-50c15 12 20 34 16 60H-16c-4-26 1-48 16-60Z" fill={PAPER} {...stroke} />
        <circle cy="-18" r="8" fill={BRAND} {...stroke} />
        <path d="M-16 0l-14 22 15-2" fill={BRAND} {...stroke} />
        <path d="M16 0l14 22-15-2" fill={BRAND} {...stroke} />
        <path d="M-9 12c1 13 5 23 9 30c4-7 8-17 9-30Z" fill={GOLD} {...stroke} strokeWidth={2} />
      </g>
      <path
        d="M66 150c-9 0-15-6-15-13s6-12 13-12c2-8 9-13 17-13c9 0 16 6 17 14c7 0 12 5 12 12s-5 12-12 12Z"
        fill={PAPER}
        {...stroke}
      />
      <path
        d="M150 150c-7 0-12-5-12-11s5-11 11-11c2-7 8-11 15-11c8 0 14 6 15 13c6 0 10 4 10 10s-4 10-10 10Z"
        fill={TINT_SOFT}
        {...stroke}
      />
      <Sparkle x={196} y={40} s={0.9} />
      <Sparkle x={46} y={60} s={0.6} fill={BRAND} />
      <Sparkle x={200} y={104} s={0.6} />
    </Frame>
  );
}

/** The market: an arrow in the bullseye. */
export function TargetIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob variant={1} />
      <Ground cx={112} cy={158} rx={42} />
      <path d="M98 140l-9 16M126 140l9 16" {...stroke} />
      <circle cx="112" cy="92" r="50" fill={PAPER} {...stroke} />
      <circle cx="112" cy="92" r="36" fill={TINT} {...stroke} />
      <circle cx="112" cy="92" r="22" fill={PAPER} {...stroke} />
      <circle cx="112" cy="92" r="9" fill={GOLD} {...stroke} />
      <path d="M114 90l58-40" stroke={INK} strokeWidth="3" strokeLinecap="round" />
      <path d="M168 53l2-17 9 9Z" fill={BRAND} {...stroke} strokeWidth={2} />
      <path d="M168 53l17-3-8 10Z" fill={BRAND} {...stroke} strokeWidth={2} />
      <Sparkle x={58} y={40} s={0.8} />
      <Sparkle x={196} y={120} s={0.7} fill={BRAND} />
      <Sparkle x={190} y={84} s={0.5} />
    </Frame>
  );
}

/** Partnership: the university and a company agree. */
export function PartnersIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob />
      <Ground cy={152} rx={78} />
      <path d="M46 148V98l28-18 28 18v50Z" fill={PAPER} {...stroke} />
      <path
        d="M60 110v30M74 110v30M88 110v30"
        stroke={LINE}
        strokeWidth="3"
        strokeLinecap="round"
      />
      <path d="M74 80V64" {...stroke} />
      <path d="M74 64h14l-4 5 4 5H74" fill={GOLD} {...stroke} strokeWidth={2} />
      <path d="M162 108v-8a5 5 0 0 1 5-5h12a5 5 0 0 1 5 5v8" {...stroke} />
      <rect x="146" y="106" width="54" height="42" rx="8" fill={TINT_SOFT} {...stroke} />
      <path d="M146 124h54" stroke={INK} strokeWidth="2.5" />
      <rect x="167" y="119" width="12" height="10" rx="2" fill={GOLD} {...stroke} strokeWidth={2} />
      <path
        d="M104 120c12-24 30-24 42 0"
        stroke={LINE}
        strokeWidth="2.5"
        strokeLinecap="round"
        strokeDasharray="2 7"
      />
      <circle cx="124" cy="86" r="16" fill={BRAND} stroke={PAPER} strokeWidth="3" />
      <path
        d="M117 86l5 5 9-10"
        stroke="#fff"
        strokeWidth="3"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <Sparkle x={200} y={70} s={0.8} />
      <Sparkle x={40} y={66} s={0.6} fill={BRAND} />
    </Frame>
  );
}

/** The university: a campus building with columns and a flag. */
export function UniversityIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob variant={1} />
      <path d="M120 44V24" {...stroke} />
      <path d="M120 24h18l-4 6 4 6h-18" fill={BRAND} {...stroke} strokeWidth={2} />
      <path d="M52 78l68-34 68 34Z" fill={TINT} {...stroke} />
      <circle cx="120" cy="64" r="7" fill={GOLD} {...stroke} strokeWidth={2} />
      <rect x="58" y="78" width="124" height="10" rx="2" fill={PAPER} {...stroke} />
      <rect x="68" y="88" width="12" height="46" rx="2" fill={PAPER} {...stroke} />
      <rect x="92" y="88" width="12" height="46" rx="2" fill={PAPER} {...stroke} />
      <rect x="136" y="88" width="12" height="46" rx="2" fill={PAPER} {...stroke} />
      <rect x="160" y="88" width="12" height="46" rx="2" fill={PAPER} {...stroke} />
      <path d="M112 134v-22a8 8 0 0 1 16 0v22" fill={GOLD_SOFT} {...stroke} />
      <rect x="50" y="134" width="140" height="10" rx="2" fill={PAPER} {...stroke} />
      <rect x="42" y="144" width="156" height="9" rx="2" fill={SOFT} {...stroke} />
      <Sparkle x={196} y={52} s={0.8} />
      <Sparkle x={42} y={56} s={0.6} fill={BRAND} />
    </Frame>
  );
}

/** Demo Day: a pitch deck on stage under a spotlight. */
export function PitchIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob />
      <path d="M120 24l-52 124h104Z" fill={GOLD_SOFT} />
      <path d="M110 14h20l-4 10h-12Z" fill={INK} />
      <path d="M96 114l-12 36M144 114l12 36M120 114v30" {...stroke} />
      <rect x="62" y="40" width="116" height="74" rx="8" fill={PAPER} {...stroke} />
      <rect x="78" y="52" width="40" height="6" rx="3" fill={LINE} />
      <rect x="78" y="86" width="12" height="16" rx="2" fill={TINT} />
      <rect x="96" y="76" width="12" height="26" rx="2" fill={TINT} />
      <rect x="114" y="66" width="12" height="36" rx="2" fill={BRAND} />
      <path
        d="M136 98l11-14 10 6 10-18"
        stroke={GOLD}
        strokeWidth="3"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <rect x="54" y="148" width="132" height="8" rx="3" fill={SOFT} {...stroke} />
      <Sparkle x={198} y={56} s={0.8} />
      <Sparkle x={42} y={86} s={0.6} fill={BRAND} />
    </Frame>
  );
}

/** Deadlines: a calendar with a marked day and a clock. */
export function CalendarIllustration({ className }: IllustrationProps) {
  const cells = [68, 90, 112, 134];
  return (
    <Frame className={className}>
      <Blob variant={1} />
      <Ground cx={112} cy={156} rx={58} />
      <rect x="52" y="44" width="116" height="104" rx="12" fill={PAPER} {...stroke} />
      <path d="M52 56a12 12 0 0 1 12-12h92a12 12 0 0 1 12 12v12H52Z" fill={BRAND} {...stroke} />
      <path d="M80 34v18M140 34v18" {...stroke} strokeWidth={4} />
      {[82, 102, 122].map((y) =>
        cells.map((x) => (
          <rect
            key={`${x}-${y}`}
            x={x}
            y={y}
            width="14"
            height="11"
            rx="3"
            fill={x === 112 && y === 102 ? GOLD : LINE}
          />
        )),
      )}
      <circle cx="170" cy="124" r="24" fill={TINT_SOFT} {...stroke} />
      <path d="M170 110v14l9 6" {...stroke} />
      <Sparkle x={196} y={64} s={0.8} />
      <Sparkle x={40} y={110} s={0.6} fill={BRAND} />
    </Frame>
  );
}

/** Security: a shield with a lock. */
export function ShieldIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob />
      <Ground cy={158} rx={42} />
      <path d="M120 28l46 16v34c0 34-20 56-46 68c-26-12-46-34-46-68V44Z" fill={PAPER} {...stroke} />
      <path d="M120 42l32 11v25c0 24-13 40-32 50c-19-10-32-26-32-50V53Z" fill={TINT} />
      <path d="M110 80v-7a10 10 0 0 1 20 0v7" {...stroke} />
      <rect x="104" y="80" width="32" height="26" rx="5" fill={GOLD} {...stroke} />
      <circle cx="120" cy="91" r="3.5" fill={INK} />
      <path d="M120 94v5" stroke={INK} strokeWidth="2.5" strokeLinecap="round" />
      <Sparkle x={186} y={50} s={0.8} />
      <Sparkle x={54} y={110} s={0.6} fill={BRAND} />
      <Sparkle x={190} y={118} s={0.5} />
    </Frame>
  );
}

/** The route of a project: a map with a path and a pin. */
export function RouteIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob variant={1} />
      <path d="M44 62l48-14 56 14 48-14v90l-48 14-56-14-48 14Z" fill={PAPER} {...stroke} />
      <path d="M92 48v90M148 62v90" stroke={LINE} strokeWidth="2.5" />
      <path
        d="M62 128c14-6 22-24 38-22s22 22 40 14s20-30 34-36"
        stroke={BRAND}
        strokeWidth="3"
        strokeLinecap="round"
        strokeDasharray="2 8"
      />
      <circle cx="62" cy="128" r="5" fill={GOLD} {...stroke} strokeWidth={2} />
      <path
        d="M176 94c-10-12-16-20-16-28a16 16 0 0 1 32 0c0 8-6 16-16 28Z"
        fill={GOLD}
        {...stroke}
      />
      <circle cx="176" cy="66" r="6" fill={PAPER} {...stroke} strokeWidth={2} />
      <Sparkle x={206} y={34} s={0.8} />
      <Sparkle x={40} y={40} s={0.6} fill={BRAND} />
    </Frame>
  );
}

/** A good fit: two puzzle pieces come together. */
export function PuzzleIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob />
      <Ground cy={154} rx={56} />
      <path d="M58 70h44v16a10 10 0 0 1 0 20v16H58Z" fill={TINT} {...stroke} />
      <g transform="translate(10 -12) rotate(8 144 96)">
        <path d="M124 70h44v52h-44v-16a10 10 0 0 0 0-20Z" fill={GOLD_SOFT} {...stroke} />
        <circle cx="146" cy="96" r="5" fill={GOLD} />
      </g>
      <circle cx="80" cy="96" r="5" fill={BRAND} />
      <g stroke={LINE} strokeWidth="2.5" strokeLinecap="round">
        <path d="M184 52l8-6" />
        <path d="M188 68h9" />
        <path d="M180 38l3-9" />
      </g>
      <Sparkle x={120} y={40} s={0.8} />
      <Sparkle x={46} y={136} s={0.6} fill={BRAND} />
    </Frame>
  );
}

/** Grants and investment: coins, a note and a rising arrow. */
export function FundingIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob variant={1} />
      <Ground cx={112} cy={156} rx={64} />
      <g transform="rotate(-12 174 74)">
        <rect x="142" y="56" width="64" height="38" rx="6" fill={TINT_SOFT} {...stroke} />
        <circle cx="174" cy="75" r="9" fill={PAPER} {...stroke} strokeWidth={2} />
        <path d="M150 64h8M190 86h8" stroke={LINE} strokeWidth="2.5" strokeLinecap="round" />
      </g>
      {[0, 1, 2, 3, 4, 5].map((i) => (
        <rect
          key={`b${String(i)}`}
          x="98"
          y={138 - i * 11}
          width="46"
          height="13"
          rx="6.5"
          fill={GOLD}
          {...stroke}
        />
      ))}
      {[0, 1, 2, 3].map((i) => (
        <rect
          key={`a${String(i)}`}
          x="56"
          y={138 - i * 11}
          width="46"
          height="13"
          rx="6.5"
          fill={GOLD}
          {...stroke}
        />
      ))}
      <path d="M176 144v-38" stroke={BRAND} strokeWidth="4" strokeLinecap="round" />
      <path
        d="M165 116l11-12 11 12"
        stroke={BRAND}
        strokeWidth="4"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <Sparkle x={48} y={84} s={0.8} />
      <Sparkle x={206} y={128} s={0.6} fill={BRAND} />
    </Frame>
  );
}

/** The startup passport: a document with a photo, a score and a QR code. */
export function PassportIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob variant={1} />
      <Ground cy={156} rx={58} />
      <rect x="56" y="40" width="128" height="106" rx="12" fill={PAPER} {...stroke} />
      <path d="M56 52a12 12 0 0 1 12-12h104a12 12 0 0 1 12 12v8H56Z" fill={BRAND} {...stroke} />
      <rect x="70" y="72" width="34" height="42" rx="6" fill={TINT} {...stroke} />
      <circle cx="87" cy="87" r="6" fill={PAPER} {...stroke} strokeWidth={2} />
      <path d="M77 110c1-7 5-10 10-10s9 3 10 10" fill={PAPER} {...stroke} strokeWidth={2} />
      <rect x="114" y="74" width="54" height="6" rx="3" fill={LINE} />
      <rect x="114" y="88" width="40" height="6" rx="3" fill={LINE} />
      <rect x="114" y="102" width="28" height="10" rx="5" fill={GOLD} />
      <rect x="70" y="126" width="62" height="6" rx="3" fill={LINE} />
      <rect
        x="146"
        y="112"
        width="26"
        height="26"
        rx="3"
        stroke={INK}
        strokeWidth="2"
        fill={PAPER}
      />
      <rect x="150" y="116" width="7" height="7" rx="1" fill={INK} />
      <rect x="161" y="116" width="7" height="7" rx="1" fill={INK} />
      <rect x="150" y="127" width="7" height="7" rx="1" fill={INK} />
      <rect x="162" y="128" width="4" height="4" rx="1" fill={BRAND} />
      <Sparkle x={200} y={46} s={0.8} />
      <Sparkle x={42} y={120} s={0.6} fill={BRAND} />
    </Frame>
  );
}

/** The prototype: a flask with bubbles and a gear. */
export function LabIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob />
      <Ground cx={116} cy={156} rx={46} />
      <path
        d="M108 42h24v32l28 56a12 12 0 0 1-10.7 17.4H90.7A12 12 0 0 1 80 130l28-56Z"
        fill={PAPER}
        {...stroke}
      />
      <path d="M93 112h54l11 20a8 8 0 0 1-7 12H89a8 8 0 0 1-7-12Z" fill={BRAND} opacity="0.85" />
      <circle cx="106" cy="126" r="4" fill={PAPER} />
      <circle cx="124" cy="132" r="3" fill={PAPER} />
      <circle cx="134" cy="120" r="2.5" fill={PAPER} />
      <rect x="102" y="34" width="36" height="9" rx="4.5" fill={SOFT} {...stroke} />
      <circle cx="114" cy="22" r="4" stroke={BRAND} strokeWidth="2.5" />
      <circle cx="128" cy="14" r="2.5" stroke={BRAND} strokeWidth="2.5" />
      <g transform="translate(176 70)">
        {[0, 45, 90, 135, 180, 225, 270, 315].map((angle) => (
          <rect
            key={angle}
            x="-3.5"
            y="-21"
            width="7"
            height="9"
            rx="1.5"
            fill={GOLD}
            transform={`rotate(${String(angle)})`}
            {...stroke}
            strokeWidth={2}
          />
        ))}
        <circle r="14" fill={GOLD_SOFT} {...stroke} />
        <circle r="5" fill={PAPER} {...stroke} strokeWidth={2} />
      </g>
      <Sparkle x={58} y={58} s={0.8} />
      <Sparkle x={196} y={124} s={0.6} fill={BRAND} />
    </Frame>
  );
}

/** Live voting: a ballot drops into a box. */
export function VoteIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob variant={1} />
      <Ground cy={158} rx={58} />
      <path d="M66 96h108v52a8 8 0 0 1-8 8H74a8 8 0 0 1-8-8Z" fill={TINT} {...stroke} />
      <g transform="rotate(-8 120 64)">
        <rect x="98" y="34" width="44" height="58" rx="5" fill={PAPER} {...stroke} />
        <path
          d="M108 60l7 7 14-15"
          stroke={SUCCESS}
          strokeWidth="3.5"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </g>
      <rect x="58" y="86" width="124" height="14" rx="4" fill={PAPER} {...stroke} />
      <rect x="98" y="91" width="44" height="4" rx="2" fill={INK} />
      <circle cx="120" cy="128" r="13" fill={GOLD} {...stroke} strokeWidth={2} />
      <Sparkle x={120} y={128} s={0.9} fill="#fff" />
      <Sparkle x={196} y={50} s={0.8} />
      <Sparkle x={44} y={64} s={0.6} fill={BRAND} />
    </Frame>
  );
}

/** Traction: a seedling grows next to a rising line. */
export function GrowthIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob />
      <Ground cx={112} cy={156} rx={46} />
      <path
        d="M146 132l16-20 12 6 20-28"
        stroke={BRAND}
        strokeWidth="3"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M184 88l11-3-1 11"
        stroke={BRAND}
        strokeWidth="3"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path d="M112 112V62" stroke={SUCCESS} strokeWidth="3.5" strokeLinecap="round" />
      <path
        d="M112 90c-4-15-17-21-31-19c2 15 15 23 31 19Z"
        fill={SUCCESS}
        {...stroke}
        strokeWidth={2}
      />
      <path
        d="M112 74c4-15 17-21 31-19c-2 15-15 23-31 19Z"
        fill={SUCCESS}
        {...stroke}
        strokeWidth={2}
      />
      <circle cx="112" cy="58" r="7" fill={GOLD_SOFT} {...stroke} strokeWidth={2} />
      <path d="M90 120h44l-6 34h-32Z" fill={GOLD} {...stroke} />
      <rect x="85" y="110" width="54" height="12" rx="3" fill={GOLD} {...stroke} />
      <Sparkle x={60} y={46} s={0.8} />
      <Sparkle x={50} y={118} s={0.6} fill={BRAND} />
      <Sparkle x={168} y={40} s={0.6} />
    </Frame>
  );
}

/** Announcements: a megaphone with sound waves. */
export function MegaphoneIllustration({ className }: IllustrationProps) {
  return (
    <Frame className={className}>
      <Blob variant={1} />
      <Ground cx={110} cy={156} rx={50} />
      <g transform="rotate(-14 112 96)">
        <path d="M96 104l8 30h14l-6-26" fill={GOLD} {...stroke} strokeWidth={2} />
        <rect x="62" y="80" width="20" height="32" rx="6" fill={BRAND} {...stroke} />
        <path d="M80 82l70-32v92l-70-32Z" fill={PAPER} {...stroke} />
        <ellipse cx="150" cy="96" rx="9" ry="46" fill={TINT} {...stroke} />
      </g>
      <g stroke={GOLD} strokeWidth="3.5" strokeLinecap="round">
        <path d="M178 54c8 8 10 18 8 28" />
        <path d="M192 40c12 12 16 30 12 46" />
      </g>
      <Sparkle x={52} y={52} s={0.8} fill={BRAND} />
      <Sparkle x={200} y={120} s={0.6} />
    </Frame>
  );
}

export const illustrations = {
  idea: IdeaIllustration,
  projects: ProjectsIllustration,
  search: SearchIllustration,
  inbox: InboxIllustration,
  team: TeamIllustration,
  trophy: TrophyIllustration,
  chart: ChartIllustration,
  mentor: MentorIllustration,
  error: ErrorIllustration,
  certificate: CertificateIllustration,
  rocket: RocketIllustration,
  target: TargetIllustration,
  partners: PartnersIllustration,
  university: UniversityIllustration,
  pitch: PitchIllustration,
  calendar: CalendarIllustration,
  shield: ShieldIllustration,
  route: RouteIllustration,
  puzzle: PuzzleIllustration,
  funding: FundingIllustration,
  passport: PassportIllustration,
  lab: LabIllustration,
  vote: VoteIllustration,
  growth: GrowthIllustration,
  megaphone: MegaphoneIllustration,
} as const;

export type IllustrationName = keyof typeof illustrations;

/** An illustration of the set by its name. */
export function Illustration({ name, className }: { name: IllustrationName; className?: string }) {
  const Drawing = illustrations[name];
  return <Drawing className={className} />;
}
