import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect } from 'vitest';

/**
 * Замок на контракт окна у ManualAddModal (волна 8, #1097). Была самодельной
 * модалкой (свой оверлей + свой крестик + свой keydown-слушатель Escape) без
 * свайпа-вниз - владелец: "Добавить вручную" оформить как и все модалки со
 * свайпом и выездом". Переведена на BaseModal, как волна 7 поступила с окнами
 * добавления машины/сотрудника (EmployeeEditModal.vue - тот же приём).
 *
 * Поведенческий тест (mount) здесь не годится: `BaseModal` держит собственный
 * `<teleport>`/свайп/Escape внутри СВОЕГО компонента, и стерегущий факт - что
 * ManualAddModal НЕ завёл параллельно свою копию того же контракта, - виден
 * только чтением исходника (behavioral-тест увидел бы два рабочих Escape и не
 * заметил бы лишний).
 */

const SFC = readFileSync(resolve(__dirname, '../ManualAddModal.vue'), 'utf8');

describe('ManualAddModal - контракт окна через BaseModal', () => {
  it('корень шаблона - BaseModal, а не самодельный оверлей', () => {
    expect(SFC).toMatch(/<BaseModal\b/);
    expect(SFC).not.toMatch(/class="manual-modal-overlay"/);
  });

  it('BaseModal получает радиус 30px (эталон окон) и show/close', () => {
    const openTag = SFC.match(/<BaseModal\b[\s\S]*?>/)[0];
    expect(openTag).toMatch(/radius="30px"/);
    expect(openTag).toMatch(/:show="show"/);
    expect(openTag).toMatch(/@close="close"/);
  });

  it('импортирует и регистрирует BaseModal', () => {
    expect(SFC).toMatch(/import BaseModal from '@\/components\/ui\/BaseModal\.vue'/);
    expect(SFC).toMatch(/components:\s*\{\s*BaseModal/);
  });

  it('не держит собственный оверлей-клик/Escape - это дублировало бы контракт BaseModal', () => {
    expect(SFC).not.toMatch(/useOverlayClose/);
    expect(SFC).not.toMatch(/onOverlayMousedown/);
    expect(SFC).not.toMatch(/addEventListener\('keydown'/);
    expect(SFC).not.toMatch(/key === 'Escape'/);
  });

  it('не держит собственную блокировку скролла фона - её несёт BaseModal', () => {
    expect(SFC).not.toMatch(/setBodyScrollLock/);
    expect(SFC).not.toMatch(/releaseBodyScrollLock/);
  });

  it('кнопки Отмена/Добавить в таблицу живут в слоте #actions, а не в самодельном футере', () => {
    expect(SFC).toMatch(/<template #actions>/);
    expect(SFC).not.toMatch(/class="manual-modal__footer"/);
    const actionsBlock = SFC.match(/<template #actions>([\s\S]*?)<\/template>/)[1];
    expect(actionsBlock).toMatch(/manual-btn--ghost/);
    expect(actionsBlock).toMatch(/data-testid="manual-add-submit"/);
  });

  it('своей fade/scale-анимации не осталось - её несёт BaseModal', () => {
    expect(SFC).not.toMatch(/manual-modal-fade/);
  });
});
