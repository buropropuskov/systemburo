<template>
  <Teleport to="body">
    <transition
      name="modal-fade"
      @after-leave="onAfterLeave"
    >
      <div
        v-if="visible"
        class="modal-overlay"
        data-testid="attachment-fields-modal"
        @mousedown="onOverlayMousedown"
        @mouseup="onOverlayMouseup"
      >
        <div
          class="fields-modal"
          @mousedown.stop
        >
          <div class="modal-header">
            <h3>Настройка полей{{ attachmentName ? ` «${attachmentName}»` : '' }}</h3>
            <button
              class="close-btn"
              aria-label="Закрыть"
              @click="requestClose"
            >
              ×
            </button>
          </div>

          <div class="modal-content">
            <div
              v-if="loading"
              class="modal-state"
            >
              <LoaderSpinner label="Загрузка настройки полей..." />
            </div>

            <template v-else>
              <p class="fields-hint">
                Скрытые поля не показываются при подаче заявки.
              </p>

              <div class="groups">
                <div
                  v-for="group in groups"
                  :key="group.key"
                  class="gcard"
                >
                  <div class="ghead">
                    <span class="ghead-dot" />
                    <h4>{{ group.label }}</h4>
                    <span class="ghead-count">{{ group.fields.length }}</span>
                  </div>
                  <div class="matrix">
                    <div class="matrix-head">
                      <span>Поле</span>
                      <span>Показывать</span>
                      <span>Обязательно</span>
                    </div>
                    <div
                      v-for="field in group.fields"
                      :key="field.key"
                      class="matrix-row"
                      :data-testid="`field-row-${field.key}`"
                    >
                      <div class="matrix-name">{{ field.label }}</div>
                      <div class="matrix-cell">
                        <ToggleSwitch
                          v-model="field.visible"
                          :data-testid="`field-visible-${field.key}`"
                        />
                      </div>
                      <div class="matrix-cell matrix-cell--req">
                        <ToggleSwitch
                          v-if="field.requirable"
                          v-model="field.required"
                          :disabled="!field.visible"
                          :data-testid="`field-required-${field.key}`"
                        />
                        <span
                          v-else
                          class="matrix-dash"
                        >—</span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <div class="gcard custom-card">
                <div class="ghead">
                  <span class="ghead-dot" />
                  <h4>Дополнительные поля</h4>
                  <span class="ghead-count">{{ customFields.length }}</span>
                </div>
                <div class="custom-body">
                  <p class="custom-hint">
                    Произвольные текстовые поля заявки. Перетаскивание за ⠿ задаёт порядок в форме подачи.
                  </p>

                  <div
                    v-if="!customFields.length"
                    class="custom-empty"
                  >
                    Дополнительных полей нет
                  </div>

                  <div
                    v-else
                    class="ctable"
                  >
                    <div class="ctable-head">
                      <span />
                      <span>Заголовок</span>
                      <span>Плейсхолдер</span>
                      <span class="ctable-head--c">Обязат.</span>
                      <span />
                    </div>
                    <div class="ctable-scroll">
                      <div
                        v-for="(cf, i) in customFields"
                        :key="cf.uid"
                        class="ctable-row"
                        :data-testid="`custom-row-${i}`"
                      >
                        <span class="ctable-grip">⠿</span>
                        <input
                          v-model="cf.label"
                          class="lk-input ctable-input"
                          maxlength="200"
                          placeholder="Заголовок"
                          :data-testid="`custom-label-${i}`"
                        >
                        <input
                          v-model="cf.placeholder"
                          class="lk-input ctable-input"
                          maxlength="200"
                          placeholder="Плейсхолдер"
                          :data-testid="`custom-placeholder-${i}`"
                        >
                        <div class="ctable-req">
                          <ToggleSwitch
                            v-model="cf.required"
                            :data-testid="`custom-required-${i}`"
                          />
                        </div>
                        <div class="ctable-ctl">
                          <button
                            type="button"
                            class="order-btn"
                            aria-label="Переместить выше"
                            :disabled="i === 0"
                            :data-testid="`custom-up-${i}`"
                            @click="moveCustom(i, -1)"
                          >
                            ↑
                          </button>
                          <button
                            type="button"
                            class="order-btn"
                            aria-label="Переместить ниже"
                            :disabled="i === customFields.length - 1"
                            :data-testid="`custom-down-${i}`"
                            @click="moveCustom(i, 1)"
                          >
                            ↓
                          </button>
                          <button
                            type="button"
                            class="custom-delete"
                            aria-label="Удалить поле"
                            :data-testid="`custom-delete-${i}`"
                            @click="removeCustom(i)"
                          >
                            ×
                          </button>
                        </div>
                      </div>
                    </div>
                  </div>

                  <button
                    type="button"
                    class="lk-button lk-button--ghost custom-add"
                    data-testid="custom-add"
                    @click="addCustom"
                  >
                    + Добавить поле
                  </button>
                </div>
              </div>
            </template>
          </div>

          <div class="modal-footer">
            <button
              class="lk-button lk-button--ghost"
              :disabled="isSaving"
              @click="requestClose"
            >
              Отмена
            </button>
            <button
              class="lk-button lk-button--primary"
              :disabled="loading || isSaving || !isDirty"
              data-testid="fields-save"
              @click="save"
            >
              {{ isSaving ? 'Сохранение...' : 'Сохранить' }}
            </button>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script>
