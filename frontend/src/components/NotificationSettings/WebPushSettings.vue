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

    <!-- Состояния перечислены явно (не каскад v-else) - у каждого свой текст
         и своя причина, почему кнопка недоступна. iOS-подсказка приоритетнее
         фактической поддержки API: Safari формально может отдавать
         Notification/PushManager вне режима "Домой", но subscribe() там
         откажет - лучше объяснить заранее, чем ловить пойманную ошибку. -->
    <div
      v-if="iosNeedsInstall"
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
import { useWebPush } from '@/composables/useWebPush';
import { formatDateTime } from '@/utils/datetime';

const {
  supported,
  iosNeedsInstall,
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
