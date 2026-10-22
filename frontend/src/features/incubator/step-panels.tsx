import { CircleAlert, CircleCheck, CircleDashed, Sparkles } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Badge,
  Button,
  Card,
  cn,
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DiffText,
  Field,
  SegmentedControl,
  Textarea,
} from '@/design-system';
import { useFormatters } from '@/shared/i18n/format';
import { useLocalized } from '@/shared/i18n/localized';

import { INCUBATOR_LIMITS, type StepCheck, type TemplateStep } from './api';
import { itemDiff, wordDiff } from './diff';
import type { Resolution } from './merge';
import type { Conflict } from './step-editor';
import { valueToText, type TemplateField } from './values';

/** Shows how a value changed: by words for texts, by items for lists and tables. */
export function ValueDiff({
  before,
  after,
  field,
}: {
  before: unknown;
  after: unknown;
  field: TemplateField | undefined;
}) {
  const structured = field?.type === 'list' || field?.type === 'table';
  const a = valueToText(before, field);
  const b = valueToText(after, field);
  const parts = structured
    ? itemDiff(a ? a.split('\n') : [], b ? b.split('\n') : []).map((p) => ({
        ...p,
        text: `${p.text}\n`,
      }))
    : wordDiff(a, b);
  return <DiffText parts={parts} />;
}

/** Both sides changed the same fields: the person picks a version per field. */
export function ConflictDialog({
  conflict,
  step,
  onResolve,
}: {
  conflict: Conflict;
  step: TemplateStep;
  onResolve: (choices: Record<string, Resolution>) => void;
}) {
  const { t } = useTranslation('incubator');
  const text = useLocalized();
  const [choices, setChoices] = useState<Record<string, Resolution>>(() =>
    Object.fromEntries(conflict.fields.map((key): [string, Resolution] => [key, 'mine'])),
  );
  const by = conflict.latest.data.updated_by?.full_name;
  return (
    <Dialog open>
      <DialogContent
        size="lg"
        hideClose
        onEscapeKeyDown={(e) => {
          e.preventDefault();
        }}
      >
        <DialogHeader>
          <DialogTitle>{t('conflict.title')}</DialogTitle>
          <DialogDescription>
            {by ? t('conflict.descriptionBy', { name: by }) : t('conflict.description')}
          </DialogDescription>
        </DialogHeader>
        <DialogBody className="flex flex-col gap-6">
          {conflict.fields.map((key) => {
            const field = step.fields.find((f) => f.key === key);
            const label = field ? text(field.label) : key;
            return (
              <section key={key} className="flex flex-col gap-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <h3 className="text-sm font-semibold text-ink">{label}</h3>
                  <SegmentedControl
                    size="sm"
                    label={t('conflict.choose', { field: label })}
                    value={choices[key] ?? 'mine'}
                    onValueChange={(value) => {
                      setChoices((prev) => ({ ...prev, [key]: value }));
                    }}
                    options={[
                      { value: 'mine', label: t('conflict.mine') },
                      { value: 'theirs', label: t('conflict.theirs') },
                    ]}
                  />
                </div>
                <div className="rounded-lg border border-border bg-surface-raised p-3">
                  <p className="mb-1.5 text-xs font-medium text-fg-subtle">
                    {t('conflict.difference')}
                  </p>
                  <ValueDiff
                    before={conflict.theirs[key]}
                    after={conflict.mine[key]}
                    field={field}
                  />
                </div>
              </section>
            );
          })}
        </DialogBody>
        <DialogFooter>
          <Button
            onClick={() => {
              onResolve(choices);
            }}
          >
            {t('conflict.apply')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/** The supervisor or an innovation manager approves a step with a comment. */
export function ApproveDialog({
  open,
  onOpenChange,
  loading,
  onApprove,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  loading: boolean;
  onApprove: (comment: string) => void;
}) {
  const { t } = useTranslation(['incubator', 'common']);
  const [comment, setComment] = useState('');
  const [touched, setTouched] = useState(false);
  const empty = comment.trim() === '';
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form
          className="contents"
          onSubmit={(e) => {
            e.preventDefault();
            setTouched(true);
            if (!empty) onApprove(comment.trim());
          }}
        >
          <DialogHeader>
            <DialogTitle>{t('approve.title')}</DialogTitle>
            <DialogDescription>{t('approve.description')}</DialogDescription>
          </DialogHeader>
          <DialogBody>
            <Field
              label={t('approve.comment')}
              required
              error={touched && empty ? t('common:validation.required') : undefined}
              counter={{ value: comment.length, max: INCUBATOR_LIMITS.approval }}
            >
              <Textarea
                value={comment}
                maxLength={INCUBATOR_LIMITS.approval}
                rows={3}
                onChange={(e) => {
                  setComment(e.target.value);
                }}
              />
            </Field>
          </DialogBody>
          <DialogFooter>
            <Button
              variant="secondary"
              onClick={() => {
                onOpenChange(false);
              }}
            >
              {t('common:actions.cancel')}
            </Button>
            <Button type="submit" loading={loading}>
              {t('approve.submit')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

const STATUS_VIEW = {
  ok: { Icon: CircleCheck, tone: 'success', classes: 'text-success' },
  weak: { Icon: CircleDashed, tone: 'warning', classes: 'text-warning-soft-fg' },
  missing: { Icon: CircleAlert, tone: 'danger', classes: 'text-danger' },
} as const;

/** The AI review of a step: verdict, checklist with advice, model and prompt. */
export function CheckCard({ check, stale }: { check: StepCheck; stale: boolean }) {
  const { t } = useTranslation('incubator');
  const format = useFormatters();
  return (
    <Card className="overflow-hidden">
      <div
        className={cn(
          'flex flex-wrap items-start justify-between gap-3 border-b border-border px-5 py-4',
          check.ready ? 'bg-success-soft' : 'bg-primary-soft',
        )}
      >
        <div className="flex min-w-0 flex-col gap-1">
          <p className="flex items-center gap-2 text-sm font-semibold text-fg">
            <Sparkles className="size-4 text-brand" aria-hidden />
            {check.ready ? t('check.ready') : t('check.notReady')}
          </p>
          <p className="text-sm text-fg-muted">{check.summary}</p>
        </div>
        {stale && <Badge tone="warning">{t('check.stale')}</Badge>}
      </div>
      <ul className="flex flex-col divide-y divide-border">
        {check.items.map((item, index) => {
          const view = STATUS_VIEW[item.status];
          return (
            <li key={index} className="flex gap-3 px-5 py-3.5">
              <view.Icon className={cn('mt-0.5 size-5 shrink-0', view.classes)} aria-hidden />
              <div className="flex min-w-0 flex-col gap-1">
                <p className="flex flex-wrap items-center gap-2 text-sm font-medium text-fg">
                  {item.criterion}
                  <Badge size="sm" tone={view.tone}>
                    {t(`check.status.${item.status}`)}
                  </Badge>
                </p>
                <p className="text-sm text-fg-muted">{item.advice}</p>
              </div>
            </li>
          );
        })}
      </ul>
      <p className="border-t border-border bg-surface-raised px-5 py-3 text-xs text-fg-subtle">
        {t('check.note', {
          model: check.model,
          prompt: check.prompt_version,
          time: format.dateTime(check.checked_at, { dateStyle: 'medium', timeStyle: 'short' }),
        })}
      </p>
    </Card>
  );
}
