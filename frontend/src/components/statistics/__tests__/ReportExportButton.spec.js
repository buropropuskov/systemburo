import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import ReportExportButton from '../ReportExportButton.vue';

describe('ReportExportButton', () => {
  it('меню скрыто до клика по триггеру', () => {
    const w = mount(ReportExportButton);
    expect(w.find('[data-testid="rr-export-excel"]').exists()).toBe(false);
    expect(w.find('[data-testid="rr-export-pdf"]').exists()).toBe(false);
  });

  it('клик по триггеру открывает меню с обоими форматами', async () => {
    const w = mount(ReportExportButton);
    await w.find('[data-testid="rr-export"]').trigger('click');
    expect(w.find('[data-testid="rr-export-excel"]').exists()).toBe(true);
    expect(w.find('[data-testid="rr-export-pdf"]').exists()).toBe(true);
  });

  it('выбор Excel эмитит export со значением excel и закрывает меню', async () => {
    const w = mount(ReportExportButton);
    await w.find('[data-testid="rr-export"]').trigger('click');
    await w.find('[data-testid="rr-export-excel"]').trigger('click');
    expect(w.emitted('export')[0]).toEqual(['excel']);
    expect(w.find('[data-testid="rr-export-excel"]').exists()).toBe(false);
  });

  it('выбор PDF эмитит export со значением pdf', async () => {
    const w = mount(ReportExportButton);
    await w.find('[data-testid="rr-export"]').trigger('click');
    await w.find('[data-testid="rr-export-pdf"]').trigger('click');
    expect(w.emitted('export')[0]).toEqual(['pdf']);
  });

  it('триггер недоступен в состоянии disabled и exporting', () => {
    const disabled = mount(ReportExportButton, { props: { disabled: true } });
    expect(disabled.find('[data-testid="rr-export"]').attributes('disabled')).toBeDefined();
    const busy = mount(ReportExportButton, { props: { exporting: true } });
    expect(busy.find('[data-testid="rr-export"]').attributes('disabled')).toBeDefined();
    expect(busy.find('[data-testid="rr-export"]').text()).toContain('Готовим');
  });
});
