import { describe, it, expect } from 'vitest';
import { REPORT_PRESETS, presetAvailable } from '../reportPresets';

// Каталог, повторяющий whitelist бэка (internal/services/report_catalog.go).
// Если бэк переименует ключ метрики/разреза/сущности, тест ниже это поймает.
const CATALOG = {
  metrics: [
    { key: 'applications_count', dimensions: ['status', 'organization', 'company', 'attachment_type', 'period'] },
    { key: 'car_entries_count', dimensions: ['period', 'hour_of_day', 'unload_place', 'organization'] },
    { key: 'people_entries_count', dimensions: ['period', 'hour_of_day', 'organization'] },
    { key: 'items_sum', dimensions: ['organization', 'company', 'period'] },
  ],
  list_entities: [
    { key: 'work_applications' },
    { key: 'applications' },
    { key: 'cars' },
    { key: 'people' },
  ],
};

describe('reportPresets', () => {
  it('каждый пресет ссылается на валидные ключи каталога', () => {
    for (const preset of REPORT_PRESETS) {
      expect(presetAvailable(preset, CATALOG), `пресет «${preset.title}»`).toBe(true);
    }
  });

  it('presetAvailable отбраковывает пресет с отсутствующей метрикой/сущностью', () => {
    expect(presetAvailable({ form: { mode: 'aggregate', metric: 'нет', dimension: 'status' } }, CATALOG)).toBe(false);
    expect(presetAvailable({ form: { mode: 'list', entity: 'нет' } }, CATALOG)).toBe(false);
    expect(presetAvailable(REPORT_PRESETS[0], null)).toBe(false);
  });

  it('presetAvailable отбраковывает разрез, не поддерживаемый метрикой', () => {
    // applications_count не умеет unload_place.
    expect(presetAvailable(
      { form: { mode: 'aggregate', metric: 'applications_count', dimension: 'unload_place' } },
      CATALOG,
    )).toBe(false);
  });
});
