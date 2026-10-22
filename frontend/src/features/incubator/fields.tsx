import { ArrowDown, ArrowUp, Plus, Trash2 } from 'lucide-react';
import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Button,
  Field,
  FieldSet,
  IconButton,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Textarea,
  TruncatedText,
} from '@/design-system';
import { useFormatters } from '@/shared/i18n/format';
import { useLocalized } from '@/shared/i18n/localized';

import { formatNumberInput, parseNumberInput } from './numbers';
import {
  listValue,
  numberValue,
  rowsValue,
  textValue,
  type FieldValue,
  type Row,
  type TemplateField,
} from './values';

interface FieldProps {
  field: TemplateField;
  value: FieldValue;
  onChange: (value: FieldValue) => void;
  disabled?: boolean;
  /** Message under the field (server error or what completion still needs). */
  error?: string | undefined;
  /** Comments and history buttons in the label row. */
  actions?: ReactNode;
  /** DOM id of the control, for scrolling to a missing field. */
  id?: string;
}

const DEFAULT_MAX = { text: 200, textarea: 4000, url: 500 } as const;

function maxLength(field: TemplateField): number {
  if (field.max_length) return field.max_length;
  if (field.type === 'textarea') return DEFAULT_MAX.textarea;
  if (field.type === 'url') return DEFAULT_MAX.url;
  return DEFAULT_MAX.text;
}

/** One answer of a step, rendered by its template type. */
export function StepField({ field, value, onChange, disabled, error, actions, id }: FieldProps) {
  const text = useLocalized();
  const label = text(field.label);
  const hint = field.hint ? text(field.hint) : undefined;
  const placeholder = field.placeholder ? text(field.placeholder) : undefined;

  if (field.type === 'list') {
    return (
      <FieldSet
        legend={label}
        hint={hint}
        error={error}
        required={field.required}
        actions={actions}
        className="scroll-mt-24"
      >
        <span id={id} />
        <ListEditor
          field={field}
          items={listValue(value)}
          label={label}
          disabled={disabled}
          onChange={onChange}
        />
      </FieldSet>
    );
  }
  if (field.type === 'table') {
    return (
      <FieldSet
        legend={label}
        hint={hint}
        error={error}
        required={field.required}
        actions={actions}
        className="scroll-mt-24"
      >
        <span id={id} />
        <TableEditor
          field={field}
          rows={rowsValue(value)}
          disabled={disabled}
          onChange={onChange}
        />
      </FieldSet>
    );
  }

  const textual = textValue(value);
  const counter =
    field.type === 'textarea' || field.max_length
      ? { value: Array.from(textual).length, max: maxLength(field) }
      : undefined;
  return (
    <Field
      label={label}
      hint={hint}
      error={error}
      required={field.required}
      counter={counter}
      actions={actions}
      id={id}
      className="scroll-mt-24"
    >
      <Control
        field={field}
        value={value}
        onChange={onChange}
        disabled={disabled}
        placeholder={placeholder}
      />
    </Field>
  );
}

