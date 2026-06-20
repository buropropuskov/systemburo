<template>
  <div class="del-stack">
    <transition-group name="del">
      <div
        v-for="item in store.items"
        :key="item.id"
        class="del-card"
        :class="{ 'del-card--error': item.type === 'error' }"
        @click="store.dismiss(item.id)"
      >
        <div
          v-if="item.title"
          class="del-title"
          :class="{ 'del-title--error': item.type === 'error' }"
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
import { onMounted } from 'vue';
import { useDeletionsStore } from '@/stores/deletions';
import { useAuthStore } from '@/stores/auth';

const store = useDeletionsStore();
const auth = useAuthStore();

// Длительности тянем только под аутентификацией. Этот компонент смонтирован в App.vue
// всегда, в т.ч. на публичных /maintenance, /500 и логине, где /settings/notifications
// отдаёт 401; client.js на 401 пробует refresh и при провале делает router.push('/'),
// сбивая юзера с публичной страницы (флак e2e /maintenance, /500). Залогиненному
// durations подтянутся при заходе на списки (Cars/People/Trash).
onMounted(() => {
  if (auth.isAuthenticated) store.loadDurations();
});

// Цвет прогресс-бара: 100% (только удалили) - зелёный, 0% (вот-вот исчезнет) - красный.
// Для type=error всегда красный - нет семантики "истекает".
function barColorFor(item) {
  if (item.type === 'error') return 'rgb(255, 102, 104)';
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
  z-index: 11000;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 10px;
  max-width: calc(100vw - 40px);
}

.del-card {
  background: #fff;
  border: 1px solid #e6e6e6;
  border-radius: 15px;
  padding: 14px 16px;
  box-shadow: 0 3px 10px rgba(0, 0, 0, 0.1);
  font-family: 'Montserrat', sans-serif;
  width: max-content;
  min-width: 300px;
  max-width: calc(100vw - 40px);
  cursor: pointer;
}

.del-title {
  font-size: 11px;
  font-weight: 500;
  color: #15803d;
  margin-bottom: 4px;
  letter-spacing: 0.02em;
}

.del-title--error {
  color: #b91c1c;
}

.del-row {
  display: flex;
  align-items: center;
  gap: 16px;
}

.del-text {
  flex: 1;
  font-size: 14px;
  color: #000;
  white-space: nowrap;
}

.del-text :deep(strong) {
  font-weight: 700;
}

.del-undo {
  flex-shrink: 0;
  background: #4F5BDF;
  color: #fff;
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
  background: #3a45b2;
}

.del-track {
  margin-top: 10px;
  height: 10px;
  border: 1px solid #e6e6e6;
  border-radius: 50px;
  overflow: hidden;
  background: #f5f5f5;
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
</style>
