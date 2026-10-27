import { Link } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, Check, Eye, EyeOff, Layers, Mail, MailCheck } from 'lucide-react';
import {
  useRef,
  useState,
  type ClipboardEvent,
  type ComponentProps,
  type KeyboardEvent,
  type ReactNode,
} from 'react';
import { useTranslation } from 'react-i18next';

import { AuthLayout } from '@/app/layouts/auth-layout';
import {
  Button,
  Callout,
  cn,
  Illustration,
  type IllustrationName,
  Input,
  Kicker,
  Logo,
  toast,
} from '@/design-system';
import { errorMessage } from '@/shared/api/errors';
import { useLocalized } from '@/shared/i18n/localized';

import { tenantQuery, useResendVerification } from './api';
import { passwordScore } from './password';

function AuthAside() {
  const { t } = useTranslation('auth');
  const tenant = useQuery(tenantQuery);
  const text = useLocalized();
  return (
    <div className="flex h-full flex-col gap-10 text-white">
      <Link to="/" className="w-fit">
        <Logo
          inverted
          caption={tenant.data ? text(tenant.data.name) : undefined}
          logoUrl={tenant.data?.logo_url}
        />
      </Link>
      <div className="my-auto flex max-w-lg flex-col gap-6">
        <div className="w-fit rounded-card bg-surface p-3 shadow-lg">
          <Illustration name="rocket" className="w-44" />
        </div>
        <Kicker tone="light">{t('aside.cardEyebrow')}</Kicker>
        <h2 className="font-display text-5xl leading-tight font-extrabold">
          {t('aside.headline')}
        </h2>
        <p className="flex items-center gap-2 text-white/75">
          <Layers className="size-4 shrink-0 text-primary-300" aria-hidden />
          {t('aside.cardText')}
        </p>
      </div>
      <ul className="flex max-w-lg flex-col gap-4 border-t border-white/15 pt-8 text-sm leading-relaxed text-white/85">
        {(['point1', 'point2', 'point3'] as const).map((key) => (
          <li key={key} className="flex items-start gap-3">
            <Check className="mt-0.5 size-4 shrink-0 text-primary-300" aria-hidden />
            {t(`aside.${key}`)}
          </li>
        ))}
      </ul>
    </div>
  );
}

/** The logo of the top bar on phones: the university's seal when it has one. */
function TenantLogo() {
  const tenant = useQuery(tenantQuery);
  return <Logo logoUrl={tenant.data?.logo_url} />;
}

function BackHome() {
  const { t } = useTranslation();
  return (
    <Link
      to="/"
      className="inline-flex items-center gap-2 text-sm font-semibold text-ink transition-colors hover:text-primary-text"
    >
      <ArrowLeft className="size-4" aria-hidden />
      {t('actions.toHome')}
    </Link>
  );
}

/** Frame of every sign-in related screen. */
export function AuthPage({
  title,
  subtitle,
  art,
  children,
}: {
  title: ReactNode;
  subtitle?: ReactNode;
  /** The drawing in the corner of the form side. */
  art?: IllustrationName;
  children: ReactNode;
}) {
  return (
    <AuthLayout
      logo={
        <Link to="/">
          <TenantLogo />
        </Link>
      }
      back={<BackHome />}
      aside={<AuthAside />}
      art={art}
    >
      <div className="flex flex-col gap-8">
        <div className="flex flex-col gap-2">
          <h1 className="font-display text-4xl font-extrabold text-ink">{title}</h1>
          {subtitle && <p className="text-fg-muted">{subtitle}</p>}
        </div>
        {children}
      </div>
    </AuthLayout>
  );
}

export function FormAlert({ message }: { message: string | null | undefined }) {
  if (!message) return null;
  return <Callout tone="danger">{message}</Callout>;
}

/** Sends the verification letter to the address again; confirms once sent. */
export function ResendVerificationButton({ email }: { email: string }) {
  const { t } = useTranslation('auth');
  const resend = useResendVerification();
  return (
    <Button
      size="sm"
      variant="secondary"
      leftIcon={resend.isSuccess ? <MailCheck /> : <Mail />}
      loading={resend.isPending}
      disabled={resend.isSuccess}
      onClick={() => {
        resend.mutate(email, {
          onError: (err) => {
            toast.error(errorMessage(err));
          },
        });
      }}
    >
      {resend.isSuccess ? t('verify.resent') : t('verify.resend')}
    </Button>
  );
}

type PasswordInputProps = Omit<ComponentProps<typeof Input>, 'type' | 'trailing'>;

export function PasswordInput(props: PasswordInputProps) {
  const { t } = useTranslation();
  const [visible, setVisible] = useState(false);
  return (
    <Input
      {...props}
      type={visible ? 'text' : 'password'}
      trailing={
        <button
          type="button"
          aria-label={visible ? t('actions.hidePassword') : t('actions.showPassword')}
          aria-pressed={visible}
          className="grid size-8 place-items-center rounded-md text-fg-subtle transition-colors hover:text-fg"
          onClick={() => {
            setVisible((v) => !v);
          }}
        >
          {visible ? <EyeOff /> : <Eye />}
        </button>
      }
    />
  );
}