function Control({
  field,
  value,
  onChange,
  disabled,
  placeholder,
  label,
  compact = false,
}: {
  field: TemplateField;
  value: FieldValue;
  onChange: (value: FieldValue) => void;
  disabled?: boolean | undefined;
  placeholder?: string | undefined;
  /** Accessible name when the control is not inside a labelled Field. */
  label?: string;
  /** Smaller text areas inside table rows. */
  compact?: boolean;
}) {
  const { t } = useTranslation('incubator');
  const text = useLocalized();
  switch (field.type) {
    case 'textarea':
      return (
        <Textarea
          value={textValue(value)}
          maxLength={maxLength(field)}
          rows={compact ? 2 : 3}
          placeholder={placeholder}
          disabled={disabled}
          aria-label={label}
          className={compact ? 'min-h-16' : 'min-h-28'}
          onChange={(e) => {
            onChange(e.target.value);
          }}
        />
      );
    case 'number':
      return (
        <NumberInput
          value={numberValue(value)}
          unit={field.unit}
          disabled={disabled}
          label={label}
          onChange={onChange}
        />
      );
    case 'select':
      return (
        <Select
          value={textValue(value) || undefined}
          disabled={disabled}
          onValueChange={(v) => {
            onChange(v);
          }}
        >
          <SelectTrigger aria-label={label}>
            <SelectValue placeholder={t('fields.choose')} />
          </SelectTrigger>
          <SelectContent>
            {(field.options ?? []).map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {text(option.label)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      );
    case 'url':
      return (
        <Input
          type="url"
          inputMode="url"
          value={textValue(value)}
          placeholder={placeholder ?? t('fields.urlPlaceholder')}
          disabled={disabled}
          aria-label={label}
          onChange={(e) => {
            onChange(e.target.value);
          }}
        />
      );
    default:
      return (
        <Input
          value={textValue(value)}
          maxLength={maxLength(field)}
          placeholder={placeholder}
          disabled={disabled}
          aria-label={label}
          onChange={(e) => {
            onChange(e.target.value);
          }}
        />
      );
  }
}

function NumberInput({
  value,
  unit,
  disabled,
  label,
  onChange,
}: {
  value: number | null;
  unit: TemplateField['unit'];
  disabled?: boolean | undefined;
  label?: string | undefined;
  onChange: (value: FieldValue) => void;
}) {
  const { t } = useTranslation('incubator');
  const { locale } = useFormatters();
  const [draft, setDraft] = useState<string | null>(null);
  const shown = draft ?? formatNumberInput(value, locale);
  const invalid = draft !== null && parseNumberInput(draft) === undefined;
  return (
    <div className="flex flex-col gap-1">
      <Input
        inputMode="decimal"
        value={shown}
        disabled={disabled}
        aria-label={label}
        aria-invalid={invalid || undefined}
        className="tabular-nums"
        trailing={unit ? <span className="text-sm">{t(`fields.units.${unit}`)}</span> : undefined}
        onFocus={() => {
          setDraft(value === null ? '' : String(value));
        }}
        onBlur={() => {
          setDraft(null);
        }}
        onChange={(e) => {
          setDraft(e.target.value);
          const parsed = parseNumberInput(e.target.value);
          if (parsed !== undefined) onChange(parsed);
        }}
      />
      {invalid && <span className="text-sm text-danger">{t('fields.invalidNumber')}</span>}
    </div>
  );
}

function move<T>(items: T[], from: number, to: number): T[] {
  const out = [...items];
  const [item] = out.splice(from, 1);
  if (item !== undefined) out.splice(to, 0, item);
  return out;
}

function ListEditor({
  field,
  items,
  label,
  disabled,
  onChange,
}: {
  field: TemplateField;
  items: string[];
  label: string;
  disabled?: boolean | undefined;
  onChange: (value: FieldValue) => void;
}) {
  const { t } = useTranslation('incubator');
  const max = field.max_items ?? 20;
  const shown = items.length ? items : disabled ? [] : [''];
  return (
    <div className="flex flex-col gap-2">
      {shown.length === 0 && <p className="text-sm text-fg-subtle">{t('fields.empty')}</p>}
      <ol className="flex flex-col gap-2">
        {shown.map((item, index) => (
          <li key={index} className="flex items-center gap-1.5">
            <span className="w-5 shrink-0 text-right text-xs text-fg-subtle tabular-nums">
              {index + 1}
            </span>
            <Input
              value={item}
              maxLength={field.max_length ?? 300}
              disabled={disabled}
              aria-label={t('fields.item', { label, number: index + 1 })}
              onChange={(e) => {
                const next = [...shown];
                next[index] = e.target.value;
                onChange(next);
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.nativeEvent.isComposing) {
                  e.preventDefault();
                  if (index === shown.length - 1 && shown.length < max) onChange([...shown, '']);
                }
              }}
            />
            {!disabled && (
              <span className="flex shrink-0">
                <IconButton
                  size="sm"
                  label={t('fields.moveUp')}
                  icon={<ArrowUp />}
                  disabled={index === 0}
                  onClick={() => {
                    onChange(move(shown, index, index - 1));
                  }}
                />
                <IconButton
                  size="sm"
                  label={t('fields.moveDown')}
                  icon={<ArrowDown />}
                  disabled={index === shown.length - 1}
                  onClick={() => {
                    onChange(move(shown, index, index + 1));
                  }}
                />
                <IconButton
                  size="sm"
                  label={t('fields.remove')}
                  icon={<Trash2 />}
                  onClick={() => {
                    onChange(shown.filter((_, i) => i !== index));
                  }}
                />
              </span>
            )}
          </li>
        ))}
      </ol>
      {!disabled && shown.length < max && (
        <Button
          variant="ghost"
          size="sm"
          className="self-start"
          leftIcon={<Plus />}
          onClick={() => {
            onChange([...shown, '']);
          }}
        >
          {t('fields.add')}
        </Button>
      )}
    </div>
  );
}

function TableEditor({
  field,
  rows,
  disabled,
  onChange,
}: {
  field: TemplateField;
  rows: Row[];
  disabled?: boolean | undefined;
  onChange: (value: FieldValue) => void;
}) {
  const { t } = useTranslation('incubator');
  const text = useLocalized();
  const columns = field.columns ?? [];
  const max = field.max_items ?? 20;
  const update = (index: number, key: string, value: FieldValue) => {
    const next = rows.map((row, i) =>
      i === index ? { ...row, [key]: (value ?? null) as Row[string] } : row,
    );
    onChange(next);
  };
  return (
    <div className="flex flex-col gap-3">
      {rows.length === 0 && <p className="text-sm text-fg-subtle">{t('fields.empty')}</p>}
      {rows.map((row, index) => {
        const title = textValue(row[columns[0]?.key ?? '']).trim();
        return (
          <div
            key={index}
            className="@container flex flex-col gap-3 rounded-xl border border-border bg-surface-raised p-3 sm:p-4"
          >
            <div className="flex items-center gap-2">
              <span className="grid size-6 shrink-0 place-items-center rounded-full bg-surface text-xs font-semibold text-fg-muted tabular-nums">
                {index + 1}
              </span>
              <TruncatedText className="flex-1 text-sm font-medium text-fg">
                {title || t('fields.row', { number: index + 1 })}
              </TruncatedText>
              {!disabled && (
                <span className="flex shrink-0">
                  <IconButton
                    size="sm"
                    label={t('fields.moveUp')}
                    icon={<ArrowUp />}
                    disabled={index === 0}
                    onClick={() => {
                      onChange(move(rows, index, index - 1));
                    }}
                  />
                  <IconButton
                    size="sm"
                    label={t('fields.moveDown')}
                    icon={<ArrowDown />}
                    disabled={index === rows.length - 1}
                    onClick={() => {
                      onChange(move(rows, index, index + 1));
                    }}
                  />
                  <IconButton
                    size="sm"
                    label={t('fields.removeRow', { number: index + 1 })}
                    icon={<Trash2 />}
                    onClick={() => {
                      onChange(rows.filter((_, i) => i !== index));
                    }}
                  />
                </span>
              )}
            </div>
            <div className="grid grid-cols-1 gap-3 @lg:grid-cols-2">
              {columns.map((column) => (
                <Field
                  key={column.key}
                  label={text(column.label)}
                  required={column.required}
                  hint={column.hint ? text(column.hint) : undefined}
                  className={column.type === 'textarea' ? '@lg:col-span-2' : undefined}
                >
                  <Control
                    field={column}
                    compact
                    value={row[column.key] as FieldValue}
                    disabled={disabled}
                    onChange={(v) => {
                      update(index, column.key, v);
                    }}
                  />
                </Field>
              ))}
            </div>
          </div>
        );
      })}
      {!disabled && rows.length < max && (
        <Button
          variant="secondary"
          size="sm"
          className="self-start"
          leftIcon={<Plus />}
          onClick={() => {
            onChange([...rows, {}]);
          }}
        >
          {t('fields.addRow')}
        </Button>
      )}
    </div>
  );
}
