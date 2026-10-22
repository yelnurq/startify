import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { BookOpenText, Download, FileText, Plus, Search, Trash2, Upload } from 'lucide-react';
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
  ConfirmDialog,
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  Field,
  IconButton,
  Input,
  Skeleton,
  Spinner,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  Textarea,
  toast,
} from '@/design-system';
import { FormAlert } from '@/features/auth/components';
import { errorMessage } from '@/shared/api/errors';
import { useDebouncedValue } from '@/shared/hooks/use-debounced-value';
import { i18n } from '@/shared/i18n';
import { useFormatters } from '@/shared/i18n/format';

import {
  KNOWLEDGE_EXTENSIONS,
  KNOWLEDGE_LIMITS,
  KNOWLEDGE_MAX_BYTES,
  knowledgeDocumentsQuery,
  knowledgeSearchQuery,
  useCreateKnowledgeDocument,
  useDeleteKnowledgeDocument,
  type KnowledgeDocument,
} from './api';

const STATUS_TONE = { pending: 'info', ready: 'success', failed: 'danger' } as const;

/** Message for the stored indexing error, which is a problem code when known. */
function indexingError(code: string | null): string {
  const generic = i18n.t('ai:knowledge.indexFailed');
  if (!code || !/^[a-z_]+\.[a-z_]+$/.test(code)) return generic;
  // Codes come from the server at runtime.
  return (
    i18n.t(`common:errors.${code}` as 'common:errors.internal', { defaultValue: '' }) || generic
  );
}

/** Regulations and guides the AI mentor answers from, citing the source. */
export function KnowledgePage() {
  const { t } = useTranslation(['ai', 'common']);
  const documents = useInfiniteQuery(knowledgeDocumentsQuery);
  const items = documents.data?.pages.flatMap((page) => page.items) ?? [];
  const [adding, setAdding] = useState(false);
  const [removing, setRemoving] = useState<KnowledgeDocument | null>(null);

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div className="flex flex-col gap-1">
          <h1 className="font-display text-3xl font-semibold text-fg">{t('knowledge.title')}</h1>
          <p className="max-w-3xl text-fg-muted">{t('knowledge.subtitle')}</p>
        </div>
        <Button
          leftIcon={<Plus />}
          className="self-start sm:self-auto"
          onClick={() => {
            setAdding(true);
          }}
        >
          {t('knowledge.add')}
        </Button>
      </header>

      {documents.isError && (
        <Callout
          tone="danger"
          action={
            <Button size="sm" variant="secondary" onClick={() => void documents.refetch()}>
              {t('common:actions.retry')}
            </Button>
          }
        >
          {errorMessage(documents.error)}
        </Callout>
      )}
      {documents.isPending && (
        <div className="flex flex-col gap-2" aria-busy>
          {Array.from({ length: 4 }, (_, index) => (
            <Skeleton key={index} className="h-14 w-full" />
          ))}
        </div>
      )}
      {documents.isSuccess && items.length === 0 && (
        <EmptyState
          illustration="mentor"
          title={t('knowledge.empty')}
          description={t('knowledge.emptyText')}
          action={
            <Button
              leftIcon={<Plus />}
              onClick={() => {
                setAdding(true);
              }}
            >
              {t('knowledge.add')}
            </Button>
          }
        />
      )}
      {items.length > 0 && (
        <Table label={t('knowledge.table')}>
          <TableHeader>
            <TableRow>
              <TableHead>{t('knowledge.columns.title')}</TableHead>
              <TableHead className="hidden sm:table-cell">
                {t('knowledge.columns.status')}
              </TableHead>
              <TableHead className="hidden text-right md:table-cell">
                {t('knowledge.columns.chunks')}
              </TableHead>
              <TableHead className="hidden text-right md:table-cell">
                {t('knowledge.columns.characters')}
              </TableHead>
              <TableHead className="hidden md:table-cell">{t('knowledge.columns.added')}</TableHead>
              <TableHead>
                <span className="sr-only">{t('knowledge.columns.actions')}</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.map((doc) => (
              <DocumentRow key={doc.id} doc={doc} onRemove={setRemoving} />
            ))}
          </TableBody>
        </Table>
      )}
      {documents.hasNextPage && (
        <Button
          variant="secondary"
          className="self-center"
          loading={documents.isFetchingNextPage}
          onClick={() => void documents.fetchNextPage()}
        >
          {t('common:actions.loadMore')}
        </Button>
      )}

      {items.length > 0 && <SearchCard />}

      <AddDocumentDialog open={adding} onOpenChange={setAdding} />
      <RemoveDialog
        doc={removing}
        onOpenChange={(open) => {
          if (!open) setRemoving(null);
        }}
      />
    </div>
  );
}

