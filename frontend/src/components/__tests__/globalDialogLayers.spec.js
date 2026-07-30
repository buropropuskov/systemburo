import { describe, it, expect } from 'vitest';
import { readdirSync, readFileSync } from 'node:fs';
import { join, resolve, basename } from 'node:path';

/**
 * Диалоги, смонтированные в App.vue, поднимаются из ЛЮБОГО контекста - в том числе
 * когда поверх страницы уже открыта модалка. Значит их слой обязан быть не ниже
 * самой высокой обычной модалки, иначе вопрос рисуется под её оверлеем: кнопки
 * неклики, а действие (навигация, подтверждение) молча ждёт ответа, которого
 * человек дать не может.
 *
 * Ловил это дважды: #481 (ConfirmDialog на 1100 за карточкой детали заявки),
 * #1594 (стек тостов на 11000 под оверлеями 12000-20000) и здесь - DirtyConfirmModal
 * на 11000 под модалкой настройки полей вложения, из-за чего «Назад» переставал работать.
 */

const GLOBAL_DIALOGS = [
  'DeleteNotifications.vue',
  'DirtyConfirmModal.vue',
  'ConfirmDialog.vue',
  'SessionExpiredModal.vue',
  'BanOverlay.vue',
  'PDConsentOverlay.vue',
];

const srcDir = resolve(__dirname, '../..');

function collectZIndexes() {
  const found = [];
  for (const entry of readdirSync(srcDir, { recursive: true, withFileTypes: true })) {
    if (!entry.isFile() || !/\.(vue|css)$/.test(entry.name)) continue;
    const file = join(entry.parentPath ?? entry.path, entry.name);
    const text = readFileSync(file, 'utf8');
    for (const m of text.matchAll(/z-index:\s*(\d+)/g)) {
      found.push({ file, name: basename(file), value: Number(m[1]) });
    }
  }
  return found;
}

describe('слои глобальных диалогов', () => {
  const all = collectZIndexes();
  const ordinary = all.filter(z => !GLOBAL_DIALOGS.includes(z.name));
  const ceiling = Math.max(...ordinary.map(z => z.value));

  it.each(GLOBAL_DIALOGS)('%s не ниже самой высокой обычной модалки', (name) => {
    const layers = all.filter(z => z.name === name);
    expect(layers.length, `${name}: не нашёл ни одного z-index`).toBeGreaterThan(0);

    const top = Math.max(...layers.map(z => z.value));
    const blockers = ordinary.filter(z => z.value > top).map(z => `${z.name}: ${z.value}`);
    expect(blockers, `${name} (${top}) ниже: ${blockers.join(', ')}`).toEqual([]);
  });

  // Стек уведомлений отдельно: он не блокирует ввод, поэтому обязан быть выше
  // вообще всех слоёв, включая остальные глобальные диалоги.
  it('стек уведомлений выше любого другого слоя', () => {
    const stack = all.filter(z => z.name === 'DeleteNotifications.vue');
    expect(stack).toHaveLength(1);

    const above = all
      .filter(z => z.name !== 'DeleteNotifications.vue' && z.value >= stack[0].value)
      .map(z => `${z.name}: ${z.value}`);
    expect(above).toEqual([]);
  });

  // Равенство не защищает: при одинаковом z-index выигрывает тот, кто позже в DOM.
  // ConfirmDialog (20000) сегодня ровно на потолке обычных модалок - фиксируем факт,
  // чтобы следующая модалка на 20000 не создала ту же ловушку незамеченной.
  it('потолок обычных модалок не выше 20000', () => {
    const tallest = ordinary.filter(z => z.value === ceiling).map(z => `${z.name}: ${z.value}`);
    expect(ceiling, `самые высокие обычные модалки: ${tallest.join(', ')}`).toBeLessThanOrEqual(20000);
  });
});
