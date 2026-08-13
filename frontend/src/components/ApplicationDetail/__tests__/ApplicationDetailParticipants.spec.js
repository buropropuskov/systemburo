import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

/**
 * Кнопка «Получатели» в шапке карточки заявки (#1952). Своего гейта у неё нет:
 * список участников отдаётся тому, кому видна сама заявка, а она уже открыта -
 * поэтому кнопка проверяется и там, где «Переслать» не положена (режим кабинета,
 * пользователь без прав Центра).
 */

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/api/applications', () => ({
  markAsRead: vi.fn().mockResolvedValue(undefined),
  getApplicationSupplements: vi.fn().mockResolvedValue([]),
  getApplicationParticipants: vi.fn().mockResolvedValue([]),
}));

import ApplicationDetail from '../ApplicationDetail.vue';
import ApplicationParticipantsModal from '../ApplicationParticipantsModal.vue';

const PARTICIPANTS_BTN = '[data-testid="app-detail-button-participants"]';
const FORWARD_BTN = '[data-testid="app-detail-button-forward"]';

const SFC = readFileSync(resolve(__dirname, '../ApplicationDetail.vue'), 'utf8');

/** Тело ПЕРВОГО правила для селектора без переносов и комментариев (см. паттерн
 *  в ApplicationDetailHeaderMobileAlignment.spec.js). */
function rule(src, selector) {
  const stripped = src.replace(/\/\*[\s\S]*?\*\//g, ' ');
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const found = stripped.match(new RegExp(`${escaped}\\s*\\{([^}]*)\\}`));
  return found ? found[1].replace(/\s+/g, ' ').trim() : null;
}

function mountDetail(props = {}) {
  return shallowMount(ApplicationDetail, {
    props: {
      application: { id: 7, application_number: 'A-7', status: 'Непрочитано' },
      currentUserId: 1,
      mode: 'center',
      ...props,
    },
  });
}

describe('ApplicationDetail - кнопка «Получатели» (#1952)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
  });

  it('кнопка есть у того, кто заявку открыл, даже без права пересылки', async () => {
    const wrapper = mountDetail();
    await wrapper.vm.$nextTick();

    expect(wrapper.find(PARTICIPANTS_BTN).exists()).toBe(true);
    // Контроль: «Переслать» этому пользователю не положена - значит кнопка получателей
    // показана не заодно с ней, а собственным условием.
    expect(wrapper.find(FORWARD_BTN).exists()).toBe(false);
  });

  it('кнопка есть и в кабинете, где режим не «Центр»', async () => {
    const wrapper = mountDetail({ mode: 'my' });
    await wrapper.vm.$nextTick();

    expect(wrapper.find(PARTICIPANTS_BTN).exists()).toBe(true);
  });

  it('до клика окно закрыто, клик его открывает', async () => {
    const wrapper = mountDetail();
    await wrapper.vm.$nextTick();

    const modal = wrapper.findComponent(ApplicationParticipantsModal);
    expect(modal.props('show')).toBe(false);
    expect(modal.props('applicationId')).toBe(7);

    await wrapper.find(PARTICIPANTS_BTN).trigger('click');

    expect(wrapper.findComponent(ApplicationParticipantsModal).props('show')).toBe(true);
  });

  it('окно закрывает себя само - деталь заявки остаётся открытой', async () => {
    const wrapper = mountDetail();
    await wrapper.vm.$nextTick();
    await wrapper.find(PARTICIPANTS_BTN).trigger('click');

    wrapper.findComponent(ApplicationParticipantsModal).vm.$emit('close');
    await wrapper.vm.$nextTick();

    expect(wrapper.findComponent(ApplicationParticipantsModal).props('show')).toBe(false);
    expect(wrapper.emitted('close')).toBeFalsy();
  });

  it('title дублирует подпись - кнопка объясняет себя даже без текста', async () => {
    const wrapper = mountDetail();
    await wrapper.vm.$nextTick();

    expect(wrapper.find(PARTICIPANTS_BTN).attributes('title')).toBe('Получатели');
  });
});

// Ряд заголовка (title/дата/"Переслать"/"Получатели") тесен уже на десктопе, не только
// на мобилке: полная пилюля "Получатели" первой не помещается и переносится одна,
// оторванно от даты - владелец увидел это на 1440/1100 при исправной раскладке на
// 1920/1280 (замерено в браузере). jsdom не считает @container, поэтому эффект
// стережём чтением исходника: кнопка сжимается в тот же кружок-иконку, что и на
// мобилке, но по ширине самого ряда, а не окна.
describe('ApplicationDetail - "Получатели" сжимается в иконку на тесном ряду заголовка (десктоп)', () => {
  it('.detail-title-row - контейнер по инлайн-размеру для @container ниже', () => {
    expect(rule(SFC, '.detail-title-row')).toMatch(/container-type:\s*inline-size/);
  });

  it('@container сворачивает пилюлю в кружок-иконку и прячет подпись', () => {
    expect(SFC).toMatch(/@container\s*\([^)]*\)\s*\{[^}]*\.participants-btn\s*\{[^}]*border-radius:\s*50%/s);
    expect(SFC).toMatch(/@container\s*\([^)]*\)\s*\{[\s\S]*?\.participants-btn__text\s*\{\s*display:\s*none;/);
  });
});
