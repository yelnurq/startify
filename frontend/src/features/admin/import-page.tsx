import { Link } from '@tanstack/react-router';
import {
  CircleAlert,
  Download,
  FileSpreadsheet,
  GraduationCap,
  Network,
  RotateCcw,
  Upload,
} from 'lucide-react';
import type { TFunction } from 'i18next';
import { useState, type DragEvent } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Badge,
  Button,
  Callout,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  cn,
  EmptyState,
  Radio,
  RadioGroup,
  Spinner,
  Switch,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/design-system';
import { errorMessage } from '@/shared/api/errors';
import { fieldErrorMessage } from '@/shared/forms/validation';
import { useFormatters } from '@/shared/i18n/format';

import {
  IMPORT_EXTENSIONS,
  IMPORT_MAX_BYTES,
  useCommitImport,
  useCreateImport,
  type ImportJob,
  type ImportKind,
  type ImportResult,
  type ImportRow,
} from './api';

/** Columns the server accepts; `required` ones must be present in every file. */
const IMPORT_COLUMNS: Record<ImportKind, { name: string; required?: boolean }[]> = {
  students: [
    { name: 'email', required: true },
    { name: 'full_name', required: true },
    { name: 'group_code' },
    { name: 'faculty_code' },
    { name: 'course' },
    { name: 'roles' },
  ],
  structure: [
    { name: 'faculty_code', required: true },
    { name: 'faculty_name_kk' },
    { name: 'faculty_name_ru' },
    { name: 'faculty_name_en' },
    { name: 'department_code' },
    { name: 'department_name_kk' },
    { name: 'department_name_ru' },
    { name: 'department_name_en' },
    { name: 'group_code' },
    { name: 'study_level' },
    { name: 'admission_year' },
  ],
};

const PAGE = 100;

/** CSV/XLSX import of the university structure or students: upload, preview, commit. */
export function ImportPage() {
  const { t } = useTranslation('admin');
  const [kind, setKind] = useState<ImportKind>('students');
  const [job, setJob] = useState<ImportJob | null>(null);
  const [result, setResult] = useState<ImportResult | null>(null);

  const restart = () => {
    setJob(null);
    setResult(null);
  };

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-1">
        <h1 className="font-display text-3xl font-bold text-ink">{t('import.title')}</h1>
        <p className="max-w-3xl text-fg-muted">{t('import.subtitle')}</p>
      </header>
      {result && job ? (
        <ImportDone result={result} kind={job.kind} onRestart={restart} />
      ) : job ? (
        <ImportPreview job={job} onRestart={restart} onCommitted={setResult} />
      ) : (
        <ImportUpload kind={kind} onKindChange={setKind} onUploaded={setJob} />
      )}
    </div>
  );
}

