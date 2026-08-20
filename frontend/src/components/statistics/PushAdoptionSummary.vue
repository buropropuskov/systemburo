<template>
  <section
    class="push-summary"
    data-testid="push-adoption-summary"
  >
    <header class="push-summary__head">
      <h2 class="push-summary__title">
        Push-уведомления
      </h2>
      <span class="push-summary__chip">снимок на сейчас, вне выбранного периода</span>
      <span class="push-summary__rule" />
    </header>

    <div
      v-if="loading && !ready"
      class="an-panel__tiles"
      data-testid="push-adoption-skeleton"
    >
      <div
        v-for="n in 4"
        :key="n"
        class="push-summary__tile push-summary__tile--skeleton"
      />
    </div>

    <template v-else>
      <div class="an-panel">
        <div class="an-panel__chart">
          <AnalyticsDonutChart
            v-if="platformData.length > 0"
            :data="platformData"
            :height="300"
            total-label="Подписок"
            :unit-forms="['подписка', 'подписки', 'подписок']"
          />
          <p
            v-else
            class="push-summary__chart-empty"
          >
            Пока нет ни одной push-подписки
          </p>
        </div>

        <div class="an-panel__col">
          <div class="an-panel__tiles">
            <div class="push-summary__tile">
              <div class="push-summary__tile-label">
                Активные пользователи
              </div>
              <div class="push-summary__tile-val">
                <AnimatedNumber :value="numberOrNull(summary.active_users_total)" />
              </div>
            </div>
            <div class="push-summary__tile">
              <div class="push-summary__tile-label">
                С push-подпиской
              </div>
              <div class="push-summary__tile-val">
                <AnimatedNumber :value="numberOrNull(summary.users_with_push)" />
              </div>
            </div>
            <div class="push-summary__tile">
              <div class="push-summary__tile-label">
                <span>Заходят с iOS (iPhone, iPad)</span>
                <HintTooltip :text="IOS_HINT" />
              </div>
              <div class="push-summary__tile-val">
                <AnimatedNumber :value="numberOrNull(summary.users_by_last_login_platform?.ios)" />
              </div>
            </div>
            <div
              class="push-summary__tile"
              data-testid="push-adoption-rate"
            >
              <div class="push-summary__tile-label">
                Доля активных с подпиской
              </div>
              <div class="push-summary__tile-val">
                {{ adoptionPct === null ? '—' : `${adoptionPct}%` }}
              </div>
            </div>
          </div>
        </div>
      </div>
    </template>
  </section>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue';
import AnalyticsDonutChart from '@/components/statistics/AnalyticsDonutChart.vue';
import AnimatedNumber from '@/components/statistics/AnimatedNumber.vue';
import HintTooltip from '@/components/ui/HintTooltip.vue';
import { getPushSummary } from '@/api/webPush';

// Человеческие подписи платформ для донат-чарта - ключи в коде технические,
// совпадают с services.DetectPlatform (ios/android/desktop/unknown).
// Формулировка "iOS (iPhone, iPad)" - по требованию: iPhone и iPad ограничены
// Apple одинаково (push работает только у установленного на "Домой"
// приложения), отдельной группы "только iPhone" нет ни в бэке, ни в тексте.
// Разрыв между «Заходят с iOS» и «С push-подпиской» объясняется требованием
// Apple, а не отказами пользователей. Текст висел отдельным абзацем во всю
// ширину блока и читался как ничей - живёт подсказкой у той плитки, о которой он.
const IOS_HINT = 'На iOS push работает только после установки сайта на экран «Домой» - '
  + 'разрыв между «Заходят с iOS» и «С push-подпиской» в основном объясняется этим '
  + 'шагом, а не нежеланием включать уведомления.';

const PLATFORM_LABELS = {
  ios: 'iOS (iPhone, iPad)',
  android: 'Android',
  desktop: 'Компьютер',
  unknown: 'Неизвестно',
};

