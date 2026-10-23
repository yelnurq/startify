import { cn } from '../lib/cn';

interface SkipLinkProps {
  /** Text of the link, e.g. «Перейти к содержимому». */
  children: string;
  /** The content to jump to: the layout's `<main id="main" tabIndex={-1}>`. */
  href?: string;
  className?: string;
}

/**
 * The first stop of the keyboard on every layout: hidden until focused, it
 * jumps past the header and the navigation to the content (WCAG 2.4.1).
 */
export function SkipLink({ children, href = '#main', className }: SkipLinkProps) {
  return (
    <a
      href={href}
      className={cn(
        'sr-only z-50 rounded-control bg-primary px-4 py-2.5 text-sm font-semibold text-primary-fg shadow-lg',
        'focus:not-sr-only focus:fixed focus:top-3 focus:left-3',
        className,
      )}
    >
      {children}
    </a>
  );
}
