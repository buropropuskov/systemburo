<template>
  <div class="form-container">
    <h2>Подать заявку</h2>
    <form @submit.prevent="submitApplication">
      <div class="form-group">
        <label>Организация:</label>
        <input
          v-model="formData.organization"
          type="text"
          required
          disabled
        >
      </div>

      <div class="form-group">
        <label>Ответственное лицо:</label>
        <input
          v-model="formData.responsible_person"
          type="text"
          required
        >
      </div>

      <div class="form-group">
        <label>Контактный телефон:</label>
        <input
          v-model="formData.contact_phone"
          type="tel"
          required
        >
      </div>

      <div class="form-group">
        <label>Дата въезда:</label>
        <input
          v-model="formData.entry_date"
          type="date"
          required
        >
      </div>

      <div class="form-group">
        <label>Время пребывания:</label>
        <input
          v-model="formData.entry_time"
          type="time"
          required
        >
      </div>

      <div class="form-group">
        <label>Номер автомобиля:</label>
        <div class="car-boxes">
          <input 
            v-model="carNumber.part1" 
            maxlength="1" 
            placeholder="X" 
            :required="isCarNumberRequired" 
            pattern="[А-ЯA-Za-z]" 
            @input="formatPart1" 
          >
          <input 
            v-model="carNumber.part2" 
            maxlength="3" 
            placeholder="123" 
            :required="isCarNumberRequired" 
            pattern="[0-9]{3}" 
            @input="formatPart2" 
          >
          <input 
            v-model="carNumber.part3" 
            maxlength="2" 
            placeholder="AB" 
            :required="isCarNumberRequired" 
            pattern="[А-ЯA-Za-z]{2}" 
            @input="formatPart3" 
          >
          <input 
            v-model="carNumber.part4" 
            maxlength="3" 
            placeholder="123" 
            :required="isCarNumberRequired" 
            pattern="[0-9]{3}" 
            @input="formatPart4" 
          >
        </div>
      </div>

      <div class="form-group select">
        <label>Марка автомобиля:</label>
        <v-select 
          v-model="carBrand"
          :options="carBrands" 
          :filterable="true"
          placeholder="Выберите или введите марку"
          :required="isCarNumberRequired"
          @search="filterBrands"
        />
      </div>

      <div class="form-group">
        <label>Место разгрузки:</label>
        <select v-model="unloadPlace">
          <option
            value=""
            disabled
          >
            Выберите место разгрузки
          </option>
          <option>1 дебаркадер</option>
          <option>2 дебаркадер</option>
          <option>3 дебаркадер</option>
          <option>4 дебаркадер</option>
          <option>5 дебаркадер</option>
          <option>27 пост</option>
          <option>Ворота Сочи</option>
        </select>
      </div>

      <button @click.prevent="addCar">
        Добавить автомобиль
      </button>
    </form>

    <div class="car-list">
      <h3>Добавленные автомобили</h3>
      <ul>
        <li
          v-for="(car, index) in formData.cars"
          :key="index"
        >
          {{ car.car_number }} | {{ car.mark_name || car.car_brand }} | {{ car.unload_place }}
        </li>
      </ul>
    </div>

    <button
      :disabled="formData.cars.length === 0"
      @click.prevent="submitApplication"
    >
      Отправить заявку
    </button>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import vSelect from "vue-select";
import "vue-select/dist/vue-select.css";

