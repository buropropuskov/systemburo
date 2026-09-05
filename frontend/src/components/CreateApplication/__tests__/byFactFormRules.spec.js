import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { BY_FACT_ALREADY_ADDED, BY_FACT_PERIOD_RULE, byFactDeadlineHint } from '@/utils/byFactVehicle';

/**
 * Форма подачи объясняет правила «По факту» на месте, а не отказом при отправке (#2320).
 *
 * Владелец наткнулся именно на это: добавил две такие машины, поставил период в месяц,
 * закрыл форму - и не узнал, что заявка так не проходит. Правило живёт на бэкенде, но
 * без зеркала в форме человек видит его только после того, как всё заполнил.
 *
 * Замки текстовые: обе проверки встроены в существующие механизмы компонентов
 * (список причин блокировки кнопки и объект ошибок блока дат), и смонтированный
 * компонент показал бы их лишь при полном наборе данных формы - подача заявки
 * требует организации, бланка, мест разгрузки и согласия.
 */

const читать = (путь) => readFileSync(resolve(__dirname, '..', путь), 'utf8');

describe('форма подачи: правила машины «По факту»', () => {
  it('кнопка «Добавить» блокируется на второй такой машине', () => {
    const src = читать('VehicleForm.vue');

    expect(
      /check:\s*!vm\.isNumberByFact \|\| !hasByFactVehicle\(/.test(src),
      'в списке причин блокировки нет правила про вторую машину «По факту»',
    ).toBe(true);
    expect(src, 'причина должна объясняться текстом из общего места').toContain('BY_FACT_ALREADY_ADDED');
  });

  it('правило считает уже добавленные машины, а не правимую', () => {
    // Иначе правка самой машины «По факту» блокировала бы себя же.
    expect(/hasByFactVehicle\(vm\.existingVehicles, vm\.editingVehicle\)/.test(читать('VehicleForm.vue'))).toBe(true);
  });

  it('кнопка «Добавить» недоступна, пока период не однодневный', () => {
    // Без этого первая машина «По факту» спокойно добавлялась при периоде в месяц:
    // ошибка появлялась уже после того, как её пропустили в список.
    expect(
      /check:\s*!vm\.isNumberByFact \|\| isOneDayPeriod\(vm\.entryPeriod\)/.test(читать('VehicleForm.vue')),
      'в списке причин блокировки нет правила про однодневный период',
    ).toBe(true);
  });

  it('тумблер сообщает наверх, чтобы предупреждение поднялось до добавления машины', () => {
    const src = читать('VehicleForm.vue');
    const обработчик = src.slice(src.indexOf('handleNumberByFactChange()'), src.indexOf('handleMarkByFactChange()'));

    expect(/\$emit\('by-fact-change'/.test(обработчик), 'форма обязана сообщать о смене режима').toBe(true);
    // Тоста здесь быть не должно: предупреждение живёт в панели, а всплывающее
    // окно в правом углу дублировало его и мешало.
    expect(/notify\(/.test(обработчик)).toBe(false);
  });

  it('предупреждение о периоде идёт в общую панель формы', () => {
    const src = читать('CreateApplication.vue');

    expect(src, 'панель должна получать группы из warningGroups').toContain(':groups="warningGroups"');
    expect(/warningGroups\(\)[\s\S]{0,200}byFactPeriodBroken/.test(src), 'группа поднимается общей проверкой').toBe(true);
    expect(src, 'предупреждение собирается общей функцией').toContain('byFactWarningGroup()');
    // Поля дат краснеют по флагу без текста - объяснение показывает панель.
    expect(src).toContain('periodInvalid: true');
  });

  it('тексты правил не пустые и объясняют, а не просто запрещают', () => {
    expect(BY_FACT_ALREADY_ADDED).toMatch(/одна/);
    expect(BY_FACT_PERIOD_RULE).toMatch(/до суток/);
    // Вторая строка называет крайнюю дату: без неё правило понятно, но неясно,
    // какой срок ставить прямо сейчас.
    expect(byFactDeadlineHint(new Date('2026-09-05T14:38:00Z'))).toContain('06.09.2026');
  });
});
