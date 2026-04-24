<template>
  <div class="table-container">
    <transition name="slide-down">
      <div
        v-if="notification.message"
        class="notification"
      >
        <span>{{ notification.message }}</span>
        <button @click="undoDelete">
          Отменить
        </button>
        <div
          class="progress-bar"
          :style="{ width: `${progress}%` }"
        />
      </div>
    </transition>
    <div class="header-container">
      <h2>Таблица автомобилей КПП №4</h2>
      <!-- Поле поиска и кнопка для массового удаления -->
      <div class="search-controls">
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Поиск"
          class="search-input"
        >
        <button
          class="delete-all"
          :disabled="selectedCars.length === 0"
          @click="deleteSelectedCars"
        >
          Удалить выбранные
        </button>
      </div>
    </div>

    <table>
      <thead>
        <tr>
          <th style="width: 40px;">
            <input
              type="checkbox"
              @change="toggleSelectAll"
            >
          </th>
          <th
            style="width: 120px; cursor: pointer;"
            @click="sortBy('car_number')"
          >
            Номер<span v-if="sortKey === 'car_number'">{{ sortOrder === 'asc' ? '▲' : '▼' }}</span>
          </th>
          <th
            style="width: 120px; cursor: pointer;"
            @click="sortBy('car_brand')"
          >
            Марка <span v-if="sortKey === 'car_brand'">{{ sortOrder === 'asc' ? '▲' : '▼' }}</span>
          </th>
          <th
            style="width: 150px; cursor: pointer;"
            @click="sortBy('organization')"
          >
            Организация <span v-if="sortKey === 'organization'">{{ sortOrder === 'asc' ? '▲' : '▼' }}</span>
          </th>
          <th style="width: 150px;">
            Место разгрузки
          </th>
          <th
            style="width: 100px; cursor: pointer;"
            @click="sortBy('entry_date')"
          >
            Дата <span v-if="sortKey === 'entry_date'">{{ sortOrder === 'asc' ? '▲' : '▼' }}</span>
          </th>
          <th style="width: 80px;">
            Время
          </th>
          <th style="width: 100px;">
            Действия
          </th>
        </tr>
      </thead>
      <tbody>
        <template v-if="filteredCars.length > 0">
          <tr
            v-for="carData in filteredCars"
            :key="carData.car.id"
            :class="{ overdue: isOverdue(carData.application) }"
          >
            <td>
              <input
                v-model="selectedCars"
                type="checkbox"
                :value="carData.car.id"
              >
            </td>
            <td>
              <input
                v-model="carData.car.car_number"
                placeholder="Номер авто"
                @blur="updateCar(carData.application.id, carData.car)"
              >
            </td>
            <td>
              <input
                v-model="carData.car.car_brand"
                placeholder="Марка авто"
                @blur="updateCar(carData.application.id, carData.car)"
              >
            </td>
            <td>{{ carData.application.organization }}</td>
            <td>
              <input
                v-model="carData.car.unload_place"
                placeholder="Место разгрузки"
                @blur="updateCar(carData.application.id, carData.car)"
              >
            </td>
            <td>
              <input
                v-model="carData.application.entry_date"
                type="date"
                @blur="updateApplication(carData.application)"
              >
            </td>
            <td>
              <input
                v-model="carData.application.entry_time"
                type="time"
                @blur="updateApplication(carData.application)"
              >
            </td>
            <td>
              <button @click="removeCarWithNotification(carData.application.id, carData.car)">
                Удалить
              </button>
            </td>
          </tr>
        </template>
        <tr v-else>
          <td colspan="8">
            Ничего не найдено
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
export default {
  data() {
    return {
      applications: [],
      searchQuery: '',
      selectedCars: [],
      sortKey: '',
      sortOrder: 'asc',
      notification: { message: null, car: null },
      progress: 100,
      progressInterval: null,
    };
  },
  computed: {
    filteredCars() {
      const query = this.searchQuery.toLowerCase();
      return this.applications
        .flatMap(application =>
          application.cars
            .filter(car =>
              car.car_number?.toLowerCase().includes(query) ||
              car.car_brand?.toLowerCase().includes(query) ||
              application.organization?.toLowerCase().includes(query)
            )
            .map(car => ({ car, application }))
        )
        .sort((a, b) => this.sortFunction(a, b));
    }
  },
  mounted() {
    this.fetchApplications();
  },
  methods: {
    async actuallyDeleteCar(applicationId, car) {
  try {
    const response = await apiRequest(`/applications/${applicationId}/cars/${car.id}`, {
      method: "PUT",
      body: JSON.stringify({ ...car, status: 0 })
    });
    
    if (!response.ok) {
      console.error("Ошибка при удалении машины");
      // Если не удалось удалить, возвращаем предыдущий статус
      car.status = this.notification.originalStatus;
    }
    this.fetchApplications(); // Обновляем данные после удаления
  } catch (error) {
    console.error("Ошибка сети при удалении машины:", error);
    car.status = this.notification.originalStatus;
  }
},
    async fetchApplications() {
      try {
        const response = await apiRequest("/applications/active-cars");
        const data = await response.json();
        this.applications = data;
      } catch {
        alert("Ошибка при загрузке заявок");
      }
    },
    async updateApplication(application) {
      try {
        const response = await apiRequest(`/applications/${application.id}`, {
          method: "PUT",
          body: JSON.stringify({
            entry_date: application.entry_date,
            entry_time: application.entry_time
          })
        });

        if (!response.ok) {
          console.error(`Ошибка при обновлении заявки: ${response.statusText}`);
          alert("Ошибка при обновлении заявки");
        }
      } catch (error) {
        console.error("Ошибка сети при обновлении заявки:", error);
        alert("Ошибка при обновлении заявки");
      }
    },
    async updateCar(applicationId, car) {
      try {
        const response = await apiRequest(`/applications/${applicationId}/cars/${car.id}`, {
          method: "PUT",
          body: JSON.stringify(car)
        });

        if (!response.ok) alert("Ошибка при обновлении машины");
      } catch {
        alert("Ошибка при обновлении машины");
      }
    },
    sortBy(key) {
      if (this.sortKey === key) {
        this.sortOrder = this.sortOrder === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortKey = key;
        this.sortOrder = 'asc';
      }
    },
    sortFunction(a, b) {
      const order = this.sortOrder === 'asc' ? 1 : -1;
      let valA, valB;

      if (this.sortKey === 'car_number') {
        const getCentralDigits = (number) => {
          const match = number && number.match(/\d{3}/);
          return match ? parseInt(match[0], 10) : 0;
        };
        valA = getCentralDigits(a.car.car_number);
        valB = getCentralDigits(b.car.car_number);
      } else if (this.sortKey === 'entry_date') {
        valA = new Date(a.application.entry_date);
        valB = new Date(b.application.entry_date);
      } else {
        valA = a.car[this.sortKey]?.toString().toLowerCase() || a.application[this.sortKey]?.toString().toLowerCase();
        valB = b.car[this.sortKey]?.toString().toLowerCase() || b.application[this.sortKey]?.toString().toLowerCase();
      }

      if (valA < valB) return -1 * order;
      if (valA > valB) return 1 * order;
      return 0;
    },
    isOverdue(application) {
      const now = new Date();
      const entryDateTime = new Date(`${application.entry_date}T${application.entry_time}`);
      return entryDateTime < now;
    },
    removeCarWithNotification(applicationId, car) {
  // Сохраняем исходное состояние машины для возможного восстановления
  const originalCar = { ...car };
  
  this.notification = { 
    message: `Машина ${car.car_number} удалена.`, 
    car,
    applicationId,
    originalStatus: car.status,
    undoFunction: async () => {
      try {
        car.status = originalCar.status;
        const response = await apiRequest(`/applications/${applicationId}/cars/${car.id}`, {
          method: "PUT",
          body: JSON.stringify({ ...car, status: originalCar.status })
        });
        
        if (!response.ok) throw new Error("Ошибка восстановления");
        this.fetchApplications(); // Обновляем данные
      } catch (error) {
        console.error("Ошибка при отмене удаления:", error);
      }
    }
  };

  // Помечаем как удаленную локально, но не отправляем на сервер
  car.status = 0;

  this.progress = 100;
  clearInterval(this.progressInterval);
  this.progressInterval = setInterval(() => {
    this.progress -= 1.8;
    if (this.progress <= 0) {
      clearInterval(this.progressInterval);
      // Только после завершения таймера отправляем запрос на сервер
      this.actuallyDeleteCar(applicationId, car);
      if (this.notification.car?.id === car.id) {
        this.notification = { message: null, car: null };
      }
    }
  }, 100);
},
    undoDelete() {
  clearInterval(this.progressInterval);
  if (this.notification.undoFunction) {
    this.notification.undoFunction();
  }
  this.notification = { message: null, car: null };
}
  }
};
</script>

