import { Illustration, type IllustrationName } from '../illustrations';
import { cn } from '../lib/cn';

const placements = {
  left: 'top-1/2 left-0 -translate-x-1/3 -translate-y-1/2',
  right: 'top-1/2 right-0 translate-x-1/3 -translate-y-1/2',
  'top-left': 'top-0 left-0 -translate-x-1/4 -translate-y-1/4',
  'top-right': 'top-0 right-0 translate-x-1/4 -translate-y-1/4',
  'bottom-left': 'bottom-0 left-0 -translate-x-1/4 translate-y-1/4',
  'bottom-right': 'right-0 bottom-0 translate-x-1/4 translate-y-1/4',
  /** Anywhere in the parent: the position comes with the class name. */
  free: '',
} as const;

export type EdgePlacement = keyof typeof placements;

interface EdgeIllustrationProps {
  name: IllustrationName;
  /** The edge or corner of the nearest positioned parent it peeks out of, or `free`. */
  placement: EdgePlacement;
  /** Size, tilt and from which width it shows, e.g. "hidden w-64 -rotate-6 xl:block". */
  className?: string;
}

/**
 * An illustration that peeks out of an edge or a corner of a section or a
 * card. The parent is `relative` and clips it (`overflow-hidden`, or
 * `overflow-x-clip` where a child is sticky); it is decoration only.
 */
export function EdgeIllustration({ name, placement, className }: EdgeIllustrationProps) {
  return (
    <div
      aria-hidden
      data-slot="edge-illustration"
      className={cn(
        'pointer-events-none absolute w-56 select-none',
        placements[placement],
        className,
      )}
    >
      <Illustration name={name} className="max-w-none" />
    </div>
  );
}
