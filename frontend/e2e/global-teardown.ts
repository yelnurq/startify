import { execFileSync } from 'node:child_process';
import path from 'node:path';

/**
 * Removes what the run created from the database of the development stack:
 * the accounts with e2e.*@qadam.test addresses (support/accounts.ts) with
 * their projects, the requests for teammates and knowledge documents titled
 * «E2E …» and the contact requests of e2e.* senders. It runs `seed clean-e2e`
 * in the worker container, as `make e2e-clean` does; the command refuses a
 * production configuration.
 *
 * E2E_CLEANUP=off keeps the data (to look at it after a failure);
 * E2E_CLEANUP_COMMAND replaces the command, e.g. for a stack started
 * differently. Without Docker or with an older image it only warns.
 */
export default function globalTeardown() {
  if (process.env.E2E_CLEANUP === 'off') return;
  const root = path.join(import.meta.dirname, '..', '..');
  const command = process.env.E2E_CLEANUP_COMMAND?.split(' ').filter(Boolean) ?? [
    'docker',
    'compose',
    '--env-file',
    path.join(root, '.env'),
    '-f',
    path.join(root, 'deploy', 'docker-compose.yml'),
    '-f',
    path.join(root, 'deploy', 'docker-compose.dev.yml'),
    'run',
    '--rm',
    '--no-deps',
    'worker',
    '/app/seed',
    'clean-e2e',
  ];
  const [program, ...args] = command;
  if (!program) return;
  try {
    execFileSync(program, args, { stdio: 'inherit', timeout: 120_000 });
  } catch (error) {
    console.warn(`The data of the run was not cleaned up (${String(error)}); run make e2e-clean.`);
  }
}
