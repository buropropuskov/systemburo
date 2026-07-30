import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';

vi.mock('@/api/pdConsent', () => ({
  getConsentGate: vi.fn(),
  acceptConsent: vi.fn(),
}));

import { getConsentGate, acceptConsent } from '@/api/pdConsent';
import { usePDConsentStore } from '../pdConsent';

const GATE_REQUIRED = {
  required: true,
  version: 2,
  text: '<p>Текст согласия</p>',
  document: { stored_name: 'doc.pdf', file_name: 'Согласие.pdf' },
};

describe('stores/pdConsent (#1567)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getConsentGate.mockReset();
    acceptConsent.mockReset();
  });

  it('refresh раскладывает состояние гейта и поднимает resolved', async () => {
    getConsentGate.mockResolvedValue(GATE_REQUIRED);
    const store = usePDConsentStore();

    await store.refresh();

    expect(store.resolved).toBe(true);
    expect(store.required).toBe(true);
    expect(store.version).toBe(2);
    expect(store.html).toBe('<p>Текст согласия</p>');
    expect(store.docMeta).toEqual(GATE_REQUIRED.document);
  });

  it('сетевая ошибка НЕ поднимает resolved - окно на догадке не показываем', async () => {
    getConsentGate.mockRejectedValue(new Error('Сеть недоступна'));
    const store = usePDConsentStore();

    await store.refresh();

    expect(store.resolved).toBe(false);
    expect(store.required).toBe(false);
  });

  it('конкурентные refresh делят один промис (in-flight дедуп)', async () => {
    let release;
    getConsentGate.mockImplementation(
      () => new Promise((resolve) => { release = () => resolve(GATE_REQUIRED); }),
    );
    const store = usePDConsentStore();

    const first = store.refresh();
    const second = store.refresh();
    release();
    await Promise.all([first, second]);

    expect(getConsentGate).toHaveBeenCalledTimes(1);
    expect(store.required).toBe(true);
  });

  it('повторный refresh без force не ходит на сервер, с force - ходит', async () => {
    getConsentGate.mockResolvedValue(GATE_REQUIRED);
    const store = usePDConsentStore();

    await store.refresh();
    await store.refresh();
    expect(getConsentGate).toHaveBeenCalledTimes(1);

    await store.refresh(true);
    expect(getConsentGate).toHaveBeenCalledTimes(2);
  });

  it('accept снимает требование по ответу сервера', async () => {
    getConsentGate.mockResolvedValue(GATE_REQUIRED);
    acceptConsent.mockResolvedValue({ required: false, version: 2, text: '<p>Текст согласия</p>' });
    const store = usePDConsentStore();
    await store.refresh();

    await store.accept();

    expect(store.required).toBe(false);
    expect(store.resolved).toBe(true);
  });

  it('accept пробрасывает ошибку наверх - молчаливого клика быть не должно', async () => {
    acceptConsent.mockRejectedValue(new Error('Пользователь не найден'));
    const store = usePDConsentStore();

    await expect(store.accept()).rejects.toThrow('Пользователь не найден');
  });

  it('markRequiredFromResponse поднимает флаг сразу и перечитывает редакцию', async () => {
    getConsentGate.mockResolvedValue({ ...GATE_REQUIRED, version: 5 });
    const store = usePDConsentStore();

    store.markRequiredFromResponse();
    expect(store.required).toBe(true);
    expect(store.resolved).toBe(true);

    await Promise.resolve();
    await Promise.resolve();
    expect(getConsentGate).toHaveBeenCalledTimes(1);
    expect(store.version).toBe(5);
  });

  it('reset чистит состояние - следующий юзер на устройстве спросит своё', async () => {
    getConsentGate.mockResolvedValue(GATE_REQUIRED);
    const store = usePDConsentStore();
    await store.refresh();

    store.reset();

    expect(store.resolved).toBe(false);
    expect(store.required).toBe(false);
    expect(store.version).toBe(0);
    expect(store.html).toBe('');
    expect(store.docMeta).toBeNull();
  });
});
