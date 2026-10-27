import { describe, expect, it } from 'vitest';

import { z } from './zod';

describe('zod setup', () => {
  it('parses without compiled code, which the CSP forbids', () => {
    expect(z.config().jitless).toBe(true);
    const schema = z.object({ title: z.string().min(2) });
    expect(schema.safeParse({ title: 'ok' }).success).toBe(true);
    expect(schema.safeParse({ title: 'x' }).success).toBe(false);
  });
});
