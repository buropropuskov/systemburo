import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import ApplicationDetail from '../ApplicationDetail.vue';
import BaseModal from '@/components/ui/BaseModal.vue';
import { resetModalStack } from '@/utils/modalStack';

/**
 * Панель заявки закрывается по Escape - как окна проекта. До этого крестик был
 * единственным способом: комментарий у `closeDetail` обещал «крестик, Esc, свайп», но
 * обработчика клавиши не было ни в панели, ни у её потребителей.
 *
 * Вторая половина проверки - стопка: пока поверх открыто окно, Escape достаётся ему,
 * а не панели под ним.
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

/** Ждём, пока панель доиграет уход и сообщит о закрытии (transition 200 мс). */
function waitForClose() {
  return new Promise((resolve) => setTimeout(resolve, 260));
}

function pressEscape() {
  document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
}

// Панель и окна слушают document, пока смонтированы: слой из прошлого кейса иначе
// забирает Escape следующего.
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
  vi.clearAllMocks();
});

afterEach(() => opened.splice(0).forEach((w) => w.unmount()));

describe('ApplicationDetail: закрытие по Escape', () => {
  it('Escape закрывает панель заявки', async () => {
    const wrapper = mountDetail();

    pressEscape();
    // Панель уходит с анимацией и сообщает о закрытии родителю после неё - иначе
    // размонтирование обрывало бы уход на середине.
    await waitForClose();

    expect(wrapper.emitted('close'), 'панель должна попросить родителя себя закрыть').toHaveLength(1);
  });

  it('пока поверх открыто окно, Escape достаётся окну, а не панели', async () => {
    const wrapper = mountDetail();
    const modal = mount(BaseModal, {
      props: { show: true, title: 'Получатели заявки', zIndex: 12000 },
      global: { stubs: { teleport: true } },
    });
    opened.push(modal);

    pressEscape();
    await waitForClose();

    expect(modal.emitted('close'), 'верхнее окно закрывается').toHaveLength(1);
    expect(wrapper.emitted('close'), 'панель под ним остаётся открытой').toBeUndefined();

    modal.unmount();
    pressEscape();
    await waitForClose();

    expect(wrapper.emitted('close'), 'после закрытия окна Escape доходит до панели').toHaveLength(1);
  });
});
