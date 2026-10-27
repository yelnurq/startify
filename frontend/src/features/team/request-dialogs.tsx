import { zodResolver } from '@hookform/resolvers/zod';
import { useQuery } from '@tanstack/react-query';
import { Copy, Mail, Send } from 'lucide-react';
import { useState } from 'react';
import { Controller, useForm, useWatch } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from '@/shared/forms/zod';

import {
  Avatar,
  Button,
  Callout,
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  Field,
  Input,
  Sheet,
  SheetContent,
  SheetDescription,
  SheetTitle,
  SkeletonText,
  Tag,
  Textarea,
  toast,
} from '@/design-system';
import { FormAlert } from '@/features/auth/components';
import { errorMessage } from '@/shared/api/errors';
import { TagPicker } from '@/shared/forms/tag-picker';
import { applyFieldErrors } from '@/shared/forms/validation';
import { useFormatters } from '@/shared/i18n/format';
import { useLocalized } from '@/shared/i18n/localized';

import {
  TEAM_LIMITS,
  TEAM_ROLES,
  teamResponsesQuery,
  useCreateTeamRequest,
  useRespondToTeamRequest,
  useUpdateTeamRequest,
  type TeamRequest,
} from './api';

const requestSchema = z.object({
  title: z.string().trim().min(TEAM_LIMITS.title[0]).max(TEAM_LIMITS.title[1]),
  description: z.string().trim().min(TEAM_LIMITS.description[0]).max(TEAM_LIMITS.description[1]),
  roles_needed: z.array(z.enum(TEAM_ROLES)).min(1).max(TEAM_LIMITS.roles),
  skills: z.array(z.string()).max(TEAM_LIMITS.skills),
});

type RequestValues = z.infer<typeof requestSchema>;

interface RequestDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Editing an existing request; creating a new one otherwise. */
  request?: TeamRequest;
}

