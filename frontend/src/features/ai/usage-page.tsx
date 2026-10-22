import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { getRouteApi, useNavigate } from '@tanstack/react-router';
import { ArrowDownToLine, ArrowUpFromLine, CircleDollarSign, MessagesSquare } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import {
  Badge,
  Button,
  Callout,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  EmptyState,
  Progress,
  SegmentedControl,
  Skeleton,
  StatTile,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/design-system';
import { errorMessage } from '@/shared/api/errors';
import { useFormatters } from '@/shared/i18n/format';

import {
  aiUsageQuery,
  isUsagePeriod,
  USAGE_PERIODS,
  type AiUsageReport,
  type UsagePeriod,
} from './api';
import { providerLabel } from './labels';

export interface UsageSearch {
  days?: UsagePeriod;
}

/** Requests, tokens and cost of AI over a period, by feature, model and day. */
export function AiUsagePage({
  days,
  onDaysChange,
}: {
  days: UsagePeriod;
  onDaysChange: (days: UsagePeriod) => void;
}) {
  const { t } = useTranslation(['ai', 'common']);
  const usage = useQuery({ ...aiUsageQuery(days), placeholderData: keepPreviousData });
  const report = usage.data;

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div className="flex flex-col gap-1">
          <h1 className="font-display text-3xl font-bold text-ink">{t('usage.title')}</h1>
          <p className="max-w-3xl text-fg-muted">{t('usage.subtitle')}</p>
        </div>
        <SegmentedControl
          label={t('usage.period')}
          value={String(days)}
          onValueChange={(value) => {
            const next = Number(value);
            if (isUsagePeriod(next)) onDaysChange(next);
          }}
          options={USAGE_PERIODS.map((period) => ({
            value: String(period),
            label: t('usage.days', { count: period }),
          }))}
          className="self-start sm:self-auto"
        />
      </header>

      {usage.isError && (
        <Callout
          tone="danger"
          action={
            <Button size="sm" variant="secondary" onClick={() => void usage.refetch()}>
              {t('common:actions.retry')}
            </Button>
          }
        >
          {errorMessage(usage.error)}
        </Callout>
      )}
      {usage.isPending && (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4" aria-busy>
          {Array.from({ length: 4 }, (_, index) => (
            <Skeleton key={index} className="h-36 w-full" />
          ))}
        </div>
      )}
      {report?.totals.requests === 0 && (
        <EmptyState
          illustration="chart"
          title={t('usage.empty')}
          description={t('usage.emptyText')}
        />
      )}
      {report && report.totals.requests > 0 && <UsageReport report={report} days={days} />}
    </div>
  );
}

/** Daily values for every day of the period, with zeros where nothing happened. */
function dailySeries(
  report: AiUsageReport,
  days: number,
  pick: (day: AiUsageReport['by_day'][number]) => number,
) {
  const values = new Map(report.by_day.map((day) => [day.day, pick(day)]));
  const start = new Date(report.since);
  return Array.from({ length: days }, (_, index) => {
    const day = new Date(start.getTime() + index * 86_400_000);
    const key = new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Almaty' }).format(day);
    return values.get(key) ?? 0;
  });
}

