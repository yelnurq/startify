import { zodResolver } from '@hookform/resolvers/zod';
import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { Camera, Copy, ExternalLink, Globe, Lock, Save, Trash2 } from 'lucide-react';
import { useId, useRef, useState, type ChangeEvent, type ReactNode } from 'react';
import { Controller, useForm, useWatch } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from '@/shared/forms/zod';

import {
  Avatar,
  Badge,
  Button,
  Callout,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  cn,
  Field,
  Input,
  Skeleton,
  Spinner,
  Switch,
  Textarea,
  toast,
  TruncatedText,
} from '@/design-system';
import { FormAlert } from '@/features/auth/components';
import { primaryRole } from '@/features/shell/roles';
import { ApiError } from '@/shared/api/client';
import { errorMessage } from '@/shared/api/errors';
import { TagPicker } from '@/shared/forms/tag-picker';
import { applyFieldErrors } from '@/shared/forms/validation';
import { i18n } from '@/shared/i18n';

import {
  AVATAR_MAX_BYTES,
  AVATAR_TYPES,
  meQuery,
  useDeleteAvatar,
  useUpdateMe,
  useUpdateProfile,
  useUploadAvatar,
  type Me,
} from './api';
import { publicProfileUrl, TELEGRAM } from './links';

const SLUG = /^[a-z0-9](?:[a-z0-9-]{1,38}[a-z0-9])$/;
const optionalUrl = z.union([z.literal(''), z.url({ protocol: /^https?$/ }).max(300)]);

const schema = z.object({
  full_name: z.string().trim().min(2).max(120),
  headline: z.string().trim().max(140),
  bio: z.string().trim().max(2000),
  skills: z.array(z.string()).max(20),
  interests: z.array(z.string()).max(12),
  website: optionalUrl,
  github: optionalUrl,
  linkedin: optionalUrl,
  telegram: z
    .string()
    .trim()
    .max(64)
    .refine((value) => value === '' || TELEGRAM.test(value), {
      error: () => i18n.t('app:profile.fields.telegramFormat'),
    }),
  public_slug: z
    .string()
    .trim()
    .regex(SLUG, { error: () => i18n.t('app:profile.fields.slugFormat') }),
  is_public: z.boolean(),
  looking_for_team: z.boolean(),
});

type Values = z.infer<typeof schema>;

function toValues(me: Me): Values {
  const { profile } = me;
  return {
    full_name: me.user.full_name,
    headline: profile.headline ?? '',
    bio: profile.bio ?? '',
    skills: profile.skills,
    interests: profile.interests,
    website: profile.links.website ?? '',
    github: profile.links.github ?? '',
    linkedin: profile.links.linkedin ?? '',
    telegram: profile.links.telegram ?? '',
    public_slug: profile.public_slug,
    is_public: profile.is_public,
    looking_for_team: profile.looking_for_team,
  };
}

function orNull(value: string): string | null {
  return value.trim() === '' ? null : value.trim();
}

/** The member's own profile: photo, about, skills, links and visibility. */
export function ProfilePage() {
  const { t } = useTranslation(['app', 'common']);
  const me = useQuery(meQuery);
  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-1">
        <h1 className="font-display text-3xl font-semibold text-fg">{t('profile.title')}</h1>
        <p className="text-fg-muted">{t('profile.subtitle')}</p>
      </header>
      {me.isPending && (
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-[20rem_1fr]">
          <Skeleton className="h-80 rounded-xl" />
          <Skeleton className="h-[32rem] rounded-xl" />
        </div>
      )}
      {me.isError && (
        <Callout
          tone="danger"
          action={
            <Button size="sm" variant="secondary" onClick={() => void me.refetch()}>
              {t('common:actions.retry')}
            </Button>
          }
        >
          {errorMessage(me.error)}
        </Callout>
      )}
      {me.isSuccess && (
        <div className="grid grid-cols-1 items-start gap-6 lg:grid-cols-[20rem_1fr]">
          <IdentityCard me={me.data} />
          <ProfileForm key={me.data.user.id} me={me.data} />
        </div>
      )}
    </div>
  );
}

