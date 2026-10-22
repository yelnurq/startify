import { useQuery } from '@tanstack/react-query';
import { Download, FileText, Presentation, ScrollText } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import {
  Badge,
  Button,
  Callout,
  Card,
  EmptyState,
  Skeleton,
  Spinner,
  toast,
} from '@/design-system';
import { errorMessage } from '@/shared/api/errors';
import { isLocale } from '@/shared/i18n';
import { useFormatters } from '@/shared/i18n/format';

import {
  materialsQuery,
  useCreateMaterial,
  type Material,
  type MaterialFormat,
  type MaterialKind,
} from './api';

const OPTIONS: { kind: MaterialKind; formats: MaterialFormat[]; Icon: typeof FileText }[] = [
  { kind: 'one_pager', formats: ['pdf'], Icon: FileText },
  { kind: 'pitch_deck', formats: ['pdf', 'pptx'], Icon: Presentation },
  { kind: 'passport', formats: ['pdf'], Icon: ScrollText },
];

const STATUS_TONE = { pending: 'info', ready: 'success', failed: 'danger' } as const;

/** Generates pitch materials from the project's answers and lists the results. */
export function MaterialsPanel({
  projectId,
  canCreate,
}: {
  projectId: string;
  canCreate: boolean;
}) {
  const { t, i18n } = useTranslation('incubator');
  const format = useFormatters();
  const query = useQuery(materialsQuery(projectId));
  const create = useCreateMaterial(projectId);
  const locale = isLocale(i18n.language) ? i18n.language : 'ru';

  const generate = (kind: MaterialKind, formatName: MaterialFormat) => {
    create.mutate(
      { kind, format: formatName, locale },
      {
        onSuccess: () => {
          toast.success(t('materials.scheduled'));
        },
        onError: (err) => {
          toast.error(errorMessage(err));
        },
      },
    );
  };

  return (
    <div className="@container flex flex-col gap-6">
      <div className="grid grid-cols-1 gap-3 @2xl:grid-cols-3">
        {OPTIONS.map(({ kind, formats, Icon }) => (
          <Card key={kind} className="flex flex-col gap-3 p-4">
            <div className="flex items-start gap-3">
              <span className="grid size-10 shrink-0 place-items-center rounded-lg bg-primary-soft text-primary-soft-fg">
                <Icon className="size-5" aria-hidden />
              </span>
              <div className="flex min-w-0 flex-col gap-0.5">
                <h3 className="text-sm font-semibold text-ink">{t(`materials.kind.${kind}`)}</h3>
                <p className="text-xs text-fg-muted">{t(`materials.about.${kind}`)}</p>
              </div>
            </div>
            {canCreate && (
              <div className="mt-auto flex flex-wrap gap-2">
                {formats.map((f) => (
                  <Button
                    key={f}
                    size="sm"
                    variant={f === 'pdf' ? 'soft' : 'secondary'}
                    disabled={create.isPending}
                    onClick={() => {
                      generate(kind, f);
                    }}
                  >
                    {t('materials.generate', { format: f.toUpperCase() })}
                  </Button>
                ))}
              </div>
            )}
          </Card>
        ))}
      </div>

      <section aria-labelledby="materials-list" className="flex flex-col gap-3">
        <h3 id="materials-list" className="text-sm font-semibold text-ink">
          {t('materials.list')}
        </h3>
        {query.isPending && <Skeleton className="h-24 rounded-xl" />}
        {query.isError && <Callout tone="danger">{errorMessage(query.error)}</Callout>}
        {query.isSuccess && query.data.length === 0 && (
          <EmptyState
            size="sm"
            illustration="certificate"
            title={t('materials.empty')}
            description={t('materials.emptyText')}
          />
        )}
        {query.isSuccess && query.data.length > 0 && (
          <ul className="flex flex-col divide-y divide-border rounded-xl border border-border">
            {query.data.map((material) => (
              <MaterialRow key={material.id} material={material} format={format} />
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

function MaterialRow({
  material,
  format,
}: {
  material: Material;
  format: ReturnType<typeof useFormatters>;
}) {
  const { t } = useTranslation(['incubator', 'common']);
  return (
    <li className="flex flex-wrap items-center gap-3 px-4 py-3">
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="text-sm font-medium text-fg">
          {t(`materials.kind.${material.kind}`)} · {material.format.toUpperCase()}
        </span>
        <span className="text-xs text-fg-subtle">
          {t(`common:language.${material.locale}`)} · {format.dateTime(material.created_at)}
        </span>
        {material.status === 'failed' && material.error && (
          <span className="text-xs text-danger">
            {t(`common:errors.${material.error}` as 'common:errors.internal', {
              defaultValue: t('common:errors.internal'),
            })}
          </span>
        )}
      </div>
      {material.status === 'pending' ? (
        <Badge tone="info" icon={<Spinner className="size-3" />}>
          {t('materials.status.pending')}
        </Badge>
      ) : (
        <Badge tone={STATUS_TONE[material.status]}>
          {t(`materials.status.${material.status}`)}
        </Badge>
      )}
      {material.status === 'ready' && material.download_url && (
        <Button asChild size="sm" variant="secondary">
          <a href={material.download_url} download>
            <Download aria-hidden />
            {t('materials.download')}
          </a>
        </Button>
      )}
    </li>
  );
}
