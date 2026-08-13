import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import ApplicationDetail from '../ApplicationDetail.vue';
import { resetModalStack } from '@/utils/modalStack';
import { resetBodyScrollLock } from '@/utils/bodyScrollLock';

/**
 * Панель заявки - полноэкранное окно (bottom-sheet на мобилке), и фон под ней
 * (Центр/кабинет) обязан стоять на месте, как под любым другим окном проекта - через
 * общий замок setBodyScrollLock/releaseBodyScrollLock (#1097 w8, дословно владельца:
 * "когда я мотаю вниз в Карточке заявки, на заднем фоне скроллится Центр заявок").
 *
 * jsdom не считает реальный скролл/chaining (см. lessons/frontend-ui.md), поэтому
 * здесь стережём КОНТРАКТ - владение замком синхронизировано с mounted/beforeUnmount,
 * а не факт визуальной прокрутки (тот проверен браузером, см. отчёт волны).
 */

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: async () => [] }),
}));
vi.mock('@/api/applications', () => ({
  markAsRead: vi.fn().mockResolvedValue({}),
  getApplicationSupplements: vi.fn().mockResolvedValue([]),
  getApplicationParticipants: vi.fn().mockResolvedValue([]),
}));
vi.mock('@/utils/eventStream', () => ({
  default: { connect: vi.fn(), disconnect: vi.fn(), on: vi.fn(() => vi.fn()) },
  eventStream: { connect: vi.fn(), disconnect: vi.fn(), on: vi.fn(() => vi.fn()) },
}));

const APPLICATION = {
  id: 7,
  application_number: '№ 20260812/001',
  status: 'В обработке',
  message: 'текст заявки',
  sender_user_id: 1,
  attachments: [],
};

const opened = [];

function mountDetail() {
  const wrapper = mount(ApplicationDetail, {
    props: { application: APPLICATION, mode: 'center' },
    global: { stubs: { teleport: true } },
  });
  opened.push(wrapper);
  return wrapper;
}

beforeEach(() => {
  setActivePinia(createPinia());
  resetModalStack();
  resetBodyScrollLock();
  vi.clearAllMocks();
});

afterEach(() => {
  opened.splice(0).forEach((w) => w.unmount());
  resetBodyScrollLock();
});

describe('ApplicationDetail: блокировка скролла фона', () => {
  it('открытие панели блокирует прокрутку документа', () => {
    mountDetail();

    expect(document.body.style.overflow).toBe('hidden');
    expect(document.documentElement.style.overflow).toBe('hidden');
  });

  it('закрытие (unmount) снимает блокировку', () => {
    const wrapper = mountDetail();

    wrapper.unmount();
    opened.pop();

    expect(document.body.style.overflow).toBe('');
    expect(document.documentElement.style.overflow).toBe('');
  });

  it('две открытые панели не снимают чужую блокировку при закрытии одной', () => {
    const first = mountDetail();
    mountDetail();

    first.unmount();
    opened.splice(opened.indexOf(first), 1);

    expect(document.body.style.overflow).toBe('hidden');
    expect(document.documentElement.style.overflow).toBe('hidden');
  });
});