import { ref } from 'vue';
import {
  getFieldConfig, saveFieldConfig,
  createCustomField, updateCustomField, deleteCustomField,
} from '@/api/attachment-templates';
import { useDeletionsStore } from '@/stores/deletions';
import { useOverlayClose } from '@/composables/useOverlayClose';
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';

// Подписи групп полей реестра. Порядок групп в ответе сохраняется (common впереди),
// поэтому собираем группы в порядке появления полей.
const GROUP_LABELS = {
  common: 'Общие поля',
  people: 'Поля сотрудников',
  cars: 'Поля транспорта',
  items: 'Поля ТМЦ',
};

export default {
  name: 'AttachmentFieldsModal',
  components: { ToggleSwitch, LoaderSpinner },
  props: {
    uniqueAttachmentId: { type: Number, required: true },
    attachmentName: { type: String, default: '' },
  },
  emits: ['close', 'saved'],
  setup(_, { emit }) {
    // Анимация открытия/закрытия по образцу UniqueAttachmentHistoryModal: внутренний
    // visible управляет enter/leave, emit('close') шлём только @after-leave - иначе
    // родительский v-if размонтирует нас мгновенно и leave-анимация не проиграется.
    const visible = ref(false);
    const requestClose = () => { visible.value = false; };
    const onAfterLeave = () => emit('close');
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(requestClose);
    return { visible, requestClose, onAfterLeave, onOverlayMousedown, onOverlayMouseup };
  },
  data() {
    return {
      loading: false,
      isSaving: false,
      fields: [],
      // Снимок настраиваемых полей на момент загрузки - для проверки "грязности".
      original: '',
      // Кастомные поля редактируются в памяти, коммитятся одним "Сохранить" вместе
      // с базовыми. Порядок в массиве = sort_order. uid - стабильный ключ списка
      // (id у новых строк ещё нет, нужен счётчик, иначе reorder путает инпуты).
      customFields: [],
      originalCustom: '',
      deletedCustomIds: [],
      customUidSeq: 0,
    };
  },
  computed: {
    groups() {
      const out = [];
      const byKey = new Map();
      this.fields.forEach((f) => {
        // Залоченные (даты/время) не настраиваются - не показываем в модалке (#529).
        if (f.locked) return;
        let group = byKey.get(f.group);
        if (!group) {
          group = { key: f.group, label: GROUP_LABELS[f.group] || f.group, fields: [] };
          byKey.set(f.group, group);
          out.push(group);
        }
        group.fields.push(f);
      });
      return out;
    },
    // Тело PUT: только настраиваемые поля (залоченные бэк игнорирует).
    payload() {
      return this.fields
        .filter((f) => !f.locked)
        .map((f) => ({ key: f.key, visible: f.visible, required: f.required }));
    },
    isBaseDirty() {
      return JSON.stringify(this.payload) !== this.original;
    },
    // Снимок кастомных полей в порядке массива (order значим) - для "грязности".
    customSnapshot() {
      return JSON.stringify(this.customFields.map((c) => ({
        id: c.id, label: c.label, placeholder: c.placeholder, required: c.required,
      })));
    },
    isCustomDirty() {
      return this.customSnapshot !== this.originalCustom || this.deletedCustomIds.length > 0;
    },
    isDirty() {
      return this.isBaseDirty || this.isCustomDirty;
    },
  },
  mounted() {
    this.visible = true;
    this.load();
    document.addEventListener('keydown', this.onKeydown);
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.onKeydown);
  },
  methods: {
    onKeydown(e) {
      if (e.key === 'Escape') this.requestClose();
    },
    async load() {
      this.loading = true;
      try {
        const data = await getFieldConfig(this.uniqueAttachmentId);
        const base = Array.isArray(data?.base) ? data.base : [];
        this.fields = base.map((f) => ({
          key: f.key,
          label: f.label,
          group: f.group,
          visible: f.visible,
          required: f.required,
          requirable: f.requirable,
          locked: f.locked,
        }));
        this.original = JSON.stringify(this.payload);

        const custom = Array.isArray(data?.custom) ? data.custom : [];
        this.deletedCustomIds = [];
        this.customFields = custom.map((c) => ({
          uid: this.customUidSeq++,
          id: c.id,
          label: c.label || '',
          placeholder: c.placeholder || '',
          required: !!c.is_required,
        }));
        this.originalCustom = this.customSnapshot;
      } catch (error) {
        console.error('Error loading field config:', error);
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'настройку полей', type: 'error' });
        this.requestClose();
      } finally {
        this.loading = false;
      }
    },
    addCustom() {
      this.customFields.push({
        uid: this.customUidSeq++, id: null, label: '', placeholder: '', required: false,
      });
    },
    removeCustom(i) {
      const cf = this.customFields[i];
      if (cf?.id != null) this.deletedCustomIds.push(cf.id);
      this.customFields.splice(i, 1);
    },
    moveCustom(i, dir) {
      const j = i + dir;
      if (j < 0 || j >= this.customFields.length) return;
      const arr = this.customFields;
      [arr[i], arr[j]] = [arr[j], arr[i]];
    },
    // Коммит кастомных полей: удаления, затем upsert в порядке массива (index = sort_order).
    // Существующие всегда PUT-им (идемпотентно) - проще, чем дифать измененные.
    // Резюмируемо: снимаем удаления из очереди и проставляем id созданным по мере
    // успеха, чтобы повторный "Сохранить" после частичного сбоя не дублировал.
    async commitCustom() {
      while (this.deletedCustomIds.length) {
        await deleteCustomField(this.deletedCustomIds[0]);
        this.deletedCustomIds.shift();
      }
      for (let i = 0; i < this.customFields.length; i += 1) {
        const cf = this.customFields[i];
        const body = {
          label: cf.label.trim(),
          placeholder: cf.placeholder || '',
          sortOrder: i,
          isRequired: cf.required,
        };
        if (cf.id == null) {
          const created = await createCustomField(this.uniqueAttachmentId, body);
          if (created?.id != null) cf.id = created.id;
        } else {
          await updateCustomField(cf.id, body);
        }
      }
    },
    async save() {
      if (!this.isDirty || this.isSaving) return;
      if (this.customFields.some((c) => !c.label.trim())) {
        useDeletionsStore().notify({ prefix: 'Заполните ', bold: 'заголовок поля', type: 'error' });
        return;
      }
      this.isSaving = true;
      try {
        if (this.isBaseDirty) await saveFieldConfig(this.uniqueAttachmentId, this.payload);
        if (this.isCustomDirty) await this.commitCustom();
        useDeletionsStore().notify({ prefix: 'Настройка полей ', bold: 'сохранена', type: 'success' });
        this.$emit('saved');
        this.requestClose();
      } catch (error) {
        console.error('Error saving field config:', error);
        useDeletionsStore().notify({ prefix: 'Не удалось ', bold: 'сохранить настройку полей', type: 'error' });
      } finally {
        this.isSaving = false;
      }
    },
  },
};
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 12000;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
}

