<template>
  <Teleport to="body">
    <div
      class="modal-overlay"
      @click.self="$emit('close')"
    >
      <div class="modal-container">
        <div class="modal-header">
          <h3 class="modal-title">
            Создать новую учётную запись
          </h3>
          <button
            class="modal-close-btn"
            @click="$emit('close')"
          >
            &times;
          </button>
        </div>
      
        <div class="modal-body">
          <div class="form-section">
            <h4 class="section-title">
              Основные данные
            </h4>
            <div class="form-grid">
              <div class="form-group">
                <label class="form-label">Логин:</label>
                <input 
                  v-model="newUser.username" 
                  type="text" 
                  placeholder="Введите логин"
                  class="form-input"
                  required
                  autocomplete="off"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                >
              </div>

              <div class="form-group">
                <label class="form-label">Пароль:</label>
                <PasswordInput
                  v-model="newUser.password"
                  placeholder="Введите пароль"
                  required
                  input-class="form-input"
                />
              </div>

              <div class="form-group form-group--full">
                <label class="form-label">Тип пользователя:</label>
                <select
                  v-model="newUser.type_id"
                  required
                  class="form-select"
                >
                  <option
                    :value="null"
                    disabled
                  >
                    Выберите тип
                  </option>
                  <option
                    v-for="type in userTypes"
                    :key="type.id"
                    :value="type.id"
                  >
                    {{ type.name }}
                  </option>
                </select>
              </div>
            </div>
          </div>

          <div class="form-section">
            <h4 class="section-title">
              Персональные данные
            </h4>
            <div class="form-grid">
              <div class="form-group">
                <label class="form-label">Фамилия:</label>
                <input 
                  v-model="newUser.last_name" 
                  type="text" 
                  placeholder="Введите фамилию"
                  class="form-input"
                  autocomplete="off"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                >
              </div>

              <div class="form-group">
                <label class="form-label">Имя:</label>
                <input 
                  v-model="newUser.first_name" 
                  type="text" 
                  placeholder="Введите имя"
                  class="form-input"
                  autocomplete="off"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                >
              </div>

              <div class="form-group">
                <label class="form-label">Отчество:</label>
                <input 
                  v-model="newUser.middle_name" 
                  type="text" 
                  placeholder="Введите отчество"
                  class="form-input"
                  autocomplete="off"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                >
              </div>

              <div class="form-group">
                <label class="form-label">Должность:</label>
                <input 
                  v-model="newUser.position" 
                  type="text" 
                  placeholder="Введите должность"
                  class="form-input"
                  autocomplete="off"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                >
              </div>

              <div class="form-group">
                <label class="form-label">Email:</label>
                <input 
                  v-model="newUser.email" 
                  type="email" 
                  placeholder="Введите email"
                  class="form-input"
                  autocomplete="off"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                >
              </div>

              <div class="form-group">
                <label class="form-label">Телефон:</label>
                <input
                  :value="newUser.phone"
                  type="tel"
                  placeholder="+7 (___) ___ __-__"
                  class="form-input"
                  autocomplete="off"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                  @input="newUser.phone = formatRussianPhone($event.target.value)"
                >
              </div>
            </div>
          </div>

          <div class="form-section">
            <h4 class="section-title">
              Организационные данные
            </h4>
            <div class="form-grid">
              <div class="form-group">
                <label class="form-label">Компания:</label>
                <select 
                  v-model="newUser.company_id" 
                  required 
                  class="form-select"
                >
                  <option
                    :value="null"
                    disabled
                  >
                    Выберите компанию
                  </option>
                  <option
                    v-for="comp in companies"
                    :key="comp.id"
                    :value="comp.id"
                  >
                    {{ comp.name }}
                  </option>
                </select>
              </div>

              <div class="form-group">
                <label class="form-label">Организация:</label>
                <select 
                  v-model="newUser.organization_id" 
                  required 
                  class="form-select"
                >
                  <option
                    :value="null"
                    disabled
                  >
                    Выберите организацию
                  </option>
                  <option
                    v-for="org in organizations"
                    :key="org.id"
                    :value="org.id"
                  >
                    {{ org.name }}
                  </option>
                </select>
              </div>
            </div>
          </div>
        </div>

        <div class="modal-footer">
          <button
            class="modal-cancel-btn"
            @click="$emit('close')"
          >
            Отмена
          </button>
          <button 
            :disabled="!isFormValid" 
            class="modal-confirm-btn"
            @click="createNewUser"
          >
            Создать
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script>
import { apiRequest } from '@/api/client'
import { mapState, mapActions } from 'pinia';
import { useOrganizationsStore } from '@/stores/organizations';
import { useCompaniesStore } from '@/stores/companies';
import { formatRussianPhone } from '@/composables/useRussianPhoneMask'
import PasswordInput from '@/components/ui/PasswordInput.vue';
export default {
  components: { PasswordInput },
  props: {
    userTypes: {
      type: Array,
      required: true
    }
  },
  emits: ['close', 'user-created'],
  data() {
    return {
      newUser: {
        username: '',
        password: '',
        company_id: null,
        organization_id: null,
        type_id: null,
        last_name: '',
        first_name: '',
        middle_name: '',
        position: '',
        email: '',
        phone: ''
      }
    };
  },
  computed: {
    ...mapState(useOrganizationsStore, { organizations: 'items' }),
    ...mapState(useCompaniesStore, { companies: 'items' }),
    isFormValid() {
      return (
        this.newUser.username &&
        this.newUser.password &&
        this.newUser.company_id &&
        this.newUser.organization_id &&
        this.newUser.type_id
      );
    }
  },
  async created() {
    // Если списки в стор'ах ещё не загружены - подтягиваем. Если уже есть -
    // pinia вернёт текущий state мгновенно, лишних запросов не будет.
    const orgs = useOrganizationsStore();
    const comps = useCompaniesStore();
    const promises = [];
    if (orgs.items.length === 0) promises.push(this.fetchOrganizations());
    if (comps.items.length === 0) promises.push(this.fetchCompanies());
    if (promises.length) await Promise.all(promises);
  },
  methods: {
    ...mapActions(useOrganizationsStore, ['fetchOrganizations']),
    ...mapActions(useCompaniesStore, ['fetchCompanies']),
    formatRussianPhone,
    async createNewUser() {
      try {
        const userData = {
          username: this.newUser.username,
          password: this.newUser.password,
          company_id: this.newUser.company_id,
          organization_id: this.newUser.organization_id,
          type_id: this.newUser.type_id,
          last_name: this.newUser.last_name || null,
          first_name: this.newUser.first_name || null,
          middle_name: this.newUser.middle_name || null,
          position: this.newUser.position || null,
          email: this.newUser.email || null,
          phone: this.newUser.phone || null
        };

        // Админская регистрация через POST /users (JWT-защищённый).
        // Публичный /register намеренно не экспонируется.
        const response = await apiRequest("/users", {
          method: "POST",
          body: JSON.stringify(userData)
        });

        if (response.ok) {
          alert("Пользователь успешно создан");
          this.resetForm();
          this.$emit('user-created');
        } else {
          const errorData = await response.json();
          alert(errorData.message || "Ошибка при создании пользователя");
        }
      } catch (error) {
        console.error("Ошибка сети при создании пользователя:", error);
        alert("Не удалось создать пользователя");
      }
    },
    resetForm() {
      this.newUser = {
        username: '',
        password: '',
        company_id: null,
        organization_id: null,
        type_id: null,
        last_name: '',
        first_name: '',
        middle_name: '',
        position: '',
        email: '',
        phone: ''
      };
    }
  }
};
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 1000;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
}

