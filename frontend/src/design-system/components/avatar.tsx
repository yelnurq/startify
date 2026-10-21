import { Avatar as AvatarPrimitive } from 'radix-ui';
import { useTranslation } from 'react-i18next';

import { cn } from '../lib/cn';

import { Tooltip } from './tooltip';

const sizes = {
  xs: 'size-6 text-xs',
  sm: 'size-8 text-xs',
  md: 'size-10 text-sm',
  lg: 'size-14 text-lg',
  xl: 'size-20 text-2xl',
} as const;

const presenceSizes = { xs: 'size-2', sm: 'size-2.5', md: 'size-3', lg: 'size-3.5', xl: 'size-4' };

// Calm, theme-aware backgrounds for initials, picked deterministically by name.
const palettes = [
  'bg-primary-100 text-primary-800 dark:bg-primary-900 dark:text-primary-200',
  'bg-gold-100 text-gold-800 dark:bg-gold-900 dark:text-gold-200',
  'bg-success-100 text-success-800 dark:bg-success-900 dark:text-success-200',
  'bg-info-100 text-info-800 dark:bg-info-900 dark:text-info-200',
  'bg-danger-100 text-danger-800 dark:bg-danger-900 dark:text-danger-200',
  'bg-warning-100 text-warning-800 dark:bg-warning-900 dark:text-warning-200',
];

function hash(value: string): number {
  let h = 0;
  for (const ch of value) h = (h * 31 + (ch.codePointAt(0) ?? 0)) >>> 0;
  return h;
}

export function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  const first = parts[0]?.[0] ?? '';
  const second = parts.length > 1 ? (parts[parts.length - 1]?.[0] ?? '') : '';
  return (first + second).toUpperCase();
}

export type AvatarSize = keyof typeof sizes;

interface AvatarProps {
  name: string;
  src?: string | null;
  size?: AvatarSize;
  /** Shows a presence dot (online in the project right now). */
  online?: boolean;
  /** Separating ring for stacked avatars. */
  ringed?: boolean;
  className?: string;
}

export function Avatar({
  name,
  src,
  size = 'md',
  online = false,
  ringed = false,
  className,
}: AvatarProps) {
  const { t } = useTranslation();
  return (
    <span className={cn('relative inline-flex shrink-0', className)}>
      <AvatarPrimitive.Root
        className={cn(
          'inline-flex shrink-0 items-center justify-center overflow-hidden rounded-full font-semibold select-none',
          sizes[size],
          palettes[hash(name) % palettes.length],
          ringed && 'ring-2 ring-surface',
        )}
      >
        {src && <AvatarPrimitive.Image src={src} alt={name} className="size-full object-cover" />}
        <AvatarPrimitive.Fallback aria-label={name} role="img" delayMs={src ? 300 : 0}>
          {initials(name)}
        </AvatarPrimitive.Fallback>
      </AvatarPrimitive.Root>
      {online && (
        <span
          role="status"
          aria-label={t('presence.online')}
          className={cn(
            'absolute right-0 bottom-0 rounded-full bg-success-500 ring-2 ring-surface',
            presenceSizes[size],
          )}
        />
      )}
    </span>
  );
}

interface AvatarGroupProps {
  people: { name: string; src?: string | null; online?: boolean }[];
  max?: number;
  size?: AvatarSize;
  className?: string;
}

const overlap: Record<AvatarSize, string> = {
  xs: '-space-x-1',
  sm: '-space-x-1.5',
  md: '-space-x-2',
  lg: '-space-x-3',
  xl: '-space-x-4',
};

export function AvatarGroup({ people, max = 4, size = 'sm', className }: AvatarGroupProps) {
  const { t } = useTranslation();
  const shown = people.slice(0, max);
  const rest = people.length - shown.length;
  return (
    <div className={cn('flex items-center', overlap[size], className)}>
      {shown.map((person) => (
        <Tooltip key={person.name} content={person.name}>
          <span className="inline-flex rounded-full">
            <Avatar name={person.name} src={person.src} size={size} online={person.online} ringed />
          </span>
        </Tooltip>
      ))}
      {rest > 0 && (
        <Tooltip content={t('avatars.more', { count: rest })}>
          <span
            className={cn(
              'inline-flex items-center justify-center rounded-full bg-surface-sunken font-semibold text-fg-muted ring-2 ring-surface',
              sizes[size],
            )}
          >
            +{rest}
          </span>
        </Tooltip>
      )}
    </div>
  );
}
