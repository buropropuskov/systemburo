<template>
    <div class="data__completion">
        <div class="completion__header">
            <h3>Добавление Т/С</h3>
        </div>
        <div class="completion__format">
            <div class="format__header">
                <label class="format__label">Формат номеров</label>
                <button class="add-button" @click="addVehicle" :disabled="!canAddVehicle">
                    Добавить
                </button>
            </div>
            <div class="format__dropdown">
                <button class="dropdown__button" @click="toggleDropdown">
                    <div class="button__content">
                        <span class="button__text">{{ selectedFormatText }}</span>
                        <img src="@/assets/icons/arrow.png" class="button__arrow" :class="{ 'button__arrow--open': isDropdownOpen }" />
                    </div>
                </button>
                <div v-if="isDropdownOpen" class="dropdown__menu">
                    <div 
                        v-for="format in availableFormats" 
                        :key="format.format.id"
                        class="dropdown__item" 
                        @click="selectFormat(format)"
                    >
                        <span class="item__text">{{ format.format.name }}</span>
                    </div>
                </div>
            </div>
        </div>
        <div class="completion__fields">
            <div class="completion__number">
                <div class="completion__number-header">
                    <label class="input__label">Номер Т/C <span class="required">*</span></label>
                    <div class="number-fact">
                        <input 
                            class="fact-checkbox" 
                            type="checkbox" 
                            v-model="isNumberByFact"
                            @change="handleNumberByFactChange"
                        />
                        <p class="fact-text">по факту</p>
                    </div>
                </div>
                <!-- Поле "по факту" -->
                <div class="number__field number__field--fact" v-if="isNumberByFact">
                    <input 
                        class="number__input number__input--fact" 
                        value="По факту"
                        readonly
                    />
                </div>
                
                <!-- Динамический формат из базы данных -->
                <div class="number__field" v-else-if="selectedFormat">
                    <input 
                        v-for="(cell, index) in selectedFormat.cells" 
                        :key="index"
                        class="number__input" 
                        :placeholder="getPlaceholder(cell)"
                        v-model="numberParts[index]"
                        @input="validatePart(index, $event, cell)"
                        @blur="formatPart(index, cell)"
                        :maxlength="cell.max_length"
                        :style="{ width: getInputWidth(cell) }"
                    />
                </div>
            </div>
            
            <div class="completion__mark">
                <div class="completion__mark-header">
                    <label class="input__label">Марка Т/С <span class="required">*</span></label>
                    <div class="mark-fact">
                        <input 
                            class="fact-checkbox" 
                            type="checkbox" 
                            v-model="isMarkByFact"
                            @change="handleMarkByFactChange"
                        />
                        <p class="fact-text">по факту</p>
                    </div>
                </div>
                <div class="mark__field mark__field--fact" v-if="isMarkByFact">
                    <input 
                        class="mark__input mark__input--fact" 
                        value="По факту"
                        readonly
                    />
                </div>
                <div class="mark__field" v-else>
                    <div class="mark__dropdown">
                        <button class="mark__dropdown-button" @click="toggleMarkDropdown">
                            <div class="mark__button-content">
                                <span class="mark__button-text">{{ selectedMark || 'Выберите марку' }}</span>
                                <img src="@/assets/icons/arrow.png" class="mark__button-arrow" :class="{ 'mark__button-arrow--open': isMarkDropdownOpen }" />
                            </div>
                        </button>
                        <div v-if="isMarkDropdownOpen" class="mark__dropdown-menu">
                            <div class="mark__search">
                                <input 
                                    class="mark__search-input" 
                                    placeholder="Поиск марки..."
                                    v-model="markSearch"
                                    @input="filterMarks"
                                />
                            </div>
                            <div class="mark__dropdown-list">
                                <div 
                                    v-for="mark in filteredMarks" 
                                    :key="mark"
                                    class="mark__dropdown-item"
                                    @click="selectMark(mark)"
                                >
                                    <span class="mark__item-text">{{ mark }}</span>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Места разгрузки -->
        <div class="completion__unloading">
            <label class="input__label">Места разгрузки (выбор) <span class="required">*</span></label>
            <div class="unloading__grid" v-if="!loadingUnloadingPlaces && allUnloadingPlaces.length > 0">
                <div 
                    v-for="place in allUnloadingPlaces" 
                    :key="place.id"
                    class="unloading__item"
                    :class="{ 
                        'unloading__item--active': selectedUnloadingPlaces.includes(place.id),
                        'unloading__item--attached': attachedPlacesIds.includes(place.id)
                    }"
                    @click="toggleUnloadingPlace(place.id)"
                >
                    {{ place.name }}
                </div>
            </div>
            <div v-else-if="loadingUnloadingPlaces" class="loading-message">
                Загрузка мест разгрузки...
            </div>
            <div v-else class="no-places-message">
                Нет доступных мест разгрузки
            </div>
            <div v-if="errors.unloadingPlaces" class="error-message">{{ errors.unloadingPlaces }}</div>
        </div>
    </div>
