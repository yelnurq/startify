import { Plus } from 'lucide-react';
import { useState, type KeyboardEvent } from 'react';
import { useTranslation } from 'react-i18next';

import { Button, Input, Tag } from '@/design-system';

interface TagPickerProps {
  value: string[];
  onChange: (value: string[]) => void;
  /** Ready-made options shown as toggles under the input. */
  suggestions?: readonly string[];
  max: number;
  maxLength?: number;
  placeholder?: string;
  /** Accessible name of the suggestion group. */
  suggestionsLabel?: string;
}

function normalize(tag: string): string {
  return tag.trim().replace(/\s+/g, ' ');
}

function contains(list: readonly string[], tag: string): boolean {
  const key = tag.toLocaleLowerCase();
  return list.some((item) => item.toLocaleLowerCase() === key);
}

/** Free-form tags with suggestions: skills, interests, technologies. */
export function TagPicker({
  value,
  onChange,
  suggestions = [],
  max,
  maxLength = 40,
  placeholder,
  suggestionsLabel,
}: TagPickerProps) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState('');
  const full = value.length >= max;

  const add = (raw: string) => {
    const tag = normalize(raw).slice(0, maxLength);
    if (!tag || full || contains(value, tag)) return;
    onChange([...value, tag]);
  };
  const remove = (tag: string) => {
    onChange(value.filter((item) => item !== tag));
  };
  const commitDraft = () => {
    add(draft);
    setDraft('');
  };

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter' || event.key === ',') {
      event.preventDefault();
      commitDraft();
    } else if (event.key === 'Backspace' && draft === '' && value.length > 0) {
      onChange(value.slice(0, -1));
    }
  };

  const rest = suggestions.filter((item) => !contains(value, item));

  return (
    <div className="flex flex-col gap-3">
      {value.length > 0 && (
        <ul className="flex flex-wrap gap-2">
          {value.map((tag) => (
            <li key={tag}>
              <Tag
                selected
                removeLabel={t('tags.remove', { tag })}
                onRemove={() => {
                  remove(tag);
                }}
              >
                {tag}
              </Tag>
            </li>
          ))}
        </ul>
      )}
      <div className="flex gap-2">
        <Input
          value={draft}
          maxLength={maxLength}
          placeholder={placeholder ?? t('tags.placeholder')}
          disabled={full}
          enterKeyHint="done"
          onChange={(event) => {
            setDraft(event.target.value);
          }}
          onKeyDown={onKeyDown}
          className="flex-1"
        />
        <Button
          type="button"
          variant="secondary"
          leftIcon={<Plus />}
          disabled={full || normalize(draft) === ''}
          onClick={commitDraft}
        >
          {t('tags.add')}
        </Button>
      </div>
      <p className="text-xs text-fg-subtle tabular-nums" aria-live="polite">
        {full ? t('tags.limit', { max }) : t('tags.counter', { value: value.length, max })}
      </p>
      {rest.length > 0 && !full && (
        <div className="flex flex-col gap-2">
          <p className="text-xs font-medium text-fg-subtle">
            {suggestionsLabel ?? t('tags.suggestions')}
          </p>
          <ul
            className="flex flex-wrap gap-2"
            aria-label={suggestionsLabel ?? t('tags.suggestions')}
          >
            {rest.map((tag) => (
              <li key={tag}>
                <Tag
                  icon={<Plus aria-hidden />}
                  onToggle={() => {
                    add(tag);
                  }}
                >
                  {tag}
                </Tag>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
