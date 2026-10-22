import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { initials } from './avatar';
import { Button } from './button';
import { EdgeIllustration } from './edge-illustration';
import { Field } from './field';
import { Input } from './input';
import { Textarea } from './textarea';

describe('Field', () => {
  it('labels the control and links hint and counter', () => {
    render(
      <Field
        label="Название проекта"
        hint="Короткое и запоминающееся"
        counter={{ value: 9, max: 40 }}
        required
      >
        <Input defaultValue="AgroSense" />
      </Field>,
    );
    const input = screen.getByLabelText(/Название проекта/);
    expect(input).toBeRequired();
    expect(input).toHaveAccessibleDescription(/Короткое и запоминающееся.*9 \/ 40/);
    expect(input).not.toHaveAttribute('aria-invalid');
  });

  it('marks the control invalid and announces the error instead of the hint', () => {
    render(
      <Field label="Электронная почта" hint="Рабочая почта" error="Укажите адрес полностью">
        <Input type="email" />
      </Field>,
    );
    const input = screen.getByLabelText('Электронная почта');
    expect(input).toHaveAttribute('aria-invalid', 'true');
    expect(input).toHaveAccessibleDescription('Укажите адрес полностью');
    expect(screen.queryByText('Рабочая почта')).not.toBeInTheDocument();
  });

  it('flags text over the counter limit', () => {
    render(
      <Field label="Проблема" counter={{ value: 610, max: 600 }}>
        <Textarea />
      </Field>,
    );
    expect(screen.getByLabelText('Проблема')).toHaveAttribute('aria-invalid', 'true');
  });

  it('shows the optional marker in the interface language', () => {
    render(
      <Field label="Сайт" optional>
        <Input />
      </Field>,
    );
    expect(screen.getByText('необязательно')).toBeInTheDocument();
  });
});

describe('Button', () => {
  it('is a non-submitting button by default', () => {
    render(<Button>Сохранить</Button>);
    expect(screen.getByRole('button', { name: 'Сохранить' })).toHaveAttribute('type', 'button');
  });

  it('disables itself and reports busy while loading', async () => {
    const onClick = vi.fn();
    render(
      <Button loading onClick={onClick}>
        Сохраняем
      </Button>,
    );
    const button = screen.getByRole('button', { name: 'Сохраняем' });
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute('aria-busy', 'true');
    await userEvent.click(button);
    expect(onClick).not.toHaveBeenCalled();
  });

  it('renders a link with button styles via asChild', () => {
    render(
      <Button asChild variant="secondary">
        <a href="/design">Дизайн-система</a>
      </Button>,
    );
    const link = screen.getByRole('link', { name: 'Дизайн-система' });
    expect(link).toHaveAttribute('href', '/design');
    expect(link.className).toContain('border-border');
  });
});

describe('initials', () => {
  it('takes the first letters of the first and last names', () => {
    expect(initials('Айгерім Сапарова')).toBe('АС');
    expect(initials('Нұрлан Әбдірахманов')).toBe('НӘ');
    expect(initials('  Мадина  ')).toBe('М');
    expect(initials('Ерасыл Серікұлы Қуанышев')).toBe('ЕҚ');
  });
});

describe('EdgeIllustration', () => {
  it('is decoration out of the way of the content', () => {
    const { container } = render(
      <EdgeIllustration name="rocket" placement="bottom-right" className="w-64" />,
    );
    const art = container.querySelector('[data-slot="edge-illustration"]');
    expect(art).toHaveAttribute('aria-hidden', 'true');
    expect(art).toHaveClass('pointer-events-none', 'absolute', 'right-0', 'bottom-0', 'w-64');
    expect(art).not.toHaveClass('w-56');
    expect(art?.querySelector('svg.illustration')).not.toBeNull();
  });
});
