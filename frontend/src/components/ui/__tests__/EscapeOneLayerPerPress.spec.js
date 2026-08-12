import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { defineComponent, h } from 'vue';
import BaseModal from '../BaseModal.vue';
import { resetModalStack, setModalOpen, releaseModal, isTopModal, isEscapeHandled, markEscapeHandled } from '@/utils/modalStack';

/**
 * Одно нажатие Escape закрывает ровно один слой - независимо от того, в каком порядке
 * слушатели встали на document.
 *
 * Слой, ответивший на нажатие, снимается со стопки только следующим тиком, а слушатели
 * одного события идут подряд: на стенде это давало закрытие списка получателей вместе с
 * панелью заявки под ним, хотя стопка была выстроена верно. В jsdom порядок оказался
 * другим, и тест на стопку эту дыру не видел - поэтому проверяем оба порядка подписки.
 */

const DETAIL_LAYER = 10002;

/** Панель с обработчиком Escape - как ApplicationDetail. */
const Panel = defineComponent({
  props: { modalShown: { type: Boolean, default: false }, subscribeFirst: { type: Boolean, default: false } },
  emits: ['close', 'close-modal'],
  mounted() {
    setModalOpen(this, true, DETAIL_LAYER);
    document.addEventListener('keydown', this.onEscape);
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.onEscape);
    releaseModal(this);
  },
  methods: {
    onEscape(e) {
      if (e.key !== 'Escape') return;
      if (isEscapeHandled(e)) return;
      if (!isTopModal(this)) return;
      markEscapeHandled(e);
      this.$emit('close');
    },
  },
  render() {
    return h('div', [
      h(BaseModal, {
        show: this.modalShown,
        zIndex: 12000,
        title: 'Получатели',
        onClose: () => this.$emit('close-modal'),
      }),
    ]);
  },
});

function pressEscape() {
  document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
}

// Смонтированные слои продолжают слушать document, пока живы: без разбора после
// каждого кейса чужая панель перехватывала бы нажатие следующего.
const mounted = [];
const track = (wrapper) => { mounted.push(wrapper); return wrapper; };

beforeEach(() => resetModalStack());
afterEach(() => { mounted.splice(0).forEach((w) => w.unmount()); });

describe('Escape закрывает один слой за нажатие', () => {
  it('окно поверх панели забирает нажатие себе', () => {
    const wrapper = track(mount(Panel, { props: { modalShown: true }, global: { stubs: { teleport: true } } }));

    pressEscape();

    expect(wrapper.emitted('close-modal'), 'закрывается окно').toHaveLength(1);
    expect(wrapper.emitted('close'), 'панель под ним остаётся').toBeUndefined();
  });

  it('панель, подписавшаяся раньше окна, всё равно не закрывается вместе с ним', () => {
    // Обратный порядок подписки: панель уже слушает document, окно открывается позже.
    // Именно этот порядок и ломался на стенде.
    const wrapper = track(mount(Panel, { props: { modalShown: false }, global: { stubs: { teleport: true } } }));
    const late = track(mount(BaseModal, {
      props: { show: true, title: 'Карточка участника', zIndex: 12500 },
      global: { stubs: { teleport: true } },
    }));

    pressEscape();

    expect(late.emitted('close'), 'верхнее окно закрывается').toHaveLength(1);
    expect(wrapper.emitted('close'), 'панель не закрывается тем же нажатием').toBeUndefined();
  });

  it('слой, снявшийся со стопки в том же нажатии, не отдаёт его нижнему', () => {
    // Ровно то, что видно на стенде: верхний слой ответил на Escape и снялся со
    // стопки прежде, чем до нажатия добрался слушатель панели. По одной стопке панель
    // в этот момент выглядит верхней - удерживает её только пометка на событии.
    const top = {};
    setModalOpen(top, true, 12000);
    const topHandler = (e) => {
      if (e.key !== 'Escape' || isEscapeHandled(e)) return;
      markEscapeHandled(e);
      releaseModal(top);
    };
    // Подписка раньше панели: в приложении окно - её ребёнок, а дочерние монтируются
    // первыми, поэтому их слушатель на document встаёт раньше родительского.
    document.addEventListener('keydown', topHandler);
    const wrapper = track(mount(Panel, { props: { modalShown: false }, global: { stubs: { teleport: true } } }));

    pressEscape();
    document.removeEventListener('keydown', topHandler);

    expect(wrapper.emitted('close'), 'панель не должна закрыться тем же нажатием').toBeUndefined();
  });

  it('когда поверх ничего нет, панель закрывается сама', () => {
    const wrapper = track(mount(Panel, { props: { modalShown: false }, global: { stubs: { teleport: true } } }));

    pressEscape();

    expect(wrapper.emitted('close')).toHaveLength(1);
  });

  it('следующее нажатие - новое событие, его забирает слой ниже', async () => {
    const wrapper = track(mount(Panel, { props: { modalShown: true }, global: { stubs: { teleport: true } } }));

    pressEscape();
    expect(wrapper.emitted('close-modal'), 'первым нажатием закрылось окно').toHaveLength(1);

    // Родитель снял показ окна - слой ушёл со стопки, панель стала верхней.
    await wrapper.setProps({ modalShown: false });
    pressEscape();

    expect(wrapper.emitted('close'), 'вторым нажатием закрывается панель').toHaveLength(1);
  });
});
