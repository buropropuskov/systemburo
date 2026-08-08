<template>
  <section
    class="lk-card webpush"
    data-testid="webpush-block"
  >
    <header class="webpush__header">
      <h2 class="webpush__title">
        Push-уведомления в браузере
      </h2>
    </header>
    <p class="webpush__hint">
      Приходят системным уведомлением, даже если вкладка бюро пропусков закрыта.
    </p>
    <!-- Живая проверка на стенде (#974): при полностью закрытом браузере
         уведомления не приходят - принимает их сам браузер, а не система, и
         закрытому процессу принимать нечем. Это не поломка, но без строчки в
         интерфейсе выглядит именно ей. -->
    <p
      class="webpush__hint"
      data-testid="webpush-delivery-note"
    >
      На компьютере уведомления доходят, пока запущен браузер. Закрыли его
      совсем - придут при следующем запуске. На телефоне доставку держит сама
      система, там они приходят всегда.
    </p>

    <!-- Состояния перечислены явно (не каскад v-else) - у каждого свой текст
         и своя причина, почему кнопка недоступна. iOS-подсказка приоритетнее
         фактической поддержки API: Safari формально может отдавать
         Notification/PushManager вне режима "Домой", но subscribe() там
         откажет - лучше объяснить заранее, чем ловить пойманную ошибку. -->
    <!-- Сторонний браузер на iOS проверяется ПЕРВЫМ: подсказка про экран «Домой»
         ему бесполезна, ярлык оттуда push не получает. Текст намеренно не требует
         «перейти на Safari»: Safari нужен ОДИН раз, чтобы поставить иконку, дальше
         бюро открывается отдельным приложением, и повседневный браузер человека это
         никак не задевает. Первая формулировка читалась как «пересядь на Safari» и
         справедливо вызвала возражение (#974). -->
    <div
      v-if="iosNeedsSafariBrowser"
      class="webpush__state webpush__state--info"
      data-testid="webpush-ios-safari-hint"
    >
      <p>
        На iOS (iPhone и iPad) уведомления получает не сайт в браузере, а отдельное
        приложение с экрана «Домой». Установить его умеет только Safari - так решила
        Apple, и это одинаково для всех сайтов.
      </p>
      <p class="webpush__hint">
        Менять браузер не нужно: Safari понадобится один раз, для установки. Дальше
        бюро пропусков открывается со своей иконки, а вы продолжаете пользоваться
        привычным браузером.
      </p>
      <ol class="webpush__steps">
        <li>
          Скопируйте адрес и откройте его в Safari:
          <button
            class="webpush__copy"
            type="button"
            data-testid="webpush-copy-url"
            @click="copySiteUrl"
          >
            {{ siteUrl }}
          </button>
        </li>
        <li>Нажмите «Поделиться» в нижней панели Safari.</li>
        <li>Выберите «На экран Домой».</li>
        <li>Откройте бюро пропусков с появившейся иконки и включите уведомления здесь.</li>
      </ol>
    </div>

    <div
      v-else-if="iosNeedsInstall"
      class="webpush__state webpush__state--info"
      data-testid="webpush-ios-hint"
    >
      <p>
        На iOS (iPhone и iPad) push работает только для сайта, добавленного на
        экран «Домой» - в обычной вкладке Safari включить уведомления нельзя.
      </p>
      <ol class="webpush__steps">
        <li>Нажмите «Поделиться» в нижней панели Safari.</li>
        <li>Выберите «На экран Домой».</li>
        <li>Откройте бюро пропусков с появившейся иконки.</li>
        <li>Включите уведомления уже в открывшемся приложении - здесь.</li>
      </ol>
    </div>

    <p
      v-else-if="!supported"
      class="webpush__state"
      data-testid="webpush-unsupported"
    >
      Этот браузер не поддерживает push-уведомления.
    </p>

    <p
      v-else-if="!serverConfigured"
      class="webpush__state"
      data-testid="webpush-not-configured"
    >
      Push-уведомления пока не настроены на сервере - обратитесь к администратору.
    </p>

    <template v-else>
      <div
        v-if="permission === 'denied'"
        class="webpush__state webpush__state--warning"
        data-testid="webpush-denied"
      >
        <p>
          Уведомления запрещены в настройках браузера - включить их отсюда нельзя.
        </p>
        <p class="webpush__hint">
          Снять запрет можно через значок замка (или "i") рядом с адресом сайта -
          там есть настройка разрешений для уведомлений.
        </p>
      </div>

      <div
        v-else-if="enabledOnDevice"
        class="webpush__enabled"
        data-testid="webpush-enabled"
      >
        <div class="webpush__row">
          <p class="webpush__status">
            Включены на этом устройстве
          </p>
          <button
            class="lk-button lk-button--secondary"
            data-testid="webpush-disable"
            :disabled="busy"
            @click="disable"
          >
            {{ busy ? 'Отключение...' : 'Отключить' }}
          </button>
        </div>

        <ul
          v-if="devices.length"
          class="webpush__devices"
        >
          <li
            v-for="(device, idx) in devices"
            :key="device.id ?? idx"
            class="webpush__device"
          >
            <p class="webpush__device-agent">
              {{ device.user_agent || 'Неизвестное устройство' }}
            </p>
            <p class="webpush__device-meta">
              Добавлено: {{ formatDateTime(device.created_at) || '—' }} ·
              Последняя доставка: {{ device.last_success_at ? formatDateTime(device.last_success_at) : 'никогда' }}
            </p>
          </li>
        </ul>
      </div>

      <div
        v-else
        class="webpush__row"
        data-testid="webpush-default"
      >
        <p class="webpush__status">
          Сейчас выключены на этом устройстве
        </p>
        <button
          class="lk-button lk-button--primary"
          data-testid="webpush-enable"
          :disabled="busy"
          @click="enable"
        >
          {{ busy ? 'Включение...' : 'Включить' }}
        </button>
      </div>
    </template>
  </section>
</template>

<script setup>
import { computed } from 'vue';
import { useWebPush } from '@/composables/useWebPush';
import { useDeletionsStore } from '@/stores/deletions';
import { formatDateTime } from '@/utils/datetime';

const siteUrl = computed(() => window.location.origin);
const deletions = useDeletionsStore();

/**
 * Адрес нужно перенести в Safari руками - на iOS страница не может открыть ссылку в
 * другом браузере. Копирование через буфер снимает ручной ввод; отказ буфера не тупик,
 * адрес виден на самой кнопке и его можно выделить.
 */
async function copySiteUrl() {
  try {
    await navigator.clipboard.writeText(siteUrl.value);
    deletions.notify({ bold: 'Адрес скопирован', suffix: ' - вставьте его в Safari' });
  } catch {
    deletions.notify({
      bold: 'Скопировать не вышло',
      suffix: ' - наберите адрес в Safari вручную',
      type: 'warning',
    });
  }
}

const {
  supported,
  iosNeedsInstall,
  iosNeedsSafariBrowser,
  permission,
  serverConfigured,
  devices,
  busy,
  enabledOnDevice,
  enable,
  disable,
} = useWebPush();
</script>

<style scoped>
.webpush {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.webpush__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.webpush__title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text);
}

.webpush__hint {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.4;
}

/* Адрес кнопкой, а не простым текстом: по нему нажимают, чтобы забрать в буфер, и
   сам адрес при этом остаётся читаемым - если буфер откажет, его можно набрать. */
.webpush__copy {
  display: inline-block;
  margin-top: 4px;
  padding: 4px 10px;
  border: 1px solid var(--border-color, #d8d9e3);
  border-radius: var(--radius-md, 15px);
  background: transparent;
  color: var(--color-primary);
  font: inherit;
  font-size: 12px;
  cursor: pointer;
  word-break: break-all;
  text-align: left;
  transition: background-color 150ms ease;
}

.webpush__copy:hover {
  background: var(--color-primary-tint, rgba(79, 91, 223, 0.08));
}

.webpush__state {
  margin: 4px 0 0;
  font-size: 13px;
  color: var(--text-muted);
}

.webpush__state--info {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.webpush__state--info > p {
  margin: 0;
}

.webpush__steps {
  margin: 0;
  padding-left: 18px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  color: var(--color-text);
}

.webpush__steps li {
  padding-left: 2px;
}

.webpush__state--warning {
  display: flex;
  flex-direction: column;
  gap: 4px;
  color: var(--color-text);
}

.webpush__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-top: 4px;
}

.webpush__status {
  margin: 0;
  font-size: 13px;
  color: var(--color-text);
}

.webpush__enabled {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.webpush__devices {
  list-style: none;
  margin: 4px 0 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.webpush__device {
  padding: 8px 10px;
  border-radius: var(--radius-md);
  background: var(--surface-2);
}

.webpush__device-agent {
  margin: 0;
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text);
}

.webpush__device-meta {
  margin: 2px 0 0;
  font-size: 11px;
  color: var(--text-muted);
}

@media (max-width: 480px) {
  .webpush__row {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
}
</style>
