import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const downloadGuideFile = vi.fn();
vi.mock('@/api/guide', () => ({
  downloadGuideFile: (...a) => downloadGuideFile(...a),
}));

import UserGuideModal from '../UserGuideModal.vue';

const SECTIONS = [
  {
    role: 'user',
    title: 'Пользователь',
    lead: 'Лид пользователя',
    items: ['Пункт 1', 'Пункт 2'],
    file: {
      name: 'Руководство пользователя.pdf',
      ext: '.pdf',
      mime_type: 'application/pdf',
      size: 2516582,
      updated_at: '2026-06-18T10:00:00Z',
      download_url: '/api/guide/sections/user/download',
    },
  },
  {
    role: 'admin',
    title: 'Администратор',
    lead: 'Лид админа',
    items: ['Админ пункт'],
    file: null,
  },
];

function mountModal(props = {}) {
  return mount(UserGuideModal, {
    props: { show: true, sections: SECTIONS, ...props },
    global: {
      stubs: {
        teleport: true,
        FileTypeIcon: true,
        LoaderSpinner: true,
      },
    },
    attachTo: document.body,
  });
}

describe('UserGuideModal — раскладка «Вкладки»', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    downloadGuideFile.mockReset();
  });

  it('рисует пилюлю на каждый пришедший раздел (гейтинг = только доступные роли)', () => {
    const wrapper = mountModal();
    const pills = wrapper.findAll('.role-pill');
    expect(pills).toHaveLength(2);
    expect(pills[0].text()).toContain('Пользователь');
    expect(pills[1].text()).toContain('Администратор');
  });

  it('по умолчанию активна первая вкладка: её файл, мета и описание', () => {
    const wrapper = mountModal();
    expect(wrapper.find('.file-card__name').text()).toBe('Руководство пользователя.pdf');
    // Размер идёт через общий formatBytes (@/utils/download): 2516582 байта -> «2.4 МБ».
    // Разделитель - точка, потому что те же числа печатает CLI server archive (humanBytes).
    expect(wrapper.find('.file-card__meta').text()).toContain('2.4 МБ');
    expect(wrapper.find('.file-card__meta').text()).toContain('18.06.2026');
    expect(wrapper.find('.descr__lead').text()).toBe('Лид пользователя');
    expect(wrapper.findAll('.descr__list li')).toHaveLength(2);
  });

  it('переключение на раздел без файла показывает заглушку без кнопки скачивания', async () => {
    const wrapper = mountModal();
    await wrapper.findAll('.role-pill')[1].trigger('click');
    expect(wrapper.find('.file-card--empty').exists()).toBe(true);
    expect(wrapper.find('.file-card__dl').exists()).toBe(false);
    expect(wrapper.find('.file-card__name').text()).toContain('ещё не загружен');
    expect(wrapper.find('.descr__lead').text()).toBe('Лид админа');
  });

  it('клик «Скачать» вызывает downloadGuideFile с url и именем файла', async () => {
    downloadGuideFile.mockResolvedValue();
    const wrapper = mountModal();
    await wrapper.find('.file-card__dl').trigger('click');
    expect(downloadGuideFile).toHaveBeenCalledWith(
      '/api/guide/sections/user/download',
      'Руководство пользователя.pdf',
    );
  });

  it('loading рисует состояние загрузки, пустой список — заглушку «нет разделов»', () => {
    const loadingW = mountModal({ loading: true, sections: [] });
    expect(loadingW.find('.guide__state').exists()).toBe(true);
    expect(loadingW.find('.role-pill').exists()).toBe(false);

    const emptyW = mountModal({ loading: false, sections: [] });
    expect(emptyW.find('.guide__state').text()).toContain('Нет доступных разделов');
  });

  it('эмитит close по кнопке «Готово» и по крестику', async () => {
    const wrapper = mountModal();
    await wrapper.find('.guide__done').trigger('click');
    await wrapper.find('.guide__close').trigger('click');
    expect(wrapper.emitted('close')).toHaveLength(2);
  });
});
