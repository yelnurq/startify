import { CircleCheck } from 'lucide-react';
import { useId, useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Button,
  Callout,
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Field,
  Illustration,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Textarea,
} from '@/design-system';
import { ApiError } from '@/shared/api/client';
import { errorMessage, fieldErrorMessage } from '@/shared/api/errors';

import { useContactTeam, type ContactCreate } from './api';
import { CONTACT_KINDS, type ContactKind } from './logic';

const LIMITS = { name: [2, 120], organization: 160, message: [20, 2000] } as const;
const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

interface Draft {
  name: string;
  email: string;
  organization: string;
  kind: ContactKind | '';
  message: string;
  /** The trap for bots: people never see or fill it. */
  website: string;
}

const EMPTY: Draft = { name: '', email: '', organization: '', kind: '', message: '', website: '' };

type Errors = Partial<Record<keyof Draft, string>>;

interface ContactDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  slug: string;
  title: string;
}

/** «Связаться с командой»: a partner's request, moderated by the university or sent straight on. */
export function ContactDialog({ open, onOpenChange, slug, title }: ContactDialogProps) {
  const { t } = useTranslation(['showcase', 'common']);
  const contact = useContactTeam(slug);
  const [draft, setDraft] = useState<Draft>(EMPTY);
  const [errors, setErrors] = useState<Errors>({});
  const ids = useId();

  const set = <K extends keyof Draft>(key: K, value: Draft[K]) => {
    setDraft((d) => ({ ...d, [key]: value }));
    setErrors((e) => ({ ...e, [key]: undefined }));
  };

  const validate = (): Errors => {
    const out: Errors = {};
    const name = draft.name.trim();
    if (name.length === 0) out.name = t('common:validation.required');
    else if (name.length < LIMITS.name[0])
      out.name = t('common:validation.minLength', { count: LIMITS.name[0] });
    if (!draft.email.trim()) out.email = t('common:validation.required');
    else if (!EMAIL.test(draft.email.trim())) out.email = t('common:validation.email');
    if (!draft.kind) out.kind = t('common:validation.choose');
    const message = draft.message.trim();
    if (message.length === 0) out.message = t('common:validation.required');
    else if (message.length < LIMITS.message[0])
      out.message = t('common:validation.minLength', { count: LIMITS.message[0] });
    else if (message.length > LIMITS.message[1])
      out.message = t('common:validation.maxLength', { count: LIMITS.message[1] });
    return out;
  };

  const submit = () => {
    const found = validate();
    setErrors(found);
    if (Object.keys(found).length > 0 || !draft.kind) return;
    const body: ContactCreate = {
      name: draft.name.trim(),
      email: draft.email.trim(),
      kind: draft.kind,
      message: draft.message.trim(),
      ...(draft.organization.trim() ? { organization: draft.organization.trim() } : {}),
      ...(draft.website ? { website: draft.website } : {}),
    };
    contact.mutate(body, {
      onError: (err) => {
        if (err instanceof ApiError && err.fieldErrors.length > 0) {
          const fields: Errors = {};
          for (const fe of err.fieldErrors) {
            if (fe.field in EMPTY) fields[fe.field as keyof Draft] = fieldErrorMessage(fe);
          }
          setErrors(fields);
        }
      },
    });
  };

  const close = (next: boolean) => {
    onOpenChange(next);
    if (!next && contact.isSuccess) {
      // A new request starts from a clean form.
      setDraft(EMPTY);
      contact.reset();
    }
  };

  const general =
    contact.error && !(contact.error instanceof ApiError && contact.error.fieldErrors.length > 0)
      ? errorMessage(contact.error)
      : undefined;

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent size="lg">
        {contact.isSuccess ? (
          <>
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2">
                <CircleCheck className="size-5 text-success" aria-hidden />
                {t('contact.success.title')}
              </DialogTitle>
              <DialogDescription>
                {contact.data.status === 'forwarded'
                  ? t('contact.success.forwarded')
                  : t('contact.success.pending')}
              </DialogDescription>
            </DialogHeader>
            <Illustration name="partners" className="mx-auto mt-2 w-52" />
            <DialogFooter className="mt-6">
              <Button
                onClick={() => {
                  close(false);
                }}
              >
                {t('contact.success.close')}
              </Button>
            </DialogFooter>
          </>
        ) : (
          <form
            noValidate
            className="flex min-h-0 flex-col"
            onSubmit={(event) => {
              event.preventDefault();
              submit();
            }}
          >
            <DialogHeader>
              <DialogTitle>{t('contact.title')}</DialogTitle>
              <DialogDescription>{t('contact.description', { title })}</DialogDescription>
            </DialogHeader>
            <DialogBody className="flex flex-col gap-4">
              {general && <Callout tone="danger">{general}</Callout>}
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <Field label={t('contact.name')} required error={errors.name}>
                  <Input
                    value={draft.name}
                    maxLength={LIMITS.name[1]}
                    autoComplete="name"
                    onChange={(e) => {
                      set('name', e.target.value);
                    }}
                  />
                </Field>
                <Field label={t('contact.email')} required error={errors.email}>
                  <Input
                    type="email"
                    value={draft.email}
                    maxLength={254}
                    autoComplete="email"
                    inputMode="email"
                    onChange={(e) => {
                      set('email', e.target.value);
                    }}
                  />
                </Field>
                <Field label={t('contact.organization')} optional error={errors.organization}>
                  <Input
                    value={draft.organization}
                    maxLength={LIMITS.organization}
                    autoComplete="organization"
                    onChange={(e) => {
                      set('organization', e.target.value);
                    }}
                  />
                </Field>
                <Field label={t('contact.kind')} required error={errors.kind}>
                  <Select
                    value={draft.kind}
                    onValueChange={(value) => {
                      set('kind', value as ContactKind);
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder={t('contact.kindPlaceholder')} />
                    </SelectTrigger>
                    <SelectContent>
                      {CONTACT_KINDS.map((kind) => (
                        <SelectItem key={kind} value={kind}>
                          {t(`contactKinds.${kind}`)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </Field>
              </div>
              <Field
                label={t('contact.message')}
                required
                hint={t('contact.messageHint')}
                error={errors.message}
                counter={{ value: draft.message.length, max: LIMITS.message[1] }}
              >
                <Textarea
                  rows={5}
                  value={draft.message}
                  onChange={(e) => {
                    set('message', e.target.value);
                  }}
                />
              </Field>
              {/* A trap for bots: off screen, out of the tab order and hidden from assistive technology. */}
              <div aria-hidden="true" className="absolute -left-[9999px] size-px overflow-hidden">
                <label htmlFor={`${ids}-website`}>{t('contact.website')}</label>
                <input
                  id={`${ids}-website`}
                  name="website"
                  type="text"
                  tabIndex={-1}
                  autoComplete="off"
                  value={draft.website}
                  onChange={(e) => {
                    set('website', e.target.value);
                  }}
                />
              </div>
              <p className="text-xs text-fg-subtle">{t('contact.consent')}</p>
            </DialogBody>
            <DialogFooter>
              <Button
                type="button"
                variant="secondary"
                onClick={() => {
                  close(false);
                }}
              >
                {t('common:actions.cancel')}
              </Button>
              <Button type="submit" loading={contact.isPending}>
                {t('contact.submit')}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