function ImportUpload({
  kind,
  onKindChange,
  onUploaded,
}: {
  kind: ImportKind;
  onKindChange: (kind: ImportKind) => void;
  onUploaded: (job: ImportJob) => void;
}) {
  const { t } = useTranslation(['admin', 'common']);
  const upload = useCreateImport();
  const [error, setError] = useState<string | null>(null);
  const [dragging, setDragging] = useState(false);

  const send = (file: File | undefined) => {
    if (!file) return;
    setError(null);
    const name = file.name.toLowerCase();
    if (!IMPORT_EXTENSIONS.some((extension) => name.endsWith(extension))) {
      setError(t('import.upload.wrongType'));
      return;
    }
    if (file.size > IMPORT_MAX_BYTES) {
      setError(t('import.upload.tooLarge'));
      return;
    }
    upload.mutate(
      { kind, file },
      {
        onSuccess: onUploaded,
        onError: (err) => {
          setError(errorMessage(err));
        },
      },
    );
  };

  const onDrop = (event: DragEvent<HTMLLabelElement>) => {
    event.preventDefault();
    setDragging(false);
    send(event.dataTransfer.files[0]);
  };

  return (
    <div className="grid grid-cols-1 items-start gap-6 xl:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
      <Card>
        <CardContent className="flex flex-col gap-6">
          <fieldset className="flex flex-col gap-3">
            <legend className="mb-1 text-sm font-medium text-fg">{t('import.kind.label')}</legend>
            <RadioGroup
              value={kind}
              onValueChange={(value) => {
                onKindChange(value === 'structure' ? 'structure' : 'students');
              }}
              className="grid grid-cols-1 gap-3 sm:grid-cols-2"
            >
              {(['students', 'structure'] as const).map((item) => {
                const Icon = item === 'students' ? GraduationCap : Network;
                return (
                  <label
                    key={item}
                    className="flex cursor-pointer items-start gap-3 rounded-xl border border-border bg-surface p-4 transition-colors hover:border-border-strong has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary-soft"
                  >
                    <Radio value={item} className="mt-0.5" />
                    <span className="flex flex-col gap-1">
                      <span className="flex items-center gap-2 font-semibold text-fg">
                        <Icon className="size-4 text-primary-text" aria-hidden />
                        {t(`import.kind.${item}`)}
                      </span>
                      <span className="text-sm text-fg-muted">{t(`import.kind.${item}Text`)}</span>
                    </span>
                  </label>
                );
              })}
            </RadioGroup>
          </fieldset>

          <label
            onDragOver={(event) => {
              event.preventDefault();
              setDragging(true);
            }}
            onDragLeave={() => {
              setDragging(false);
            }}
            onDrop={onDrop}
            className={cn(
              'flex cursor-pointer flex-col items-center justify-center gap-3 rounded-2xl border-2 border-dashed px-6 py-12 text-center transition-colors',
              'has-[:focus-visible]:ring-4 has-[:focus-visible]:ring-primary-500/20',
              dragging
                ? 'border-primary bg-primary-soft'
                : 'border-border-strong bg-surface-raised hover:border-primary-border hover:bg-primary-soft/40',
            )}
            data-testid="import-dropzone"
          >
            <input
              type="file"
              accept=".csv,.xlsx,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
              className="sr-only"
              disabled={upload.isPending}
              onChange={(event) => {
                send(event.target.files?.[0]);
                event.target.value = '';
              }}
            />
            {upload.isPending ? (
              <Spinner className="size-8 text-brand" />
            ) : (
              <span className="grid size-14 place-items-center rounded-2xl bg-surface text-primary-text shadow-sm">
                <Upload className="size-6" aria-hidden />
              </span>
            )}
            <span className="font-semibold text-fg">
              {upload.isPending ? t('import.upload.checking') : t('import.upload.drop')}
            </span>
            <span className="text-sm text-fg-subtle">{t('import.upload.rules')}</span>
          </label>
          {error && <Callout tone="danger">{error}</Callout>}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('import.columns.title')}</CardTitle>
          <CardDescription>{t('import.columns.text')}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <ul className="flex flex-wrap gap-1.5">
            {IMPORT_COLUMNS[kind].map((column) => (
              <li key={column.name}>
                <Badge tone={column.required ? 'primary' : 'neutral'} className="font-mono">
                  {column.name}
                  {column.required && <span aria-hidden>*</span>}
                </Badge>
              </li>
            ))}
          </ul>
          <p className="text-sm text-fg-muted">{t(`import.columns.${kind}Hint`)}</p>
          <p className="text-xs text-fg-subtle">{t('import.columns.required')}</p>
          <Button asChild variant="secondary" className="self-start">
            <a href={`/templates/${kind}.csv`} download>
              <Download aria-hidden />
              {t('import.columns.template')}
            </a>
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}