function IdentityCard({ me }: { me: Me }) {
  const { t } = useTranslation(['app', 'common']);
  const upload = useUploadAvatar();
  const remove = useDeleteAvatar();
  const input = useRef<HTMLInputElement>(null);
  const role = primaryRole(me.membership.roles);
  const busy = upload.isPending || remove.isPending;

  const onFile = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (!file) return;
    if (!AVATAR_TYPES.includes(file.type)) {
      toast.error(t('common:errors.files.unsupported_type'), {
        description: t('profile.avatar.rules'),
      });
      return;
    }
    if (file.size > AVATAR_MAX_BYTES) {
      toast.error(t('common:errors.files.too_large'), { description: t('profile.avatar.rules') });
      return;
    }
    upload.mutate(file, {
      onSuccess: () => {
        toast.success(t('profile.avatar.uploaded'));
      },
      onError: (err) => {
        toast.error(errorMessage(err));
      },
    });
  };

  return (
    <Card className="lg:sticky lg:top-24">
      <CardContent className="flex flex-col items-center gap-4 text-center">
        <div className="relative">
          <Avatar name={me.user.full_name} src={me.user.avatar_url} size="xl" />
          {busy && (
            <span className="absolute inset-0 grid place-items-center rounded-full bg-surface/70">
              <Spinner className="text-brand" />
            </span>
          )}
        </div>
        <div className="flex w-full min-w-0 flex-col gap-1">
          <TruncatedText as="p" className="text-lg font-semibold text-fg">
            {me.user.full_name}
          </TruncatedText>
          <p className="text-sm break-all text-fg-subtle">{me.user.email}</p>
          {role && (
            <div>
              <Badge tone="primary">{t(`common:roles.${role}`)}</Badge>
            </div>
          )}
        </div>
        <input
          ref={input}
          type="file"
          accept={AVATAR_TYPES.join(',')}
          className="sr-only"
          tabIndex={-1}
          aria-hidden
          onChange={onFile}
        />
        <div className="flex flex-wrap justify-center gap-2">
          <Button
            size="sm"
            variant="secondary"
            leftIcon={<Camera />}
            disabled={busy}
            onClick={() => input.current?.click()}
          >
            {me.user.avatar_url ? t('profile.avatar.change') : t('profile.avatar.upload')}
          </Button>
          {me.user.avatar_url && (
            <Button
              size="sm"
              variant="ghost"
              leftIcon={<Trash2 />}
              disabled={busy}
              onClick={() => {
                remove.mutate(undefined, {
                  onError: (err) => {
                    toast.error(errorMessage(err));
                  },
                });
              }}
            >
              {t('profile.avatar.remove')}
            </Button>
          )}
        </div>
        <p className="text-xs text-fg-subtle">{t('profile.avatar.rules')}</p>
        <PublicLink me={me} />
      </CardContent>
    </Card>
  );
}

function PublicLink({ me }: { me: Me }) {
  const { t } = useTranslation(['app', 'common']);
  if (!me.profile.is_public) {
    return (
      <div className="flex w-full items-start gap-2 rounded-lg bg-surface-raised p-3 text-left text-sm text-fg-muted">
        <Lock className="mt-0.5 size-4 shrink-0" aria-hidden />
        {t('profile.link.hidden')}
      </div>
    );
  }
  const url = publicProfileUrl(me.profile.public_slug);
  return (
    <div className="flex w-full flex-col gap-2 rounded-lg bg-primary-soft p-3 text-left">
      <p className="flex items-center gap-2 text-sm font-medium text-primary-soft-fg">
        <Globe className="size-4" aria-hidden />
        {t('profile.link.title')}
      </p>
      <p className="font-mono text-xs break-all text-fg" data-testid="public-profile-url">
        {url}
      </p>
      <div className="flex gap-2">
        <Button
          size="sm"
          variant="secondary"
          leftIcon={<Copy />}
          onClick={() => {
            void navigator.clipboard.writeText(url).then(
              () => toast.success(t('common:actions.copied')),
              () => toast.error(t('common:errors.internal')),
            );
          }}
        >
          {t('common:actions.copy')}
        </Button>
        <Button asChild size="sm" variant="ghost">
          <Link to="/u/$slug" params={{ slug: me.profile.public_slug }} target="_blank">
            <ExternalLink aria-hidden />
            {t('common:actions.open')}
          </Link>
        </Button>
      </div>
    </div>
  );
}

