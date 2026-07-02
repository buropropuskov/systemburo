import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import RolePermissionsModal from '../RolePermissionsModal.vue';

const catalog = [
  { key: 'page.center', display_name: 'Центр заявок', category: 'Навигация' },
  { key: 'entity.cars.read', display_name: 'Автомобили: просмотр', category: 'Сотрудники и автомобили' },
  { key: 'page.analytics', display_name: 'Аналитика', category: 'Навигация' },
  { key: 'page.admin.system_control', display_name: 'Техработы', category: 'Администрирование', super_only: true },
];

const groups = [
  { id: 1, name: 'Аналитика', keys: ['page.analytics'] },
  { id: 2, name: 'Базовые', keys: ['page.center'] },
];

function mountModal(props = {}) {
  return mount(RolePermissionsModal, {
    props: {
      show: true,
      catalog,
      groups,
      initialDirectKeys: ['entity.cars.read'],
      initialGroupIds: [],
      ...props,
    },
    global: { stubs: { teleport: true } },
  });
}

const toggle = (w, key) => w.find(`[data-key="${key}"] .tgl`);
const groupBox = (w, id) => w.find(`[data-testid="role-perms-group"][data-group-id="${id}"]`);
const save = (w) => w.find('[data-testid="role-permissions-save"]').trigger('click');
const lastSave = (w) => w.emitted('save')[0][0];

describe('RolePermissionsModal', () => {
  it('собственные точечные права роли показаны редактируемым тумблером с бейджем "роль"', () => {
    const w = mountModal();
    const row = w.get('[data-key="entity.cars.read"]');
    expect(row.find('.tgl').attributes('aria-pressed')).toBe('true');
    expect(row.find('.tgl').attributes('disabled')).toBeUndefined();
    expect(row.find('.src--role').text()).toBe('роль');
  });

  it('включение собственного права уходит в directKeys, группы не трогаются', async () => {
    const w = mountModal();
    await toggle(w, 'page.center').trigger('click');
    await save(w);
    const payload = lastSave(w);
    expect([...payload.directKeys].sort()).toEqual(['entity.cars.read', 'page.center']);
    expect(payload.groupIds).toEqual([]);
  });

  it('выключение собственного права убирает его из directKeys', async () => {
    const w = mountModal();
    await toggle(w, 'entity.cars.read').trigger('click');
    await save(w);
    expect(lastSave(w).directKeys).toEqual([]);
  });

  it('ключ из выбранной группы залочен, помечен "группа" и не редактируется', async () => {
    const w = mountModal({ initialGroupIds: [1] });
    const t = toggle(w, 'page.analytics');
    expect(t.attributes('aria-pressed')).toBe('true');
    expect(t.attributes('disabled')).toBeDefined();
    expect(w.get('[data-key="page.analytics"]').find('.src--group').exists()).toBe(true);
    // Клик по locked-ключу ничего не меняет.
    await t.trigger('click');
    await save(w);
    expect(lastSave(w).directKeys).toEqual(['entity.cars.read']);
  });

  it('добавление группы зажигает её ключи, но собственные directKeys не меняются (инвариант)', async () => {
    const w = mountModal();
    // page.analytics до добавления группы выключен.
    expect(toggle(w, 'page.analytics').attributes('aria-pressed')).toBe('false');
    await groupBox(w, 1).trigger('change');
    // Теперь зажёгся от группы: on + locked.
    expect(toggle(w, 'page.analytics').attributes('aria-pressed')).toBe('true');
    expect(toggle(w, 'page.analytics').attributes('disabled')).toBeDefined();
    await save(w);
    const payload = lastSave(w);
    expect(payload.directKeys).toEqual(['entity.cars.read']);
    expect(payload.groupIds).toEqual([1]);
  });

  it('снятие группы возвращает как было: групповой ключ гаснет и снова редактируем, собственные остаются', async () => {
    const w = mountModal({ initialGroupIds: [1] });
    await groupBox(w, 1).trigger('change');
    const t = toggle(w, 'page.analytics');
    expect(t.attributes('aria-pressed')).toBe('false');
    expect(t.attributes('disabled')).toBeUndefined();
    await save(w);
    const payload = lastSave(w);
    expect(payload.directKeys).toEqual(['entity.cars.read']);
    expect(payload.groupIds).toEqual([]);
  });

  it('ключ, изначально и собственный, и покрытый группой: залочен, но выживает в directKeys', async () => {
    // page.center есть и в initialDirectKeys, и в группе 2 (Базовые -> page.center).
    const w = mountModal({ initialDirectKeys: ['page.center', 'entity.cars.read'], initialGroupIds: [2] });
    const t = toggle(w, 'page.center');
    expect(t.attributes('aria-pressed')).toBe('true');
    expect(t.attributes('disabled')).toBeDefined();
    expect(w.get('[data-key="page.center"]').find('.src--group').exists()).toBe(true);
    await save(w);
    const payload = lastSave(w);
    // Собственный грант не теряется, даже пока замаскирован группой.
    expect([...payload.directKeys].sort()).toEqual(['entity.cars.read', 'page.center']);
    expect(payload.groupIds).toEqual([2]);
  });

  it('осиротевший ключ (нет в каталоге) не уходит в directKeys при сохранении', async () => {
    const w = mountModal({ initialDirectKeys: ['entity.cars.read', 'page.obsolete'] });
    await save(w);
    expect(lastSave(w).directKeys).toEqual(['entity.cars.read']);
  });

  it('super_only-право заблокировано и не попадает в directKeys', async () => {
    const w = mountModal();
    const su = toggle(w, 'page.admin.system_control');
    expect(su.attributes('disabled')).toBeDefined();
    await su.trigger('click');
    await save(w);
    expect(lastSave(w).directKeys).not.toContain('page.admin.system_control');
  });

  it('поиск фильтрует дерево по названию и ключу', async () => {
    const w = mountModal();
    await w.find('[data-testid="role-permissions-search"]').setValue('аналитик');
    expect(toggle(w, 'page.analytics').exists()).toBe(true);
    expect(toggle(w, 'entity.cars.read').exists()).toBe(false);
  });
});
