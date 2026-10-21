import { beforeAll, describe, expect, it } from 'vitest';

import { i18n } from '@/shared/i18n';

import {
  courseFromAdmission,
  toRequest,
  validateStep,
  type OnboardingDraft,
  type StepContext,
} from './steps';

beforeAll(async () => {
  await i18n.loadNamespaces('onboarding');
});

const empty: OnboardingDraft = {
  locale: 'ru',
  faculty_id: null,
  department_id: null,
  group: null,
  study_level: null,
  course: null,
  interests: [],
  skills: [],
  has_idea: null,
  idea: '',
};

const student: StepContext = { student: true, hasFaculties: true };
const staff: StepContext = { student: false, hasFaculties: true };

describe('onboarding steps', () => {
  it('accepts the preselected language', () => {
    expect(validateStep('language', empty, student)).toEqual({});
  });

  it('asks students for the faculty, level and course', () => {
    expect(validateStep('study', empty, student)).toEqual({
      faculty_id: 'Выберите значение',
      study_level: 'Выберите значение',
      course: 'Выберите значение',
    });
    expect(
      validateStep(
        'study',
        { ...empty, faculty_id: 'f1', study_level: 'bachelor', course: 2 },
        student,
      ),
    ).toEqual({});
  });

  it('asks staff only for the faculty, and nobody when the structure is not imported yet', () => {
    expect(validateStep('study', empty, staff)).toEqual({ faculty_id: 'Выберите значение' });
    expect(validateStep('study', empty, { student: false, hasFaculties: false })).toEqual({});
  });

  it('needs at least one interest and respects the limits', () => {
    expect(validateStep('interests', empty, student)).toEqual({
      interests: 'Выберите хотя бы одно направление',
    });
    const many = Array.from({ length: 13 }, (_, i) => `Направление ${String(i + 1)}`);
    expect(validateStep('interests', { ...empty, interests: many }, student)).toEqual({
      interests: 'Не более 12 элементов',
    });
    expect(validateStep('interests', { ...empty, interests: ['EdTech'] }, student)).toEqual({});
  });

  it('wants an answer about the idea and a real sentence when there is one', () => {
    expect(validateStep('idea', empty, student)).toEqual({ has_idea: 'Выберите значение' });
    expect(validateStep('idea', { ...empty, has_idea: true, idea: 'Сервис' }, student)).toEqual({
      idea: 'Минимум 10 символов',
    });
    expect(validateStep('idea', { ...empty, has_idea: false }, student)).toEqual({});
  });

  it('builds the request without the student-only fields for staff', () => {
    const draft: OnboardingDraft = {
      ...empty,
      faculty_id: 'f1',
      group: { id: 'g1', code: 'ВТиПО-24-1' },
      study_level: 'bachelor',
      course: 3,
      interests: ['EdTech'],
      has_idea: false,
      idea: 'забытый черновик',
    };
    expect(toRequest(draft, student)).toMatchObject({
      group_id: 'g1',
      study_level: 'bachelor',
      course: 3,
      has_idea: false,
      idea: null,
    });
    expect(toRequest(draft, staff)).toMatchObject({
      faculty_id: 'f1',
      group_id: null,
      study_level: null,
      course: null,
    });
  });

  it('derives the course from the admission year; the year starts in September', () => {
    expect(courseFromAdmission(2023, new Date(2026, 9, 21))).toBe(4);
    expect(courseFromAdmission(2023, new Date(2026, 2, 1))).toBe(3);
    expect(courseFromAdmission(2026, new Date(2026, 8, 1))).toBe(1);
  });
});
