<template>
  <div
    class="del-stack"
    role="status"
    aria-live="polite"
  >
    <transition-group name="del">
      <div
        v-for="item in store.items"
        :key="item.id"
        class="del-card"
        :class="'del-card--' + item.type"
        @click="store.dismiss(item.id)"
      >
        <div
          v-if="item.title"
          class="del-title"
          :class="'del-title--' + item.type"
        >
          {{ item.title }}
        </div>
        <div class="del-row">
          <span class="del-text">
            {{ item.prefix }}<strong>{{ item.bold }}</strong>{{ item.suffix }}
          </span>
          <button
            v-if="item.showUndo"
            class="del-undo"
            @click.stop="store.undo(item.id)"
          >
            Отменить
          </button>
        </div>
        <div class="del-track">
          <div
            class="del-fill"
            :style="{ width: item.progress + '%', background: barColorFor(item) }"
          />
        </div>
      </div>
    </transition-group>
  </div>
</template>

<script setup>
import { watch } from 'vue';
import { useDeletionsStore } from '@/stores/deletions';
import { useAuthStore } from '@/stores/auth';

const store = useDeletionsStore();
const auth = useAuthStore();

// Длительности тянем только под аутентификацией. Этот компонент смонтирован в App.vue
// всегда, в т.ч. на публичных /maintenance, /500 и логине, где /settings/notifications
// отдаёт 401; client.js на 401 пробует refresh и при провале делает router.push('/'),
// сбивая юзера с публичной страницы (флак e2e /maintenance, /500).
// immediate покрывает вход по живой сессии, реакция на смену флага - вход через форму:
// логин уводит на /news роутером, компонент не перемонтируется, и одного onMounted было
// мало - настроенные в админке длительности не применялись до захода в Машины/Люди/Корзину.
watch(() => auth.isAuthenticated, (authed) => {
  if (authed) store.loadDurations();
}, { immediate: true });

// Цвет прогресс-бара: 100% (только удалили) - зелёный, 0% (вот-вот исчезнет) - красный.
// Для error/warning/info бар статичного цвета - нет семантики "истекает".
function barColorFor(item) {
  if (item.type === 'error') return 'var(--danger)';
  if (item.type === 'warning') return 'var(--warning)';
  if (item.type === 'info') return 'var(--info)';
  // Успех - не статичный цвет, а плавный переход зелёный->красный по остатку
  // времени, поэтому считается числами, а не переменной темы. Оба конца
  // читаются и на светлом, и на тёмном фоне - это заливка, а не текст.
  const t = Math.min(1, Math.max(0, (100 - item.progress) / 100));
  const green = [52, 199, 89];
  const red = [255, 102, 104];
  const c = green.map((g, i) => Math.round(g + (red[i] - g) * t));
  return `rgb(${c[0]}, ${c[1]}, ${c[2]})`;
}
</script>

<style scoped>
.del-stack {
  position: fixed;
  top: 75px;
  right: 20px;
  /* Выше всей лестницы модалок (история 12000-13000, ConfirmDialog 20000,
     SessionExpiredModal 25000, BanOverlay 26000): их оверлеи fixed inset:0 накрывают
     угол со стеком, и тост об ошибке из открытой модалки пропадал под затемнением. */
  z-index: 29000;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 10px;
  max-width: calc(100vw - 40px);
}

.del-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 15px;
  padding: 14px 16px;
  box-shadow: 0 3px 10px var(--shadow-drop);
  font-family: 'Montserrat', sans-serif;
  width: max-content;
  min-width: 300px;
  max-width: calc(100vw - 40px);
  cursor: pointer;
}

.del-title {
  font-size: 11px;
  font-weight: 500;
  color: var(--success-text);
  margin-bottom: 4px;
  letter-spacing: 0.02em;
}

.del-title--error {
  color: var(--danger-text);
}

.del-title--warning {
  color: var(--warning-text);
}

.del-title--info {
  color: var(--info-text);
}

.del-row {
  display: flex;
  align-items: center;
  gap: 16px;
  min-width: 0;
}

.del-text {
  flex: 1;
  min-width: 0;
  font-size: 14px;
  color: var(--text);
  /* Текст ошибки приходит с бэка произвольной длины (notify bold: result.message):
     при nowrap он не переносился и вылезал за карточку. */
  overflow-wrap: anywhere;
}

.del-text :deep(strong) {
  font-weight: 700;
}

.del-undo {
  flex-shrink: 0;
  background: var(--accent);
  color: var(--accent-contrast);
  border: none;
  padding: 6px 16px;
  border-radius: 50px;
  font-family: 'Montserrat', sans-serif;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s;
}

.del-undo:hover {
  background: var(--accent-hover);
}

.del-track {
  margin-top: 10px;
  height: 10px;
  border: 1px solid var(--border);
  border-radius: 50px;
  overflow: hidden;
  background: var(--surface-2);
}

.del-fill {
  height: 100%;
  border-radius: 50px;
  transition: width 0.1s linear, background 0.1s linear;
}

.del-enter-active,
.del-leave-active {
  transition: all 0.3s ease;
}

.del-enter-from,
.del-leave-to {
  transform: translateX(20px);
  opacity: 0;
}

/* Плавный сдвиг оставшихся карточек при удалении одной из стека. */
.del-move {
  transition: transform 0.3s ease;
}

.del-leave-active {
  position: absolute;
  right: 0;
}

@media (max-width: 768px) {
  /* Карточка тянется во всю ширину экрана: при width:max-content замер на 390
     давал ширину 490px и правый край на 527. Перенос текста теперь базовый. */
  .del-stack {
    left: 12px;
    right: 12px;
    max-width: none;
    align-items: stretch;
  }

  .del-card {
    width: auto;
    min-width: 0;
    max-width: 100%;
  }
}
</style>