/** Publishes or edits a request for teammates. */
export function RequestDialog({ open, onOpenChange, request }: RequestDialogProps) {
  const { t } = useTranslation('team');
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="lg">
        <DialogHeader>
          <DialogTitle>{request ? t('form.editTitle') : t('form.createTitle')}</DialogTitle>
          <DialogDescription>{t('form.description')}</DialogDescription>
        </DialogHeader>
        {open && (
          <RequestForm
            request={request}
            onDone={() => {
              onOpenChange(false);
            }}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function RequestForm({ request, onDone }: { request?: TeamRequest; onDone: () => void }) {
  const { t } = useTranslation(['team', 'common']);
  const create = useCreateTeamRequest();
  const update = useUpdateTeamRequest();
  const [formError, setFormError] = useState<string | null>(null);
  const form = useForm<RequestValues>({
    resolver: zodResolver(requestSchema),
    defaultValues: {
      title: request?.title ?? '',
      description: request?.description ?? '',
      roles_needed: request?.roles_needed ?? [],
      skills: request?.skills ?? [],
    },
  });
  const { errors, isSubmitting } = form.formState;
  const [title, description] = useWatch({ control: form.control, name: ['title', 'description'] });

  const onSubmit = form.handleSubmit(async (values) => {
    setFormError(null);
    try {
      if (request) {
        await update.mutateAsync({ id: request.id, body: values });
        toast.success(t('form.updated'));
      } else {
        await create.mutateAsync(values);
        toast.success(t('form.created'));
      }
      onDone();
    } catch (err) {
      if (!applyFieldErrors(err, form.setError)) setFormError(errorMessage(err));
    }
  });

  return (
    <form
      className="flex min-h-0 flex-1 flex-col"
      onSubmit={(e) => void onSubmit(e)}
      noValidate
      aria-label={request ? t('form.editTitle') : t('form.createTitle')}
    >
      <DialogBody className="flex flex-col gap-5">
        <FormAlert message={formError} />
        <Field
          label={t('form.title')}
          required
          error={errors.title?.message}
          counter={{ value: title.length, max: TEAM_LIMITS.title[1] }}
        >
          <Input placeholder={t('form.titlePlaceholder')} {...form.register('title')} />
        </Field>
        <Field
          label={t('form.text')}
          hint={t('form.textHint')}
          required
          error={errors.description?.message}
          counter={{ value: description.length, max: TEAM_LIMITS.description[1] }}
        >
          <Textarea
            autoGrow
            rows={5}
            placeholder={t('form.textPlaceholder')}
            {...form.register('description')}
          />
        </Field>
        <Controller
          control={form.control}
          name="roles_needed"
          render={({ field }) => (
            <fieldset className="flex flex-col gap-2">
              <legend className="mb-1.5 text-sm font-medium text-fg">
                {t('form.roles')}
                <span aria-hidden className="ml-0.5 text-danger">
                  *
                </span>
              </legend>
              <p className="text-sm text-fg-subtle">
                {t('form.rolesHint', { max: TEAM_LIMITS.roles })}
              </p>
              <ul className="flex flex-wrap gap-2">
                {TEAM_ROLES.map((role) => {
                  const selected = field.value.includes(role);
                  return (
                    <li key={role}>
                      <Tag
                        selected={selected}
                        onToggle={() => {
                          field.onChange(
                            selected
                              ? field.value.filter((item) => item !== role)
                              : field.value.length < TEAM_LIMITS.roles
                                ? [...field.value, role]
                                : field.value,
                          );
                        }}
                      >
                        {t(`roles.${role}`)}
                      </Tag>
                    </li>
                  );
                })}
              </ul>
              {errors.roles_needed?.message && (
                <p className="text-sm text-danger" role="alert">
                  {errors.roles_needed.message}
                </p>
              )}
            </fieldset>
          )}
        />
        <Field label={t('form.skills')} optional error={errors.skills?.message}>
          <Controller
            control={form.control}
            name="skills"
            render={({ field }) => (
              <TagPicker
                value={field.value}
                onChange={field.onChange}
                max={TEAM_LIMITS.skills}
                placeholder={t('form.skillsPlaceholder')}
              />
            )}
          />
        </Field>
      </DialogBody>
      <DialogFooter>
        <Button type="button" variant="secondary" onClick={onDone}>
          {t('common:actions.cancel')}
        </Button>
        <Button type="submit" loading={isSubmitting}>
          {request ? t('common:actions.save') : t('form.publish')}
        </Button>
      </DialogFooter>
    </form>
  );
}

const respondSchema = z.object({
  message: z.string().trim().min(TEAM_LIMITS.message[0]).max(TEAM_LIMITS.message[1]),
});

/** Sends a reply with the respondent's contact to the author of a request. */
export function RespondDialog({
  request,
  onOpenChange,
}: {
  request: TeamRequest | null;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation(['team', 'common']);
  return (
    <Dialog open={request !== null} onOpenChange={onOpenChange}>
      <DialogContent size="md">
        <DialogHeader>
          <DialogTitle>{t('respond.title')}</DialogTitle>
          <DialogDescription>{request?.title}</DialogDescription>
        </DialogHeader>
        {request && (
          <RespondForm
            request={request}
            onDone={() => {
              onOpenChange(false);
            }}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function RespondForm({ request, onDone }: { request: TeamRequest; onDone: () => void }) {
  const { t } = useTranslation(['team', 'common']);
  const respond = useRespondToTeamRequest();
  const [formError, setFormError] = useState<string | null>(null);
  const form = useForm<z.infer<typeof respondSchema>>({
    resolver: zodResolver(respondSchema),
    defaultValues: { message: '' },
  });
  const message = useWatch({ control: form.control, name: 'message' });

  const onSubmit = form.handleSubmit(async (values) => {
    setFormError(null);
    try {
      await respond.mutateAsync({ id: request.id, message: values.message });
      toast.success(t('respond.sent'), { description: t('respond.sentText') });
      onDone();
    } catch (err) {
      if (!applyFieldErrors(err, form.setError)) setFormError(errorMessage(err));
    }
  });

  return (
    <form className="flex min-h-0 flex-1 flex-col" onSubmit={(e) => void onSubmit(e)} noValidate>
      <DialogBody className="flex flex-col gap-4">
        <FormAlert message={formError} />
        <Field
          label={t('respond.message')}
          hint={t('respond.hint')}
          required
          error={form.formState.errors.message?.message}
          counter={{ value: message.length, max: TEAM_LIMITS.message[1] }}
        >
          <Textarea
            autoGrow
            rows={4}
            placeholder={t('respond.placeholder')}
            {...form.register('message')}
          />
        </Field>
        <Callout tone="info">{t('respond.privacy')}</Callout>
      </DialogBody>
      <DialogFooter>
        <Button type="button" variant="secondary" onClick={onDone}>
          {t('common:actions.cancel')}
        </Button>
        <Button type="submit" leftIcon={<Send />} loading={form.formState.isSubmitting}>
          {t('respond.submit')}
        </Button>
      </DialogFooter>
    </form>
  );
}

/** Replies to the author's own request, with the respondents' contact e-mails. */
export function ResponsesSheet({
  request,
  onOpenChange,
}: {
  request: TeamRequest | null;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation('team');
  return (
    <Sheet open={request !== null} onOpenChange={onOpenChange}>
      <SheetContent side="right" size="md">
        <div className="flex flex-col gap-1 border-b border-border px-6 pt-6 pr-14 pb-4">
          <SheetTitle className="text-lg font-semibold text-fg">{t('responses.title')}</SheetTitle>
          <SheetDescription className="text-sm text-fg-muted">{request?.title}</SheetDescription>
        </div>
        <div className="flex-1 overflow-y-auto px-6 py-5">
          {request && <ResponsesList requestId={request.id} />}
        </div>
      </SheetContent>
    </Sheet>
  );
}

export function ResponsesList({ requestId }: { requestId: string }) {
  const { t } = useTranslation(['team', 'common']);
  const format = useFormatters();
  const text = useLocalized();
  const responses = useQuery(teamResponsesQuery(requestId));

  if (responses.isPending) return <SkeletonText lines={6} />;
  if (responses.isError) return <Callout tone="danger">{errorMessage(responses.error)}</Callout>;
  if (responses.data.length === 0) {
    return (
      <EmptyState
        size="sm"
        illustration="inbox"
        title={t('responses.empty')}
        description={t('responses.emptyText')}
      />
    );
  }
  return (
    <ul className="flex flex-col gap-4">
      {responses.data.map((response) => (
        <li key={response.id} className="flex flex-col gap-3 rounded-xl border border-border p-4">
          <div className="flex items-center gap-3">
            <Avatar name={response.author.full_name} src={response.author.avatar_url} size="sm" />
            <div className="flex min-w-0 flex-1 flex-col">
              <p className="font-medium text-fg">{response.author.full_name}</p>
              <p className="text-xs text-fg-subtle">
                {response.author.faculty ? `${text(response.author.faculty.name)} · ` : ''}
                {format.relative(response.created_at)}
              </p>
            </div>
          </div>
          <p className="text-sm leading-relaxed whitespace-pre-line text-fg">{response.message}</p>
          {response.email && (
            <div className="flex flex-wrap items-center gap-2">
              <Button asChild size="sm" variant="soft">
                <a href={`mailto:${response.email}`}>
                  <Mail aria-hidden />
                  {response.email}
                </a>
              </Button>
              <Button
                size="sm"
                variant="ghost"
                leftIcon={<Copy />}
                onClick={() => {
                  const email = response.email ?? '';
                  void navigator.clipboard.writeText(email).then(
                    () => toast.success(t('common:actions.copied')),
                    () => toast.error(t('common:errors.internal')),
                  );
                }}
              >
                {t('common:actions.copy')}
              </Button>
            </div>
          )}
        </li>
      ))}
    </ul>
  );
}
