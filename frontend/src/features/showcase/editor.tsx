import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { getRouteApi, Link } from '@tanstack/react-router';
import { ExternalLink, ImagePlus, Inbox, Mail, Trash2, Wand2 } from 'lucide-react';
import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Badge,
  Button,
  Callout,
  ConfirmDialog,
  EmptyState,
  Field,
  FieldSet,
  Input,
  Radio,
  RadioGroup,
  Skeleton,
  SkeletonText,
  Switch,
  Tag,
  Textarea,
  toast,
  type Tone,
} from '@/design-system';
import { canEdit, type Project } from '@/features/projects/api';
import { ApiError } from '@/shared/api/client';
import { errorMessage, fieldErrorMessage } from '@/shared/api/errors';
import { TagPicker } from '@/shared/forms/tag-picker';
import { useFormatters } from '@/shared/i18n/format';

import {
  projectContactsQuery,
  showcaseEntryQuery,
  useDeleteShowcaseCover,
  usePublishShowcaseEntry,
  useSaveShowcaseEntry,
  useSetShowcaseCover,
  type ShowcaseCard,
  type ShowcaseEntry,
  type ShowcaseEntryUpdate,
  type ShowcaseStatus,
  type ShowcaseVisibility,
} from './api';
import { ShowcaseCardView } from './card';
import { LIMITS, mergeForm, NEEDS, slugify, type ShowcaseNeed } from './logic';
import { useShowcasePreview } from './preview-store';

const appRoute = getRouteApi('/app');

const MAX_COVER_BYTES = 10 * 1024 * 1024;
const THIS_YEAR = new Date().getFullYear();
const COVER_TYPES = 'image/jpeg,image/png,image/webp';

type SectionField = 'problem' | 'solution' | 'market' | 'traction' | 'ask';
const SECTION_FIELDS: SectionField[] = ['problem', 'solution', 'market', 'traction', 'ask'];

/** The form, flat: every key is a field the editor merges on its own. */
type Draft = Record<SectionField, string> & {
  slug: string;
  title: string;
  tagline: string;
  video_url: string;
  needs: ShowcaseNeed[];
  tags: string[];
  visibility: ShowcaseVisibility;
  show_score: boolean;
  contact_enabled: boolean;
};

/** The label key of a draft field, to name it in a conflict. */
const FIELD_LABELS: Record<keyof Draft, string> = {
  slug: 'editor.fields.slug',
  title: 'editor.fields.title',
  tagline: 'editor.fields.tagline',
  video_url: 'editor.fields.video',
  problem: 'editor.fields.problem',
  solution: 'editor.fields.solution',
  market: 'editor.fields.market',
  traction: 'editor.fields.traction',
  ask: 'editor.fields.ask',
  needs: 'editor.fields.needs',
  tags: 'editor.fields.tags',
  visibility: 'editor.fields.visibility',
  show_score: 'editor.fields.showScore',
  contact_enabled: 'editor.fields.contactEnabled',
};

function draftOf(entry: ShowcaseEntry): Draft {
  return {
    slug: entry.slug,
    title: entry.title,
    tagline: entry.tagline,
    video_url: entry.video_url ?? '',
    problem: entry.sections.problem ?? '',
    solution: entry.sections.solution ?? '',
    market: entry.sections.market ?? '',
    traction: entry.sections.traction ?? '',
    ask: entry.sections.ask ?? '',
    needs: [...entry.needs],
    tags: [...entry.tags],
    visibility: entry.visibility,
    show_score: entry.show_score,
    contact_enabled: entry.contact_enabled,
  };
}

function bodyOf(draft: Draft): ShowcaseEntryUpdate {
  const sections: ShowcaseEntryUpdate['sections'] = {};
  for (const key of SECTION_FIELDS) {
    const value = draft[key].trim();
    if (value) sections[key] = value;
  }
  return {
    slug: draft.slug.trim(),
    title: draft.title.trim(),
    tagline: draft.tagline.trim(),
    ...(draft.video_url.trim() ? { video_url: draft.video_url.trim() } : {}),
    sections,
    needs: draft.needs,
    tags: draft.tags,
    visibility: draft.visibility,
    show_score: draft.show_score,
    contact_enabled: draft.contact_enabled,
  };
}

