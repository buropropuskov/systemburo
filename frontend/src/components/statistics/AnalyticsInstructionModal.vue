<template>
  <BaseModal
    :show="show"
    title="Как пользоваться Аналитикой"
    width="620px"
    @close="$emit('close')"
  >
    <div class="instruction">
      <section class="instruction__section">
        <h4 class="instruction__heading">Дашборд</h4>
        <p class="instruction__text">
          Дашборд показывает текущее состояние системы и ключевые показатели за выбранный период.
        </p>
        <ul class="instruction__list">
          <li>
            <strong>Метрики «Данные»</strong> — заявки, вложения по типам, статусы обработки,
            проезды машин и проходы людей. Все числа относятся к выбранному периоду (фильтр
            в шапке).
          </li>
          <li>
            <strong>Метрики «Система»</strong> — технические показатели: пользователи, чёрные
            списки, уникальные записи в базе. Они не зависят от периода.
          </li>
          <li>
            <strong>График динамики</strong> — визуализирует тренд выбранной метрики (заявки,
            проходы людей или проезды машин) с разбивкой по дням, неделям или месяцам.
          </li>
          <li>
            <strong>Живые ленты</strong> — «Проход людей» и «Проезд машин» обновляются каждые
            10 секунд и показывают последние события в реальном времени.
          </li>
        </ul>
        <p class="instruction__tip">
          Зелёная пульсирующая точка рядом с заголовком ленты означает, что данные обновляются
          автоматически. Если нужно принудительно обновить метрики, нажмите «Обновить» в шапке.
        </p>
      </section>

      <section class="instruction__section">
        <h4 class="instruction__heading">Как сформировать отчёт</h4>
        <p class="instruction__text">
          Раздел «Отчёты» строит произвольные выборки по данным бюро. Слева — готовые наборы
          и ваши сохранённые шаблоны, справа — мастер. Порядок работы:
        </p>
        <ol class="instruction__ordered">
          <li>
            <strong>Что считаем / выгружаем</strong> — выберите метрики (можно несколько:
            заявки, проезды машин, проходы людей, среднее машин в день) или режим выгрузки строк.
          </li>
          <li>
            <strong>По чему разбить (разрез)</strong> — укажите измерение: по организации,
            месту разгрузки, типу вложения, статусу, периоду (день/неделя/месяц) и т.д.
          </li>
          <li>
            <strong>Фильтры</strong> — необязательно. Сузьте выборку по конкретной организации,
            месту, типу вложения или другому атрибуту.
          </li>
          <li>
            <strong>Период</strong> — тот же фильтр дат, что и на дашборде.
          </li>
          <li>
            <strong>Результат</strong> — таблица с итогами и переключателем на график; готовый
            отчёт выгружается в Excel одной кнопкой.
          </li>
        </ol>
      </section>

      <section class="instruction__section instruction__section--last">
        <h4 class="instruction__heading">Смена периода</h4>
        <p class="instruction__text">
          Фильтр периода в шапке страницы применяется сразу к дашборду. Доступны быстрые
          пресеты (сегодня, эта неделя, этот месяц) и ручной выбор диапазона. После смены
          периода данные обновляются автоматически.
        </p>
      </section>
    </div>
  </BaseModal>
</template>

<script setup>
import BaseModal from '@/components/ui/BaseModal.vue';

defineProps({
  show: {
    type: Boolean,
    required: true,
  },
});

defineEmits(['close']);
</script>

<style scoped>
.instruction {
  padding: 20px 24px 24px;
  display: flex;
  flex-direction: column;
  gap: 0;
}

.instruction__section {
  padding-bottom: 20px;
  margin-bottom: 20px;
  border-bottom: 1px solid var(--color-border);
}

.instruction__section--last {
  border-bottom: none;
  padding-bottom: 0;
  margin-bottom: 0;
}

.instruction__heading {
  font-size: 14px;
  font-weight: 700;
  color: var(--color-text);
  margin: 0 0 8px;
}

.instruction__text {
  font-size: 13px;
  color: var(--color-text);
  line-height: 1.55;
  margin: 0 0 10px;
}

.instruction__list,
.instruction__ordered {
  font-size: 13px;
  color: var(--color-text);
  line-height: 1.55;
  padding-left: 20px;
  margin: 0 0 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.instruction__list li,
.instruction__ordered li {
  padding-left: 2px;
}

.instruction__tip {
  font-size: 12px;
  color: var(--color-text-muted);
  line-height: 1.5;
  background: var(--color-bg);
  border-radius: var(--radius-sm);
  padding: 8px 12px;
  margin: 0;
}
</style>