function Section({
  title,
  description,
  children,
}: {
  title: string;
  description?: string;
  children: ReactNode;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      <CardContent className="flex flex-col gap-5">{children}</CardContent>
    </Card>
  );
}

function ToggleRow({
  title,
  description,
  checked,
  onCheckedChange,
}: {
  title: string;
  description: string;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
}) {
  const id = useId();
  return (
    <div className="flex items-start justify-between gap-4">
      <div className="flex flex-col gap-0.5">
        <label htmlFor={id} className="text-sm font-medium text-fg">
          {title}
        </label>
        <p id={`${id}-description`} className="text-sm text-fg-muted">
          {description}
        </p>
      </div>
      <Switch
        id={id}
        aria-describedby={`${id}-description`}
        checked={checked}
        onCheckedChange={onCheckedChange}
        className="mt-0.5"
      />
    </div>
  );
}

function ProfileForm({ me }: { me: Me }) {
  const { t } = useTranslation(['app', 'common', 'onboarding']);
  const updateMe = useUpdateMe();
  const updateProfile = useUpdateProfile();
  const [formError, setFormError] = useState<string | null>(null);
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: toValues(me) });
  const { errors, isDirty, isSubmitting } = form.formState;
  const [bio, slug] = useWatch({ control: form.control, name: ['bio', 'public_slug'] });

  const onSubmit = form.handleSubmit(async (values) => {
    setFormError(null);
    try {
      if (values.full_name !== me.user.full_name) {
        await updateMe.mutateAsync({ full_name: values.full_name });
      }
      const saved = await updateProfile.mutateAsync({
        headline: orNull(values.headline),
        bio: orNull(values.bio),
        skills: values.skills,
        interests: values.interests,
        links: {
          website: orNull(values.website),
          github: orNull(values.github),
          linkedin: orNull(values.linkedin),
          telegram: orNull(values.telegram),
        },
        public_slug: values.public_slug,
        is_public: values.is_public,
        looking_for_team: values.looking_for_team,
      });
      form.reset(toValues(saved));
      toast.success(t('profile.saved'));
    } catch (err) {
      if (err instanceof ApiError && err.code === 'profile.slug_taken') {
        form.setError('public_slug', { message: t('common:errors.profile.slug_taken') });
        return;
      }
      const mapped = applyFieldErrors(err, form.setError, {
        'links.website': 'website',
        'links.github': 'github',
        'links.linkedin': 'linkedin',
        'links.telegram': 'telegram',
      });
      if (!mapped) setFormError(errorMessage(err));
    }
  });

  return (
    <form className="flex min-w-0 flex-col gap-6" onSubmit={(e) => void onSubmit(e)} noValidate>
      <FormAlert message={formError} />
      <Section title={t('profile.sections.about')}>
        <Field label={t('profile.fields.fullName')} required error={errors.full_name?.message}>
          <Input autoComplete="name" {...form.register('full_name')} />
        </Field>
        <Field
          label={t('profile.fields.headline')}
          hint={t('profile.fields.headlineHint')}
          optional
          error={errors.headline?.message}
        >
          <Input
            maxLength={140}
            placeholder={t('profile.fields.headlinePlaceholder')}
            {...form.register('headline')}
          />
        </Field>
        <Field
          label={t('profile.fields.bio')}
          optional
          error={errors.bio?.message}
          counter={{ value: bio.length, max: 2000 }}
        >
          <Textarea
            autoGrow
            rows={4}
            placeholder={t('profile.fields.bioPlaceholder')}
            {...form.register('bio')}
          />
        </Field>
      </Section>

      <Section title={t('profile.sections.skills')} description={t('profile.sections.skillsText')}>
        <Field label={t('profile.fields.skills')} optional error={errors.skills?.message}>
          <Controller
            control={form.control}
            name="skills"
            render={({ field }) => (
              <TagPicker
                value={field.value}
                onChange={field.onChange}
                max={20}
                placeholder={t('onboarding:interests.skillsPlaceholder')}
              />
            )}
          />
        </Field>
        <Field label={t('profile.fields.interests')} optional error={errors.interests?.message}>
          <Controller
            control={form.control}
            name="interests"
            render={({ field }) => (
              <TagPicker
                value={field.value}
                onChange={field.onChange}
                max={12}
                placeholder={t('onboarding:interests.interestsPlaceholder')}
              />
            )}
          />
        </Field>
      </Section>

      <Section title={t('profile.sections.links')}>
        <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
          <Field label={t('profile.fields.website')} optional error={errors.website?.message}>
            <Input
              type="url"
              inputMode="url"
              placeholder={t('profile.fields.websitePlaceholder')}
              {...form.register('website')}
            />
          </Field>
          <Field label={t('profile.fields.github')} optional error={errors.github?.message}>
            <Input
              type="url"
              inputMode="url"
              placeholder={t('profile.fields.githubPlaceholder')}
              {...form.register('github')}
            />
          </Field>
          <Field label={t('profile.fields.linkedin')} optional error={errors.linkedin?.message}>
            <Input
              type="url"
              inputMode="url"
              placeholder={t('profile.fields.linkedinPlaceholder')}
              {...form.register('linkedin')}
            />
          </Field>
          <Field label={t('profile.fields.telegram')} optional error={errors.telegram?.message}>
            <Input
              placeholder={t('profile.fields.telegramPlaceholder')}
              {...form.register('telegram')}
            />
          </Field>
        </div>
      </Section>

      <Section title={t('profile.sections.visibility')}>
        <Controller
          control={form.control}
          name="is_public"
          render={({ field }) => (
            <ToggleRow
              title={t('profile.fields.isPublic')}
              description={t('profile.fields.isPublicHint')}
              checked={field.value}
              onCheckedChange={field.onChange}
            />
          )}
        />
        <Controller
          control={form.control}
          name="looking_for_team"
          render={({ field }) => (
            <ToggleRow
              title={t('profile.fields.lookingForTeam')}
              description={t('profile.fields.lookingForTeamHint')}
              checked={field.value}
              onCheckedChange={field.onChange}
            />
          )}
        />
        <Field
          label={t('profile.fields.slug')}
          hint={t('profile.fields.slugHint', { url: publicProfileUrl(slug || '…') })}
          error={errors.public_slug?.message}
        >
          <Input autoCapitalize="none" spellCheck={false} {...form.register('public_slug')} />
        </Field>
      </Section>

      <div
        className={cn(
          'flex items-center justify-end gap-3 rounded-xl border border-border bg-surface/90 p-3 backdrop-blur',
          isDirty && 'sticky bottom-4 z-10 shadow-md',
        )}
      >
        {isDirty && <p className="mr-auto text-sm text-fg-muted">{t('profile.unsaved')}</p>}
        <Button
          type="button"
          variant="ghost"
          disabled={!isDirty || isSubmitting}
          onClick={() => {
            form.reset(toValues(me));
          }}
        >
          {t('common:actions.reset')}
        </Button>
        <Button type="submit" leftIcon={<Save />} loading={isSubmitting} disabled={!isDirty}>
          {t('common:actions.save')}
        </Button>
      </div>
    </form>
  );
}
