import { useMutation, useQuery, type UseQueryResult } from '@tanstack/react-query';
import { getRouteApi, Link, useNavigate } from '@tanstack/react-router';
import {
  ArrowLeft,
  ArrowRight,
  CircleAlert,
  Compass,
  Lightbulb,
  Search,
  Sparkles,
  type LucideIcon,
} from 'lucide-react';
import { AnimatePresence, LazyMotion, m } from 'motion/react';
import { useRef, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Button,
  Callout,
  Card,
  celebrateStage,
  cn,
  EdgeIllustration,
  Field,
  Illustration,
  type IllustrationName,
  Input,
  LanguageSwitcher,
  Logo,
  Radio,
  RadioGroup,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Skeleton,
  SkipLink,
  Tag,
  Textarea,
  toast,
  useReducedMotion,
} from '@/design-system';
import { safeRedirect, useSignOut, type Session } from '@/features/auth/api';
import { FormAlert } from '@/features/auth/components';
import { useApplyMe } from '@/features/profile/api';
import {
  facultiesQuery,
  studyGroupsQuery,
  type Faculty,
  type StudyGroup,
} from '@/features/structure/api';
import { api, unwrap } from '@/shared/api/client';
import { errorMessage } from '@/shared/api/errors';
import { TagPicker } from '@/shared/forms/tag-picker';
import { useDebouncedValue } from '@/shared/hooks/use-debounced-value';
import { changeLocale, isLocale, LOCALES } from '@/shared/i18n';
import { loadMotionFeatures } from '@/shared/motion';
import { useLocalized } from '@/shared/i18n/localized';

import {
  COURSES,
  courseFromAdmission,
  LIMITS,
  STEPS,
  STUDY_LEVELS,
  toRequest,
  validateStep,
  type OnboardingDraft,
  type OnboardingRequest,
  type StepContext,
  type StepErrors,
  type StepKey,
} from './steps';

const INTERESTS = [
  'ai',
  'edtech',
  'agritech',
  'fintech',
  'medtech',
  'ecology',
  'logistics',
  'govtech',
  'foodtech',
  'fashion',
  'energy',
  'robotics',
  'games',
  'social',
  'media',
  'tourism',
] as const;

const SKILLS = [
  'frontend',
  'backend',
  'mobile',
  'ml',
  'data',
  'uiux',
  'graphic',
  'marketing',
  'smm',
  'sales',
  'finance',
  'law',
  'pitching',
  'research',
  'hardware',
  'video',
  'copywriting',
  'pm',
] as const;

function initialDraft(session: Session, language: string): OnboardingDraft {
  const { membership } = session;
  return {
    locale: isLocale(language) ? language : session.user.locale,
    faculty_id: membership.faculty?.id ?? null,
    department_id: membership.department?.id ?? null,
    group: membership.group,
    study_level: membership.study_level,
    course: membership.course,
    interests: [],
    skills: [],
    has_idea: null,
    idea: '',
  };
}

function useCompleteOnboarding() {
  const apply = useApplyMe();
  return useMutation({
    mutationFn: (body: OnboardingRequest) => unwrap(api.PUT('/me/onboarding', { body })),
    onSuccess: apply,
  });
}

type Update = (patch: Partial<OnboardingDraft>) => void;

interface StepProps {
  draft: OnboardingDraft;
  update: Update;
  errors: StepErrors;
}

/** Four short steps after the first sign-in: language, studies, interests, idea. */

/** A drawing for each step of onboarding. */
const STEP_ART: Record<StepKey, IllustrationName> = {
  language: 'route',
  study: 'university',
  interests: 'puzzle',
  idea: 'idea',
};

export interface OnboardingSearch {
  /** Where the member was going; opened once onboarding is done. */
  redirect?: string;
}

