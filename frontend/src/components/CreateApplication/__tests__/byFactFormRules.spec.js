import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { BY_FACT_ALREADY_ADDED, BY_FACT_ONE_DAY_HINT } from '@/utils/byFactVehicle';

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

  it('период длиннее дня подсвечивается ошибкой у поля дат', () => {
    const src = читать('CreateApplication.vue');

    expect(
      /currentAttachmentErrors\(\)[\s\S]{0,400}hasByFactVehicle\(this\.vehicles\)/.test(src),
      'ошибка периода должна идти через currentAttachmentErrors - его читает DateRangeSection',
    ).toBe(true);
    expect(src).toContain('BY_FACT_ONE_DAY_HINT');
  });

  it('тексты правил не пустые и объясняют, а не просто запрещают', () => {
    expect(BY_FACT_ALREADY_ADDED).toMatch(/одна/);
    expect(BY_FACT_ONE_DAY_HINT).toMatch(/один день/);
  });
});