.modal-container {
  background-color: white;
  border-radius: 12px;
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.2);
  width: 100%;
  max-width: 600px;
  max-height: 90vh;
  overflow-y: auto;
  margin: 20px;
  animation: modal-appear 0.3s ease-out;
}

@keyframes modal-appear {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.modal-header {
  padding: 20px;
  border-bottom: 1px solid #eee;
  display: flex;
  justify-content: space-between;
  align-items: center;
  background-color: #f9fafb;
  border-radius: 12px 12px 0 0;
}

.modal-title {
  margin: 0;
  font-size: 1.3em;
  color: #2d3748;
  font-weight: 600;
}

.modal-close-btn {
  background: none;
  border: none;
  font-size: 1.8em;
  cursor: pointer;
  color: #718096;
  padding: 0;
  line-height: 1;
  transition: color 0.2s, transform 0.2s;
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
}

.modal-close-btn:hover {
  color: #2d3748;
  background-color: #edf2f7;
  transform: rotate(90deg);
}

.modal-body {
  padding: 20px;
}

.form-section {
  margin-bottom: 24px;
}

.section-title {
  margin: 0 0 16px 0;
  font-size: 1.1em;
  color: #4a5568;
  font-weight: 600;
  padding-bottom: 8px;
  border-bottom: 1px solid #eee;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
  gap: 16px;
}

.form-group {
  margin-bottom: 0;
}

/* Растягиваем "Тип пользователя" на всю ширину form-grid (2 колонки) */
.form-group--full {
  grid-column: 1 / -1;
}

.form-label {
  display: block;
  margin-bottom: 6px;
  font-weight: 500;
  color: #4a5568;
  font-size: 0.9em;
}

.form-input {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  font-size: 0.95em;
  box-sizing: border-box;
  transition: all 0.2s;
  background-color: #fff;
  color: #2d3748;
}

.form-input:focus {
  border-color: #4299e1;
  outline: none;
  box-shadow: 0 0 0 3px rgba(66, 153, 225, 0.2);
}

.form-select {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  font-size: 0.95em;
  box-sizing: border-box;
  transition: all 0.2s;
  background-color: #fff;
  color: #2d3748;
  appearance: none;
  background-image: url("data:image/svg+xml;charset=UTF-8,%3csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='currentColor' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3e%3cpolyline points='6 9 12 15 18 9'%3e%3c/polyline%3e%3c/svg%3e");
  background-repeat: no-repeat;
  background-position: right 12px center;
  background-size: 1em;
}

.form-select:focus {
  border-color: #4299e1;
  outline: none;
  box-shadow: 0 0 0 3px rgba(66, 153, 225, 0.2);
}

.modal-footer {
  padding: 16px 20px;
  border-top: 1px solid #eee;
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  background-color: #f9fafb;
  border-radius: 0 0 12px 12px;
}

.modal-cancel-btn {
  padding: 10px 20px;
  background-color: #fff;
  color: #4a5568;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.95em;
  font-weight: 500;
  transition: all 0.2s;
}

.modal-cancel-btn:hover {
  background-color: #f7fafc;
  border-color: #cbd5e0;
}

.modal-confirm-btn {
  padding: 10px 20px;
  background-color: #4299e1;
  color: white;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.95em;
  font-weight: 500;
  transition: all 0.2s;
}

.modal-confirm-btn:hover {
  background-color: #3182ce;
}

.modal-confirm-btn:disabled {
  background-color: #a0aec0;
  cursor: not-allowed;
}

@media (max-width: 768px) {
  .modal-container {
    width: calc(100% - 40px);
    margin: 20px;
  }
  
  .modal-body {
    padding: 16px;
  }
  
  .form-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 480px) {
  .modal-footer {
    flex-direction: column;
  }
  
  .modal-cancel-btn, .modal-confirm-btn {
    width: 100%;
  }
}
</style>