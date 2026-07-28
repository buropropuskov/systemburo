import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CreateApplication from '../CreateApplication.vue';
import DateRangeSection from '../DateRangeSection.vue';
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue';
import { getFieldConfig } from '@/api/attachment-templates';

// Выбор и добавление вложения открывали форму до ответа /field-config: пустой конфиг
// деградирует к «видимы все», поэтому в «Дополнительно» на миг вставали все тумблеры
// шаблона и лишь потом лишние исчезали.

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

const CARS = { local_id: 'a1', id: 7, template_id: 7, attachment_type: 'cars', display_name: 'Машины' };
const PEOPLE = { local_id: 'a2', id: 8, template_id: 8, attachment_type: 'people', display_name: 'Люди' };

beforeEach(() => {
  setActivePinia(createPinia());
  localStorage.clear();
  getFieldConfig.mockReset();
  getFieldConfig.mockResolvedValue(CONFIG);
});

async function mountApp() {
  const w = shallowMount(CreateApplication);
  await flushPromises();
  return w;
}

describe('CreateApplication - конфиг полей при выборе вложения', () => {
  it('клик по вложению открывает форму только после конфига', async () => {
    const w = await mountApp();
    let resolveConfig;
    getFieldConfig.mockReturnValue(new Promise((r) => { resolveConfig = r; }));

    w.vm.attachments = [CARS];
    const selecting = w.vm.handleAttachmentSelected(CARS);
    await flushPromises();

    // Пока конфиг в пути - формы нет, а значит нет и промежутка «видны все тумблеры»
    expect(w.vm.selectedAttachment).toBeNull();

    resolveConfig(CONFIG);
    await selecting;

    expect(w.vm.selectedAttachment.local_id).toBe('a1');
    expect(w.vm.currentFieldConfig.roof_access.visible).toBe(false);
  });

  it('добавленное вложение показывается уже с конфигом', async () => {
    const w = await mountApp();
    let resolveConfig;
    getFieldConfig.mockReturnValue(new Promise((r) => { resolveConfig = r; }));

    const adding = w.vm.handleAttachmentAdded({ ...CARS });
    await flushPromises();

    expect(w.vm.selectedAttachment).toBeNull();
    // Черновик пишется сразу: вложение уже в списке
    expect(w.vm.attachments).toHaveLength(1);

    resolveConfig(CONFIG);
    await adding;

    expect(w.vm.selectedAttachment.local_id).toBe('a1');
    expect(w.vm.currentFieldConfig.roof_access.visible).toBe(false);
  });

  it('восстановление черновика не перебивает вложение, выбранное руками', async () => {
    localStorage.setItem('draftApplicationState', JSON.stringify({
      message: 'черновик',
      attachments: [CARS, PEOPLE],
    }));

    let resolveRestore;
    getFieldConfig.mockImplementationOnce(() => new Promise((r) => { resolveRestore = r; }));

    const w = shallowMount(CreateApplication);
    await flushPromises();
    expect(w.vm.selectedAttachment).toBeNull();

    // Список вложений уже кликабелен - пользователь открыл второе, не дожидаясь
    getFieldConfig.mockResolvedValue(CONFIG);
    await w.vm.handleAttachmentSelected(PEOPLE);
    expect(w.vm.selectedAttachment.local_id).toBe('a2');

    resolveRestore(CONFIG);
    await flushPromises();

    expect(w.vm.selectedAttachment.local_id).toBe('a2');
  });

  it('поздний ответ прошлого выбора не открывает чужую форму', async () => {
    const w = await mountApp();
    w.vm.attachments = [CARS, PEOPLE];

    let resolveFirst;
    getFieldConfig.mockImplementationOnce(() => new Promise((r) => { resolveFirst = r; }));
    const first = w.vm.handleAttachmentSelected(CARS);
    await flushPromises();

    // Пользователь не дождался и кликнул второе вложение
    getFieldConfig.mockResolvedValue(CONFIG);
    await w.vm.handleAttachmentSelected(PEOPLE);
    expect(w.vm.selectedAttachment.local_id).toBe('a2');

    resolveFirst(CONFIG);
    await first;

    expect(w.vm.selectedAttachment.local_id).toBe('a2');
  });
});

describe('DateRangeSection - тумблеры «Дополнительно»', () => {
  const mountSection = (fieldConfig = {}) => mount(DateRangeSection, {
    props: { fieldConfig, errors: {}, roofAccess: false, freeParking: true },
  });

  it('используют общий ToggleSwitch, а не самодельный переключатель', () => {
    const w = mountSection();
    const toggles = w.findAllComponents(ToggleSwitch);

    expect(toggles).toHaveLength(2);
    // Выключенный и включённый различимы состоянием, а не только цветом трека
    expect(toggles[0].props('modelValue')).toBe(false);
    expect(toggles[1].props('modelValue')).toBe(true);
    expect(w.find('.option-toggle__switch').exists()).toBe(false);
  });

  it('скрытое конфигом поле не рисует свой тумблер', () => {
    const w = mountSection({ roof_access: { visible: false, required: false } });
    const toggles = w.findAllComponents(ToggleSwitch);

    expect(toggles).toHaveLength(1);
    expect(w.text()).not.toContain('Доступ на крышу');
    expect(w.text()).toContain('Бесплатная парковка');
  });
});