</template>

<script>
export default {
    name: 'VehicleForm',
    props: {
        userOrganization: {
            type: String,
            default: ''
        },
        userOrganizationId: {
            type: Number,
            default: null
        },
        userCompany: {
            type: String,
            default: ''
        },
        userCompanyId: {
            type: Number,
            default: null
        }
    },
    data() {
        return {
            // Number parts
            numberParts: [],
            isNumberByFact: false,
            
            // Format data
            availableFormats: [],
            selectedFormat: null,
            
            // Mark data
            isMarkByFact: false,
            selectedMark: '',
            isMarkDropdownOpen: false,
            markSearch: '',
            
            // Available marks
            marks: [
                'ВАЗ', 'Мерседес', 'БМВ', 'Газель', 'ГАЗ', 'Вольво', 'Тойота', 'Митсубиси',
                'Ауди', 'Фольксваген', 'Шевроле', 'Хендай', 'Киа', 'Ниссан', 'Рено', 'Пежо',
                'Ситроен', 'Форд', 'Опель', 'Шкода', 'Лада', 'УАЗ'
            ],
            filteredMarks: [],
            
            // Unloading places
            allUnloadingPlaces: [],
            attachedUnloadingPlaces: [],
            selectedUnloadingPlaces: [],
            loadingUnloadingPlaces: false,
            
            // Validation
            errors: {
                unloadingPlaces: ''
            },
            
            // Dropdown
            isDropdownOpen: false,
            
            // Character sets
            allowedCyrillicLetters: ['А', 'В', 'Е', 'К', 'М', 'Н', 'О', 'Р', 'С', 'Т', 'У', 'Х'],
            allowedLatinLetters: ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z']
        }
    },
    computed: {
        selectedFormatText() {
            return this.selectedFormat ? this.selectedFormat.format.name : 'Выберите формат';
        },
        canAddVehicle() {
            if (this.isNumberByFact && this.isMarkByFact && this.selectedUnloadingPlaces.length > 0) {
                return true;
            }

            if (!this.isNumberByFact) {
                if (!this.selectedFormat || !this.numberParts.length) {
                    return false;
                }

                // Проверяем каждую клетку формата
                for (let i = 0; i < this.selectedFormat.cells.length; i++) {
                    const cell = this.selectedFormat.cells[i];
                    const part = this.numberParts[i];
                    
                    if (!part || part.length < cell.min_length || part.length > cell.max_length) {
                        return false;
                    }
                }
            }
            
            if (!this.isMarkByFact && !this.selectedMark) {
                return false;
            }
            
            if (this.selectedUnloadingPlaces.length === 0) {
                return false;
            }
            
            return true;
        },
        attachedPlacesIds() {
            return this.attachedUnloadingPlaces.map(place => place.id);
        },
    },
    methods: {
        async loadLicensePlateFormats() {
  try {
    const token = localStorage.getItem("token");
    const response = await fetch("http://localhost:8080/license-plate-formats", {
      method: "GET",
      headers: {
        "Authorization": `Bearer ${token}`
      }
    });

    if (response.ok) {
      this.availableFormats = await response.json();
      // Выбираем формат по умолчанию или первый формат
      const defaultFormat = this.availableFormats.find(f => f.format.is_default);
      this.selectedFormat = defaultFormat || this.availableFormats[0];
      this.initializeNumberParts();
    } else {
      console.error("Ошибка при загрузке форматов номеров");
    }
  } catch (error) {
    console.error("Ошибка при загрузке форматов номеров:", error);
  }
},

        async loadUnloadingPlaces() {
            this.loadingUnloadingPlaces = true;
            this.allUnloadingPlaces = [];
            this.attachedUnloadingPlaces = [];
            this.selectedUnloadingPlaces = [];
            
            try {
                const token = localStorage.getItem("token");
                if (!token) {
                    console.error("Токен не найден");
                    return;
                }

                // Загружаем все доступные места разгрузки
                const allPlacesResponse = await fetch("http://localhost:8080/unload-places", {
                    method: "GET",
                    headers: {
                        "Authorization": `Bearer ${token}`
                    }
                });

                if (allPlacesResponse.ok) {
                    this.allUnloadingPlaces = await allPlacesResponse.json();
                }

                // Загружаем привязанные места разгрузки организации
                if (this.userOrganizationId) {
                    const orgPlacesResponse = await fetch(`http://localhost:8080/organizations/${this.userOrganizationId}/unload-places`, {
                        method: "GET",
                        headers: {
                            "Authorization": `Bearer ${token}`
                        }
                    });

                    if (orgPlacesResponse.ok) {
                        this.attachedUnloadingPlaces = await orgPlacesResponse.json();
                        // Автоматически выбираем привязанные места
                        this.selectedUnloadingPlaces = this.attachedUnloadingPlaces.map(place => place.id);
                    }
                }

                // Если нет привязанных мест организации, пробуем компанию
                if (this.attachedUnloadingPlaces.length === 0 && this.userCompanyId) {
                    const companyPlacesResponse = await fetch(`http://localhost:8080/companies/${this.userCompanyId}/unload-places`, {
                        method: "GET",
                        headers: {
                            "Authorization": `Bearer ${token}`
                        }
                    });

                    if (companyPlacesResponse.ok) {
                        this.attachedUnloadingPlaces = await companyPlacesResponse.json();
                        // Автоматически выбираем привязанные места
                        this.selectedUnloadingPlaces = this.attachedUnloadingPlaces.map(place => place.id);
                    }
                }

                this.validateUnloadingPlaces();

            } catch (error) {
                console.error("Ошибка при загрузке мест разгрузки:", error);
                this.allUnloadingPlaces = [];
                this.attachedUnloadingPlaces = [];
            } finally {
                this.loadingUnloadingPlaces = false;
            }
        },

        initializeNumberParts() {
            if (this.selectedFormat) {
                this.numberParts = new Array(this.selectedFormat.cells.length).fill('');
            } else {
                this.numberParts = [];
            }
        },

        getPlaceholder(cell) {
            if (cell.cell_type === 'numbers') {
                return '0'.repeat(cell.max_length);
            } else {
                return 'A'.repeat(cell.max_length);
            }
        },

        getInputWidth(cell) {
            // Рассчитываем ширину на основе максимальной длины
            const baseWidth = 25; // Базовая ширина для одного символа
            const minWidth = 50; // Минимальная ширина
            const width = Math.max(minWidth, cell.max_length * baseWidth);
            return `${width}px`;
        },

        validatePart(index, event, cell) {
            let value = event.target.value.toUpperCase();
            
            if (cell.cell_type === 'numbers') {
                // Только цифры
                value = value.replace(/\D/g, '');
            } else if (cell.cell_type === 'letters') {
                // Только буквы в зависимости от алфавита
                if (cell.alphabet_type === 'cyrillic') {
                    value = this.filterCyrillicLetters(value, cell.allowed_letters);
                } else if (cell.alphabet_type === 'latin') {
                    value = this.filterLatinLetters(value, cell.allowed_letters);
                } else if (cell.alphabet_type === 'both') {
                    value = this.filterBothLetters(value, cell.allowed_letters);
                }
            } else if (cell.cell_type === 'mixed') {
                // Буквы и цифры
                if (cell.alphabet_type === 'cyrillic') {
                    value = this.filterMixedCyrillic(value, cell.allowed_letters);
                } else if (cell.alphabet_type === 'latin') {
                    value = this.filterMixedLatin(value, cell.allowed_letters);
                } else if (cell.alphabet_type === 'both') {
                    value = this.filterMixedBoth(value, cell.allowed_letters);
                }
            }
            
            // Ограничиваем максимальную длину
            if (value.length > cell.max_length) {
                value = value.slice(0, cell.max_length);
            }
            
            this.numberParts[index] = value;
            event.target.value = value;
        },

        formatPart(index, cell) {
            // Дополняем нулями только если есть введенное значение
            if (cell.cell_type === 'numbers' && cell.padding_side && this.numberParts[index]) {
                let value = this.numberParts[index];
                const targetLength = cell.max_length;
                
                if (value.length < targetLength) {
                    const paddingChar = cell.padding_char || '0';
                    if (cell.padding_side === 'left') {
                        value = value.padStart(targetLength, paddingChar);
                    } else {
                        value = value.padEnd(targetLength, paddingChar);
                    }
                    this.numberParts[index] = value;
                }
            }
        },

        filterCyrillicLetters(value, allowedLetters) {
            if (allowedLetters) {
                const allowedChars = allowedLetters.split('');
                return value.split('').filter(char => allowedChars.includes(char)).join('');
            } else {
                return value.replace(/[^АВЕКМНОРСТУХ]/g, '');
            }
        },

        filterLatinLetters(value, allowedLetters) {
            if (allowedLetters) {
                const allowedChars = allowedLetters.split('');
                return value.split('').filter(char => allowedChars.includes(char)).join('');
            } else {
                return value.replace(/[^A-Z]/g, '');
            }
        },

        filterBothLetters(value, allowedLetters) {
            if (allowedLetters) {
                const allowedChars = allowedLetters.split('');
                return value.split('').filter(char => allowedChars.includes(char)).join('');
            } else {
                // Разрешаем и кириллицу и латиницу
                return value.replace(/[^A-ZА-Я]/g, '');
            }
        },

        filterMixedCyrillic(value, allowedLetters) {
            // Цифры + кириллица
            const numericPart = value.replace(/\D/g, '');
            const letterPart = this.filterCyrillicLetters(value.replace(/[0-9]/g, ''), allowedLetters);
            return numericPart + letterPart;
        },

        filterMixedLatin(value, allowedLetters) {
            // Цифры + латиница
            const numericPart = value.replace(/\D/g, '');
            const letterPart = this.filterLatinLetters(value.replace(/[0-9]/g, ''), allowedLetters);
            return numericPart + letterPart;
        },

        filterMixedBoth(value, allowedLetters) {
            // Цифры + кириллица + латиница
            const numericPart = value.replace(/\D/g, '');
            const letterPart = this.filterBothLetters(value.replace(/[0-9]/g, ''), allowedLetters);
            return numericPart + letterPart;
        },

        handleNumberByFactChange() {
            if (this.isNumberByFact) {
                this.numberParts = [];
            } else {
                this.initializeNumberParts();
            }
        },
        
        handleMarkByFactChange() {
            if (this.isMarkByFact) {
                this.selectedMark = '';
            }
        },
        
        toggleUnloadingPlace(placeId) {
            const index = this.selectedUnloadingPlaces.indexOf(placeId);
            if (index > -1) {
                this.selectedUnloadingPlaces.splice(index, 1);
            } else {
                this.selectedUnloadingPlaces.push(placeId);
            }
            this.validateUnloadingPlaces();
        },
        
        validateUnloadingPlaces() {
            this.errors.unloadingPlaces = this.selectedUnloadingPlaces.length === 0 ? 'Выберите хотя бы одно место разгрузки' : '';
        },
        
        formatUnloadingPlaces() {
            if (this.selectedUnloadingPlaces.length === 0) return '';
            
            const placeNames = this.selectedUnloadingPlaces.map(placeId => {
                const place = this.allUnloadingPlaces.find(p => p.id === placeId);
                return place ? place.name : '';
            }).filter(name => name);
            
            return placeNames.join(', ');
        },
        
        addVehicle() {
            if (!this.canAddVehicle) {
                alert('Заполните все обязательные поля правильно');
                return;
            }
            
            let plateNumber = '';
            if (this.isNumberByFact) {
                plateNumber = 'По факту';
            } else {
                plateNumber = this.numberParts.join(' ');
            }
            
            const mark = this.isMarkByFact ? 'По факту' : this.selectedMark;
            const unloadingPlace = this.formatUnloadingPlaces();

            const newVehicle = {
                plateNumber: plateNumber,
                mark: mark,
                unloadingPlace: unloadingPlace,
                unloadPlaces: [...this.selectedUnloadingPlaces],
                formatId: this.selectedFormat ? this.selectedFormat.format.id : null
            };
            
            this.$emit('vehicle-added', newVehicle);
            
            // Очищаем только номер и марку, места разгрузки остаются выбранными
            this.clearVehicleFormPartial();
        },
        
        clearVehicleFormPartial() {
            // Очищаем только номер и марку, места разгрузки остаются выбранными
            this.initializeNumberParts();
            this.selectedMark = '';
            this.isNumberByFact = false;
            this.isMarkByFact = false;
        },
        
        clearVehicleForm() {
            this.initializeNumberParts();
            this.selectedMark = '';
            this.selectedUnloadingPlaces = [];
            this.isNumberByFact = false;
            this.isMarkByFact = false;
            this.errors.unloadingPlaces = '';
        },
        
        // Mark dropdown methods
        toggleMarkDropdown() {
            this.isMarkDropdownOpen = !this.isMarkDropdownOpen;
            if (this.isMarkDropdownOpen) {
                this.filterMarks();
            }
        },
        
        filterMarks() {
            if (!this.markSearch) {
                this.filteredMarks = this.marks;
            } else {
                const searchTerm = this.markSearch.toLowerCase();
                this.filteredMarks = this.marks.filter(mark => 
                    mark.toLowerCase().includes(searchTerm)
                );
            }
        },
        
        selectMark(mark) {
            this.selectedMark = mark;
            this.isMarkDropdownOpen = false;
            this.markSearch = '';
        },
        
        // Dropdown methods
        toggleDropdown() {
            this.isDropdownOpen = !this.isDropdownOpen;
        },
        
        selectFormat(format) {
            this.selectedFormat = format;
            this.initializeNumberParts();
            this.isDropdownOpen = false;
        }
    },
    async mounted() {
        // Загружаем форматы номеров и места разгрузки
        await Promise.all([
            this.loadLicensePlateFormats(),
            this.loadUnloadingPlaces()
        ]);
        
        this.filteredMarks = this.marks;

        document.addEventListener('click', (e) => {
            if (!e.target.closest('.format__dropdown')) {
                this.isDropdownOpen = false;
            }
            
            if (!e.target.closest('.mark__dropdown')) {
                this.isMarkDropdownOpen = false;
            }
        });
    }
}
</script>

