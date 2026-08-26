import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';

import ForwardModal from '../ForwardModal.vue';

const ATTACHMENTS = [
  { id: 1, attachment_display_name: 'Toyota А124ВС', unique_attachment_title: 'Автомобили' },
  { id: 2, attachment_display_name: 'Иванов И.И.', unique_attachment_title: 'Сотрудники' },
  { id: 3, attachment_display_name: 'Ноутбук', unique_attachment_title: 'ТМЦ' },
];

const USER = { id: 42, username: 'ivanov', first_name: 'Иван', last_name: 'Иванов' };

// BaseModal не рендерит контент при show=false, а watcher reset() в ForwardModal
// срабатывает на переходе false->true, поэтому открываем модалку через setProps.
async function mountOpened(props = {}) {
  const wrapper = mount(ForwardModal, {
    props: {
      show: false,
      allUsers: [USER],
      attachments: ATTACHMENTS,
      ...props,
    },
    global: { stubs: { teleport: true } },
  });
  await wrapper.setProps({ show: true });
  return wrapper;
}

function attachmentBoxes(wrapper) {
  return wrapper.findAll('[data-testid="forward-modal-attachment"] input[type="checkbox"]');
}

describe('ForwardModal — выбор вложений (#680, срез fe-select)', () => {
  it('по умолчанию выбраны все вложения', async () => {
    const wrapper = await mountOpened();
    expect(wrapper.vm.selectedAttachmentIds).toEqual([1, 2, 3]);
    const boxes = attachmentBoxes(wrapper);
    expect(boxes).toHaveLength(3);
    boxes.forEach(box => expect(box.element.checked).toBe(true));
  });

  it('мастер-чекбокс снимает и возвращает все вложения', async () => {
    const wrapper = await mountOpened();
    const all = wrapper.find('[data-testid="forward-modal-attachments-all"]');
    await all.setValue(false);
    expect(wrapper.vm.selectedAttachmentIds).toEqual([]);
    await all.setValue(true);
    expect(wrapper.vm.selectedAttachmentIds).toEqual([1, 2, 3]);
  });

  it('мастер-тумблер вкл при всех выбранных и выкл при частичном выборе', async () => {
    const wrapper = await mountOpened();
    const all = wrapper.find('[data-testid="forward-modal-attachments-all"]');
    await wrapper.vm.$nextTick();
    expect(all.element.checked).toBe(true);

    await attachmentBoxes(wrapper)[1].setValue(false);
    await wrapper.vm.$nextTick();
    expect(all.element.checked).toBe(false);

    // Возврат всех - мастер-тумблер снова вкл.
    await attachmentBoxes(wrapper)[1].setValue(true);
    await wrapper.vm.$nextTick();
    expect(all.element.checked).toBe(true);
  });

  it('снятие галочки с вложения исключает его id из выбора', async () => {
    const wrapper = await mountOpened();
    await attachmentBoxes(wrapper)[1].setValue(false);
    expect(wrapper.vm.selectedAttachmentIds).toEqual([1, 3]);
  });

  it('send эмитит выбранные attachment_ids вместе с пользователями', async () => {
    const wrapper = await mountOpened();
    wrapper.vm.addUser(USER);
    await wrapper.vm.$nextTick();
    await attachmentBoxes(wrapper)[2].setValue(false);

    await wrapper.find('[data-testid="forward-modal-button-send"]').trigger('click');

    const events = wrapper.emitted('send');
    expect(events).toHaveLength(1);
    expect(events[0][0].attachment_ids).toEqual([1, 2]);
    expect(events[0][0].users).toHaveLength(1);
    expect(events[0][0].users[0].user_id).toBe(42);
  });

  it('при снятии всех вложений отправка заблокирована и показан hint', async () => {
    const wrapper = await mountOpened();
    wrapper.vm.addUser(USER);
    await wrapper.vm.$nextTick();
    const sendBtn = wrapper.find('[data-testid="forward-modal-button-send"]');
    expect(sendBtn.attributes('disabled')).toBeUndefined();

    await wrapper.find('[data-testid="forward-modal-attachments-all"]').setValue(false);
    expect(wrapper.vm.selectedAttachmentIds).toEqual([]);
    expect(sendBtn.attributes('disabled')).toBeDefined();
    expect(wrapper.find('.forward-attachments-hint').exists()).toBe(true);
  });

  it('без вложений секция скрыта и отправка не блокируется по вложениям', async () => {
    const wrapper = await mountOpened({ attachments: [] });
    expect(wrapper.find('[data-testid="forward-modal-attachments"]').exists()).toBe(false);
    wrapper.vm.addUser(USER);
    await wrapper.vm.$nextTick();
    expect(wrapper.find('[data-testid="forward-modal-button-send"]').attributes('disabled')).toBeUndefined();
  });

  it('повторное открытие сбрасывает выбор вложений к "все"', async () => {
    const wrapper = await mountOpened();
    await attachmentBoxes(wrapper)[0].setValue(false);
    expect(wrapper.vm.selectedAttachmentIds).toEqual([2, 3]);
    await wrapper.setProps({ show: false });
    await wrapper.setProps({ show: true });
    expect(wrapper.vm.selectedAttachmentIds).toEqual([1, 2, 3]);
  });
});

