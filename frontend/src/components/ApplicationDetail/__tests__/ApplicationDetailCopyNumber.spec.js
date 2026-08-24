import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import ApplicationDetail from '../ApplicationDetail.vue';
import { useDeletionsStore } from '@/stores/deletions';

/**
 * Номер заявки копируется кликом по самому номеру - тем же жестом, что в списке
 * заявок. До этого номер в детали был обычным текстом: выделять его мышью в
 * модалке неудобно, а пересказывать по телефону приходится постоянно.
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
  application_number: '№ 20260815/001',
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

let writeText;

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  writeText = vi.fn().mockResolvedValue(undefined);
  Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
});

afterEach(() => opened.splice(0).forEach((w) => w.unmount()));

describe('ApplicationDetail: копирование номера заявки', () => {
  it('клик по номеру кладёт его в буфер', async () => {
    const wrapper = mountDetail();
    await wrapper.find('[data-testid="app-detail-number"]').trigger('click');

    expect(writeText).toHaveBeenCalledWith('№ 20260815/001');
  });

  it('номер копируется и с клавиатуры - у него роль кнопки и фокус', async () => {
    const wrapper = mountDetail();
    const number = wrapper.find('[data-testid="app-detail-number"]');
    expect(number.attributes('role')).toBe('button');
    expect(number.attributes('tabindex')).toBe('0');

    await number.trigger('keydown.enter');
    expect(writeText).toHaveBeenCalledWith('№ 20260815/001');
  });

  it('удачное копирование подтверждается уведомлением с номером', async () => {
    const wrapper = mountDetail();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await wrapper.find('[data-testid="app-detail-number"]').trigger('click');
    await Promise.resolve();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ bold: '№ 20260815/001', type: 'success' }));
  });

  it('буфер недоступен - человек видит ошибку, а не тишину', async () => {
    writeText.mockRejectedValue(new Error('denied'));
    const wrapper = mountDetail();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await wrapper.find('[data-testid="app-detail-number"]').trigger('click');
    await Promise.resolve();
    await Promise.resolve();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
  });

  it('кликабелен только номер, а не весь заголовок', () => {
    const SFC = readFileSync(resolve(__dirname, '../ApplicationDetail.vue'), 'utf8');
    const title = SFC.match(/<h3 class="detail-title">([\s\S]*?)<\/h3>/);

    expect(title).not.toBeNull();
    expect(title[1]).toMatch(/Заявка <span/);
    expect(SFC).toMatch(/\.detail-number\s*\{[^}]*cursor:\s*pointer/);
  });
});
