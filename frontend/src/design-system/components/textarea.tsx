import type { ComponentProps } from 'react';

import { cn } from '../lib/cn';

import { useFieldControl } from './field';
import { controlClasses } from './input';

interface TextareaProps extends ComponentProps<'textarea'> {
  /** Grow with the content (CSS field-sizing) up to the max height. */
  autoGrow?: boolean;
}

export function Textarea({
  className,
  autoGrow = true,
  id,
  required,
  'aria-describedby': describedBy,
  'aria-invalid': invalid,
  ...props
}: TextareaProps) {
  const a11y = useFieldControl({
    id,
    required,
    'aria-describedby': describedBy,
    'aria-invalid': invalid,
  });
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        controlClasses,
        'min-h-24 px-3 py-2.5 leading-relaxed',
        autoGrow ? 'field-sizing-content max-h-[28rem] resize-none' : 'resize-y',
        className,
      )}
      {...a11y}
      {...props}
    />
  );
}
