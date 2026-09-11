<template>
  <BaseModal
    :show="!!request"
    title="Отмена отметки"
    width="420px"
    content-testid="passage-revert-modal"
    :z-index="20000"
    @close="close"
  >
    <p
      v-if="request"
      class="revert__subject"
    >
      <span
        class="revert__direction"
        :class="`revert__direction--${request.direction}`"
      >{{ directionLabel }}</span>
      {{ request.subject }}
    </p>
    <p class="revert__hint">
      Отметка останется в журнале с пометкой об отмене, но в отчёты и счётчики не попадёт.
    </p>
    <div class="revert__reasons">
      <button
        v-for="preset in reasons"
        :key="preset"
        type="button"
        class="revert__reason"
        :class="{ 'revert__reason--active': reason === preset }"
        @click="reason = preset"
      >
        {{ preset }}
      </button>
    </div>
    <input
      v-model="reason"
      class="lk-input revert__input"
      type="text"
      maxlength="200"
      placeholder="Причина отмены"
      data-testid="passage-revert-reason"
    >
    <template #actions>
      <button
        type="button"
        class="lk-button lk-button--secondary"
        @click="close"
      >
        Закрыть
      </button>
      <button
        type="button"
        class="lk-button lk-button--danger"
        data-testid="passage-revert-submit"
        :disabled="!reason.trim() || saving"
        @click="submit"
      >
        {{ saving ? 'Отменяю...' : 'Отменить отметку' }}
      </button>
    </template>
  </BaseModal>
</template>

<script>
import { computed, ref, watch } from 'vue';
import BaseModal from '@/components/ui/BaseModal.vue';
import { usePassageRevertStore } from '@/stores/passageRevert';
import { useDeletionsStore } from '@/stores/deletions';
import { PASSAGE_REVERT_REASONS, revertPassage } from '@/utils/passageMarks';

/**
 * Окно причины отмены ошибочной отметки прохода (#2437).
 *
 * Смонтировано один раз в App.vue и слушает стор: таблицы прохода давно за порогом
 * размера блоков, и заводить в каждой свою копию окна было нельзя.
 */
export default {
  name: 'PassageRevertModal',
  components: { BaseModal },
  setup() {
    const store = usePassageRevertStore();
    const request = computed(() => store.request);
    const reason = ref('');
    const saving = ref(false);

    // Новый запрос всегда начинается с чистого поля: причина прошлой отмены к новой
    // строке отношения не имеет, а подставленный текст охранник отправит не глядя.
    watch(request, (value) => {
      if (value) reason.value = '';
    });

    // Люди ходят, машины ездят: в таблице проезда кнопки называются «Въезд» и «Выезд»,
    // и окно обязано повторять ту же пару - иначе охранник читает про вход там, где
    // только что нажал въезд.
    const directionLabel = computed(() => {
      const entry = request.value?.direction === 'entry';
      if (request.value?.kind === 'cars') return entry ? 'Въезд' : 'Выезд';
      return entry ? 'Вход' : 'Выход';
    });

    function close() {
      if (saving.value) return;
      store.close();
    }

    async function submit() {
      const current = request.value;
      const text = reason.value.trim();
      if (!current || !text || saving.value) return;

      saving.value = true;
      try {
        const { ok, error } = await revertPassage({
          kind: current.kind,
          id: current.id,
          direction: current.direction,
          tableId: current.tableId,
          reason: text,
        });
        if (!ok) {
          useDeletionsStore().notify({ prefix: '', bold: error, type: 'error' });
          return;
        }
        useDeletionsStore().notify({
          prefix: `${directionLabel.value} отменён: `,
          bold: current.subject,
        });
        current.onDone?.();
        store.close();
      } finally {
        saving.value = false;
      }
    }

    return { request, reason, saving, directionLabel, close, submit, reasons: PASSAGE_REVERT_REASONS };
  },
};
</script>

<style scoped>
.revert__subject {
  margin: 0 0 8px;
  font-size: 15px;
  font-weight: 600;
  color: var(--text);
}

.revert__direction {
  display: inline-block;
  margin-right: 6px;
  padding: 2px 10px;
  border-radius: var(--radius-md);
  font-size: 13px;
  font-weight: 600;
}

.revert__direction--entry {
  background: var(--success-bg);
  color: var(--success-text);
}

.revert__direction--exit {
  background: var(--danger-bg);
  color: var(--danger-text);
}

.revert__hint {
  margin: 0 0 14px;
  font-size: 13px;
  line-height: 1.4;
  color: var(--text-muted);
}

.revert__reasons {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 10px;
}

.revert__reason {
  padding: 6px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
  color: var(--text);
  font-size: 13px;
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease;
}

.revert__reason--active {
  border-color: var(--primary);
  background: var(--surface-2);
}

.revert__input {
  width: 100%;
}
</style>