export function OnboardingPage({ session, redirect }: { session: Session; redirect?: string }) {
  const { t, i18n } = useTranslation(['onboarding', 'common']);
  const navigate = useNavigate();
  const reduce = useReducedMotion();
  const faculties = useQuery(facultiesQuery);
  const complete = useCompleteOnboarding();
  const signOut = useSignOut();

  const [step, setStep] = useState(0);
  const [direction, setDirection] = useState(1);
  const [draft, setDraft] = useState(() => initialDraft(session, i18n.language));
  const [errors, setErrors] = useState<StepErrors>({});
  const [formError, setFormError] = useState<string | null>(null);
  const moved = useRef(false);

  const student = session.membership.roles.includes('student');
  const ctx: StepContext = { student, hasFaculties: (faculties.data?.length ?? 0) > 0 };
  const key = STEPS[step] ?? 'language';
  const last = step === STEPS.length - 1;

  const update: Update = (patch) => {
    setDraft((current) => ({ ...current, ...patch }));
    setErrors((current) =>
      Object.fromEntries(Object.entries(current).filter(([field]) => !(field in patch))),
    );
  };

  const go = (target: number) => {
    moved.current = true;
    setDirection(target > step ? 1 : -1);
    setStep(target);
  };

  const finish = async () => {
    setFormError(null);
    try {
      await complete.mutateAsync(toRequest(draft, ctx));
      celebrateStage();
      toast.success(t('done.title'), { description: t('done.text') });
      await navigate({ href: safeRedirect(redirect) ?? '/app', replace: true });
    } catch (err) {
      setFormError(errorMessage(err));
    }
  };

  const onSubmit = () => {
    const found = validateStep(key, draft, ctx);
    setErrors(found);
    if (Object.keys(found).length > 0) return;
    if (last) void finish();
    else go(step + 1);
  };

  const offset = reduce ? 0 : 32;
  const props: StepProps = { draft, update, errors };

  return (
    <div className="relative flex min-h-dvh flex-col overflow-x-clip bg-bg">
      <SkipLink>{t('common:actions.skipToContent')}</SkipLink>
      <EdgeIllustration
        name="route"
        placement="bottom-left"
        className="hidden w-80 -translate-y-1/4 rotate-6 xl:block"
      />
      <EdgeIllustration
        name="rocket"
        placement="right"
        className="hidden w-64 -rotate-12 xl:block"
      />
      <header className="sticky top-0 z-40 border-b border-border/70 surface-glass">
        <div className="mx-auto flex h-16 max-w-page items-center gap-3 px-4 sm:px-6">
          <Link to="/" className="rounded-lg">
            <Logo />
          </Link>
          <div className="ml-auto flex items-center gap-1">
            <LanguageSwitcher />
            <Button
              variant="ghost"
              size="sm"
              loading={signOut.isPending}
              onClick={() => {
                signOut.mutate(undefined, {
                  onSettled: () => void navigate({ to: '/' }),
                });
              }}
            >
              {t('common:actions.signOut')}
            </Button>
          </div>
        </div>
      </header>

      <main
        id="main"
        tabIndex={-1}
        className="relative flex flex-1 justify-center overflow-hidden px-4 py-8 sm:py-12"
      >
        <div
          aria-hidden
          className="absolute -top-40 left-1/2 h-[26rem] w-[50rem] -translate-x-1/2 rounded-full bg-primary-300/25 blur-3xl dark:bg-primary-600/15"
        />
        <div className="relative flex w-full max-w-2xl flex-col gap-6">
          <WizardProgress step={step} />
          <Card className="overflow-hidden shadow-md">
            <form
              noValidate
              onSubmit={(event) => {
                event.preventDefault();
                onSubmit();
              }}
            >
              <div className="p-5 sm:p-8">
                <LazyMotion features={loadMotionFeatures} strict>
                  <AnimatePresence mode="wait" initial={false} custom={direction}>
                    <m.div
                      key={key}
                      custom={direction}
                      variants={{
                        enter: (dir: number) => ({ opacity: 0, x: dir * offset }),
                        center: { opacity: 1, x: 0 },
                        exit: (dir: number) => ({ opacity: 0, x: dir * -offset }),
                      }}
                      initial="enter"
                      animate="center"
                      exit="exit"
                      transition={{ duration: 0.22, ease: [0.16, 1, 0.3, 1] }}
                      className="flex flex-col gap-6"
                    >
                      <div className="flex items-center justify-between gap-6">
                        <div className="flex flex-col gap-2">
                          <h1
                            tabIndex={-1}
                            ref={(element) => {
                              if (element && moved.current) element.focus();
                            }}
                            className="font-display text-2xl font-bold text-ink outline-none sm:text-3xl"
                          >
                            {key === 'language'
                              ? t('language.title', { name: session.user.full_name })
                              : key === 'study'
                                ? t(student ? 'study.title' : 'study.titleStaff')
                                : t(`${key}.title`)}
                          </h1>
                          <p className="text-fg-muted">
                            {key === 'study'
                              ? t(student ? 'study.subtitle' : 'study.subtitleStaff')
                              : t(`${key}.subtitle`)}
                          </p>
                        </div>
                        <Illustration
                          name={STEP_ART[key]}
                          className="hidden w-32 shrink-0 sm:block"
                        />
                      </div>
                      {key === 'language' && <LanguageStep {...props} />}
                      {key === 'study' && (
                        <StudyStep {...props} student={student} faculties={faculties} />
                      )}
                      {key === 'interests' && <InterestsStep {...props} />}
                      {key === 'idea' && <IdeaStep {...props} />}
                    </m.div>
                  </AnimatePresence>
                </LazyMotion>
                <FormAlert message={formError} />
              </div>
              <div className="flex items-center justify-between gap-3 border-t border-border bg-surface-raised px-5 py-4 sm:px-8">
                {step > 0 ? (
                  <Button
                    type="button"
                    variant="ghost"
                    leftIcon={<ArrowLeft />}
                    onClick={() => {
                      go(step - 1);
                    }}
                  >
                    {t('common:actions.back')}
                  </Button>
                ) : (
                  <span />
                )}
                <Button
                  type="submit"
                  loading={complete.isPending}
                  disabled={key === 'study' && faculties.isPending}
                  leftIcon={last ? <Sparkles /> : undefined}
                  rightIcon={last ? undefined : <ArrowRight />}
                >
                  {last ? t('finish') : t('common:actions.next')}
                </Button>
              </div>
            </form>
          </Card>
        </div>
      </main>
    </div>
  );
}

