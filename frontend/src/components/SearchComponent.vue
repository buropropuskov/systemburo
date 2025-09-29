<template>
    <div class="search">
        <input 
            type="text" 
            :placeholder="title" 
            class="search__input"
            :value="modelValue"
            @input="$emit('update:modelValue', $event.target.value)"
        />
        <img src="@/assets/icons/search.png" class="search__icon" />
    </div>
</template>

<script>
export default {
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
    emits: ['update:modelValue']
}
</script>

<style scoped>
.search {
  width: 220px;
  height: 35px;
  border: 1px solid #e6e6e6;
  outline: none;
  border-radius: 10px;
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