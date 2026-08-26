<template>
  <div class="search">
    <input 
      type="text" 
      :placeholder="title" 
      class="search__input"
      :value="modelValue"
      @input="handleInput"
    >
    <AppIcon
      name="search"
      class="search__icon"
    />
  </div>
</template>

<script>
import { buildSearchVariants } from '@/utils/searchVariants';
import AppIcon from '@/components/icons/AppIcon.vue';

export default {
    name: 'SearchComponent',
    components: { AppIcon },
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
            this.$emit('search', buildSearchVariants(value));
        },
    }
}
</script>

<style scoped>
.search {
  width: 220px;
  height: 35px;
  border: 1px solid var(--border);
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
  color: var(--text);
  /* 1.7 при поле 24 даёт на 15px обводку 1.06px - бледнее прежнего растра. */
  stroke-width: 2.2;
}
</style>