<style scoped>
.table-container {
  max-width: 1100px;
  margin: 0 auto;
  margin-top: 50px;
  padding: 20px;
  border: 1px solid #ccc;
  border-radius: 8px;
  background-color: #f9f9f9;
}

.notification {
  position: fixed;
  top: 20px;
  left: 50%;
  width: 450px;
  transform: translateX(-50%);
  padding: 15px;
  background-color: #fff;
  color: #000;
  border: 2px solid #ddd;
  box-shadow: 0px 4px 6px rgba(0, 0, 0, 0.2);
  display: flex;
  align-items: center;
  justify-content: space-between;
  transition: all 0.5s ease;
  animation: slide-down 0.5s ease;
  overflow: hidden;
}

.slide-down-enter-active,
.slide-down-leave-active {
  transition: transform 0.3s ease, opacity 0.3s ease;
}

.slide-down-enter,
.slide-down-leave-to {
  transform: translate(-50%, -100%);
  opacity: 0;
}

.notification button {
  background: red;
  color: #fff;
  border: none;
  margin: 0;
  padding: 5px;
  cursor: pointer;
  font-size: 0.9em;
  border-radius: 4px;
}

.progress-bar {
  position: absolute;
  bottom: 0;
  left: 0;
  height: 5px;
  background-color: lime;
  transition: width 0.1s linear;
}

.header-container {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.search-input {
  width: 180px;
  padding: 5px;
  font-size: 14px;
  margin-right: 25px;
  border-radius: 5px;
  border: 2px solid #ddd;
}

table {
  width: 100%;
  border-collapse: collapse;
}

th, td {
  padding: 5px;
  border: 1px solid #ddd;
  text-align: center;
  font-size: 14px;
}

input[type="checkbox"] {
  transform: scale(1.2);
}

input {
  width: 100%;
  border: none;
  background: transparent;
  text-align: center;
}

button {
  padding: 5px 10px;
  background-color: red;
  color: white;
  border: none;
  cursor: pointer;
  border-radius: 4px;
  transition: .2s;
  margin-top: 10px;
}

button:disabled {
  background-color: #ccc;
  cursor: not-allowed;
}

button:hover:not(:disabled) {
  background-color: rgb(135, 20, 20);
}

.delete-all {
  width: 160px;
  height: 30px;
  border-radius: 5px;
}

.overdue {
  background-color: rgba(255, 0, 0, 0.089);
}

@keyframes slide-down {
  from {
    transform: translate(-50%, -100%);
  }
  to {
    transform: translate(-50%, 0);
  }
}
</style>
