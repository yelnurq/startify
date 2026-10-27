import { render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { radarPoint, radarPolygon, RadarChart, wrapLabel } from './radar-chart';

describe('radar geometry', () => {
  it('starts at the top and goes clockwise', () => {
    const [x0, y0] = radarPoint(0, 4, 10, 10, 100, 150);
    expect(x0).toBeCloseTo(150);
    expect(y0).toBeCloseTo(50);
    const [x1, y1] = radarPoint(1, 4, 10, 10, 100, 150);
    expect(x1).toBeCloseTo(250);
    expect(y1).toBeCloseTo(150);
  });

  it('scales and clamps values', () => {
    expect(radarPoint(2, 4, 5, 10, 100, 150)[1]).toBeCloseTo(200);
    expect(radarPoint(0, 4, 20, 10, 100, 150)[1]).toBeCloseTo(50);
    expect(radarPoint(0, 4, -3, 10, 100, 150)).toEqual([150, 150]);
  });

  it('builds polygon points', () => {
    expect(radarPolygon([10, 0, 10], 10, 10, 20).split(' ')).toHaveLength(3);
    expect(radarPolygon([10, 10, 10, 10], 10, 10, 20)).toBe(
      '20.00,10.00 30.00,20.00 20.00,30.00 10.00,20.00',
    );
  });

  it('wraps long labels into a few lines', () => {
    expect(wrapLabel('Проблема и её значимость')).toEqual(['Проблема и её', 'значимость']);
    const long = wrapLabel('Соответствие приоритетам РК и социальный эффект', 14, 2);
    expect(long).toHaveLength(2);
    expect(long[1]?.endsWith('…')).toBe(true);
    expect(wrapLabel('')).toEqual([]);
  });
});

describe('RadarChart', () => {
  it('names the chart and lists the values for screen readers', () => {
    render(
      <RadarChart
        label="Scores"
        axes={[
          { key: 'a', label: 'Problem' },
          { key: 'b', label: 'Market' },
          { key: 'c', label: 'Team' },
        ]}
        series={[
          { key: 'now', label: 'Now', values: [7, 5, 9] },
          { key: 'before', label: 'Before', values: [4, 5, 6], tone: 'muted' },
        ]}
      />,
    );
    expect(screen.getByRole('img', { name: 'Scores' })).toBeInTheDocument();
    const table = screen.getByRole('table', { name: 'Scores' });
    const row = within(table).getByRole('row', { name: /Problem/ });
    expect(
      within(row)
        .getAllByRole('cell')
        .map((c) => c.textContent),
    ).toEqual(['7', '4']);
  });

  it('draws nothing for fewer than three axes', () => {
    const { container } = render(
      <RadarChart
        label="x"
        axes={[{ key: 'a', label: 'A' }]}
        series={[{ key: 's', label: 's', values: [1] }]}
      />,
    );
    expect(container).toBeEmptyDOMElement();
  });
});