.fields-modal {
  background: white;
  border-radius: 30px;
  width: 960px;
  max-width: 95%;
  max-height: calc(var(--app-vh, 1vh) * 85);
  display: flex;
  flex-direction: column;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
}

/* Анимация открытия/закрытия (паттерн BaseModal): overlay fade + контент scale */
.modal-fade-enter-active {
  transition: opacity 0.25s ease;
}

.modal-fade-leave-active {
  transition: opacity 0.2s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-fade-enter-active .fields-modal {
  animation: modal-scale-in 0.25s ease;
}

.modal-fade-leave-active .fields-modal {
  animation: modal-scale-out 0.2s ease;
}

@keyframes modal-scale-in {
  from { transform: scale(0.95); opacity: 0; }
  to { transform: scale(1); opacity: 1; }
}

@keyframes modal-scale-out {
  from { transform: scale(1); opacity: 1; }
  to { transform: scale(0.95); opacity: 0; }
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 15px 25px;
  border-bottom: 1px solid #e6e6e6;
}

.modal-header h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: #333;
}

.close-btn {
  background: none;
  border: none;
  font-size: 24px;
  color: #a2a2a2;
  cursor: pointer;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  transition: all 0.2s ease;
}

.close-btn:hover {
  background: #f5f5f5;
  color: #333;
}

.modal-content {
  padding: 20px 25px;
  overflow-y: auto;
}