function DocumentRow({
  doc,
  onRemove,
}: {
  doc: KnowledgeDocument;
  onRemove: (doc: KnowledgeDocument) => void;
}) {
  const { t } = useTranslation('ai');
  const format = useFormatters();
  return (
    <TableRow data-testid="knowledge-row">
      <TableCell>
        <div className="flex min-w-44 items-start gap-3">
          <span className="mt-0.5 grid size-8 shrink-0 place-items-center rounded-lg bg-primary-soft text-primary-soft-fg [&_svg]:size-4">
            {doc.file_url ? <FileText aria-hidden /> : <BookOpenText aria-hidden />}
          </span>
          <div className="flex min-w-0 flex-col gap-1.5">
            <span className="font-medium break-words text-fg">{doc.title}</span>
            {/* Phones have no room for the status column. */}
            <DocumentStatus doc={doc} className="sm:hidden" />
          </div>
        </div>
      </TableCell>
      <TableCell className="hidden sm:table-cell">
        <DocumentStatus doc={doc} className="min-w-36" />
      </TableCell>
      <TableCell className="hidden text-right tabular-nums md:table-cell">
        {doc.status === 'ready' ? format.number(doc.chunk_count) : '—'}
      </TableCell>
      <TableCell className="hidden text-right tabular-nums md:table-cell">
        {format.number(doc.characters)}
      </TableCell>
      <TableCell className="hidden text-sm whitespace-nowrap text-fg-muted md:table-cell">
        {format.relative(doc.created_at)}
      </TableCell>
      <TableCell className="text-right">
        <div className="flex justify-end gap-1">
          {doc.file_url && (
            <IconButton
              asChild
              size="sm"
              label={t('knowledge.download', { title: doc.title })}
              icon={
                <a href={doc.file_url} download>
                  <Download aria-hidden />
                </a>
              }
            />
          )}
          <IconButton
            size="sm"
            label={t('knowledge.remove', { title: doc.title })}
            icon={<Trash2 />}
            onClick={() => {
              onRemove(doc);
            }}
          />
        </div>
      </TableCell>
    </TableRow>
  );
}

function DocumentStatus({ doc, className }: { doc: KnowledgeDocument; className?: string }) {
  const { t } = useTranslation('ai');
  return (
    <div className={cn('flex flex-col items-start gap-1', className)}>
      <Badge tone={STATUS_TONE[doc.status]} dot={doc.status !== 'pending'}>
        {doc.status === 'pending' && <Spinner className="size-3" />}
        {t(`knowledge.status.${doc.status}`)}
      </Badge>
      {doc.status === 'failed' && (
        <span className="max-w-64 text-xs text-danger">{indexingError(doc.error)}</span>
      )}
    </div>
  );
}

