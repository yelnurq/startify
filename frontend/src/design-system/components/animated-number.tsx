import { useEffect, useRef, useState } from 'react';

import { cn } from '../lib/cn';
import { useReducedMotion } from '../lib/reduced-motion';

interface AnimatedNumberProps {
  value: number;
  format: (value: number) => string;
  /** Seconds. */
  duration?: number;
  className?: string;
}

// The `--ease-out` curve of the tokens (expo out), close enough for counting.
function easeOut(t: number): number {
  return t >= 1 ? 1 : 1 - 2 ** (-10 * t);
}

/**
 * Counts up to the value once it scrolls into view, and between values when it
 * changes. Screen readers get the final value only, not every frame.
 */
export function AnimatedNumber({ value, format, duration = 1.1, className }: AnimatedNumberProps) {
  const ref = useRef<HTMLSpanElement>(null);
  const from = useRef(0);
  // Without the API (old browsers, tests) the number is shown at once.
  const [inView, setInView] = useState(() => typeof IntersectionObserver === 'undefined');
  const reduce = useReducedMotion();

  useEffect(() => {
    const el = ref.current;
    if (!el || inView) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          setInView(true);
          observer.disconnect();
        }
      },
      { rootMargin: '0px 0px -10% 0px' },
    );
    observer.observe(el);
    return () => {
      observer.disconnect();
    };
  }, [inView]);

  useEffect(() => {
    const el = ref.current;
    if (!el || !inView) return;
    const start = from.current;
    from.current = value;
    if (reduce || start === value) {
      el.textContent = format(value);
      return;
    }
    let frame = 0;
    const began = performance.now();
    const tick = (now: number) => {
      const progress = Math.min((now - began) / (duration * 1000), 1);
      el.textContent = format(start + (value - start) * easeOut(progress));
      if (progress < 1) frame = requestAnimationFrame(tick);
    };
    frame = requestAnimationFrame(tick);
    return () => {
      cancelAnimationFrame(frame);
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
