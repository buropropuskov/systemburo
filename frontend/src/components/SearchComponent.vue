<template>
    <div class="search">
        <input 
            type="text" 
            :placeholder="title" 
            class="search__input"
            :value="modelValue"
            @input="handleInput"
        />
        <img src="@/assets/icons/search.png" class="search__icon" />
    </div>
</template>

<script>
export default {
    name: 'SearchComponent',
    props: {
        title: {
            type: String,
            required: true,
        },
        modelValue: {
            type: String,
            default: ''
        }
    },
    emits: ['update:modelValue', 'search'],
    methods: {
        handleInput(event) {
            const value = event.target.value;
            this.$emit('update:modelValue', value);
            
            // Получаем варианты поиска и отправляем событие
            const searchVariants = this.getSearchVariants(value.toLowerCase().trim());
            this.$emit('search', searchVariants);
        },
        
        // Функция для получения всех вариантов поиска (кириллица <-> латиница)
        getSearchVariants(searchTerm) {
            if (!searchTerm) return [''];
            
            const variants = new Set();
            
            // Оригинальный запрос
            variants.add(searchTerm);
            
            // Кириллица -> латиница (транслитерация)
            const cyrillicToLatinMap = {
                'а': 'a', 'б': 'b', 'в': 'v', 'г': 'g', 'д': 'd', 'е': 'e', 'ё': 'yo', 'ж': 'zh',
                'з': 'z', 'и': 'i', 'й': 'y', 'к': 'k', 'л': 'l', 'м': 'm', 'н': 'n', 'о': 'o',
                'п': 'p', 'р': 'r', 'с': 's', 'т': 't', 'у': 'u', 'ф': 'f', 'х': 'kh', 'ц': 'ts',
                'ч': 'ch', 'ш': 'sh', 'щ': 'shch', 'ъ': '', 'ы': 'y', 'ь': '', 'э': 'e', 'ю': 'yu',
                'я': 'ya'
            };
            
            let transliterated = '';
            for (let char of searchTerm) {
                transliterated += cyrillicToLatinMap[char] || char;
            }
            if (transliterated !== searchTerm) {
                variants.add(transliterated);
            }
            
            // Латиница -> кириллица (обратная транслитерация)
            const latinToCyrillicMap = {
                'a': 'а', 'b': 'б', 'c': 'ц', 'd': 'д', 'e': 'е', 'f': 'ф', 'g': 'г', 'h': 'х',
                'i': 'и', 'j': 'й', 'k': 'к', 'l': 'л', 'm': 'м', 'n': 'н', 'o': 'о', 'p': 'п',
                'q': 'к', 'r': 'р', 's': 'с', 't': 'т', 'u': 'у', 'v': 'в', 'w': 'в', 'x': 'кс',
                'y': 'ы', 'z': 'з'
            };
            
            let cyrillicized = '';
            for (let char of searchTerm) {
                cyrillicized += latinToCyrillicMap[char] || char;
            }
            if (cyrillicized !== searchTerm) {
                variants.add(cyrillicized);
            }
            
            // Добавляем варианты без пробелов
            const variantsArray = Array.from(variants);
            const variantsWithNoSpaces = new Set(variantsArray);
            
            variantsArray.forEach(variant => {
                if (variant.includes(' ')) {
                    variantsWithNoSpaces.add(variant.replace(/\s+/g, ''));
                }
            });
            
            return Array.from(variantsWithNoSpaces);
        }
    }
}
</script>

<style scoped>
.search {
  width: 220px;
  height: 35px;
  border: 1px solid #e6e6e6;
  outline: none;
  border-radius: 15px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 10px;
}

.search__input {
  background-color: transparent;
  outline: none;
  border: none;
  font-size: 14px;
  width: 100%;
}

.search__icon {
  width: 15px;
  height: 15px;
}
</style>