import type { QueryClient } from '@tanstack/react-query';

import { projectQuery } from '@/features/projects/api';

import { stepQuery, stepsQuery, templateQuery } from './api';

/**
 * A step loaded by the route this long ago is the latest version to start
 * editing from; anything older is fetched again first.
 */
export const STEP_FRESH_MS = 5_000;

/**
 * What every page of the project workspace shows, requested together by the
 * route loader instead of one after another by the components. Failures are
 * left to the page, which explains them.
 */
export function prefetchWorkspace(client: QueryClient, projectId: string): Promise<unknown> {
  return Promise.all([
    client.query(templateQuery).catch(() => undefined),
    client.query(projectQuery(projectId)).catch(() => undefined),
    client.query(stepsQuery(projectId)).catch(() => undefined),
  ]);
}

/** The workspace and the step itself, so the step opens without a skeleton. */
export function prefetchStepPage(
  client: QueryClient,
  projectId: string,
  stepKey: string,
): Promise<unknown> {
  return Promise.all([
    prefetchWorkspace(client, projectId),
    client
      .query({ ...stepQuery(projectId, stepKey), staleTime: STEP_FRESH_MS })
      .catch(() => undefined),
  ]);
}