.modal-state {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
  color: #a2a2a2;
}

.fields-hint {
  margin: 0 0 16px;
  font-size: 12px;
  line-height: 1.5;
  color: #a2a2a2;
}

/* Группы - явные блоки-карточки, тайлятся горизонтально; узко -> в столбец. */
.groups {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  align-items: flex-start;
}

.gcard {
  flex: 1 1 300px;
  min-width: 280px;
  border: 1px solid #e6e9f0;
  border-radius: var(--radius-md);
  background: #fff;
  overflow: hidden;
  box-shadow: 0 1px 2px rgba(20, 24, 40, 0.04);
}

/* Доп. поля - отдельный блок на всю ширину снизу. */
.custom-card {
  flex: none;
  width: 100%;
  margin-top: 16px;
}

.ghead {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 11px 16px;
  background: #fafbfc;
  border-bottom: 1px solid #e6e9f0;
}

.ghead-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--color-primary);
  flex: none;
}

.ghead h4 {
  margin: 0;
  font-size: 13px;
  font-weight: 700;
  color: #333;
}

.ghead-count {
  margin-left: auto;
  font-size: 11px;
  color: #a2a2a2;
}

/* Матрица: Поле | Показывать | Обязательно - колонки чётко разделены. */
.matrix-head,
.matrix-row {
  display: grid;
  grid-template-columns: 1fr 92px 92px;
  align-items: center;
}

.matrix-head {
  background: #f3f4fb;
  border-bottom: 1px solid #e6e9f0;
}

.matrix-head span {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: #7c8190;
  text-align: center;
  padding: 9px 6px;
}

.matrix-head span:first-child {
  text-align: left;
  padding-left: 16px;
}

.matrix-head span:nth-child(2),
.matrix-head span:nth-child(3) {
  border-left: 1px solid #e6e9f0;
}

.matrix-head span:nth-child(3) {
  background: #f6f7fe;
}

.matrix-row {
  border-bottom: 1px solid #f1f3f6;
}

.matrix-row:last-child {
  border-bottom: none;
}

.matrix-name {
  font-size: 13.5px;
  font-weight: 600;
  color: #333;
  padding: 11px 8px 11px 16px;
}

.matrix-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  align-self: stretch;
  border-left: 1px solid #eef1f4;
  padding: 9px 0;
}

.matrix-cell--req {
  background: #f6f7fe;
}

.matrix-dash {
  color: #cfd3dc;
  font-size: 16px;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 15px 25px;
  border-top: 1px solid #e6e6e6;
}

.custom-hint {
  margin: 0 0 12px;
  font-size: 12px;
  line-height: 1.5;
  color: #a2a2a2;
}

.custom-empty {
  font-size: 13px;
  color: #a2a2a2;
  padding: 8px 0 12px;
}

.custom-body {
  padding: 14px 16px;
}

/* Доп. поля - компактная таблица со скроллом (масштабируется при многих). */
.ctable {
  border: 1px solid #e6e9f0;
  border-radius: 12px;
  overflow: hidden;
}

.ctable-head,
.ctable-row {
  display: grid;
  grid-template-columns: 18px 1.2fr 1.2fr 84px 96px;
  gap: 8px;
  align-items: center;
}

.ctable-head {
  background: #f3f4fb;
  padding: 8px 12px;
}

.ctable-head span {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: #7c8190;
}

.ctable-head--c {
  text-align: center;
}

.ctable-scroll {
  max-height: 220px;
  overflow-y: auto;
}

.ctable-row {
  padding: 7px 12px;
  border-top: 1px solid #f1f3f6;
}

.ctable-grip {
  color: #c4c8d2;
  cursor: grab;
  text-align: center;
  font-size: 13px;
}

.ctable-input {
  width: 100%;
  min-width: 0;
}

.ctable-req {
  display: flex;
  justify-content: center;
}

.ctable-ctl {
  display: flex;
  gap: 4px;
  align-items: center;
  justify-content: flex-end;
}

.order-btn {
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f6f8;
  border: 1px solid #e6e6e6;
  border-radius: 8px;
  color: #555;
  font-size: 14px;
  cursor: pointer;
  transition: background 0.2s ease, color 0.2s ease;
}

.order-btn:hover:not(:disabled) {
  background: #e9ebef;
  color: #333;
}

.order-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.custom-delete {
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  border-radius: 50%;
  color: #c0392b;
  font-size: 20px;
  cursor: pointer;
  transition: background 0.2s ease;
}

.custom-delete:hover {
  background: #fdecea;
}

.custom-add {
  margin-top: 12px;
}
</style>
