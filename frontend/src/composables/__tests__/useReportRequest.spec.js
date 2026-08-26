import { describe, it, expect } from 'vitest';
import {
  buildReportRequest, defaultReportLimit, DEFAULT_REPORT_LIMIT, MAX_REPORT_LIMIT,
} from '../useReportRequest';

const PERIOD = { from: '2026-06-01', to: '2026-06-07' };

describe('buildReportRequest', () => {
  describe('aggregate', () => {
    it('собирает метрику+разрез и период фильтром date_range', () => {
      const req = buildReportRequest(
        { mode: 'aggregate', metric: 'car_entries_count', dimension: 'unload_place' },
        PERIOD,
        ['date_range'],
      );
      expect(req.mode).toBe('aggregate');
      expect(req.metric).toBe('car_entries_count');
      expect(req.dimension).toBe('unload_place');
      expect(req.filters).toContainEqual({ key: 'date_range', from: '2026-06-01', to: '2026-06-07' });
    });

    it('добавляет granularity только для разреза period', () => {
      const period = buildReportRequest(
        { mode: 'aggregate', metric: 'applications_count', dimension: 'period', granularity: 'week' },
        PERIOD, ['date_range'],
      );
      expect(period.granularity).toBe('week');

      const status = buildReportRequest(
        { mode: 'aggregate', metric: 'applications_count', dimension: 'status', granularity: 'week' },
        PERIOD, ['date_range'],
      );
      expect(status.granularity).toBeUndefined();
    });

    it('шлёт metrics[] при мультивыборе и опускает одиночный metric', () => {
      const req = buildReportRequest(
        { mode: 'aggregate', metrics: ['applications_count', 'items_sum'], dimension: 'organization' },
        PERIOD, ['date_range'],
      );
      expect(req.metrics).toEqual(['applications_count', 'items_sum']);
      expect(req.metric).toBeUndefined();
      expect(req.dimension).toBe('organization');
    });

    it('отбрасывает пустые ключи метрик и при пустом metrics откатывается на metric', () => {
      const empty = buildReportRequest(
        { mode: 'aggregate', metrics: ['  ', ''], metric: 'applications_count', dimension: 'status' },
        PERIOD, ['date_range'],
      );
      expect(empty.metrics).toBeUndefined();
      expect(empty.metric).toBe('applications_count');
    });

    it('включает granularity для разреза period и при мультивыборе метрик', () => {
      const req = buildReportRequest(
        { mode: 'aggregate', metrics: ['applications_count', 'people_entries_count'], dimension: 'period', granularity: 'week' },
        PERIOD, ['date_range'],
      );
      expect(req.metrics).toEqual(['applications_count', 'people_entries_count']);
      expect(req.granularity).toBe('week');
    });

    it('не шлёт date_range, если период пустой', () => {
      const req = buildReportRequest(
        { mode: 'aggregate', metric: 'applications_count', dimension: 'status' },
        { from: '', to: '' }, ['date_range'],
      );
      expect(req.filters).toEqual([]);
    });

    it('шлёт pivot (cross-tab ось) только при разрезе period', () => {
      const period = buildReportRequest(
        { mode: 'aggregate', metrics: ['applications_count'], dimension: 'period', granularity: 'week', pivot: 'attachment_type' },
        PERIOD, ['date_range'],
      );
      expect(period.pivot).toBe('attachment_type');

      // Вне period ось не имеет смысла -> бэк её не примет, фронт не шлёт.
      const status = buildReportRequest(
        { mode: 'aggregate', metrics: ['applications_count'], dimension: 'status', pivot: 'attachment_type' },
        PERIOD, ['date_range'],
      );
      expect(status.pivot).toBeUndefined();
    });

    it('опускает pivot, когда ось не выбрана', () => {
      const req = buildReportRequest(
        { mode: 'aggregate', metrics: ['applications_count'], dimension: 'period', granularity: 'day' },
        PERIOD, ['date_range'],
      );
      expect(req.pivot).toBeUndefined();
    });
  });

  describe('list', () => {
    it('включает date_range, когда сущность его поддерживает', () => {
      const req = buildReportRequest(
        { mode: 'list', entity: 'work_applications' },
        PERIOD, ['date_range', 'organization', 'status'],
      );
      expect(req.mode).toBe('list');
      expect(req.entity).toBe('work_applications');
      expect(req.filters).toContainEqual({ key: 'date_range', from: '2026-06-01', to: '2026-06-07' });
    });

    it('НЕ включает date_range, если сущность его не поддерживает (машины/люди)', () => {
      const req = buildReportRequest(
        { mode: 'list', entity: 'cars' },
        PERIOD, ['organization', 'unload_place'],
      );
      expect(req.filters.find((f) => f.key === 'date_range')).toBeUndefined();
    });
  });

  describe('фильтры значений', () => {
    it('включает только применимые ключи с непустыми значениями', () => {
      const req = buildReportRequest(
        {
          mode: 'list',
          entity: 'work_applications',
          filters: {
            organization: ['ООО Ромашка', '  '],
            status: [],
            citizenship: ['РФ'], // не применим к work_applications
          },
        },
        { from: '', to: '' },
        ['date_range', 'organization', 'status'],
      );
      expect(req.filters).toContainEqual({ key: 'organization', values: ['ООО Ромашка'] });
      expect(req.filters.find((f) => f.key === 'status')).toBeUndefined();
      expect(req.filters.find((f) => f.key === 'citizenship')).toBeUndefined();
    });
  });

  describe('limit', () => {
    it('нормализует лимит: пусто -> дефолт 100, дробь -> floor, потолок 1000', () => {
      expect(buildReportRequest({ mode: 'list', entity: 'cars', limit: null }, {}, []).limit).toBe(100);
      expect(buildReportRequest({ mode: 'list', entity: 'cars', limit: '' }, {}, []).limit).toBe(100);
      expect(buildReportRequest({ mode: 'list', entity: 'cars', limit: '50.9' }, {}, []).limit).toBe(50);
      expect(buildReportRequest({ mode: 'list', entity: 'cars', limit: 5000 }, {}, []).limit).toBe(1000);
      expect(buildReportRequest({ mode: 'list', entity: 'cars', limit: -3 }, {}, []).limit).toBe(100);
    });

    // Разрез «период» движок сортирует хронологически ДО обрезки: дефолтные 100
    // отрезали бы хвост периода (год по дням заканчивался бы в начале апреля).
    it('пустой лимит на разрезе «период» -> потолок 1000, а не дефолт', () => {
      const req = buildReportRequest(
        { mode: 'aggregate', metrics: ['applications_count'], dimension: 'period', granularity: 'day', limit: '' },
        { from: '2026-01-01', to: '2026-12-31' },
        ['date_range'],
      );
      expect(req.limit).toBe(1000);
    });

    it('явный лимит на разрезе «период» уважается', () => {
      const req = buildReportRequest(
        { mode: 'aggregate', metrics: ['applications_count'], dimension: 'period', limit: 30 },
        {},
        [],
      );
      expect(req.limit).toBe(30);
    });

    it('прочие разрезы остаются на дефолте 100', () => {
      const req = buildReportRequest(
        { mode: 'aggregate', metrics: ['applications_count'], dimension: 'organization', limit: '' },
        {},
        [],
      );
      expect(req.limit).toBe(100);
    });
  });

  describe('defaultReportLimit', () => {
    it('период -> потолок, остальное -> дефолт', () => {
      expect(defaultReportLimit({ mode: 'aggregate', dimension: 'period' })).toBe(MAX_REPORT_LIMIT);
      expect(defaultReportLimit({ mode: 'aggregate', dimension: 'status' })).toBe(DEFAULT_REPORT_LIMIT);
      // list разреза не имеет — выгрузка строк живёт на дефолте.
      expect(defaultReportLimit({ mode: 'list', entity: 'cars' })).toBe(DEFAULT_REPORT_LIMIT);
    });
  });
});
