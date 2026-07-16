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
    // Метрики обработки заявок (#1240): длительности этапов и quality — разрезы
    // взяты 1:1 из report_duration_metrics.go/report_quality_metrics.go/
    // report_approver_metrics.go (длительностям и долям НЕ дан attachment_type,
    // by_approver — только метрикам согласующих).
    { key: 'avg_approval_time', dimensions: ['status', 'organization', 'company', 'period'] },
    { key: 'avg_processing_time', dimensions: ['status', 'organization', 'company', 'period'] },
    { key: 'refusal_rate', dimensions: ['organization', 'company', 'period'] },
    { key: 'avg_approver_response_time', dimensions: ['by_approver', 'status', 'organization', 'company', 'period'] },
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

  it('пресеты обработки заявок (#1240) построены на метриках длительностей/quality', () => {
    const byId = Object.fromEntries(REPORT_PRESETS.map((p) => [p.id, p]));
    const processing = [
      ['approval_time_trend', 'avg_approval_time', 'period'],
      ['processing_time_by_org', 'avg_processing_time', 'organization'],
      ['refusal_rate_trend', 'refusal_rate', 'period'],
      ['approver_response_time', 'avg_approver_response_time', 'by_approver'],
    ];
    for (const [id, metric, dimension] of processing) {
      const preset = byId[id];
      expect(preset, `пресет ${id}`).toBeTruthy();
      expect(preset.form.metric).toBe(metric);
      expect(preset.form.dimension).toBe(dimension);
      expect(presetAvailable(preset, CATALOG), `доступность ${id}`).toBe(true);
    }
  });
});
