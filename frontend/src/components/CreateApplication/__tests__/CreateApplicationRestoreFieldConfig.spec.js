import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CreateApplication from '../CreateApplication.vue';
import { getFieldConfig } from '@/api/attachment-templates';

// После F5 черновик открывал первое вложение мимо handleAttachmentSelected: конфиг
// полей не грузился, currentFieldConfig оставался пустым, и «Дополнительно» показывало
// все тумблеры шаблона. Плюс подсветка чипа в BlankSelector не следовала за формой.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/stores/auth', () => ({
  useAuthStore: vi.fn().mockReturnValue({ token: 'test-token' }),
}));
vi.mock('@/api/attachment-templates', () => ({
  getFieldConfig: vi.fn(),
}));

const CONFIG = {
  base: [
    { key: 'roof_access', visible: false, required: false, locked: false, requirable: true },
    { key: 'free_parking', visible: true, required: false, locked: false, requirable: true },
  ],
  custom: [],
};

function saveDraft() {
  localStorage.setItem('draftApplicationState', JSON.stringify({
    message: 'черновик',
    attachments: [
      { local_id: 'a1', id: 7, template_id: 7, attachment_type: 'cars', display_name: 'Машины' },
      { local_id: 'a2', id: 8, template_id: 8, attachment_type: 'people', display_name: 'Люди' },
    ],
  }));
}

beforeEach(() => {
  setActivePinia(createPinia());
  localStorage.clear();
  getFieldConfig.mockReset();
  getFieldConfig.mockResolvedValue(CONFIG);
});

describe('CreateApplication - конфиг полей восстановленного вложения', () => {
  it('после восстановления черновика конфиг загружен, скрытое поле скрыто', async () => {
    saveDraft();

    const w = shallowMount(CreateApplication);
    await flushPromises();

    expect(getFieldConfig).toHaveBeenCalledWith(7);
    expect(w.vm.currentFieldConfig.roof_access).toEqual(
      expect.objectContaining({ visible: false }),
    );
    expect(w.vm.currentFieldConfig.free_parking).toEqual(
      expect.objectContaining({ visible: true }),
    );
  });

  it('форма показывается только после загрузки конфига, без промежутка «видны все»', async () => {
    saveDraft();
    let resolveConfig;
    getFieldConfig.mockReturnValue(new Promise((r) => { resolveConfig = r; }));

    const w = shallowMount(CreateApplication);
    await flushPromises();

    // Конфиг ещё не пришёл - вложение не выбрано, форма не отрисована.
    expect(w.vm.selectedAttachment).toBeNull();

    resolveConfig(CONFIG);
    await flushPromises();

    expect(w.vm.selectedAttachment.local_id).toBe('a1');
    expect(w.vm.currentFieldConfig.roof_access.visible).toBe(false);
  });

  it('поздний ответ прежнего восстановления не перебивает выбранный дубль', async () => {
    saveDraft();
    let resolveFirst;
    getFieldConfig.mockImplementationOnce(() => new Promise((r) => { resolveFirst = r; }));

    const w = shallowMount(CreateApplication);
    await flushPromises();
    expect(w.vm.selectedAttachment).toBeNull();

    // «Заменить» в конфликте дублей переписывает черновик и перезапускает восстановление.
    localStorage.setItem('draftApplicationState', JSON.stringify({
      message: 'дубль',
      attachments: [
        { local_id: 'b1', id: 9, template_id: 9, attachment_type: 'people', display_name: 'Люди' },
      ],
    }));
    w.vm.restoreFromLocalStorage();
    await flushPromises();
    expect(w.vm.selectedAttachment.local_id).toBe('b1');

    resolveFirst(CONFIG);
    await flushPromises();
    expect(w.vm.selectedAttachment.local_id).toBe('b1');
  });

  it('недоступный конфиг не блокирует форму', async () => {
    saveDraft();
    getFieldConfig.mockRejectedValue(new Error('network'));

    const w = shallowMount(CreateApplication);
    await flushPromises();

    expect(w.vm.selectedAttachment.local_id).toBe('a1');
    expect(w.vm.currentFieldConfig).toEqual({});
  });
});
