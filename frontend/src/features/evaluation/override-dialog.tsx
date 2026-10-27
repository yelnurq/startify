import { Minus, Plus } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Button,
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Field,
  IconButton,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Slider,
  Textarea,
  toast,
} from '@/design-system';
import { errorMessage } from '@/shared/api/errors';
import { useLocalized } from '@/shared/i18n/localized';

import { useOverrideScore, type Evaluation } from './api';
import { OVERRIDE_LIMITS } from './logic';

export function OverrideDialog({
  open,
  onOpenChange,
  projectId,
  evaluation,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectId: string;
  evaluation: Evaluation;
}) {
  const { t } = useTranslation(['evaluation', 'common']);
  const text = useLocalized();
  const override = useOverrideScore(projectId, evaluation.id);
  const [key, setKey] = useState(evaluation.criteria[0]?.key ?? '');
  const criterion = evaluation.criteria.find((c) => c.key === key);
  const [score, setScore] = useState(criterion?.score ?? 5);
  const [comment, setComment] = useState('');
  const [touched, setTouched] = useState(false);
  const [min, max] = OVERRIDE_LIMITS.score;
  const short = comment.trim().length < OVERRIDE_LIMITS.comment[0];
  const unchanged = criterion?.score === score;

  const choose = (next: string) => {
    setKey(next);
    const c = evaluation.criteria.find((x) => x.key === next);
    if (c) setScore(c.score);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form
          className="contents"
          onSubmit={(e) => {
            e.preventDefault();
            setTouched(true);
            if (short || unchanged || !criterion) return;
            override.mutate(
              { criterion: criterion.key, score, comment: comment.trim() },
              {
                onSuccess: () => {
                  toast.success(t('override.saved'));
                  setComment('');
                  setTouched(false);
                  onOpenChange(false);
                },
                onError: (err) => {
                  toast.error(errorMessage(err));
                },
              },
            );
          }}
        >
          <DialogHeader>
            <DialogTitle>{t('override.title')}</DialogTitle>
            <DialogDescription>{t('override.description')}</DialogDescription>
          </DialogHeader>
          <DialogBody className="flex flex-col gap-5">
            <Field label={t('override.criterion')} required>
              <Select value={key} onValueChange={choose}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {evaluation.criteria.map((c) => (
                    <SelectItem key={c.key} value={c.key}>
                      {text(c.title)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
            {criterion && (
              <p className="flex flex-wrap gap-x-4 gap-y-1 text-sm text-fg-muted">
                <span>{t('override.aiScore', { value: criterion.ai_score })}</span>
                <span>{t('override.currentScore', { value: criterion.score })}</span>
              </p>
            )}
            <Field label={t('override.score')} required>
              <div className="flex items-center gap-3">
                <IconButton
                  label={t('override.decrease')}
                  icon={<Minus />}
                  variant="secondary"
                  disabled={score <= min}
                  onClick={() => {
                    setScore((v) => Math.max(min, v - 1));
                  }}
                />
                <Slider
                  min={min}
                  max={max}
                  step={1}
                  value={[score]}
                  thumbLabels={[t('override.score')]}
                  onValueChange={([v]) => {
                    if (v !== undefined) setScore(v);
                  }}
                />
                <IconButton
                  label={t('override.increase')}
                  icon={<Plus />}
                  variant="secondary"
                  disabled={score >= max}
                  onClick={() => {
                    setScore((v) => Math.min(max, v + 1));
                  }}
                />
                <output
                  aria-live="polite"
                  className="w-10 text-center font-figures text-2xl font-bold text-ink tabular-nums"
                >
                  {score}
                </output>
              </div>
            </Field>
            <Field
              label={t('override.comment')}
              required
              hint={t('override.commentHint')}
              error={
                touched && short
                  ? t('common:validation.minLength', { count: OVERRIDE_LIMITS.comment[0] })
                  : undefined
              }
              counter={{ value: comment.length, max: OVERRIDE_LIMITS.comment[1] }}
            >
              <Textarea
                value={comment}
                rows={3}
                maxLength={OVERRIDE_LIMITS.comment[1]}
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
            <Button type="submit" loading={override.isPending} disabled={unchanged}>
              {t('override.save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