function SearchCard() {
  const { t } = useTranslation(['ai', 'common']);
  const format = useFormatters();
  const [query, setQuery] = useState('');
  const q = useDebouncedValue(query.trim(), 400);
  const enough = q.length >= KNOWLEDGE_LIMITS.query[0];
  const hits = useQuery(knowledgeSearchQuery(enough ? q : ''));
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('knowledge.search.title')}</CardTitle>
        <CardDescription>{t('knowledge.search.subtitle')}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <Input
          type="search"
          value={query}
          leading={<Search />}
          maxLength={KNOWLEDGE_LIMITS.query[1]}
          aria-label={t('knowledge.search.label')}
          placeholder={t('knowledge.search.placeholder')}
          onChange={(event) => {
            setQuery(event.target.value);
          }}
        />
        {hits.isFetching && !hits.data && (
          <div className="flex flex-col gap-2" aria-busy>
            <Skeleton className="h-20 w-full" />
            <Skeleton className="h-20 w-full" />
          </div>
        )}
        {hits.isError && <Callout tone="danger">{errorMessage(hits.error)}</Callout>}
        {enough && hits.data?.length === 0 && (
          <p className="text-sm text-fg-muted">{t('knowledge.search.empty')}</p>
        )}
        {enough && hits.data && hits.data.length > 0 && (
          <ol className="flex flex-col gap-3" data-testid="knowledge-hits">
            {hits.data.map((hit, index) => (
              <li
                // Several passages of one document may match.
                key={`${hit.document_id}-${String(index)}`}
                className="flex flex-col gap-1.5 rounded-xl border border-border p-4"
              >
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <p className="min-w-0 text-sm font-semibold break-words text-fg">
                    {hit.title}
                    {hit.heading && (
                      <span className="font-normal text-fg-muted">
                        {' · '}
                        {hit.heading}
                      </span>
                    )}
                  </p>
                  <Badge size="sm" tone={hit.score >= 0.5 ? 'success' : 'neutral'}>
                    {t('knowledge.search.score', {
                      value: format.number(hit.score, {
                        style: 'percent',
                        maximumFractionDigits: 0,
                      }),
                    })}
                  </Badge>
                </div>
                <p className="text-sm leading-relaxed text-fg-muted">{hit.excerpt}</p>
              </li>
            ))}
          </ol>
        )}
      </CardContent>
    </Card>
  );
}

type SourceKind = 'text' | 'file';

function AddDocumentDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation('ai');
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="lg">
        <DialogHeader>
          <DialogTitle>{t('knowledge.addTitle')}</DialogTitle>
          <DialogDescription>{t('knowledge.addText')}</DialogDescription>
        </DialogHeader>
        {open && (
          <AddDocumentForm
            onDone={() => {
              onOpenChange(false);
            }}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function AddDocumentForm({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation(['ai', 'common']);
  const create = useCreateKnowledgeDocument();
  const [title, setTitle] = useState('');
  const [kind, setKind] = useState<SourceKind>('text');
  const [text, setText] = useState('');
  const [file, setFile] = useState<File | null>(null);
  const [dragging, setDragging] = useState(false);
  const [errors, setErrors] = useState<{ title?: string; source?: string }>({});
  const [formError, setFormError] = useState<string | null>(null);

  const pick = (picked: File | undefined) => {
    if (!picked) return;
    const name = picked.name.toLowerCase();
    if (!KNOWLEDGE_EXTENSIONS.some((extension) => name.endsWith(extension))) {
      setErrors((current) => ({ ...current, source: t('knowledge.form.wrongType') }));
      return;
    }
    if (picked.size > KNOWLEDGE_MAX_BYTES) {
      setErrors((current) => ({ ...current, source: t('knowledge.form.tooLarge') }));
      return;
    }
    setErrors((current) => ({ ...current, source: undefined }));
    setFile(picked);
    if (title.trim() === '')
      setTitle(picked.name.replace(/\.[^.]+$/, '').slice(0, KNOWLEDGE_LIMITS.title));
  };

  const onDrop = (event: DragEvent<HTMLLabelElement>) => {
    event.preventDefault();
    setDragging(false);
    pick(event.dataTransfer.files[0]);
  };

  const submit = () => {
    setFormError(null);
    const next: typeof errors = {};
    const cleanTitle = title.trim();
    if (cleanTitle === '') next.title = t('common:validation.required');
    else if (cleanTitle.length > KNOWLEDGE_LIMITS.title) {
      next.title = t('common:validation.maxLength', { count: KNOWLEDGE_LIMITS.title });
    }
    if (kind === 'text') {
      if (text.trim() === '') next.source = t('knowledge.form.textRequired');
      else if (new TextEncoder().encode(text).length > KNOWLEDGE_LIMITS.textBytes) {
        next.source = t('knowledge.form.textTooLong');
      }
    } else if (!file) {
      next.source = t('knowledge.form.fileRequired');
    }
    setErrors(next);
    if (next.title ?? next.source) return;
    create.mutate(
      {
        title: cleanTitle,
        source: kind === 'file' && file ? { kind: 'file', file } : { kind: 'text', text },
      },
      {
        onSuccess: () => {
          toast.success(t('knowledge.added'));
          onDone();
        },
        onError: (err) => {
          setFormError(errorMessage(err));
        },
      },
    );
  };

  return (
    <form
      className="flex min-h-0 flex-1 flex-col"
      noValidate
      onSubmit={(event) => {
        event.preventDefault();
        submit();
      }}
    >
      <DialogBody className="flex flex-col gap-5">
        <FormAlert message={formError} />
        <Field
          label={t('knowledge.form.title')}
          required
          error={errors.title}
          counter={{ value: title.length, max: KNOWLEDGE_LIMITS.title }}
        >
          <Input
            value={title}
            placeholder={t('knowledge.form.titlePlaceholder')}
            onChange={(event) => {
              setTitle(event.target.value);
            }}
          />
        </Field>
        <Tabs
          value={kind}
          onValueChange={(value) => {
            setKind(value === 'file' ? 'file' : 'text');
            setErrors((current) => ({ ...current, source: undefined }));
          }}
        >
          <TabsList aria-label={t('knowledge.form.source')}>
            <TabsTrigger value="text">{t('knowledge.form.text')}</TabsTrigger>
            <TabsTrigger value="file">{t('knowledge.form.file')}</TabsTrigger>
          </TabsList>
          <TabsContent value="text" className="pt-4">
            <Field label={t('knowledge.form.textLabel')} error={errors.source}>
              <Textarea
                value={text}
                rows={8}
                className="max-h-96 min-h-40"
                placeholder={t('knowledge.form.textPlaceholder')}
                onChange={(event) => {
                  setText(event.target.value);
                }}
              />
            </Field>
          </TabsContent>
          <TabsContent value="file" className="flex flex-col gap-2 pt-4">
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
                'flex cursor-pointer flex-col items-center justify-center gap-2 rounded-2xl border-2 border-dashed px-6 py-10 text-center transition-colors',
                'has-[:focus-visible]:ring-4 has-[:focus-visible]:ring-primary-500/20',
                dragging
                  ? 'border-primary bg-primary-soft'
                  : 'border-border-strong bg-surface-raised hover:border-primary-border hover:bg-primary-soft/40',
              )}
            >
              <Upload className="size-6 text-primary-text" aria-hidden />
              <span className="font-medium text-fg">
                {file ? file.name : t('knowledge.form.drop')}
              </span>
              <span className="text-sm text-fg-subtle">{t('knowledge.form.fileRules')}</span>
              <input
                type="file"
                className="sr-only"
                accept={KNOWLEDGE_EXTENSIONS.join(',')}
                aria-label={t('knowledge.form.chooseFile')}
                onChange={(event) => {
                  pick(event.target.files?.[0]);
                  event.target.value = '';
                }}
              />
            </label>
            {errors.source && (
              <p className="text-sm text-danger" role="alert">
                {errors.source}
              </p>
            )}
          </TabsContent>
        </Tabs>
      </DialogBody>
      <DialogFooter>
        <Button type="button" variant="secondary" onClick={onDone}>
          {t('common:actions.cancel')}
        </Button>
        <Button type="submit" loading={create.isPending}>
          {t('knowledge.form.submit')}
        </Button>
      </DialogFooter>
    </form>
  );
}

function RemoveDialog({
  doc,
  onOpenChange,
}: {
  doc: KnowledgeDocument | null;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation(['ai', 'common']);
  const remove = useDeleteKnowledgeDocument();
  return (
    <ConfirmDialog
      open={doc !== null}
      onOpenChange={onOpenChange}
      tone="danger"
      title={t('knowledge.removeTitle', { title: doc?.title ?? '' })}
      description={t('knowledge.removeText')}
      confirmLabel={t('common:actions.delete')}
      loading={remove.isPending}
      onConfirm={() => {
        if (!doc) return;
        remove.mutate(doc.id, {
          onSuccess: () => {
            toast.success(t('knowledge.removed'));
            onOpenChange(false);
          },
          onError: (err) => {
            toast.error(errorMessage(err));
          },
        });
      }}
    />
  );
}