const STATUS_TONE: Record<ShowcaseStatus, Tone> = {
  draft: 'neutral',
  in_review: 'warning',
  published: 'success',
  rejected: 'danger',
};

/** The team's editor of the project's showcase page. */
export function ShowcaseEditor({ project }: { project: Project }) {
  const entry = useQuery(showcaseEntryQuery(project.id));
  if (entry.isError) {
    return (
      <div className="px-4 py-8 lg:px-10">
        <Callout tone="danger">{errorMessage(entry.error)}</Callout>
      </div>
    );
  }
  if (!entry.data) {
    return (
      <div className="flex flex-col gap-6 px-4 py-8 lg:px-10" aria-busy>
        <Skeleton className="h-8 w-56" />
        <Skeleton className="h-20 rounded-xl" />
        <SkeletonText lines={6} />
      </div>
    );
  }
  return (
    <EditorForm
      key={project.id}
      project={project}
      entry={entry.data.data}
      etag={entry.data.etag}
      refetch={async () => (await entry.refetch()).data}
    />
  );
}

interface EditorFormProps {
  project: Project;
  entry: ShowcaseEntry;
  etag: string | undefined;
  refetch: () => Promise<{ data: ShowcaseEntry; etag: string | undefined } | undefined>;
}

/** Fields someone else changed differently while this editor was open. */
interface Conflict {
  fields: (keyof Draft)[];
  theirs: Draft;
}

