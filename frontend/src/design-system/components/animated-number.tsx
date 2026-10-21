import { animate, useInView, useReducedMotion } from 'motion/react';
import { useEffect, useRef } from 'react';

import { cn } from '../lib/cn';

interface AnimatedNumberProps {
  value: number;
  format: (value: number) => string;
  /** Seconds. */
  duration?: number;
  className?: string;
}

/**
 * Counts up to the value once it scrolls into view, and between values when it
 * changes. Screen readers get the final value only, not every frame.
 */
export function AnimatedNumber({ value, format, duration = 1.1, className }: AnimatedNumberProps) {
  const ref = useRef<HTMLSpanElement>(null);
  const from = useRef(0);
  const inView = useInView(ref, { once: true, margin: '0px 0px -10% 0px' });
  const reduce = useReducedMotion();

  useEffect(() => {
    const el = ref.current;
    if (!el || !inView) return;
    if (reduce) {
      el.textContent = format(value);
      from.current = value;
      return;
    }
    const controls = animate(from.current, value, {
      duration,
      ease: [0.16, 1, 0.3, 1],
      onUpdate: (latest) => {
        el.textContent = format(latest);
      },
    });
    from.current = value;
    return () => {
      controls.stop();
    };
  }, [inView, value, reduce, duration, format]);

  return (
    <span className={cn('tabular-nums', className)}>
      <span ref={ref} aria-hidden>
        {format(reduce ? value : 0)}
      </span>
      <span className="sr-only">{format(value)}</span>
    </span>
  );
}