const STRENGTH = ['weak', 'weak', 'fair', 'good', 'strong'] as const;
const STRENGTH_COLOR = ['bg-danger', 'bg-danger', 'bg-warning', 'bg-primary', 'bg-success'];

export function PasswordStrength({ password, id }: { password: string; id?: string }) {
  const { t } = useTranslation('auth');
  const score = passwordScore(password);
  return (
    <div id={id} className="flex flex-col gap-1.5" aria-live="polite">
      <div className="grid grid-cols-4 gap-1" aria-hidden>
        {[1, 2, 3, 4].map((step) => (
          <span
            key={step}
            className={cn(
              'h-1 rounded-full transition-colors duration-300',
              score >= step ? STRENGTH_COLOR[score] : 'bg-surface-sunken',
            )}
          />
        ))}
      </div>
      <p className="text-xs text-fg-subtle">
        {password.length > 0
          ? `${t('password.strength.label')}: ${t(`password.strength.${STRENGTH[score] ?? 'weak'}`)}`
          : t('password.hint')}
      </p>
    </div>
  );
}

const CODE_LENGTH = 6;

/**
 * Six boxes for the e-mailed code: typing advances, Backspace goes back,
 * pasting or browser autofill fills every box at once.
 */
export function CodeInput({
  value,
  onChange,
  onComplete,
  disabled,
  invalid,
  labelledBy,
}: {
  value: string;
  onChange: (value: string) => void;
  onComplete: (value: string) => void;
  disabled?: boolean;
  invalid?: boolean;
  labelledBy: string;
}) {
  const { t } = useTranslation('auth');
  const refs = useRef<(HTMLInputElement | null)[]>([]);
  const digits = Array.from({ length: CODE_LENGTH }, (_, i) => value[i] ?? '');

  const setDigits = (next: string, focusIndex: number) => {
    const clean = next.replace(/\D/g, '').slice(0, CODE_LENGTH);
    onChange(clean);
    refs.current[Math.min(focusIndex, CODE_LENGTH - 1)]?.focus();
    if (clean.length === CODE_LENGTH) onComplete(clean);
  };

  const handleInput = (index: number, raw: string) => {
    const typed = raw.replace(/\D/g, '');
    if (typed.length > 1) {
      // Autofill or a fast typist: spread the digits from this box onwards.
      setDigits(value.slice(0, index) + typed, index + typed.length);
      return;
    }
    const chars = digits.slice();
    chars[index] = typed;
    setDigits(chars.join(''), typed ? index + 1 : index);
  };

  const handleKey = (index: number, event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Backspace' && !digits[index] && index > 0) {
      event.preventDefault();
      const chars = digits.slice();
      chars[index - 1] = '';
      setDigits(chars.join(''), index - 1);
    } else if (event.key === 'ArrowLeft' && index > 0) {
      refs.current[index - 1]?.focus();
    } else if (event.key === 'ArrowRight' && index < CODE_LENGTH - 1) {
      refs.current[index + 1]?.focus();
    }
  };

  const handlePaste = (event: ClipboardEvent<HTMLInputElement>) => {
    event.preventDefault();
    setDigits(event.clipboardData.getData('text'), CODE_LENGTH - 1);
  };

  return (
    <div role="group" aria-labelledby={labelledBy} className="flex justify-between gap-2">
      {digits.map((digit, index) => (
        <input
          key={index}
          ref={(el) => {
            refs.current[index] = el;
          }}
          value={digit}
          inputMode="numeric"
          autoComplete={index === 0 ? 'one-time-code' : 'off'}
          maxLength={index === 0 ? CODE_LENGTH : 1}
          disabled={disabled}
          aria-label={t('code.digit', { n: index + 1 })}
          aria-invalid={invalid ? true : undefined}
          onChange={(e) => {
            handleInput(index, e.target.value);
          }}
          onKeyDown={(e) => {
            handleKey(index, e);
          }}
          onPaste={handlePaste}
          onFocus={(e) => {
            e.target.select();
          }}
          className={cn(
            'h-14 w-full min-w-0 rounded-lg border border-border bg-surface text-center font-figures text-2xl font-bold text-ink tabular-nums shadow-sm',
            'transition-[border-color,box-shadow] focus-visible:border-primary-400 focus-visible:ring-4 focus-visible:ring-primary-500/15 focus-visible:outline-none',
            'disabled:opacity-60 aria-invalid:border-danger',
          )}
        />
      ))}
    </div>
  );
}

export function Divider({ label }: { label: string }) {
  return (
    <div className="flex items-center gap-3 text-xs text-fg-subtle">
      <span className="h-px flex-1 bg-border" />
      {label}
      <span className="h-px flex-1 bg-border" />
    </div>
  );
}
