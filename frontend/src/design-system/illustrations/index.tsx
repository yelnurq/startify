/*
 * Qadam illustration set for empty states and onboarding. One visual grammar:
 * a soft background blob, objects drawn with a 2.5px ink outline, sky blue and
 * gold accents from the flag, small four-point sparkles. Colours come from the
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
} as const;

export type IllustrationName = keyof typeof illustrations;
