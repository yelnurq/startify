import { describe, expect, it } from 'vitest';

import { ApiError, NetworkError } from './client';
import { errorMessage } from './errors';

function problem(code: string, status: number, params?: Record<string, unknown>) {
  return new ApiError({
    type: 'about:blank',
    title: 'Error',
    status,
    code,
    ...(params ? { params } : {}),
  });
}

describe('errorMessage', () => {
  it('translates known codes with their parameters', () => {
    expect(errorMessage(problem('ai.limit_reached', 429, { resets_at: '00:00' }))).toBe(
      'Лимит AI на сегодня исчерпан, он сбросится в 00:00.',
    );
    expect(errorMessage(problem('request.invalid_limit', 400, { min: 1, max: 100 }))).toBe(
      'Число записей на странице должно быть от 1 до 100.',
    );
    expect(errorMessage(problem('import.too_many_columns', 422, { max: 50 }))).toBe(
      'Слишком много столбцов: не более 50.',
    );
  });

  it('picks the variant named by the scope parameter', () => {
    expect(
      errorMessage(problem('ai.limit_reached', 429, { scope: 'university', resets_at: '00:00' })),
    ).toBe('Лимит AI университета на сегодня исчерпан, он сбросится в 00:00.');
    expect(
      errorMessage(problem('ai.limit_reached', 429, { scope: 'user', resets_at: '00:00' })),
    ).toBe('Ваш лимит AI на сегодня исчерпан, он сбросится в 00:00.');
    expect(errorMessage(problem('knowledge.not_found', 404, { scope: 'other' }))).toBe(
      'Документ не найден.',
    );
  });

  it('explains an outdated page cursor of a list', () => {
    expect(errorMessage(problem('request.invalid_cursor', 400))).toBe(
      'Не удалось показать следующую часть списка: он устарел. Обновите страницу.',
    );
  });

  it('falls back by status and for network failures', () => {
    expect(errorMessage(problem('ai.something_new', 429))).toBe(
      'Слишком много запросов. Подождите немного и повторите.',
    );
    expect(errorMessage(new NetworkError(new Error('down')))).toBe(
      'Нет соединения с сервером. Проверьте подключение к интернету.',
    );
    expect(errorMessage(undefined)).toBe('Что-то пошло не так. Попробуйте ещё раз.');
  });
});