const summary = ref({});
const loading = ref(false);
const ready = ref(false);

/** null/не-число -> null, чтобы AnimatedNumber рисовал прочерк, а не 0 (нет данных != ноль). */
function numberOrNull(value) {
  const n = Number(value);
  return Number.isFinite(n) ? n : null;
}

// subscriptions_by_platform - разрез ПОДПИСОК (устройств), не пользователей:
// один человек с двумя устройствами даёт два сегмента донат-чарта.
const platformData = computed(() => {
  const counts = summary.value.subscriptions_by_platform || {};
  return Object.entries(counts)
    .map(([platform, count]) => ({ label: PLATFORM_LABELS[platform] || platform, value: Number(count) || 0 }))
    .filter((row) => row.value > 0);
});

// Доля СРЕДИ АКТИВНЫХ пользователей (не среди подписавшихся - это была бы
// тавтология) - показывает разрыв, ради которого блок и существует.
const adoptionPct = computed(() => {
  const active = numberOrNull(summary.value.active_users_total);
  const subscribed = numberOrNull(summary.value.users_with_push);
  if (!active || active <= 0 || subscribed === null) return null;
  return Math.round((subscribed / active) * 100);
});

async function load() {
  loading.value = true;
  try {
    const data = await getPushSummary();
    summary.value = data || {};
  } catch {
    // Сбой сводки не должен ронять вкладку аналитики - блок остаётся с прочерками.
    summary.value = {};
  } finally {
    loading.value = false;
    ready.value = true;
  }
}

async function refresh() {
  await load();
}

defineExpose({ refresh });

onMounted(load);
</script>

<style scoped>
/*
 * Свой namespace классов (push-summary__*), а не переиспользование
 * .dashboard__* - те живут в scoped-стилях StatisticsDashboard.vue и сюда не
 * дотягиваются (разные компоненты = разный scope-хэш). Значения токенов и
 * геометрия сознательно повторяют вид плиток/групп дашборда (см.
 * StatisticsDashboard.vue), чтобы блок не выглядел чужеродным.
 */
.push-summary {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.push-summary__head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.push-summary__title {
  font-size: 15px;
  font-weight: 700;
  color: var(--color-text);
  margin: 0;
  white-space: nowrap;
}

.push-summary__chip {
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-muted);
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  padding: 3px 10px;
  border-radius: var(--radius-pill);
  white-space: nowrap;
}

.push-summary__rule {
  flex: 1;
  height: 1px;
  background: var(--color-border);
}

.push-summary__chart-empty {
  margin: 0;
  font-size: 13px;
  color: var(--color-text-muted);
  text-align: center;
}

.push-summary__tile {
  background: var(--surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 12px 14px;
}

.push-summary__tile--skeleton {
  background: var(--color-skeleton);
  min-height: 72px;
  border: none;
  animation: push-summary-pulse 1.4s ease-in-out infinite;
}

@keyframes push-summary-pulse {
  0%, 100% { opacity: 1; }
  50%       { opacity: 0.55; }
}

.push-summary__tile-label {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  color: var(--color-text-muted);
  font-weight: 500;
  line-height: 1.3;
}

.push-summary__tile-val {
  font-size: 26px;
  font-weight: 700;
  color: var(--color-text);
  margin-top: 6px;
  line-height: 1;
}

/* ===== МОБИЛКА (<=768) ===== */
@media (max-width: 768px) {
  /* Чип-пояснение не переносился и торчал за правый край панели (289px рядом с
     заголовком при 364 доступных на 390): шапке разрешаем вторую строку, чипу -
     перенос текста, линейка добирает остаток. Правило то же, что у шапок групп
     дашборда (StatisticsDashboard, тот же блок). */
  .push-summary__head {
    flex-wrap: wrap;
    gap: 8px;
  }

  .push-summary__chip {
    white-space: normal;
  }

  .push-summary__rule {
    min-width: 24px;
  }
}
</style>
