import { useTranslation } from 'react-i18next';

import { cn } from '../lib/cn';

export interface DiffPart {
  kind: 'same' | 'added' | 'removed';
  text: string;
}

interface DiffTextProps {
  parts: DiffPart[];
  className?: string;
}

/**
 * A change between two texts: added words highlighted, removed words struck
 * through. Screen readers hear "added" and "removed" before each change.
 */
export function DiffText({ parts, className }: DiffTextProps) {
  const { t } = useTranslation();
  return (
    <p className={cn('text-sm leading-relaxed break-words whitespace-pre-wrap text-fg', className)}>
      {parts.map((part, index) => {
        if (part.kind === 'same') return <span key={index}>{part.text}</span>;
        if (part.kind === 'added') {
          return (
            <ins
              key={index}
              className="rounded-sm bg-success-soft px-0.5 text-success-soft-fg no-underline"
            >
              <span className="sr-only">{t('diff.added')}</span>
              {part.text}
            </ins>
          );
        }
        return (
          <del key={index} className="rounded-sm bg-danger-soft px-0.5 text-danger-soft-fg">
            <span className="sr-only">{t('diff.removed')}</span>
            {part.text}
          </del>
        );
      })}
    </p>
  );
}
