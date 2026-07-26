import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { useThemeStore } from '../theme';
import { useDeletionsStore } from '../deletions';
import { getTheme, saveTheme } from '@/api/theme';
import { THEME_STORAGE_KEY } from '@/utils/theme';

vi.mock('@/api/theme', () => ({
  getTheme: vi.fn(),
  saveTheme: vi.fn(),
}));

/** Разрешает промис отложенно - для проверки гонки ответа с выбором юзера. */
function deferred() {
  let resolve;
  const promise = new Promise((r) => { resolve = r; });
  return { promise, resolve };
}

describe('theme store', () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.removeAttribute('data-theme');
    setActivePinia(createPinia());
    getTheme.mockReset();
    saveTheme.mockReset();
    saveTheme.mockResolvedValue({ message: 'ok' });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('стартует со светлой и применяет её к <html>', () => {
    const store = useThemeStore();
    expect(store.current).toBe('light');
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
  });

  it('поднимает сохранённую тему из localStorage', () => {
    localStorage.setItem(THEME_STORAGE_KEY, 'dark');
    const store = useThemeStore();

    expect(store.current).toBe('dark');
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
  });

  it('setTheme применяет тему, пишет в localStorage и сохраняет в профиль', async () => {
    const store = useThemeStore();
    await store.setTheme('corporate-orange');

    expect(store.current).toBe('corporate-orange');
    expect(document.documentElement.getAttribute('data-theme')).toBe('corporate-orange');
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('corporate-orange');
    expect(saveTheme).toHaveBeenCalledWith('corporate-orange');
  });

  it('setTheme игнорирует неизвестную тему и повторный выбор текущей', async () => {
    const store = useThemeStore();

    await store.setTheme('neon-hacker');
    expect(store.current).toBe('light');

    await store.setTheme('light');
    expect(saveTheme).not.toHaveBeenCalled();
  });

  // Тема на экране уже переключилась - откатывать её из-за сетевой ошибки нельзя,
  // но юзер должен знать, что на другое устройство выбор не переедет.
  it('setTheme при ошибке сохранения оставляет тему и показывает уведомление', async () => {
    const store = useThemeStore();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');
    saveTheme.mockRejectedValue(new Error('503 Service Unavailable'));

    await store.setTheme('dark');

    expect(store.current).toBe('dark');
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      type: 'error',
      bold: '503 Service Unavailable',
    }));
  });

  it('syncFromServer применяет тему из профиля', async () => {
    getTheme.mockResolvedValue({ theme: 'official-blue' });
    const store = useThemeStore();

    await store.syncFromServer();

    expect(store.current).toBe('official-blue');
    expect(document.documentElement.getAttribute('data-theme')).toBe('official-blue');
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('official-blue');
  });

  // Пустая тема в профиле = юзер не выбирал. На общем компьютере он не должен
  // унаследовать чужое оформление, оставшееся в localStorage.
  it('syncFromServer сбрасывает на светлую, если в профиле темы нет', async () => {
    localStorage.setItem(THEME_STORAGE_KEY, 'dark');
    getTheme.mockResolvedValue({ theme: null });
    const store = useThemeStore();
    expect(store.current).toBe('dark');

    await store.syncFromServer();

    expect(store.current).toBe('light');
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('light');
  });

  it('syncFromServer игнорирует неизвестную тему из профиля', async () => {
    getTheme.mockResolvedValue({ theme: 'neon-hacker' });
    const store = useThemeStore();

    await store.syncFromServer();

    expect(store.current).toBe('light');
  });

  it('syncFromServer при ошибке сети оставляет локальную тему', async () => {
    localStorage.setItem(THEME_STORAGE_KEY, 'dark-orange');
    getTheme.mockRejectedValue(new Error('offline'));
    const store = useThemeStore();

    await store.syncFromServer();

    expect(store.current).toBe('dark-orange');
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark-orange');
  });

  // Гонка: юзер кликает тему, пока летит ответ /users/me/theme. Устаревший ответ
  // не должен откатывать свежий выбор (last-resolve-wins на общий ref).
  it('syncFromServer не перебивает выбор, сделанный во время запроса', async () => {
    const pending = deferred();
    getTheme.mockReturnValue(pending.promise);
    const store = useThemeStore();

    const sync = store.syncFromServer();
    await store.setTheme('dark');
    pending.resolve({ theme: 'official-blue' });
    await sync;

    expect(store.current).toBe('dark');
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
  });

  // Заливка от клика (#1415): View Transitions зовёт коллбэк не сразу, а после
  // снятия кадра. Профиль обязан получить ВЫБРАННУЮ тему - раньше сохранялся
  // `current.value`, то есть прошлая тема, и выбор не переживал перезагрузку.
  it('с заливкой сохраняет в профиль выбранную тему, а не прошлую', async () => {
    document.startViewTransition = (callback) => {
      setTimeout(callback, 0);
      return { ready: Promise.resolve(), skipTransition: () => {} };
    };
    document.documentElement.animate = () => ({ finished: Promise.resolve() });
    const store = useThemeStore();

    await store.setTheme('dark-orange', { x: 10, y: 20 });

    expect(saveTheme).toHaveBeenCalledWith('dark-orange');
    await vi.waitFor(() => expect(store.current).toBe('dark-orange'));
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark-orange');
    delete document.startViewTransition;
    delete document.documentElement.animate;
  });
});
