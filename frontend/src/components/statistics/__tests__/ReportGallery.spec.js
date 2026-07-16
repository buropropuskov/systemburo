import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import ReportGallery from '../ReportGallery.vue';
import { REPORT_PRESETS } from '../reportPresets';

const FULL_CATALOG = {
  metrics: [
    { key: 'applications_count', dimensions: ['status', 'period'] },
    { key: 'car_entries_count', dimensions: ['unload_place', 'period'] },
    { key: 'people_entries_count', dimensions: ['period'] },
    // Метрики обработки заявок (#1240) — чтобы новые пресеты галереи считались
    // доступными (разрезы из каталога бэка, см. reportPresets.spec.js).
    { key: 'avg_approval_time', dimensions: ['organization', 'period'] },
    { key: 'avg_processing_time', dimensions: ['organization', 'period'] },
    { key: 'refusal_rate', dimensions: ['organization', 'period'] },
    { key: 'avg_approver_response_time', dimensions: ['by_approver', 'period'] },
  ],
  list_entities: [{ key: 'work_applications' }, { key: 'cars' }],
};

describe('ReportGallery', () => {
  it('рендерит карточки только доступных пресетов', () => {
    const wrapper = mount(ReportGallery, { props: { catalog: FULL_CATALOG } });
    // Все пресеты валидны против FULL_CATALOG.
    expect(wrapper.findAll('.gallery__card')).toHaveLength(REPORT_PRESETS.length);
  });

  it('скрывает пресеты, чья сущность/метрика отсутствует в каталоге', () => {
    // Каталог без list-сущностей -> list-пресеты (Проведение работ, Машины по местам) выпадают.
    const aggregateOnly = { metrics: FULL_CATALOG.metrics, list_entities: [] };
    const wrapper = mount(ReportGallery, { props: { catalog: aggregateOnly } });
    const listPresets = REPORT_PRESETS.filter((p) => p.form.mode === 'list').length;
    expect(wrapper.findAll('.gallery__card')).toHaveLength(REPORT_PRESETS.length - listPresets);
  });

  it('эмитит apply с пресетом по клику на карточку', async () => {
    const wrapper = mount(ReportGallery, { props: { catalog: FULL_CATALOG } });
    await wrapper.findAll('.gallery__card')[0].trigger('click');
    const applied = wrapper.emitted('apply')[0][0];
    expect(applied.id).toBe(REPORT_PRESETS[0].id);
    expect(applied.form).toBeDefined();
  });

  it('без каталога не рендерит карточки', () => {
    const wrapper = mount(ReportGallery, { props: { catalog: null } });
    expect(wrapper.findAll('.gallery__card')).toHaveLength(0);
  });
});