<style scoped>
    .input__label {
        font-size: 13px;
        color: #a2a2a2;
    }

    .required {
        color: #ff4444;
    }

.data__completion {
    padding: 15px;
    width: 450px;
    border-right: 1px solid #e6e6e6;
}

.completion__format {
    display: flex;
    flex-direction: column;
    gap: 5px;
    position: relative;
    padding-bottom: 10px;
}

.format__header {
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.format__label {
    font-size: 13px;
    color: #a2a2a2;
}

.add-button {
    background: #4F5BDF;
    color: white;
    border: none;
    border-radius: 15px;
    padding: 8px 15px;
    font-size: 12px;
    cursor: pointer;
    transition: background-color 0.2s;
    margin-top: 0;
}

.add-button:hover:not(:disabled) {
    background: #3a45c0;
}

.add-button:disabled {
    background: #a2a2a2;
    cursor: not-allowed;
    opacity: 0.6;
}

.format__dropdown {
    position: relative;
}

.dropdown__button {
    width: 200px;
    height: 30px;
    border: 1px solid #e6e6e6;
    background-color: #FFF;
    border-radius: 50px;
    outline: none;
    cursor: pointer;
    padding: 0 15px;
    transition: border-color 0.2s;
}

.dropdown__button:hover {
    border-color: #4F5BDF;
}

.button__content {
    display: flex;
    align-items: center;
    width: 100%;
    height: 100%;
    justify-content: space-between;
}

.completion__header {
    padding-bottom: 15px;
}

.button__text {
    font-size: 14px;
    color: #000;
    font-weight: 500;
}

.button__arrow {
    width: 10px;
    height: 10px;
    transition: transform 0.2s;
    transform: rotate(90deg);
}

.button__arrow--open {
    transform: rotate(-90deg);
}

.dropdown__menu {
    position: absolute;
    top: 100%;
    left: 0;
    width: 200px;
    background: #FFF;
    border: 1px solid #e6e6e6;
    border-radius: 10px;
    margin-top: 5px;
    box-shadow: 0 3px 10px rgba(0,0,0,0.1);
    z-index: 1000;
    max-height: 300px;
    overflow-y: auto;
}

.dropdown__item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 15px;
    cursor: pointer;
    transition: background-color 0.2s;
}

