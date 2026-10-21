import { useLayoutEffect, useRef, useState, type ElementType } from 'react';

import { cn } from '../lib/cn';

import { Tooltip } from './tooltip';

interface TruncatedTextProps {
  children: string;
  as?: ElementType;
  /** Number of lines before the ellipsis. */
  lines?: 1 | 2 | 3;
  className?: string;
}

/**
 * Text cut with an ellipsis that reveals the full value in a tooltip — but
 * only when it is actually cut, so short values stay plain.
 */
export function TruncatedText({
  children,
  as: Tag = 'span',
  lines = 1,
  className,
}: TruncatedTextProps) {
  const ref = useRef<HTMLElement>(null);
  const [clipped, setClipped] = useState(false);

  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const measure = () => {
      setClipped(el.scrollWidth > el.clientWidth + 1 || el.scrollHeight > el.clientHeight + 1);
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => {
      observer.disconnect();
    };
  }, [children]);

  const clamp = { 1: 'truncate', 2: 'line-clamp-2', 3: 'line-clamp-3' }[lines];
  return (
    <Tooltip content={children} disabled={!clipped}>
      <Tag
        ref={ref}
        tabIndex={clipped ? 0 : undefined}
        className={cn('block min-w-0', clamp, className)}
      >
        {children}
      </Tag>
    </Tooltip>
  );
}
