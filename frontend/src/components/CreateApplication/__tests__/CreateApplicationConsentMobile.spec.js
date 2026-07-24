import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CreateApplication from '../CreateApplication.vue';

// #1380: на телефоне ссылка "согласие" открывает модалку с содержимым документа
// (нативный <embed> PDF на мобилке пуст), на десктопе - нативный переход на /data-processing.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/stores/auth', () => ({
  useAuthStore: vi.fn().mockReturnValue({ token: 'test-token' }),
}));
// DataProcessingModal -> PdfDocumentViewer статически тянет ?worker-конструктор воркера pdf.js;
// мок делает спек герметичным (children тут и так застаблены shallowMount).
vi.mock('pdfjs-dist/build/pdf.worker.min.mjs?worker', () => ({ default: class {} }));

beforeEach(() => {
  setActivePinia(createPinia());
  localStorage.clear();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

async function mountApp() {
  const w = shallowMount(CreateApplication);
  await flushPromises();
  return w;
}

describe('CreateApplication - согласие на мобилке (#1380)', () => {
  it('на телефоне: клик гасит нативный переход и открывает модалку', async () => {
    vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: true })));
    const w = await mountApp();
    const e = { preventDefault: vi.fn() };

    w.vm.onConsentClick(e);

    expect(e.preventDefault).toHaveBeenCalledTimes(1);
    expect(w.vm.showConsentModal).toBe(true);
  });

  it('на десктопе: клик не подавляется (переход по href), модалка не открывается', async () => {
    vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: false })));
    const w = await mountApp();
    const e = { preventDefault: vi.fn() };

    w.vm.onConsentClick(e);

    expect(e.preventDefault).not.toHaveBeenCalled();
    expect(w.vm.showConsentModal).toBe(false);
  });
});
