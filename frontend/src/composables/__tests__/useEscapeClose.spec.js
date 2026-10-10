import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, enableAutoUnmount } from '@vue/test-utils';
import { ref, nextTick } from 'vue';

// Окна снимаются после каждого кейса: их слушатели висят на document, и живое окно
// из прошлого теста перехватывало Escape у нового - стопка о нём уже не знала.
enableAutoUnmount(afterEach);

import { useEscapeClose } from '../useEscapeClose';
import { resetModalStack, markEscapeHandled } from '@/utils/modalStack';

// Окно на композабле: слой задаётся пропом, закрытие эмитится наружу.
function makeModal(zIndex) {
  return {
    props: { show: { type: Boolean, default: true } },
    emits: ['close'],
    setup(props, { emit }) {
      useEscapeClose(() => emit('close'), () => props.show, zIndex);
      return () => null;
    },
  };
}

function pressEscape() {
  document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
}

describe('useEscapeClose', () => {
  beforeEach(() => resetModalStack());

  it('одно окно закрывается по Escape', () => {
    const wrapper = mount(makeModal(10001));
    pressEscape();

    expect(wrapper.emitted('close')).toHaveLength(1);
  });

  it('закрытое окно (guard false) нажатие не берёт', () => {
    const wrapper = mount(makeModal(10001), { props: { show: false } });
    pressEscape();

    expect(wrapper.emitted('close')).toBeUndefined();
  });

  it('одно нажатие закрывает только верхний слой, нижний остаётся', () => {
    // Панель заявки (10002) и карточка элемента поверх неё (10003) - ровно тот случай,
    // на котором один Escape закрывал оба окна разом.
    const lower = mount(makeModal(10002));
    const upper = mount(makeModal(10003));

    pressEscape();

    expect(upper.emitted('close')).toHaveLength(1);
    expect(lower.emitted('close')).toBeUndefined();
  });

  it('после закрытия верхнего следующее нажатие закрывает нижний', async () => {
    const lower = mount(makeModal(10002));
    const upper = mount(makeModal(10003));

    pressEscape();
    upper.unmount();

    pressEscape();
    expect(lower.emitted('close')).toHaveLength(1);
  });

  it('после смены источника карточки учитывает её новый слой', async () => {
    const layer = ref(10001);
    const card = mount({ emits: ['close'], setup(_, { emit }) {
      useEscapeClose(() => emit('close'), () => true, () => layer.value);
      return () => null;
    } });
    const history = mount(makeModal(13000));
    layer.value = 14000;
    await nextTick();
    pressEscape();
    expect(card.emitted('close')).toHaveLength(1);
    expect(history.emitted('close')).toBeUndefined();
  });

  it('нажатие, уже разобранное другим слоем, окно не берёт', () => {
    const wrapper = mount(makeModal(10001));
    const event = new KeyboardEvent('keydown', { key: 'Escape' });

    // Слой выше, живущий вне композабла (BaseModal), помечает нажатие своим.
    const marker = vi.fn(() => markEscapeHandled(event));
    document.addEventListener('keydown', marker, { capture: true });
    document.dispatchEvent(event);
    document.removeEventListener('keydown', marker, { capture: true });

    expect(wrapper.emitted('close')).toBeUndefined();
  });
});