function WizardProgress({ step }: { step: number }) {
  const { t } = useTranslation('onboarding');
  const key = STEPS[step] ?? 'language';
  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm font-medium text-fg-subtle" aria-live="polite">
        {t('progress', { current: step + 1, total: STEPS.length, step: t(`steps.${key}`) })}
      </p>
      <ol className="grid grid-cols-4 gap-2" aria-hidden>
        {STEPS.map((item, index) => (
          <li key={item} className="flex flex-col gap-2">
            <span
              className={cn(
                'h-1.5 rounded-full transition-colors duration-300',
                index < step && 'bg-primary',
                index === step && 'bg-brand',
                index > step && 'bg-surface-sunken',
              )}
            />
            <span
              className={cn(
                'hidden text-xs font-medium sm:block',
                index === step ? 'text-fg' : 'text-fg-subtle',
              )}
            >
              {t(`steps.${item}`)}
            </span>
          </li>
        ))}
      </ol>
    </div>
  );
}

function ErrorText({ message }: { message: string | undefined }) {
  if (!message) return null;
  return (
    <p className="flex items-center gap-1.5 text-sm text-danger" role="alert">
      <CircleAlert className="size-4 shrink-0" aria-hidden />
      {message}
    </p>
  );
}

function Group({
  legend,
  error,
  optional,
  required,
  children,
}: {
  legend: string;
  error?: string;
  optional?: boolean;
  required?: boolean;
  children: ReactNode;
}) {
  const { t } = useTranslation();
  return (
    <fieldset className="flex flex-col gap-2">
      <legend className="mb-1.5 text-sm font-medium text-fg">
        {legend}
        {required && (
          <span aria-hidden className="ml-0.5 text-danger">
            *
          </span>
        )}
        {optional && (
          <span className="ml-1.5 font-normal text-fg-subtle">{t('field.optional')}</span>
        )}
      </legend>
      {children}
      <ErrorText message={error} />
    </fieldset>
  );
}

function ChoiceCard({
  value,
  icon: Icon,
  title,
  description,
}: {
  value: string;
  icon?: LucideIcon;
  title: string;
  description: string;
}) {
  return (
    <label className="flex cursor-pointer items-start gap-3 rounded-xl border border-border bg-surface p-4 transition-colors hover:border-border-strong has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary-soft">
      <Radio value={value} className="mt-0.5" />
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="flex items-center gap-2 font-semibold text-fg">
          {Icon && <Icon className="size-4 text-primary-text" aria-hidden />}
          {title}
        </span>
        <span className="text-sm text-fg-muted">{description}</span>
      </span>
    </label>
  );
}

