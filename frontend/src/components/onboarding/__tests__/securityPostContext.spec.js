import { describe, expect, it } from 'vitest';
import { buildSecurityFactSteps, resolveSecurityPostContext } from '../securityFactSteps';
import { groupStepsBySection, tourChapters } from '../stepsFlow';

describe('security post capability', () => {
  it('keeps ordinary passage instruction when the Fact block is disabled', () => {
    expect(resolveSecurityPostContext([{ table: { name: 'north', is_active: true, table_type: 'cars', show_fact_table: false } }], () => true))
      .toEqual({ route: '/table/north', tableType: 'cars', showFactTable: false });
  });
  it('selects an allowed people post without pretending it has the car Fact view', () => {
    const tables = [
      { name: 'cars', is_active: true, table_type: 'cars', show_fact_table: true },
      { name: 'people', is_active: true, table_type: 'people', show_fact_table: false },
    ];
    expect(resolveSecurityPostContext(tables, name => name === 'people'))
      .toEqual({ route: '/table/people', tableType: 'people', showFactTable: false });
    expect(resolveSecurityPostContext(tables, () => false)).toBeNull();
  });

  it('always retains one required Fact explanation on an allowed page without a live post', () => {
    const steps = buildSecurityFactSteps(null);
    expect(steps.map(step => step.id)).toEqual(['sec-fact-intro']);
    expect(steps[0]).toMatchObject({ route: '/news', element: null, demo: 'factTable', sectionTitle: 'По факту' });
    expect(steps[0].optional).not.toBe(true);
    expect(steps[0].dataOwner).toBeUndefined();
  });

  it('retains ordinary car marks when the live Fact block is disabled', () => {
    const steps = buildSecurityFactSteps('/table/north', { tableType: 'cars', showFactTable: false });
    expect(steps.find(step => step.id === 'sec-pass-entry').route).toBe('/table/north');
    expect(steps.find(step => step.id === 'sec-fact-intro').route).toBe('/news');
    expect(steps.filter(step => step.id === 'sec-fact-intro')).toHaveLength(1);
  });

  it('uses people controls and keeps the car example separate', () => {
    const steps = buildSecurityFactSteps('/table/staff', { tableType: 'people', showFactTable: true });
    expect(steps.find(step => step.id === 'sec-pass-intro').element).toBe('[data-testid="people-table"]');
    expect(steps.find(step => step.id === 'sec-pass-entry')).toMatchObject({ title: 'Отметка входа' });
    expect(steps.some(step => step.id === 'sec-on-territory')).toBe(false);
    expect(steps.find(step => step.id === 'sec-fact-intro')).toMatchObject({ route: '/news', element: null });
  });

  it.each([undefined, 'items'])('does not invent car controls for an unknown descriptor type %s', tableType => {
    const steps = buildSecurityFactSteps('/table/legacy', { tableType, showFactTable: true });
    expect(steps.map(step => step.id)).toEqual(['sec-fact-intro']);
    expect(steps[0]).toMatchObject({ route: '/news', element: null, demo: 'factTable' });
    expect(steps[0].dataOwner).toBeUndefined();
  });

  it('keeps live Fact required and labels the example as its own learning section', () => {
    const live = buildSecurityFactSteps('/table/north').find(step => step.id === 'sec-fact-intro');
    expect(live).toMatchObject({ route: '/table/north', dataOwner: 'fact-list', element: '[data-testid="fact-table"]' });
    expect(live.optional).not.toBe(true);
    const example = [...buildSecurityFactSteps(null), { id: 'finish', route: '/news', title: 'Готово' }];
    expect(groupStepsBySection(example).map(group => group.title)).toEqual(['По факту', 'Обзор и новости']);
    expect(tourChapters(example).map(chapter => chapter.title)).toEqual(['По факту', 'Обзор и новости']);
  });
});