function EditorForm({ project, entry, etag, refetch }: EditorFormProps) {
  const { t } = useTranslation(['showcase', 'common']);
  const { session } = appRoute.useRouteContext();
  const editable = canEdit(project);
  const [draft, setDraft] = useState<Draft>(() => draftOf(entry));
  // The version on the server the draft started from (the base of a merge).
  const [base, setBase] = useState<Draft>(() => draftOf(entry));
  const baseline = JSON.stringify(bodyOf(base));
  const [conflict, setConflict] = useState<Conflict | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [confirmUnpublish, setConfirmUnpublish] = useState(false);
  const save = useSaveShowcaseEntry(project.id);
  const publish = usePublishShowcaseEntry(project.id);
  const setCover = useSetShowcaseCover(project.id);
  const deleteCover = useDeleteShowcaseCover(project.id);
  const fileInput = useRef<HTMLInputElement>(null);
  const setPreview = useShowcasePreview((s) => s.setCard);

  const dirty = JSON.stringify(bodyOf(draft)) !== baseline;
  const busy = save.isPending || publish.isPending;
  const evaluation = project.evaluation;

  const tenant = session.tenant;
  const previewCard: ShowcaseCard = {
    slug: draft.slug,
    url: entry.url ?? '',
    title: draft.title || project.title,
    tagline: draft.tagline,
    industry: entry.industry,
    stage: entry.stage,
    faculty: null,
    year: entry.published_at ? new Date(entry.published_at).getFullYear() : THIS_YEAR,
    cover_url: entry.cover_url,
    needs: draft.needs,
    tags: draft.tags,
    score: draft.show_score && evaluation ? evaluation.total : null,
    level: draft.show_score && evaluation ? evaluation.level : null,
    university: {
      slug: tenant.slug,
      name: tenant.name,
      short_name: tenant.short_name,
      color: '',
      logo_url: null,
    },
    published_at: entry.published_at ?? '',
  };
  // The side panel shows the card as it will look; it renders outside this form.
  const previewKey = JSON.stringify(previewCard);
  useEffect(() => {
    setPreview(JSON.parse(previewKey) as ShowcaseCard);
  }, [previewKey, setPreview]);
  useEffect(
    () => () => {
      setPreview(null);
    },
    [setPreview],
  );

  const set = <K extends keyof Draft>(key: K, value: Draft[K]) => {
    setDraft((d) => ({ ...d, [key]: value }));
    clearError(key);
  };
  const setSection = (key: SectionField, value: string) => {
    setDraft((d) => ({ ...d, [key]: value }));
    clearError(`sections.${key}`);
  };
  function clearError(field: string) {
    setErrors((e) => {
      if (!(field in e)) return e;
      const { [field]: _cleared, ...rest } = e;
      return rest;
    });
  }

  const validate = (): Record<string, string> => {
    const out: Record<string, string> = {};
    const slug = draft.slug.trim();
    if (!slug) out.slug = t('common:validation.required');
    else if (slug.length < 3) out.slug = t('common:validation.minLength', { count: 3 });
    else if (!/^[a-z0-9]+(-[a-z0-9]+)*$/.test(slug)) out.slug = t('common:validation.format');
    if (draft.title.trim().length < 2) out.title = t('common:validation.minLength', { count: 2 });
    return out;
  };

  const showErrors = (err: unknown): boolean => {
    if (!(err instanceof ApiError) || err.fieldErrors.length === 0) return false;
    setErrors(Object.fromEntries(err.fieldErrors.map((fe) => [fe.field, fieldErrorMessage(fe)])));
    return true;
  };

  /**
   * Someone saved the page meanwhile (412): their changes are merged into the
   * draft field by field. Without overlapping edits the merged page is saved
   * at once; overlapping fields keep this editor's value until the person
   * chooses (D-036, as in the step editor).
   */
  const mergeNewer = async (): Promise<boolean> => {
    const fresh = await refetch();
    if (!fresh) {
      toast.error(t('editor.actions.stale'));
      return false;
    }
    const theirs = draftOf(fresh.data);
    const { merged, conflicts } = mergeForm(base, draft, theirs);
    setDraft(merged);
    setBase(theirs);
    if (conflicts.length > 0) {
      setConflict({ fields: conflicts, theirs });
      return false;
    }
    await save.mutateAsync({ body: bodyOf(merged), etag: fresh.etag });
    setBase(merged);
    toast.success(t('editor.actions.merged'));
    return true;
  };

  /** Saves the page if needed; resolves false when it could not be saved. */
  const persist = async (): Promise<boolean> => {
    if (!dirty && entry.saved) return true;
    const found = validate();
    setErrors(found);
    if (Object.keys(found).length > 0) return false;
    const saving = draft;
    try {
      await save.mutateAsync({ body: bodyOf(saving), etag });
      setBase(saving);
      setConflict(null);
      return true;
    } catch (err) {
      try {
        if (err instanceof ApiError && err.status === 412) return await mergeNewer();
        throw err;
      } catch (failed) {
        if (!showErrors(failed)) toast.error(errorMessage(failed));
        return false;
      }
    }
  };

  const onSave = async () => {
    if (await persist()) toast.success(t('editor.actions.saved'));
  };

  const onPublish = async () => {
    if (!(await persist())) return;
    try {
      const { data: result } = await publish.mutateAsync(true);
      toast.success(
        result.status === 'in_review'
          ? t('editor.actions.submitted')
          : t('editor.actions.published'),
      );
    } catch (err) {
      // `showcase.incomplete` names the missing sections as field errors.
      showErrors(err);
      toast.error(errorMessage(err));
    }
  };

  const onUnpublish = () => {
    publish.mutate(false, {
      onSuccess: () => {
        setConfirmUnpublish(false);
        toast.success(t('editor.actions.unpublished'));
      },
      onError: (err) => {
        toast.error(errorMessage(err));
      },
    });
  };

  const onCover = (file: File | undefined) => {
    if (!file) return;
    if (file.size > MAX_COVER_BYTES) {
      toast.error(t('common:errors.files.too_large'));
      return;
    }
    setCover.mutate(file, {
      onSuccess: () => {
        toast.success(t('editor.fields.coverUploaded'));
      },
      onError: (err) => {
        toast.error(errorMessage(err));
      },
    });
  };

  const fieldError = (key: string): string | undefined => errors[key];
  const pageUrl = entry.url ?? `${window.location.origin}/showcase/${draft.slug.trim() || '…'}`;
  const canPublish = entry.status === 'draft' || entry.status === 'rejected';
  const canUnpublish = entry.status === 'published' || entry.status === 'in_review';

  return (
    <form
      noValidate
      className="mx-auto flex w-full max-w-3xl flex-col gap-8 px-4 py-6 lg:px-10 lg:py-8"
      onSubmit={(event) => {
        event.preventDefault();
        void onSave();
      }}
    >
      <header className="flex flex-col gap-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex flex-col gap-1.5">
            <h1 className="font-display text-2xl font-bold text-ink">{t('editor.title')}</h1>
            <p className="max-w-xl text-sm text-fg-muted">{t('editor.intro')}</p>
          </div>
          <Badge tone={STATUS_TONE[entry.status]} size="lg" dot>
            {t(`status.${entry.status}`)}
          </Badge>
        </div>
        <StatusCallout entry={entry} />
        {!entry.saved && editable && (
          <Callout tone="ai" title={t('editor.prefilled.title')}>
            {t('editor.prefilled.text')}
          </Callout>
        )}
        {editable && (canPublish || canUnpublish) && (
          <div className="flex flex-wrap gap-2">
            {canPublish && (
              <Button type="button" loading={publish.isPending} onClick={() => void onPublish()}>
                {draft.visibility === 'public' || entry.held
                  ? t('editor.actions.submit')
                  : t('editor.actions.publish')}
              </Button>
            )}
            {canUnpublish && (
              <Button
                type="button"
                variant="ghost"
                onClick={() => {
                  setConfirmUnpublish(true);
                }}
              >
                {t('editor.actions.unpublish')}
              </Button>
            )}
          </div>
        )}
      </header>

      <Group title={t('editor.groups.page')}>
        <Field
          label={t('editor.fields.title')}
          required
          error={fieldError('title')}
          counter={{ value: draft.title.length, max: LIMITS.title }}
        >
          <Input
            value={draft.title}
            maxLength={LIMITS.title}
            disabled={!editable}
            onChange={(e) => {
              set('title', e.target.value);
            }}
          />
        </Field>
        <Field
          label={t('editor.fields.slug')}
          required
          hint={t('editor.fields.slugPreview', { url: pageUrl })}
          error={fieldError('slug')}
          actions={
            editable ? (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                leftIcon={<Wand2 />}
                disabled={!slugify(draft.title)}
                onClick={() => {
                  set('slug', slugify(draft.title));
                }}
              >
                {t('editor.fields.slugGenerate')}
              </Button>
            ) : undefined
          }
        >
          <Input
            value={draft.slug}
            maxLength={LIMITS.slug}
            disabled={!editable}
            spellCheck={false}
            autoCapitalize="off"
            autoComplete="off"
            className="font-mono"
            onChange={(e) => {
              set('slug', e.target.value.toLowerCase());
            }}
          />
        </Field>
        <Field
          label={t('editor.fields.tagline')}
          hint={t('editor.fields.taglineHint')}
          error={fieldError('tagline')}
          counter={{ value: draft.tagline.length, max: LIMITS.tagline }}
        >
          <Input
            value={draft.tagline}
            maxLength={LIMITS.tagline}
            disabled={!editable}
            onChange={(e) => {
              set('tagline', e.target.value);
            }}
          />
        </Field>
      </Group>

      <Group title={t('editor.groups.media')}>
        <div className="flex flex-col gap-3">
          <div className="flex items-baseline justify-between gap-3">
            <p className="text-sm font-medium text-fg">{t('editor.fields.cover')}</p>
          </div>
          <div className="flex flex-col gap-3">
            <div className="aspect-video w-full max-w-md overflow-hidden rounded-xl border border-border bg-surface-sunken">
              {entry.cover_url ? (
                <img
                  src={entry.cover_url}
                  alt={t('editor.fields.coverAlt', { title: draft.title })}
                  width={1600}
                  height={900}
                  className="size-full object-cover"
                />
              ) : (
                <div className="grid size-full place-items-center text-fg-faint">
                  <ImagePlus className="size-8" aria-hidden />
                </div>
              )}
            </div>
            <div className="flex flex-col gap-2">
              <p className="text-sm text-fg-subtle">{t('editor.fields.coverHint')}</p>
              {editable && (
                <div className="flex flex-wrap gap-2">
                  <input
                    ref={fileInput}
                    type="file"
                    accept={COVER_TYPES}
                    className="sr-only"
                    tabIndex={-1}
                    aria-hidden
                    onChange={(e) => {
                      onCover(e.target.files?.[0]);
                      e.target.value = '';
                    }}
                  />
                  <Button
                    type="button"
                    variant="secondary"
                    size="sm"
                    leftIcon={<ImagePlus />}
                    loading={setCover.isPending}
                    onClick={() => fileInput.current?.click()}
                  >
                    {entry.cover_url
                      ? t('editor.fields.coverReplace')
                      : t('editor.fields.coverUpload')}
                  </Button>
                  {entry.cover_url && (
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      leftIcon={<Trash2 />}
                      loading={deleteCover.isPending}
                      onClick={() => {
                        deleteCover.mutate(undefined, {
                          onSuccess: () => toast.success(t('editor.fields.coverRemoved')),
                          onError: (err) => toast.error(errorMessage(err)),
                        });
                      }}
                    >
                      {t('editor.fields.coverRemove')}
                    </Button>
                  )}
                </div>
              )}
            </div>
          </div>
        </div>
        <Field
          label={t('editor.fields.video')}
          optional
          hint={t('editor.fields.videoHint')}
          error={fieldError('video_url')}
        >
          <Input
            type="url"
            inputMode="url"
            value={draft.video_url}
            maxLength={LIMITS.video}
            disabled={!editable}
            onChange={(e) => {
              set('video_url', e.target.value);
            }}
          />
        </Field>
      </Group>

      <Group title={t('editor.groups.content')}>
        {SECTION_FIELDS.map((key) => (
          <Field
            key={key}
            label={t(`editor.fields.${key}`)}
            required={key === 'problem' || key === 'solution'}
            hint={t(`editor.fields.${key}Hint`)}
            error={fieldError(`sections.${key}`)}
            counter={{ value: draft[key].length, max: LIMITS[key] }}
          >
            <Textarea
              rows={key === 'problem' || key === 'solution' ? 5 : 4}
              value={draft[key]}
              disabled={!editable}
              onChange={(e) => {
                setSection(key, e.target.value);
              }}
            />
          </Field>
        ))}
      </Group>

      <Group title={t('editor.groups.needs')}>
        <FieldSet legend={t('editor.fields.needs')} hint={t('editor.fields.needsHint')}>
          <div className="flex flex-wrap gap-2">
            {NEEDS.map((need) => {
              const selected = draft.needs.includes(need);
              return editable ? (
                <Tag
                  key={need}
                  selected={selected}
                  onToggle={() => {
                    set(
                      'needs',
                      selected
                        ? draft.needs.filter((n) => n !== need)
                        : [...draft.needs, need].slice(0, LIMITS.needs),
                    );
                  }}
                >
                  {t(`needs.${need}`)}
                </Tag>
              ) : (
                selected && (
                  <Badge key={need} tone="gold">
                    {t(`needs.${need}`)}
                  </Badge>
                )
              );
            })}
          </div>
        </FieldSet>
        <FieldSet
          legend={t('editor.fields.tags')}
          hint={t('editor.fields.tagsHint')}
          error={fieldError('tags')}
        >
          {editable ? (
            <TagPicker
              value={draft.tags}
              max={LIMITS.tags}
              maxLength={LIMITS.tag}
              onChange={(tags) => {
                // The API keeps tags in lower case.
                set('tags', [...new Set(tags.map((tag) => tag.toLocaleLowerCase()))]);
              }}
            />
          ) : (
            <div className="flex flex-wrap gap-1.5">
              {draft.tags.map((tag) => (
                <Badge key={tag}>{tag}</Badge>
              ))}
            </div>
          )}
        </FieldSet>
      </Group>

      <Group title={t('editor.groups.settings')}>
        <FieldSet legend={t('editor.fields.visibility')}>
          <RadioGroup
            value={draft.visibility}
            disabled={!editable}
            onValueChange={(value) => {
              set('visibility', value as ShowcaseVisibility);
            }}
          >
            {(['university', 'public'] as const).map((value) => (
              <label
                key={value}
                className="flex cursor-pointer gap-3 rounded-xl border border-border bg-surface p-4 transition-colors hover:border-border-strong has-[[data-state=checked]]:border-primary-border has-[[data-state=checked]]:bg-primary-soft/40"
              >
                <Radio value={value} className="mt-0.5" />
                <span className="flex flex-col gap-1">
                  <span className="text-sm font-medium text-fg">{t(`visibility.${value}`)}</span>
                  <span className="text-sm text-fg-muted">
                    {t(
                      value === 'university'
                        ? 'editor.fields.universityHint'
                        : 'editor.fields.publicHint',
                    )}
                  </span>
                </span>
              </label>
            ))}
          </RadioGroup>
        </FieldSet>
        <SwitchRow
          label={t('editor.fields.showScore')}
          hint={
            evaluation
              ? t('editor.fields.showScoreHint', { score: evaluation.total })
              : t('editor.fields.showScoreNone')
          }
          checked={draft.show_score}
          disabled={!editable}
          onChange={(value) => {
            set('show_score', value);
          }}
        />
        <SwitchRow
          label={t('editor.fields.contactEnabled')}
          hint={t('editor.fields.contactEnabledHint')}
          checked={draft.contact_enabled}
          disabled={!editable}
          onChange={(value) => {
            set('contact_enabled', value);
          }}
        />
      </Group>

      {conflict && (
        <Callout
          tone="warning"
          title={t('editor.conflict.title')}
          action={
            <span className="flex flex-wrap gap-2">
              <Button
                type="button"
                size="sm"
                variant="secondary"
                onClick={() => {
                  const theirs = conflict.theirs;
                  setDraft((d) => {
                    const next = { ...d };
                    for (const field of conflict.fields)
                      Object.assign(next, { [field]: theirs[field] });
                    return next;
                  });
                  setConflict(null);
                }}
              >
                {t('editor.conflict.theirs')}
              </Button>
              <Button
                type="button"
                size="sm"
                variant="ghost"
                onClick={() => {
                  setConflict(null);
                }}
              >
                {t('editor.conflict.mine')}
              </Button>
            </span>
          }
        >
          {t('editor.conflict.text', {
            fields: conflict.fields
              .map((field) => t(FIELD_LABELS[field] as 'editor.fields.title'))
              .join(', '),
          })}
        </Callout>
      )}

      {editable && (
        <div className="sticky bottom-0 -mx-4 flex flex-wrap items-center justify-end gap-2 border-t border-border surface-glass px-4 py-3 lg:-mx-10 lg:px-10">
          <Button type="submit" loading={save.isPending} disabled={busy || (!dirty && entry.saved)}>
            {t('common:actions.save')}
          </Button>
        </div>
      )}

      <ConfirmDialog
        open={confirmUnpublish}
        onOpenChange={setConfirmUnpublish}
        title={t('editor.unpublish.title')}
        description={t('editor.unpublish.text')}
        confirmLabel={t('editor.actions.unpublish')}
        tone="danger"
        loading={publish.isPending}
        onConfirm={onUnpublish}
      />
    </form>
  );
}

