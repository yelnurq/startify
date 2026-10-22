import { CircleAlert } from 'lucide-react';
import { createContext, use, useId, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { cn } from '../lib/cn';

interface FieldContextValue {
  id: string;
  describedBy: string | undefined;
  invalid: boolean;
  required: boolean;
}

const FieldContext = createContext<FieldContextValue | null>(null);

/**
 * Accessibility attributes for a control rendered inside <Field>: the label
 * targets its id, the hint and error describe it, invalid state is announced.
 */
export function useFieldControl(props: {
  id?: string | undefined;
  'aria-describedby'?: string | undefined;
  'aria-invalid'?: boolean | 'true' | 'false' | 'grammar' | 'spelling' | undefined;
  required?: boolean | undefined;
}) {
  const field = use(FieldContext);
  if (!field) return props;
  const describedBy = [field.describedBy, props['aria-describedby']].filter(Boolean).join(' ');
  return {
    id: props.id ?? field.id,
    'aria-describedby': describedBy || undefined,
    'aria-invalid': props['aria-invalid'] ?? (field.invalid || undefined),
    required: props.required ?? (field.required || undefined),
  };
}

interface FieldProps {
  label: ReactNode;
  hint?: ReactNode;
  error?: ReactNode;
  required?: boolean;
  /** Marks the field as optional next to the label. */
  optional?: boolean;
  /** Character counter, e.g. { value: 120, max: 280 }. */
  counter?: { value: number; max: number };
  /** Small buttons at the end of the label row (comments, history). */
  actions?: ReactNode;
  id?: string;
  className?: string;
  children: ReactNode;
}

export function Field({
  label,
  hint,
  error,
  required = false,
  optional = false,
  counter,
  actions,
  id,
  className,
  children,
}: FieldProps) {
  const { t } = useTranslation();
  const generated = useId();
  const controlId = id ?? `field-${generated}`;
  const hintId = hint ? `${controlId}-hint` : undefined;
  const errorId = error ? `${controlId}-error` : undefined;
  const counterId = counter ? `${controlId}-counter` : undefined;
  const over = counter ? counter.value > counter.max : false;

  return (
    <FieldContext
      value={{
        id: controlId,
        describedBy: [errorId, hintId, counterId].filter(Boolean).join(' ') || undefined,
        invalid: Boolean(error) || over,
        required,
      }}
    >
      <div className={cn('flex flex-col gap-1.5', className)}>
        <div className="flex items-baseline justify-between gap-3">
          <label
            htmlFor={controlId}
            className="text-2xs font-bold tracking-label text-ink uppercase"
          >
            {label}
            {required && (
              <span aria-hidden className="ml-0.5 text-danger">
                *
              </span>
            )}
            {optional && (
              <span className="ml-1.5 text-xs font-normal tracking-normal text-fg-subtle normal-case">
                {t('field.optional')}
              </span>
            )}
          </label>
          {(counter ?? actions) && (
            <span className="flex shrink-0 items-center gap-1">
              {counter && (
                <span
                  id={counterId}
                  className={cn('text-xs tabular-nums', over ? 'text-danger' : 'text-fg-subtle')}
                >
                  {t('field.counter', { value: counter.value, max: counter.max })}
                </span>
              )}
              {actions}
            </span>
          )}
        </div>
        {children}
        {error ? (
          <p
            id={errorId}
            className="flex items-start gap-1.5 text-sm text-danger"
            aria-live="polite"
          >
            <CircleAlert className="mt-0.5 size-4 shrink-0" aria-hidden />
            <span>{error}</span>
          </p>
        ) : (
          hint && (
            <p id={hintId} className="text-sm text-fg-subtle">
              {hint}
            </p>
          )
        )}
      </div>
    </FieldContext>
  );
}

interface FieldSetProps {
  legend: ReactNode;
  hint?: ReactNode;
  error?: ReactNode;
  required?: boolean;
  optional?: boolean;
  actions?: ReactNode;
  className?: string;
  children: ReactNode;
}

/** A group of controls answering one question (a list, a table) under one legend. */
export function FieldSet({
  legend,
  hint,
  error,
  required = false,
  optional = false,
  actions,
  className,
  children,
}: FieldSetProps) {
  const { t } = useTranslation();
  const generated = useId();
  const hintId = hint ? `fieldset-${generated}-hint` : undefined;
  const errorId = error ? `fieldset-${generated}-error` : undefined;
  return (
    <fieldset
      aria-describedby={[errorId, hintId].filter(Boolean).join(' ') || undefined}
      aria-invalid={error ? true : undefined}
      className={cn('flex min-w-0 flex-col gap-1.5', className)}
    >
      <div className="flex items-baseline justify-between gap-3">
        <legend className="float-left text-2xs font-bold tracking-label text-ink uppercase">
          {legend}
          {required && (
            <span aria-hidden className="ml-0.5 text-danger">
              *
            </span>
          )}
          {optional && (
            <span className="ml-1.5 text-xs font-normal tracking-normal text-fg-subtle normal-case">
              {t('field.optional')}
            </span>
          )}
        </legend>
        {actions && <span className="flex shrink-0 items-center gap-1">{actions}</span>}
      </div>
      {hint && !error && (
        <p id={hintId} className="text-sm text-fg-subtle">
          {hint}
        </p>
      )}
      {children}
      {error && (
        <p id={errorId} className="flex items-start gap-1.5 text-sm text-danger" aria-live="polite">
          <CircleAlert className="mt-0.5 size-4 shrink-0" aria-hidden />
          <span>{error}</span>
        </p>
      )}
    </fieldset>
  );
}
