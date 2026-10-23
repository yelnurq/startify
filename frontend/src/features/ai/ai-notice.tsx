import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';

import { Callout, cn, Progress } from '@/design-system';
import { useFormatters } from '@/shared/i18n/format';

import { aiStatusQuery, type AiStatus } from './api';

interface NoticeProps {
  className?: string;
  /** Only the warning that data leaves the university, without the daily limit. */
  externalOnly?: boolean;
}

/**
 * Tells a member before they use AI that data leaves the university (with
 * personal data masked) and how much of today's limit is left.
 */
export function AiNotice({ className, externalOnly = false }: NoticeProps) {
  const status = useQuery(aiStatusQuery);
  if (!status.data) return null;
  return <AiNoticeView status={status.data} className={className} externalOnly={externalOnly} />;
}

export function AiNoticeView({
  status,
  className,
  externalOnly = false,
}: NoticeProps & { status: AiStatus }) {
  const { t } = useTranslation('ai');
  const format = useFormatters();
  const limited = !externalOnly && status.daily_limit > 0;
  if (!status.external && !limited) return null;
  const share = limited ? Math.min(1, status.used_today / status.daily_limit) : 0;
  const resets = format.dateTime(status.resets_at, { timeStyle: 'short' });
  return (
    <Callout
      tone={share >= 0.9 ? 'warning' : 'ai'}
      title={status.external ? t('notice.externalTitle') : t('notice.localTitle')}
      className={cn('max-w-3xl', className)}
    >
      <div className="flex flex-col gap-3">
        {status.external && <p>{t('notice.externalText')}</p>}
        {limited && (
          <div className="flex flex-col gap-1.5" data-testid="ai-limit">
            <Progress
              value={share * 100}
              size="sm"
              tone={share >= 0.9 ? 'gold' : 'primary'}
              label={t('notice.limitLabel')}
            />
            <p className="text-xs">
              {t('notice.limit', {
                used: format.number(status.used_today),
                limit: format.number(status.daily_limit),
                time: resets,
              })}
            </p>
          </div>
        )}
      </div>
    </Callout>
  );
}