function Group({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-5">
      <h2 className="border-b border-border pb-2 text-base font-semibold text-ink">{title}</h2>
      {children}
    </section>
  );
}

function SwitchRow({
  label,
  hint,
  checked,
  disabled,
  onChange,
}: {
  label: string;
  hint: string;
  checked: boolean;
  disabled: boolean;
  onChange: (value: boolean) => void;
}) {
  const id = useId();
  return (
    <div className="flex items-start justify-between gap-4 rounded-xl border border-border bg-surface p-4">
      <div className="flex flex-col gap-1">
        <label htmlFor={id} className="text-sm font-medium text-fg">
          {label}
        </label>
        <p id={`${id}-hint`} className="text-sm text-fg-muted">
          {hint}
        </p>
      </div>
      <Switch
        id={id}
        aria-describedby={`${id}-hint`}
        checked={checked}
        disabled={disabled}
        onCheckedChange={onChange}
        className="mt-0.5"
      />
    </div>
  );
}

/** What the status means for the team, with the moderator's comment and the page link. */
function StatusCallout({ entry }: { entry: ShowcaseEntry }) {
  const { t } = useTranslation('showcase');
  // Pages in review or rejected stay live inside the university (D-043).
  const link =
    entry.url && entry.published_at ? (
      <Button asChild size="sm" variant="secondary">
        <a href={entry.url} target="_blank" rel="noopener">
          {t('editor.state.open')}
          <ExternalLink aria-hidden />
        </a>
      </Button>
    ) : undefined;
  switch (entry.status) {
    case 'draft':
      if (entry.held) {
        return (
          <Callout tone="warning" title={t('editor.state.heldTitle')}>
            {t('editor.state.held')}
          </Callout>
        );
      }
      return entry.saved ? <Callout tone="info">{t('editor.state.draft')}</Callout> : null;
    case 'in_review':
      // A page a moderator took down waits for review hidden from everyone.
      return (
        <Callout tone="warning" action={link}>
          {entry.published_at ? t('editor.state.in_review') : t('editor.state.in_reviewHidden')}
        </Callout>
      );
    case 'published':
      return (
        <Callout tone="success" action={link}>
          {entry.visibility === 'public'
            ? t('editor.state.publishedPublic')
            : t('editor.state.published')}
        </Callout>
      );
    case 'rejected':
      return (
        <Callout tone="danger" title={t('editor.state.rejectedTitle')} action={link}>
          <span className="flex flex-col gap-2">
            <span>{t('editor.state.rejected')}</span>
            {entry.review_comment && (
              <span className="flex flex-col gap-1">
                <span className="font-medium">{t('editor.state.comment')}</span>
                <span className="whitespace-pre-line">{entry.review_comment}</span>
              </span>
            )}
          </span>
        </Callout>
      );
  }
}

