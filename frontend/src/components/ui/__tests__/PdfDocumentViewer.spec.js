import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const getDocument = vi.fn();
const GlobalWorkerOptions = { workerPort: null };

vi.mock('pdfjs-dist', () => ({
  getDocument: (...a) => getDocument(...a),
  GlobalWorkerOptions,
}));
// ?worker отдаёт конструктор Worker - подменяем безобидной заглушкой.
vi.mock('pdfjs-dist/build/pdf.worker.min.mjs?worker', () => ({
  default: class {
    postMessage() {}

    terminate() {}

    addEventListener() {}

    removeEventListener() {}
  },
}));

import PdfDocumentViewer from '../PdfDocumentViewer.vue';

function makePdf(numPages) {
  return {
    numPages,
    getPage: vi.fn(async () => ({
      getViewport: ({ scale }) => ({ width: 100 * scale, height: 200 * scale }),
      render: () => ({ promise: Promise.resolve() }),
    })),
    destroy: vi.fn(),
  };
}

function makeBlob() {
  return { arrayBuffer: async () => new ArrayBuffer(8) };
}

describe('PdfDocumentViewer', () => {
  beforeEach(() => {
    getDocument.mockReset();
    GlobalWorkerOptions.workerPort = null;
  });

  it('рендерит по canvas на каждую страницу PDF и настраивает воркер', async () => {
    getDocument.mockReturnValue({ promise: Promise.resolve(makePdf(3)) });
    const wrapper = mount(PdfDocumentViewer, { props: { blob: makeBlob() } });
    await flushPromises();
    await flushPromises();

    expect(getDocument).toHaveBeenCalledTimes(1);
    expect(GlobalWorkerOptions.workerPort).toBeTruthy();
    expect(wrapper.findAll('canvas.pdf-viewer__page')).toHaveLength(3);
    expect(wrapper.emitted('loaded')?.[0]).toEqual([3]);
  });

  it('показывает ошибку и эмитит error, если PDF не открылся', async () => {
    getDocument.mockReturnValue({ promise: Promise.reject(new Error('bad pdf')) });
    const wrapper = mount(PdfDocumentViewer, { props: { blob: makeBlob() } });
    await flushPromises();
    await flushPromises();

    expect(wrapper.text()).toContain('Не удалось показать документ');
    expect(wrapper.emitted('error')).toBeTruthy();
    expect(wrapper.findAll('canvas.pdf-viewer__page')).toHaveLength(0);
  });

  it('без blob ничего не грузит', async () => {
    const wrapper = mount(PdfDocumentViewer, { props: { blob: null } });
    await flushPromises();

    expect(getDocument).not.toHaveBeenCalled();
    expect(wrapper.findAll('canvas').length).toBe(0);
  });

  it('перерендеривает при смене blob и не плодит canvas от старого документа', async () => {
    getDocument.mockReturnValue({ promise: Promise.resolve(makePdf(2)) });
    const wrapper = mount(PdfDocumentViewer, { props: { blob: makeBlob() } });
    await flushPromises();
    await flushPromises();
    expect(wrapper.findAll('canvas.pdf-viewer__page')).toHaveLength(2);

    getDocument.mockReturnValue({ promise: Promise.resolve(makePdf(1)) });
    await wrapper.setProps({ blob: makeBlob() });
    await flushPromises();
    await flushPromises();

    expect(getDocument).toHaveBeenCalledTimes(2);
    expect(wrapper.findAll('canvas.pdf-viewer__page')).toHaveLength(1);
  });
});