describe('ForwardModal — поиск пользователей (#1157)', () => {
  it('без запроса показывает всех доступных пользователей', async () => {
    const wrapper = await mountOpened();
    expect(wrapper.vm.filteredUsers.map(u => u.id)).toEqual([USER.id]);
  });

  it('поиск матчит по варианту раскладки - EN-ввод находит кириллицу ФИО', async () => {
    // "bdfyjd" на EN-раскладке физически совпадает с "иванов" на RU.
    const wrapper = await mountOpened();
    wrapper.vm.searchQuery = 'bdfyjd';
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.filteredUsers.map(u => u.id)).toEqual([USER.id]);
  });

  it('пустой поисковый запрос снова показывает всех пользователей', async () => {
    const wrapper = await mountOpened();
    wrapper.vm.searchQuery = 'bdfyjd';
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.filteredUsers).toHaveLength(1);

    wrapper.vm.searchQuery = '';
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.filteredUsers.map(u => u.id)).toEqual([USER.id]);
  });

  it('нерелевантный запрос не находит пользователя', async () => {
    const wrapper = await mountOpened();
    wrapper.vm.searchQuery = 'zzz-no-match';
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.filteredUsers).toHaveLength(0);
  });
});

describe('ForwardModal — сопроводительное сообщение (#967)', () => {
  it('send эмитит введённое сообщение (обрезанное по краям)', async () => {
    const wrapper = await mountOpened({ attachments: [] });
    wrapper.vm.addUser(USER);
    await wrapper.vm.$nextTick();
    await wrapper.find('[data-testid="forward-modal-message"]').setValue('  Прошу согласовать  ');

    await wrapper.find('[data-testid="forward-modal-button-send"]').trigger('click');

    const events = wrapper.emitted('send');
    expect(events).toHaveLength(1);
    expect(events[0][0].message).toBe('Прошу согласовать');
  });

  it('без текста сообщение в payload пустое', async () => {
    const wrapper = await mountOpened({ attachments: [] });
    wrapper.vm.addUser(USER);
    await wrapper.vm.$nextTick();

    await wrapper.find('[data-testid="forward-modal-button-send"]').trigger('click');

    expect(wrapper.emitted('send')[0][0].message).toBe('');
  });

  it('показывает предупреждение о видимости бюро пропусков', async () => {
    const wrapper = await mountOpened();
    const warning = wrapper.find('[data-testid="forward-modal-warning"]');
    expect(warning.exists()).toBe(true);
    expect(warning.text()).toContain('бюро пропусков');
  });

  it('повторное открытие сбрасывает сообщение', async () => {
    const wrapper = await mountOpened({ attachments: [] });
    await wrapper.find('[data-testid="forward-modal-message"]').setValue('черновик');
    expect(wrapper.vm.message).toBe('черновик');
    await wrapper.setProps({ show: false });
    await wrapper.setProps({ show: true });
    expect(wrapper.vm.message).toBe('');
  });
});
