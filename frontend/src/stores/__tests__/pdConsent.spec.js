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
  hash: 'a'.repeat(64),
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

  // Ревью поймало: force, пущенный поверх летящего запроса, переиспользовал его
  // промис - и ответ прошлого пользователя дописывался в сессию следующего.
  it('force пробивает летящий запрос, поздний ответ прежнего не применяется', async () => {
    const pending = {};
    getConsentGate
      .mockImplementationOnce(() => new Promise((resolve) => { pending.first = resolve; }))
      .mockResolvedValueOnce({ required: false, version: 9, text: '<p>Новый юзер</p>', document: null });
    const store = usePDConsentStore();

    const stale = store.refresh();
    const fresh = store.refresh(true);
    await fresh;

    expect(getConsentGate).toHaveBeenCalledTimes(2);
    expect(store.required).toBe(false);
    expect(store.version).toBe(9);

    // Ответ первого запроса приходит последним и обязан быть проигнорирован.
    pending.first(GATE_REQUIRED);
    await stale;
    expect(store.required).toBe(false);
    expect(store.version).toBe(9);
  });

  it('reset во время летящего запроса не даёт ему воскресить состояние', async () => {
    let release;
    getConsentGate.mockImplementation(
      () => new Promise((resolve) => { release = () => resolve(GATE_REQUIRED); }),
    );
    const store = usePDConsentStore();

    const pending = store.refresh();
    store.reset();
    release();
    await pending;

    expect(store.resolved).toBe(false);
    expect(store.required).toBe(false);
    expect(store.html).toBe('');
  });

  it('accept побеждает летящий запрос состояния - окно не возвращается', async () => {
    let release;
    getConsentGate.mockImplementation(
      () => new Promise((resolve) => { release = () => resolve(GATE_REQUIRED); }),
    );
    acceptConsent.mockResolvedValue({ required: false, version: 2, text: '<p>Текст согласия</p>' });
    const store = usePDConsentStore();

    const pending = store.refresh();
    await store.accept();
    expect(store.required).toBe(false);

    release();
    await pending;
    expect(store.required).toBe(false);
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

    expect(acceptConsent).toHaveBeenCalledWith({ version: 2, hash: GATE_REQUIRED.hash });
    expect(store.required).toBe(false);
    expect(store.resolved).toBe(true);
  });

  it('accept пробрасывает ошибку наверх - молчаливого клика быть не должно', async () => {
    acceptConsent.mockRejectedValue(new Error('Пользователь не найден'));
    const store = usePDConsentStore();

    await expect(store.accept()).rejects.toThrow('Пользователь не найден');
  });

  it('конфликт перечитывает текст той же версии без повторного подтверждения', async () => {
    getConsentGate.mockResolvedValueOnce(GATE_REQUIRED)
      .mockResolvedValueOnce({ ...GATE_REQUIRED, hash: 'b'.repeat(64), text: '<p>Новый текст</p>' });
    acceptConsent.mockRejectedValueOnce(Object.assign(new Error('Документ изменился'), { status: 409 }));
    const store = usePDConsentStore();
    await store.refresh();
    await expect(store.accept()).rejects.toThrow('Документ изменился');
    expect(store.required).toBe(true);
    expect(store.hash).toBe('b'.repeat(64));
    expect(store.html).toBe('<p>Новый текст</p>');
    expect(acceptConsent).toHaveBeenCalledTimes(1);
  });

  it('ошибка перечтения после конфликта блокирует старый текст; повтор только загружает документ', async () => {
    getConsentGate.mockResolvedValueOnce(GATE_REQUIRED).mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce({ ...GATE_REQUIRED, hash: 'b'.repeat(64) });
    acceptConsent.mockRejectedValueOnce(Object.assign(new Error('Документ изменился'), { status: 409 }));
    const store = usePDConsentStore();
    await store.refresh();
    await expect(store.accept()).rejects.toThrow('Не удалось загрузить');
    expect(store.required).toBe(true);
    expect(store.needsReload).toBe(true);
    expect(store.hash).toBe('');
    expect(store.html).toBe('');
    await store.accept();
    expect(store.needsReload).toBe(false);
    expect(store.required).toBe(true);
    expect(acceptConsent).toHaveBeenCalledTimes(1);
  });

  it.each([false, true])('поздний ответ подтверждения прежней сессии игнорируется, конфликт=%s', async (conflict) => {
    let finish;
    getConsentGate.mockResolvedValueOnce(GATE_REQUIRED).mockResolvedValueOnce({ ...GATE_REQUIRED, version: 9 });
    acceptConsent.mockImplementationOnce(() => new Promise((resolve, reject) => {
      finish = () => conflict ? reject(Object.assign(new Error('conflict'), { status: 409 })) : resolve({ required: false });
    }));
    const store = usePDConsentStore();
    await store.refresh();
    const previous = store.accept();
    store.reset();
    await store.refresh(true);
    finish();
    await previous;
    expect(store.version).toBe(9);
    expect(store.required).toBe(true);
    expect(getConsentGate).toHaveBeenCalledTimes(2);
  });

  it.each([false, true])('после фонового 403 и успешного POST сверяет итог по GET, required=%s', async (required) => {
    let completePost;
    let completeOldRead;
    getConsentGate.mockResolvedValueOnce(GATE_REQUIRED)
      .mockImplementationOnce(() => new Promise((resolve) => { completeOldRead = resolve; }))
      .mockResolvedValueOnce({ ...GATE_REQUIRED, required });
    acceptConsent.mockImplementationOnce(() => new Promise((resolve) => { completePost = resolve; }));
    const store = usePDConsentStore();
    await store.refresh();
    const accepting = store.accept();
    store.markRequiredFromResponse();
    completeOldRead(GATE_REQUIRED);
    await store.refresh();
    expect(store.required).toBe(true);
    completePost({ ...GATE_REQUIRED, required: false });
    await accepting;
    expect(store.required).toBe(required);
    expect(store.needsReload).toBe(false);
    expect(getConsentGate).toHaveBeenCalledTimes(3);
    expect(acceptConsent).toHaveBeenCalledTimes(1);
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