export default {
  components: {
    vSelect
  },
  emits: ['submitted'],
  data() {
    return {
      formData: {
        organization: '',
        responsible_person: '',
        contact_phone: '',
        entry_date: '',
        entry_time: '',
        cars: []
      },
      carNumber: { part1: '', part2: '', part3: '', part4: '' },
      carBrand: '',
      unloadPlace: '',
      existingCars: [],
      // carBrands заполняется из API /marks (активные), см. fetchMarks.
      // Раньше был hardcoded список - теперь динамический справочник (#185).
      carBrands: [],
      markId: null,
    };
  },
  computed: {
    isCarNumberRequired() {
      return this.formData.cars.length === 0;
    },
    fullCarNumber() {
      return `${this.carNumber.part1} ${this.carNumber.part2} ${this.carNumber.part3} ${this.carNumber.part4}`;
    }
  },
  async mounted() {
    await this.fetchOrganization();
    await this.fetchMarks();
  },
  methods: {
    formatPart1() {
      this.carNumber.part1 = this.carNumber.part1
        .toUpperCase()
        .replace(/[^А-ЯA-Za-z]/g, '');
    },
    formatPart2() {
      this.carNumber.part2 = this.carNumber.part2.replace(/[^0-9]/g, '');
    },
    formatPart3() {
      this.carNumber.part3 = this.carNumber.part3
        .toUpperCase()
        .replace(/[^А-ЯA-Za-z]/g, '');
    },
    formatPart4() {
      this.carNumber.part4 = this.carNumber.part4.replace(/[^0-9]/g, '');
    },
    async fetchMarks() {
      try {
        const { listMarks } = await import('@/api/marks');
        const data = await listMarks({ includeArchived: false });
        const arr = Array.isArray(data) ? data : [];
        // v-select поддерживает options как массив объектов или строк.
        // Используем массив {label, value} - label отображается, value идёт в v-model.
        this.carBrands = arr.map(m => ({ label: m.name, value: m.id, name: m.name }));
      } catch {
        // Fallback на пустой список - юзер сможет ввести вручную.
        this.carBrands = [];
      }
    },
    async fetchOrganization() {
      const authStore = useAuthStore();
      if (!authStore.token) {
        console.error("Токен не найден. Пользователь не авторизован.");
        return;
      }

      try {
        const response = await apiRequest("/get-organization", {
          method: "GET"});

        if (response.ok) {
          const data = await response.json();
          this.formData.organization = data.organization;
          await this.fetchExistingCars();
        } else {
          const errorText = await response.text();
          console.error("Ошибка при получении данных организации:", errorText);
        }
      } catch (error) {
        console.error("Ошибка при обращении к серверу:", error);
      }
    },
    async fetchExistingCars() {
      try {
        const response = await apiRequest("/applications/active-cars", {});
        
        if (response.ok) {
          const data = await response.json();
          this.existingCars = data.flatMap(application => 
            application.cars.map(car => ({
              car_number: car.car_number,
              organization: application.organization,
              entry_date: application.entry_date,
              entry_time: application.entry_time
            }))
          );
        }
      } catch (error) {
        console.error("Ошибка загрузки машин:", error);
      }
    },
    addCar() {
      if (this.carBrand && this.unloadPlace && this.fullCarNumber.replace(/\s/g, '').length >= 9) {
        const cleanCarNumber = this.fullCarNumber.replace(/\s/g, '');
        const existingCar = this.existingCars.find(
          car => car.car_number.replace(/\s/g, '') === cleanCarNumber && 
                 car.organization === this.formData.organization
        );

        if (existingCar) {
          const existingDateTime = new Date(`${existingCar.entry_date}T${existingCar.entry_time}`);
          const newDateTime = new Date(`${this.formData.entry_date}T${this.formData.entry_time}`);

          if (newDateTime <= existingDateTime) {
            alert("Такая машина уже зарегистрирована на более длительный срок.");
            return;
          }
        }

        // carBrand может быть объектом (выбран из справочника) или строкой (ручной ввод).
        // Передаём обе формы - backend решает: при наличии mark_id сохраняет
        // snapshot имени в mark_name, иначе используется свободный car_brand.
        const isMarkObject = this.carBrand && typeof this.carBrand === 'object';
        const car = {
          car_number: this.fullCarNumber,
          car_brand: isMarkObject ? this.carBrand.name : this.carBrand,
          mark_id: isMarkObject ? this.carBrand.value : null,
          mark_name: isMarkObject ? this.carBrand.name : this.carBrand,
          unload_place: this.unloadPlace
        };
        
        this.formData.cars.push(car);
        this.resetCarForm();
      } else {
        alert("Заполните все поля для автомобиля перед добавлением.");
      }
    },
    resetCarForm() {
      this.carNumber = { part1: '', part2: '', part3: '', part4: '' };
      this.carBrand = '';
      this.unloadPlace = '';
    },
    async submitApplication() {
      if (this.formData.cars.length === 0) {
        alert("Добавьте хотя бы один автомобиль");
        return;
      }

      try {
        const response = await apiRequest("/submit", {
          method: "POST",
          body: JSON.stringify(this.formData)
        });
        
        const responseData = await response.json().catch(() => {
          return { message: "Не удалось разобрать ответ сервера" };
        });

        if (response.ok) {
          alert("Заявка успешно отправлена");
          this.resetForm();
          this.$emit('submitted');
        } else {
          console.error("Ошибка от сервера:", responseData);
          alert(`Ошибка при отправке: ${responseData.message || "Неизвестная ошибка"}`);
        }
      } catch (error) {
        console.error("Ошибка при отправке заявки:", error);
        alert("Произошла ошибка при отправке заявки. Проверьте консоль для подробностей.");
      }
    },
    resetForm() {
      this.formData = {
        organization: this.formData.organization,
        responsible_person: '',
        contact_phone: '',
        entry_date: '',
        entry_time: '',
        cars: []
      };
      this.resetCarForm();
    }
  }
};
</script>

<style scoped>
.form-container {
  max-width: 600px;
  margin: 30px auto;
  padding: 25px;
  border-radius: 10px;
  box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
  background-color: #ffffff;
}

h2 {
  font-size: 1.8em;
  margin-bottom: 20px;
  text-align: center;
  color: #333;
}

.form-group {
  display: flex;
  flex-direction: column;
  margin-bottom: 15px;
}

label {
  font-size: 1em;
  font-weight: 600;
  color: #555;
  margin-bottom: 5px;
}

input[type="text"],
input[type="tel"],
input[type="date"],
input[type="time"],
select {
  padding: 10px;
  border: 1px solid #ddd;
  border-radius: 5px;
  font-size: 1em;
  color: #333;
  background-color: #f9f9f9;
  outline: none;
  transition: border-color 0.2s ease;
}

input:focus,
select:focus {
  border-color: #F70;
}

.select {
  cursor: pointer;
}

button {
  width: 100%;
  padding: 12px;
  font-size: 1em;
  font-weight: 600;
  background-color: #a2a2a2;
  color: white;
  border: none;
  cursor: pointer;
  border-radius: 5px;
  transition: background-color 0.3s ease;
  margin-top: 10px;
}

button:hover {
  background-color: #f70;
}

button:disabled {
  background-color: #ccc;
  cursor: not-allowed;
}

.car-boxes {
  display: flex;
  gap: 5px;
}

.car-boxes input {
  padding: 10px;
  border: 1px solid #ddd;
  border-radius: 5px;
  text-align: center;
  font-size: 1em;
  color: #333;
  background-color: #f9f9f9;
  width: 20%;
}

.car-list {
  margin-top: 20px;
  text-align: center;
  background-color: #f0f0f0;
  border-radius: 8px;
  padding: 5px;
}

.car-list h3 {
  font-size: 1.2em;
}

.car-list ul {
  list-style: none;
  padding: 0;
}

.car-list li {
  padding: 8px;
  margin: 5px 0;
  background-color: #e9e9e9;
  border-radius: 5px;
  color: #333;
}
</style>