.dropdown__item:hover {
    background-color: #f5f5f5;
}

.dropdown__item:first-child {
    border-radius: 10px 10px 0 0;
}

.dropdown__item:last-child {
    border-radius: 0 0 10px 10px;
}

.item__text {
    font-size: 13px;
    color: #333;
}

.completion__fields {
    display: flex;
    gap: 20px;
    align-items: flex-start;
    margin-bottom: 15px;
}

.completion__number,
.completion__mark {
    flex: 1;
}

.completion__number-header,
.completion__mark-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-bottom: 5px;
}

.number-fact,
.mark-fact {
    display: flex;
    align-items: center;
    gap: 5px;
}

.fact-checkbox {
    width: 12px;
    height: 12px;
    cursor: pointer;
}

.fact-text {
    font-size: 13px;
}

.number__field {
    max-width: 202px;
    min-width: 202px;
    height: 40px;
    display: flex;
    border: 1px solid #e6e6e6;
    border-radius: 15px;
    overflow: hidden;
    background: #FFF;
}

.number__field--fact {
    display: block;
}

.number__input {
    border: none;
    height: 100%;
    outline: none;
    text-align: center;
    font-size: 14px;
    background: transparent;
    flex: 1;
    min-width: 0;
}

.number__input--fact {
    width: 100%;
    text-align: left;
    padding: 0 15px;
    color: #a2a2a2;
}