function UsageReport({ report, days }: { report: AiUsageReport; days: number }) {
  const { t } = useTranslation('ai');
  const format = useFormatters();
  const usd = (value: number) =>
    format.number(value, {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2,
      maximumFractionDigits: value > 0 && value < 1 ? 4 : 2,
    });
  const tokens = (value: number) =>
    format.number(value, { notation: 'compact', maximumFractionDigits: 1 });
  const maxTokens = Math.max(1, ...report.by_day.map((day) => day.tokens));

  return (
    <>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatTile
          label={t('usage.requests')}
          value={report.totals.requests}
          format={(value) => format.number(Math.round(value))}
          icon={<MessagesSquare />}
          trend={dailySeries(report, days, (day) => day.requests)}
        />
        <StatTile
          label={t('usage.inputTokens')}
          value={report.totals.input_tokens}
          format={(value) => tokens(Math.round(value))}
          icon={<ArrowDownToLine />}
          tone="info"
        />
        <StatTile
          label={t('usage.outputTokens')}
          value={report.totals.output_tokens}
          format={(value) => tokens(Math.round(value))}
          icon={<ArrowUpFromLine />}
          tone="success"
        />
        <StatTile
          label={t('usage.cost')}
          value={report.totals.cost_usd}
          format={usd}
          icon={<CircleDollarSign />}
          tone="gold"
          trend={dailySeries(report, days, (day) => day.cost_usd)}
          footnote={t('usage.costHint')}
        />
      </div>

      <div className="grid grid-cols-1 gap-6 2xl:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>{t('usage.byFeature')}</CardTitle>
          </CardHeader>
          <CardContent>
            <Table label={t('usage.byFeature')}>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('usage.columns.feature')}</TableHead>
                  <TableHead className="text-right">{t('usage.columns.requests')}</TableHead>
                  <TableHead className="hidden text-right sm:table-cell">
                    {t('usage.columns.tokens')}
                  </TableHead>
                  <TableHead className="hidden text-right sm:table-cell">
                    {t('usage.columns.failures')}
                  </TableHead>
                  <TableHead className="text-right">{t('usage.columns.cost')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {report.by_feature.map((row) => (
                  <TableRow key={row.feature}>
                    <TableCell className="font-medium whitespace-nowrap text-fg">
                      {t(`features.${row.feature}`)}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      {format.number(row.requests)}
                    </TableCell>
                    <TableCell className="hidden text-right whitespace-nowrap tabular-nums sm:table-cell">
                      {tokens(row.input_tokens + row.output_tokens)}
                    </TableCell>
                    <TableCell className="hidden text-right sm:table-cell">
                      {row.failures > 0 ? (
                        <Badge size="sm" tone="danger">
                          {format.number(row.failures)}
                        </Badge>
                      ) : (
                        <span className="text-fg-subtle tabular-nums">0</span>
                      )}
                    </TableCell>
                    <TableCell className="text-right whitespace-nowrap tabular-nums">
                      {usd(row.cost_usd)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t('usage.byModel')}</CardTitle>
          </CardHeader>
          <CardContent>
            <Table label={t('usage.byModel')}>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('usage.columns.model')}</TableHead>
                  <TableHead className="text-right">{t('usage.columns.requests')}</TableHead>
                  <TableHead className="hidden text-right sm:table-cell">
                    {t('usage.columns.tokens')}
                  </TableHead>
                  <TableHead className="text-right">{t('usage.columns.cost')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {report.by_model.map((row) => (
                  <TableRow key={`${row.provider}/${row.model}`}>
                    <TableCell>
                      <div className="flex min-w-0 flex-col sm:min-w-40">
                        <span className="font-mono text-sm break-all text-fg">{row.model}</span>
                        <span className="text-xs text-fg-subtle">
                          {providerLabel(t, row.provider)}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      {format.number(row.requests)}
                    </TableCell>
                    <TableCell className="hidden text-right whitespace-nowrap tabular-nums sm:table-cell">
                      {tokens(row.tokens)}
                    </TableCell>
                    <TableCell className="text-right whitespace-nowrap tabular-nums">
                      {usd(row.cost_usd)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t('usage.byDay')}</CardTitle>
        </CardHeader>
        <CardContent>
          <Table label={t('usage.byDay')}>
            <TableHeader>
              <TableRow>
                <TableHead>{t('usage.columns.day')}</TableHead>
                <TableHead className="text-right">{t('usage.columns.requests')}</TableHead>
                <TableHead className="text-right sm:w-full sm:min-w-32 sm:text-left">
                  {t('usage.columns.tokens')}
                </TableHead>
                <TableHead className="text-right">{t('usage.columns.cost')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {[...report.by_day].reverse().map((row) => (
                <TableRow key={row.day}>
                  <TableCell className="whitespace-nowrap text-fg">
                    {format.dateTime(`${row.day}T00:00:00`, { dateStyle: 'medium' })}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    {format.number(row.requests)}
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-3">
                      <div className="hidden flex-1 sm:block" aria-hidden>
                        <Progress value={Math.max(2, (row.tokens / maxTokens) * 100)} size="sm" />
                      </div>
                      <span className="ml-auto w-14 shrink-0 text-right text-sm whitespace-nowrap tabular-nums">
                        {tokens(row.tokens)}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell className="text-right whitespace-nowrap tabular-nums">
                    {usd(row.cost_usd)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </>
  );
}

const usageRoute = getRouteApi('/app/admin/ai/usage');

export function AiUsageRoute() {
  const { days } = usageRoute.useSearch();
  const navigate = useNavigate({ from: '/app/admin/ai/usage' });
  return (
    <AiUsagePage
      days={days ?? 30}
      onDaysChange={(next) => {
        void navigate({ search: next === 30 ? {} : { days: next }, replace: true });
      }}
    />
  );
}