function ImportPreview({
  job,
  onRestart,
  onCommitted,
}: {
  job: ImportJob;
  onRestart: () => void;
  onCommitted: (result: ImportResult) => void;
}) {
  const { t } = useTranslation(['admin', 'common']);
  const commit = useCommitImport();
  const format = useFormatters();
  const [onlyErrors, setOnlyErrors] = useState(job.summary.invalid > 0);
  const [limit, setLimit] = useState(PAGE);
  const [error, setError] = useState<string | null>(null);
  const rows = onlyErrors ? job.rows.filter((row) => row.errors.length > 0) : job.rows;
  const shown = rows.slice(0, limit);
  const { summary } = job;

  const stats = [
    { key: 'total', value: summary.total, tone: 'text-fg' },
    { key: 'valid', value: summary.valid, tone: 'text-success' },
    {
      key: 'invalid',
      value: summary.invalid,
      tone: summary.invalid > 0 ? 'text-danger' : 'text-fg',
    },
    { key: 'create', value: summary.to_create, tone: 'text-fg' },
    { key: 'update', value: summary.to_update, tone: 'text-fg' },
  ] as const;

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardContent className="flex flex-col gap-5">
          <div className="flex flex-wrap items-center gap-3">
            <FileSpreadsheet className="size-5 text-primary-text" aria-hidden />
            <p className="min-w-0 font-semibold break-all text-fg">{job.file_name}</p>
            <Badge tone="primary">{t(`import.kind.${job.kind}`)}</Badge>
          </div>
          <dl
            className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5"
            data-testid="import-summary"
          >
            {stats.map((stat) => (
              <div key={stat.key} className="flex flex-col gap-1 rounded-xl bg-surface-raised p-4">
                <dt className="text-xs text-fg-subtle">{t(`import.preview.${stat.key}`)}</dt>
                <dd className={cn('font-figures text-2xl font-bold tabular-nums', stat.tone)}>
                  {format.number(stat.value)}
                </dd>
              </div>
            ))}
          </dl>
          {summary.invalid > 0 && (
            <Callout tone="warning">
              {t('import.preview.invalidHint', { count: summary.invalid })}
            </Callout>
          )}
        </CardContent>
      </Card>

      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-lg font-semibold text-ink">{t('import.preview.title')}</h2>
        {summary.invalid > 0 && (
          <label className="flex items-center gap-2 text-sm text-fg">
            <Switch checked={onlyErrors} onCheckedChange={setOnlyErrors} />
            {t('import.preview.onlyErrors')}
          </label>
        )}
      </div>

      <ul className="flex flex-col gap-3 md:hidden" aria-label={t('import.preview.table')}>
        {shown.map((row) => (
          <PreviewCard key={row.line} row={row} columns={job.columns} />
        ))}
      </ul>
      <div className="hidden md:block">
        <Table label={t('import.preview.table')}>
          <TableHeader>
            <TableRow>
              <TableHead numeric>{t('import.preview.line')}</TableHead>
              <TableHead>{t('import.preview.action')}</TableHead>
              {job.columns.map((column) => (
                <TableHead key={column}>{fieldLabel(t, column)}</TableHead>
              ))}
              <TableHead>{t('import.preview.errors')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {shown.map((row) => (
              <PreviewRow key={row.line} row={row} columns={job.columns} />
            ))}
          </TableBody>
        </Table>
      </div>
      {rows.length > shown.length && (
        <Button
          variant="secondary"
          className="self-center"
          onClick={() => {
            setLimit((value) => value + PAGE);
          }}
        >
          {t('import.preview.more', { shown: shown.length, total: rows.length })}
        </Button>
      )}

      {error && <Callout tone="danger">{error}</Callout>}
      <div className="flex flex-col gap-3 rounded-xl border border-border bg-surface/90 p-4 shadow-md backdrop-blur sm:sticky sm:bottom-4 sm:z-10 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-sm text-fg-muted">
          {job.kind === 'students'
            ? t('import.preview.studentsNote')
            : t('import.preview.structureNote')}
        </p>
        <div className="flex shrink-0 flex-col-reverse gap-2 sm:flex-row">
          <Button
            variant="ghost"
            leftIcon={<RotateCcw />}
            onClick={onRestart}
            disabled={commit.isPending}
          >
            {t('import.preview.another')}
          </Button>
          <Button
            loading={commit.isPending}
            disabled={summary.valid === 0}
            onClick={() => {
              setError(null);
              commit.mutate(job.id, {
                onSuccess: onCommitted,
                onError: (err) => {
                  setError(errorMessage(err));
                },
              });
            }}
          >
            {t('import.preview.commit', { count: summary.valid })}
          </Button>
        </div>
      </div>
    </div>
  );
}

const ACTION_TONE = { create: 'success', update: 'info', skip: 'neutral' } as const;

