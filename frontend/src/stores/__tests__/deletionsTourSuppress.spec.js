import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { useDeletionsStore } from '../deletions';
import { useUiStore } from '../ui';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) }) }));

describe('уведомления во время онбординг-тура', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it('фоновая подсказка во время тура не показывается', () => {
    useUiStore().tourActive = true;
    const store = useDeletionsStore();
    expect(store.notify({ prefix: 'Место разгрузки выбрано автоматически' })).toBeNull();
    expect(store.items).toHaveLength(0);
  });

  // Ошибку глушить нельзя: человек не поймёт, почему действие не сработало,
  // и решит, что виноват тур.
  it('ошибка проходит сквозь тур', () => {
    useUiStore().tourActive = true;
    const store = useDeletionsStore();
    expect(store.notify({ prefix: 'Не удалось сохранить', type: 'error' })).not.toBeNull();
    expect(store.items).toHaveLength(1);
  });

  it('после тура подсказки снова показываются', () => {
    const ui = useUiStore();
    const store = useDeletionsStore();
    ui.tourActive = true;
    store.notify({ prefix: 'скрыта' });
    ui.tourActive = false;
    store.notify({ prefix: 'видна' });
    expect(store.items).toHaveLength(1);
    expect(store.items[0].prefix).toBe('видна');
  });

  // Плашка удаления несёт кнопку «Отменить» - её проглатывание отняло бы у
  // человека возможность вернуть данные, это дороже аккуратного вида тура.
  it('плашка с отменой во время тура остаётся', () => {
    useUiStore().tourActive = true;
    const store = useDeletionsStore();
    store.enqueue({ prefix: 'Удалено', onUndo: () => {} });
    expect(store.items).toHaveLength(1);
  });
});