.number__input:not(:last-child) {
    border-right: 1px solid #e6e6e6;
}

.number__input:first-child {
    border-radius: 15px 0 0 15px;
}

.number__input:last-child {
    border-radius: 0 15px 15px 0;
}

.number__input::placeholder {
    color: #a2a2a2;
    font-size: 12px;
}

.number__input:focus {
    background-color: #f8f8f8;
}

/* Mark dropdown styles */
.mark__field {
    width: 100%;
    height: 40px;
    position: relative;
}

.mark__field--fact {
    border: 1px solid #e6e6e6;
    border-radius: 15px;
    overflow: hidden;
}

.mark__dropdown {
    width: 100%;
    height: 100%;
}

.mark__dropdown-button {
    width: 100%;
    height: 100%;
    border: 1px solid #e6e6e6;
    background-color: #FFF;
    border-radius: 15px;
    outline: none;
    cursor: pointer;
    padding: 0 15px;
    transition: border-color 0.2s;
}

.mark__dropdown-button:hover {
    border-color: #4F5BDF;
}

.mark__button-content {
    display: flex;
    align-items: center;
    width: 100%;
    height: 100%;
    justify-content: space-between;
}

.mark__button-text {
    font-size: 14px;
    color: #000;
}

.mark__button-arrow {
    width: 10px;
    height: 10px;
    transition: transform 0.2s;
    transform: rotate(90deg);
}

