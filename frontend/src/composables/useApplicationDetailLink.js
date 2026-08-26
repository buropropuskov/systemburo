import { ref } from 'vue';

/**
 * Открытие карточки заявки из реестра сотрудников или машин.
 *
 * Обе страницы показывают в строке активную заявку и дают её открыть, и обе держали
 * для этого одинаковую пару полей с одинаковыми методами.
 *
 * Деталь сама догружает вложения, читателей и историю по id, поэтому передаём только
 * идентификатор, а не строку списка: полей реестра ей всё равно не хватило бы.
 *
 * @returns {{selectedApplication: import('vue').Ref, showApplicationDetail: import('vue').Ref,
 *            handleOpenApplication: (id: number) => void, closeApplicationDetail: () => void}}
 */
export function useApplicationDetailLink() {
  const selectedApplication = ref(null);
  const showApplicationDetail = ref(false);

  function handleOpenApplication(applicationId) {
    if (!applicationId) return;
    selectedApplication.value = { id: applicationId };
    showApplicationDetail.value = true;
  }

  function closeApplicationDetail() {
    showApplicationDetail.value = false;
    selectedApplication.value = null;
  }

  return { selectedApplication, showApplicationDetail, handleOpenApplication, closeApplicationDetail };
}
