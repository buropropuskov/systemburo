import { describe, it, expect, vi, beforeEach } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { directoryBulkComputed, directoryBulkMethods } from '../directoryBulk';

/**
 * Выбор строк и групповые операции справочников живут в одном месте (#871).
 *
 * Организации и компании написаны копипастой: после нормализации имён сущности
 * их файлы совпадали почти дословно, и эта логика существовала в двух
 * экземплярах. Различались только имена списков, поэтому расхождение между
 * копиями было вопросом времени - за два месяца двенадцать коммитов правили три
 * и более справочников разом.
 *
 * Замок держит оба конца: поведение самих методов и то, что копии не завелись
 * обратно.
 */

const componentsDir = resolve(__dirname, '..', '..', 'components');
const TWINS = ['OrganizationsManagement', 'CompaniesManagement'];
const MOVED = [
  'allSelected',
  'someSelected',
  'isSelected',
  'toggleSelect',
  'onRowCheck',
  'toggleSelectAll',
  'clearSelection',
  'startBulkOperation',
  'closeBulkModal',
  'cancelBulkConfirm',
  'handleBulkResult',
];

/** Компонент-заглушка с состоянием, которое ожидает общий набор. */
function scene(rows = [{ id: 1 }, { id: 2 }, { id: 3 }]) {
  const computed = directoryBulkComputed({ sortedKey: 'sorted' });
  const methods = directoryBulkMethods({ sortedKey: 'sorted' });
  const ctx = {
    sorted: rows,
    selectedIds: [],
    lastSelectedId: null,
    pendingBulkOp: null,
    bulkModalVisible: false,
    bulkConfirmVisible: false,
    bulkSubmitting: false,
    refreshData: vi.fn(),
  };
  for (const [name, fn] of Object.entries(methods)) ctx[name] = fn.bind(ctx);
  Object.defineProperties(ctx, Object.fromEntries(
    Object.entries(computed).map(([name, fn]) => [name, { get: fn.bind(ctx) }]),
  ));
  return ctx;
}

describe('общий набор групповых операций справочников', () => {
  beforeEach(() => setActivePinia(createPinia()));

  it('shift-клик выделяет диапазон от прошлой строки', () => {
    const s = scene([{ id: 10 }, { id: 20 }, { id: 30 }, { id: 40 }]);
    s.onRowCheck({ id: 10 }, 0, {});
    s.onRowCheck({ id: 30 }, 2, { shiftKey: true });

    expect(s.selectedIds, 'диапазон должен захватить и промежуточную строку').toEqual([10, 20, 30]);
  });

  it('без якоря shift-клик работает как обычный', () => {
    const s = scene();
    s.onRowCheck({ id: 2 }, 1, { shiftKey: true });
    expect(s.selectedIds).toEqual([2]);
  });

  it('«выбрать всё» переключает и сбрасывает якорь диапазона', () => {
    const s = scene();
    s.onRowCheck({ id: 1 }, 0, {});
    s.toggleSelectAll();

    expect(s.selectedIds).toEqual([1, 2, 3]);
    expect(s.allSelected).toBe(true);
    expect(s.lastSelectedId, 'якорь остался - следующий shift-клик выделит лишнее').toBe(null);

    s.toggleSelectAll();
    expect(s.selectedIds).toEqual([]);
  });

  it('пустой список не считается выбранным целиком', () => {
    const s = scene([]);
    expect(s.allSelected).toBe(false);
    expect(s.someSelected).toBe(false);
  });

  it('архивация и восстановление спрашивают подтверждение, остальное открывает окно', () => {
    const s = scene();
    s.startBulkOperation('archive');
    expect(s.bulkConfirmVisible).toBe(true);
    expect(s.bulkModalVisible).toBe(false);

    s.cancelBulkConfirm();
    s.startBulkOperation('type');
    expect(s.bulkModalVisible).toBe(true);
  });

  it('пока операция идёт, окно не закрывается', () => {
    const s = scene();
    s.startBulkOperation('type');
    s.bulkSubmitting = true;
    s.closeBulkModal();

    expect(s.bulkModalVisible, 'закрыли окно посреди отправки - пользователь решит, что операция отменена').toBe(true);
  });

  it('ответ без счётчика считается провалом', () => {
    const s = scene();
    expect(s.handleBulkResult('archive', null, 3)).toBe(false);
    expect(s.refreshData).not.toHaveBeenCalled();
  });

  it('успех чистит выбор и перечитывает список', () => {
    const s = scene();
    s.selectedIds = [1, 2];
    expect(s.handleBulkResult('archive', { success_count: 2, error_count: 0 }, 2)).toBe(true);
    expect(s.selectedIds).toEqual([]);
    expect(s.refreshData).toHaveBeenCalled();
  });

  it.each(TWINS)('%s не держит своей копии', (name) => {
    const src = readFileSync(resolve(componentsDir, `${name}.vue`), 'utf8');

    expect(
      src.includes('directoryBulkMethods('),
      'общий набор не подключён - логика снова разъедется между близнецами',
    ).toBe(true);

    const own = MOVED.filter((member) => new RegExp(`^\\s{4}(async )?${member}\\(`, 'm').test(src));
    expect(own, 'члены объявлены заново поверх общего набора').toEqual([]);
  });
});