/** A row of the preview on narrow screens, where a wide table hides the errors. */
function PreviewCard({ row, columns }: { row: ImportRow; columns: string[] }) {
  const { t } = useTranslation(['admin', 'common']);
  const invalid = row.errors.length > 0;
  return (
    <li
      className={cn(
        'flex flex-col gap-3 rounded-xl border border-border bg-surface p-4',
        invalid && 'border-danger/40 bg-danger-soft/40',
      )}
    >
      <div className="flex items-center justify-between gap-3">
        <span className="text-sm text-fg-subtle">
          {t('import.preview.line')} {row.line}
        </span>
        {invalid ? (
          <Badge tone="danger" size="sm">
            {t('import.preview.invalidRow')}
          </Badge>
        ) : (
          <Badge tone={ACTION_TONE[row.action]} size="sm">
            {t(`import.preview.actions.${row.action}`)}
          </Badge>
        )}
      </div>
      <dl className="grid grid-cols-1 gap-2 text-sm">
        {columns
          .filter((column) => row.values[column])
          .map((column) => (
            <div key={column} className="flex min-w-0 flex-col">
              <dt className="text-xs text-fg-subtle">{fieldLabel(t, column)}</dt>
              <dd className="break-all text-fg">{row.values[column]}</dd>
            </div>
          ))}
      </dl>
      {invalid && (
        <ul className="flex flex-col gap-1 text-sm text-danger">
          {row.errors.map((error) => (
            <li key={`${error.field}-${error.code}`} className="flex items-start gap-1.5">
              <CircleAlert className="mt-0.5 size-3.5 shrink-0" aria-hidden />
              <span>
                <span className="font-medium">{fieldLabel(t, error.field)}</span>
                {': '}
                {fieldErrorMessage(error)}
              </span>
            </li>
          ))}
        </ul>
      )}
    </li>
  );
}

const FIELD_KEYS = [
  'email',
  'full_name',
  'group_code',
  'faculty_code',
  'course',
  'roles',
  'faculty_name_kk',
  'faculty_name_ru',
  'faculty_name_en',
  'department_code',
  'department_name_kk',
  'department_name_ru',
  'department_name_en',
  'study_level',
  'admission_year',
] as const;

type FieldKey = (typeof FIELD_KEYS)[number];

/** The column name in the interface language; unknown columns keep their own name. */
function fieldLabel(t: TFunction<['admin', 'common']>, key: string): string {
  return (FIELD_KEYS as readonly string[]).includes(key)
    ? t(`import.fields.${key as FieldKey}`)
    : key;
}

function PreviewRow({ row, columns }: { row: ImportRow; columns: string[] }) {
  const { t } = useTranslation(['admin', 'common']);
  const invalid = row.errors.length > 0;
  const fields = new Set(row.errors.map((error) => error.field));
  return (
    <TableRow className={cn(invalid && 'bg-danger-soft/40 hover:bg-danger-soft/60')}>
      <TableCell numeric className="text-fg-subtle">
        {row.line}
      </TableCell>
      <TableCell>
        {invalid ? (
          <Badge tone="danger" size="sm">
            {t('import.preview.invalidRow')}
          </Badge>
        ) : (
          <Badge tone={ACTION_TONE[row.action]} size="sm">
            {t(`import.preview.actions.${row.action}`)}
          </Badge>
        )}
      </TableCell>
      {columns.map((column) => (
        <TableCell
          key={column}
          className={cn('whitespace-nowrap', fields.has(column) && 'font-medium text-danger')}
        >
          {row.values[column] ?? ''}
        </TableCell>
      ))}
      <TableCell className="min-w-64">
        {invalid && (
          <ul className="flex flex-col gap-1 text-sm text-danger">
            {row.errors.map((error) => (
              <li key={`${error.field}-${error.code}`} className="flex items-start gap-1.5">
                <CircleAlert className="mt-0.5 size-3.5 shrink-0" aria-hidden />
                <span>
                  <span className="font-medium">{fieldLabel(t, error.field)}</span>
                  {': '}
                  {fieldErrorMessage(error)}
                </span>
              </li>
            ))}
          </ul>
        )}
      </TableCell>
    </TableRow>
  );
}

function ImportDone({
  result,
  kind,
  onRestart,
}: {
  result: ImportResult;
  kind: ImportKind;
  onRestart: () => void;
}) {
  const { t } = useTranslation('admin');
  return (
    <Card>
      <CardContent className="py-10">
        <EmptyState
          illustration="certificate"
          title={t('import.done.title')}
          description={
            <span className="flex flex-col gap-1" data-testid="import-result">
              <span>{t('import.done.created', { count: result.created })}</span>
              <span>{t('import.done.updated', { count: result.updated })}</span>
              <span>{t('import.done.skipped', { count: result.skipped })}</span>
              {kind === 'students' && (
                <span>{t('import.done.invited', { count: result.invited })}</span>
              )}
            </span>
          }
          action={
            <>
              <Button variant="secondary" leftIcon={<RotateCcw />} onClick={onRestart}>
                {t('import.done.again')}
              </Button>
              {kind === 'students' && (
                <Button asChild>
                  <Link to="/app/admin/members">{t('import.done.toMembers')}</Link>
                </Button>
              )}
            </>
          }
        />
      </CardContent>
    </Card>
  );
}