.mark__button-arrow--open {
    transform: rotate(-90deg);
}

.mark__dropdown-menu {
    position: absolute;
    top: 100%;
    left: 0;
    width: 100%;
    background: #FFF;
    border: 1px solid #e6e6e6;
    border-radius: 10px;
    margin-top: 5px;
    box-shadow: 0 3px 10px rgba(0,0,0,0.1);
    z-index: 1000;
    max-height: 200px;
    overflow: hidden;
}

.mark__search {
    padding: 10px;
    border-bottom: 1px solid #e6e6e6;
}

.mark__search-input {
    width: 100%;
    border: 1px solid #e6e6e6;
    border-radius: 5px;
    padding: 5px 10px;
    outline: none;
    font-size: 14px;
}

.mark__dropdown-list {
    max-height: 150px;
    overflow-y: auto;
}

.mark__dropdown-item {
    padding: 8px 15px;
    cursor: pointer;
    transition: background-color 0.2s;
    border-bottom: 1px solid #f5f5f5;
}

.mark__dropdown-item:hover {
    background-color: #f5f5f5;
}

.mark__dropdown-item:last-child {
    border-bottom: none;
}

.mark__item-text {
    font-size: 14px;
    color: #333;
}

.mark__input {
    width: 100%;
    height: 100%;
    border: none;
    outline: none;
    background: transparent;
    padding: 0 15px;
    font-size: 14px;
    color: #a2a2a2;
}