/** Short mutually exclusive options that wrap on narrow screens. */
function PillGroup({
  value,
  onValueChange,
  options,
}: {
  value: string;
  onValueChange: (value: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <RadioGroup value={value} onValueChange={onValueChange} className="flex flex-wrap gap-2">
      {options.map((option) => (
        <label
          key={option.value}
          className="flex h-10 min-w-10 cursor-pointer items-center justify-center gap-2 rounded-full border border-border bg-surface px-4 text-sm font-medium text-fg-muted transition-colors hover:border-border-strong hover:text-fg has-[:focus-visible]:ring-4 has-[:focus-visible]:ring-primary-500/20 has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary-soft has-[[data-state=checked]]:text-primary-soft-fg"
        >
          <Radio value={option.value} className="sr-only" />
          {option.label}
        </label>
      ))}
    </RadioGroup>
  );
}

function LanguageStep({ draft, update, errors }: StepProps) {
  const { t } = useTranslation(['onboarding', 'common']);
  return (
    <Group legend={t('language.legend')} error={errors.locale}>
      <RadioGroup
        value={draft.locale}
        onValueChange={(value) => {
          if (!isLocale(value)) return;
          update({ locale: value });
          void changeLocale(value);
        }}
        className="grid grid-cols-1 gap-3 sm:grid-cols-3"
      >
        {LOCALES.map((locale) => (
          <ChoiceCard
            key={locale}
            value={locale}
            title={t(`common:language.${locale}`)}
            description={t(`language.samples.${locale}`)}
          />
        ))}
      </RadioGroup>
    </Group>
  );
}

function StudyStep({
  draft,
  update,
  errors,
  student,
  faculties,
}: StepProps & {
  student: boolean;
  faculties: UseQueryResult<Faculty[]>;
}) {
  const { t } = useTranslation(['onboarding', 'common']);
  const text = useLocalized();
  const faculty = faculties.data?.find((item) => item.id === draft.faculty_id);
  const courses = draft.study_level ? COURSES[draft.study_level] : [];

  const pickGroup = (group: StudyGroup) => {
    const course = group.admission_year
      ? courseFromAdmission(group.admission_year, new Date())
      : draft.course;
    update({
      group: { id: group.id, code: group.code },
      faculty_id: group.faculty_id,
      department_id: group.department_id ?? draft.department_id,
      study_level: group.study_level,
      course: course !== null && COURSES[group.study_level].includes(course) ? course : null,
    });
  };

  return (
    <div className="flex flex-col gap-6">
      {faculties.isPending && (
        <div className="flex flex-col gap-2" aria-busy>
          <Skeleton className="h-4 w-24" />
          <Skeleton className="h-10 w-full" />
        </div>
      )}
      {faculties.isError && (
        <Callout
          tone="danger"
          action={
            <Button size="sm" variant="secondary" onClick={() => void faculties.refetch()}>
              {t('common:actions.retry')}
            </Button>
          }
        >
          {errorMessage(faculties.error)}
        </Callout>
      )}
      {faculties.isSuccess && faculties.data.length === 0 && (
        <Callout tone="info">{t('study.noStructure')}</Callout>
      )}
      {faculties.isSuccess && faculties.data.length > 0 && (
        <div className="grid grid-cols-1 gap-5">
          <Field label={t('study.faculty')} required error={errors.faculty_id}>
            <Select
              value={draft.faculty_id ?? ''}
              onValueChange={(id) => {
                if (id !== draft.faculty_id)
                  update({ faculty_id: id, department_id: null, group: null });
              }}
            >
              <SelectTrigger>
                <SelectValue placeholder={t('study.choose')} />
              </SelectTrigger>
              <SelectContent>
                {faculties.data.map((item) => (
                  <SelectItem key={item.id} value={item.id}>
                    {text(item.name)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
          {faculty && faculty.departments.length > 0 && (
            <Field label={t('study.department')} optional>
              <Select
                value={draft.department_id ?? ''}
                onValueChange={(id) => {
                  update({ department_id: id });
                }}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t('study.choose')} />
                </SelectTrigger>
                <SelectContent>
                  {faculty.departments.map((item) => (
                    <SelectItem key={item.id} value={item.id}>
                      {text(item.name)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
          )}
        </div>
      )}
      {student && (
        <>
          <div className="flex flex-col gap-6">
            <Group legend={t('study.level')} required error={errors.study_level}>
              <PillGroup
                value={draft.study_level ?? ''}
                onValueChange={(value) => {
                  const level = STUDY_LEVELS.find((item) => item === value);
                  if (!level) return;
                  update({
                    study_level: level,
                    course:
                      draft.course !== null && COURSES[level].includes(draft.course)
                        ? draft.course
                        : null,
                  });
                }}
                options={STUDY_LEVELS.map((level) => ({
                  value: level,
                  label: t(`common:studyLevels.${level}`),
                }))}
              />
            </Group>
            <Group legend={t('study.course')} required error={errors.course}>
              {courses.length > 0 ? (
                <PillGroup
                  value={draft.course === null ? '' : String(draft.course)}
                  onValueChange={(course) => {
                    update({ course: Number(course) });
                  }}
                  options={courses.map((course) => ({
                    value: String(course),
                    label: String(course),
                  }))}
                />
              ) : (
                <p className="flex h-9 items-center text-sm text-fg-subtle">
                  {t('study.levelFirst')}
                </p>
              )}
            </Group>
          </div>
          <Group legend={t('study.group')} optional>
            {draft.group ? (
              <div>
                <Tag
                  selected
                  removeLabel={t('study.groupRemove', { group: draft.group.code })}
                  onRemove={() => {
                    update({ group: null });
                  }}
                >
                  {draft.group.code}
                </Tag>
              </div>
            ) : (
              <GroupSearch
                facultyId={draft.faculty_id}
                departmentId={draft.department_id}
                onPick={pickGroup}
              />
            )}
          </Group>
        </>
      )}
    </div>
  );
}

function GroupSearch({
  facultyId,
  departmentId,
  onPick,
}: {
  facultyId: string | null;
  departmentId: string | null;
  onPick: (group: StudyGroup) => void;
}) {
  const { t } = useTranslation('onboarding');
  const [query, setQuery] = useState('');
  const q = useDebouncedValue(query.trim(), 250);
  const groups = useQuery({
    ...studyGroupsQuery({
      ...(facultyId ? { faculty_id: facultyId } : {}),
      ...(departmentId ? { department_id: departmentId } : {}),
      ...(q ? { q } : {}),
    }),
    enabled: Boolean(q || facultyId),
  });

  return (
    <div className="flex flex-col gap-3">
      <Input
        value={query}
        maxLength={40}
        leading={<Search />}
        aria-label={t('study.groupSearch')}
        placeholder={t('study.groupPlaceholder')}
        onChange={(event) => {
          setQuery(event.target.value);
        }}
      />
      {groups.isSuccess && groups.data.length > 0 && (
        <ul className="flex flex-wrap gap-2" aria-label={t('study.groupResults')}>
          {groups.data.map((group) => (
            <li key={group.id}>
              <Tag
                onToggle={() => {
                  onPick(group);
                }}
              >
                {group.code}
              </Tag>
            </li>
          ))}
        </ul>
      )}
      {groups.isSuccess && groups.data.length === 0 && (
        <p className="text-sm text-fg-subtle">{t('study.groupEmpty')}</p>
      )}
    </div>
  );
}

function InterestsStep({ draft, update, errors }: StepProps) {
  const { t } = useTranslation('onboarding');
  return (
    <div className="flex flex-col gap-6">
      <Field label={t('interests.interests')} required error={errors.interests}>
        <TagPicker
          value={draft.interests}
          onChange={(interests) => {
            update({ interests });
          }}
          max={LIMITS.interests}
          maxLength={LIMITS.tag}
          placeholder={t('interests.interestsPlaceholder')}
          suggestions={INTERESTS.map((item) => t(`suggestions.interests.${item}`))}
        />
      </Field>
      <Field label={t('interests.skills')} optional error={errors.skills}>
        <TagPicker
          value={draft.skills}
          onChange={(skills) => {
            update({ skills });
          }}
          max={LIMITS.skills}
          maxLength={LIMITS.tag}
          placeholder={t('interests.skillsPlaceholder')}
          suggestions={SKILLS.map((item) => t(`suggestions.skills.${item}`))}
        />
      </Field>
    </div>
  );
}

function IdeaStep({ draft, update, errors }: StepProps) {
  const { t } = useTranslation('onboarding');
  const value = draft.has_idea === null ? '' : draft.has_idea ? 'yes' : 'no';
  return (
    <div className="flex flex-col gap-5">
      <Group legend={t('idea.legend')} required error={errors.has_idea}>
        <RadioGroup
          value={value}
          onValueChange={(next) => {
            update({ has_idea: next === 'yes' });
          }}
          className="grid grid-cols-1 gap-3 sm:grid-cols-2"
        >
          <ChoiceCard
            value="yes"
            icon={Lightbulb}
            title={t('idea.yes')}
            description={t('idea.yesText')}
          />
          <ChoiceCard
            value="no"
            icon={Compass}
            title={t('idea.no')}
            description={t('idea.noText')}
          />
        </RadioGroup>
      </Group>
      {draft.has_idea === true && (
        <Field
          label={t('idea.oneLiner')}
          hint={t('idea.hint')}
          required
          error={errors.idea}
          counter={{ value: draft.idea.length, max: LIMITS.idea }}
        >
          <Textarea
            autoGrow
            rows={3}
            value={draft.idea}
            placeholder={t('idea.placeholder')}
            onChange={(event) => {
              update({ idea: event.target.value });
            }}
          />
        </Field>
      )}
      {draft.has_idea === false && <Callout tone="ai">{t('idea.teamHint')}</Callout>}
    </div>
  );
}

const onboardingRoute = getRouteApi('/onboarding');

export function OnboardingRoute() {
  const { session } = onboardingRoute.useRouteContext();
  const { redirect } = onboardingRoute.useSearch();
  return <OnboardingPage session={session} redirect={redirect} />;
}