/** Side panel of the editor: the live card and the requests passed on to the team. */
export function ShowcaseAside({ projectId }: { projectId: string }) {
  const { t } = useTranslation(['showcase', 'common']);
  const card = useShowcasePreview((s) => s.card);
  const contacts = useInfiniteQuery(projectContactsQuery(projectId));
  const items = contacts.data?.pages.flatMap((page) => page.items) ?? [];
  const { dateTime } = useFormatters();
  return (
    <div className="flex h-full flex-col gap-6 overflow-y-auto p-4">
      <section className="flex flex-col gap-3" aria-labelledby="preview-card-title">
        <h2 id="preview-card-title" className="text-sm font-semibold text-ink">
          {t('editor.preview.card')}
        </h2>
        {card ? (
          <div className="pointer-events-none select-none" inert>
            <ShowcaseCardView card={card} external={false} showUniversity={false} linked={false} />
          </div>
        ) : (
          <Skeleton className="aspect-[4/5] rounded-xl" />
        )}
      </section>
      <section className="flex flex-col gap-3" aria-labelledby="contacts-title">
        <h2 id="contacts-title" className="flex items-center gap-2 text-sm font-semibold text-ink">
          <Inbox className="size-4 text-fg-subtle" aria-hidden />
          {t('editor.contacts.title')}
          {items.length > 0 && (
            <Badge size="sm" tone="primary">
              {items.length}
            </Badge>
          )}
        </h2>
        {contacts.isPending ? (
          <SkeletonText lines={3} />
        ) : contacts.isError ? (
          <Callout tone="danger">{errorMessage(contacts.error)}</Callout>
        ) : items.length === 0 ? (
          <EmptyState
            size="sm"
            illustration="inbox"
            title={t('editor.contacts.empty')}
            description={t('editor.contacts.emptyText')}
          />
        ) : (
          <ul className="flex flex-col gap-3">
            {items.map((c) => (
              <li
                key={c.id}
                className="flex flex-col gap-2 rounded-xl border border-border bg-surface p-3"
              >
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span className="text-sm font-medium text-fg">{c.name}</span>
                  <Badge size="sm" tone="info">
                    {t(`contactKinds.${c.kind}`)}
                  </Badge>
                </div>
                {c.organization && <span className="text-xs text-fg-muted">{c.organization}</span>}
                <p className="text-sm leading-relaxed whitespace-pre-line text-fg">{c.message}</p>
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span className="text-xs text-fg-subtle">
                    {t('editor.contacts.received', { time: dateTime(c.created_at) })}
                  </span>
                  <Button asChild size="sm" variant="secondary">
                    <a href={`mailto:${c.email}`}>
                      <Mail aria-hidden />
                      {t('editor.contacts.reply')}
                    </a>
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}
        {contacts.hasNextPage && (
          <Button
            size="sm"
            variant="secondary"
            className="self-start"
            loading={contacts.isFetchingNextPage}
            onClick={() => void contacts.fetchNextPage()}
          >
            {t('common:actions.loadMore')}
          </Button>
        )}
      </section>
      <Link to="/showcase" className="text-sm text-primary-text hover:underline">
        {t('site.showcase')}
      </Link>
    </div>
  );
}