.mark__input--fact::placeholder {
    color: #a2a2a2;
    font-size: 12px;
}

/* Unloading places styles */
.completion__unloading {
    margin-top: 15px;
}

.unloading-info {
    margin-bottom: 10px;
}

.unloading-source {
    font-size: 12px;
    color: #4F5BDF;
    font-weight: 500;
    background: #f0f2ff;
    padding: 5px 10px;
    border-radius: 8px;
    display: inline-block;
}

.attached-count {
    font-size: 11px;
    color: #666;
    font-weight: normal;
}

.attached-badge {
    position: absolute;
    top: 2px;
    right: 2px;
    background: #4F5BDF;
    color: white;
    border-radius: 50%;
    width: 16px;
    height: 16px;
    font-size: 10px;
    display: flex;
    align-items: center;
    justify-content: center;
}

.unloading__grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 10px;
    row-gap: 5px;
    max-width: 425px;
    margin-top: 5px;
}

.unloading__item {
    height: 35px;
    background: #F2F2F2;
    color: #a2a2a2;
    border-radius: 50px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 12px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s;
    padding: 0 10px;
    text-align: center;
    border: 1px solid transparent;
    position: relative;
}

.unloading__item:hover:not(.unloading__item--active) {
    background: #e8e8e8;
}

.unloading__item--active {
    background: #4F5BDF;
    color: #fff;
    border-color: #4F5BDF;
}

.error-message {
    font-size: 11px;
    color: #ff4444;
    margin-top: 5px;
}

.loading-message {
    font-size: 12px;
    color: #a2a2a2;
    text-align: center;
    padding: 20px;
}

.no-places-message {
    font-size: 12px;
    color: #ff6b6b;
    text-align: center;
    padding: 20px;
    background: #fff5f5;
    border-radius: 8px;
    margin-top: 10px;
}
</style>