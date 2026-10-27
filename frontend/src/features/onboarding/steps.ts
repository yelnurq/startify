import { z } from '@/shared/forms/zod';

import type { components } from '@/shared/api/schema.gen';
import { i18n, LOCALES, type Locale } from '@/shared/i18n';

export type StudyLevel = components['schemas']['StudyLevel'];
export type OnboardingRequest = components['schemas']['OnboardingRequest'];

export const STEPS = ['language', 'study', 'interests', 'idea'] as const;
export type StepKey = (typeof STEPS)[number];

export const STUDY_LEVELS = [
  'bachelor',
  'master',
  'doctorate',
] as const satisfies readonly StudyLevel[];

/** Courses offered at each level of study. */
export const COURSES: Record<StudyLevel, number[]> = {
  bachelor: [1, 2, 3, 4, 5],
  master: [1, 2],
  doctorate: [1, 2, 3],
};

export const LIMITS = { interests: 12, skills: 20, tag: 40, idea: 500, ideaMin: 10 } as const;

export interface OnboardingDraft {
  locale: Locale;
  faculty_id: string | null;
  department_id: string | null;
  group: { id: string; code: string } | null;
  study_level: StudyLevel | null;
  course: number | null;
  interests: string[];
  skills: string[];
  has_idea: boolean | null;
  idea: string;
}

export interface StepContext {
  /** Students also give their level, course and group. */
  student: boolean;
  /** Faculty is required once the university has imported its structure. */
  hasFaculties: boolean;
}

export type StepErrors = Partial<Record<keyof OnboardingDraft, string>>;

const choose = () => i18n.t('common:validation.choose');
const tag = z.string().trim().min(1).max(LIMITS.tag);

function schemaFor(step: StepKey, ctx: StepContext) {
  switch (step) {
    case 'language':
      return z.object({ locale: z.enum(LOCALES, { error: choose }) });
    case 'study':
      return z.object({
        faculty_id: ctx.hasFaculties ? z.string({ error: choose }) : z.string().nullable(),
        study_level: ctx.student
          ? z.enum(STUDY_LEVELS, { error: choose })
          : z.enum(STUDY_LEVELS).nullable(),
        course: ctx.student
          ? z.number({ error: choose }).int().min(1).max(7)
          : z.number().int().min(1).max(7).nullable(),
      });
    case 'interests':
      return z.object({
        interests: z
          .array(tag)
          .min(1, { error: () => i18n.t('onboarding:interests.required') })
          .max(LIMITS.interests),
        skills: z.array(tag).max(LIMITS.skills),
      });
    case 'idea':
      return z
        .object({
          has_idea: z.boolean({ error: choose }),
          idea: z.string().trim().max(LIMITS.idea),
        })
        .superRefine((value, ctx) => {
          if (value.has_idea && value.idea.trim().length < LIMITS.ideaMin) {
            ctx.addIssue({
              code: 'custom',
              path: ['idea'],
              message: i18n.t('common:validation.minLength', { count: LIMITS.ideaMin }),
            });
          }
        });
  }
}

/** Validates one step of the wizard; returns the first message per field. */
export function validateStep(step: StepKey, draft: OnboardingDraft, ctx: StepContext): StepErrors {
  const result = schemaFor(step, ctx).safeParse(draft);
  if (result.success) return {};
  const errors: StepErrors = {};
  for (const issue of result.error.issues) {
    const field = issue.path[0] as keyof OnboardingDraft | undefined;
    if (field && !errors[field]) errors[field] = issue.message;
  }
  return errors;
}

/** The request body for `PUT /me/onboarding`. */
export function toRequest(draft: OnboardingDraft, ctx: StepContext): OnboardingRequest {
  const idea = draft.idea.trim();
  return {
    locale: draft.locale,
    faculty_id: draft.faculty_id,
    department_id: draft.department_id,
    group_id: ctx.student ? (draft.group?.id ?? null) : null,
    study_level: ctx.student ? draft.study_level : null,
    course: ctx.student ? draft.course : null,
    interests: draft.interests,
    skills: draft.skills,
    has_idea: draft.has_idea === true,
    idea: draft.has_idea === true && idea ? idea : null,
  };
}

/** Course of a group admitted in `admissionYear`; the academic year starts in September. */
export function courseFromAdmission(admissionYear: number, today: Date): number {
  const academicYear = today.getMonth() >= 8 ? today.getFullYear() : today.getFullYear() - 1;
  return Math.min(7, Math.max(1, academicYear - admissionYear + 1));
}
