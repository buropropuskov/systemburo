import demoApplications from '@/assets/onboarding/demo-applications.png';
import demoCars from '@/assets/onboarding/demo-cars.png';
import demoEmployees from '@/assets/onboarding/demo-employees.png';
import demoCarForm from '@/assets/onboarding/demo-car-form.png';
import demoPeopleForm from '@/assets/onboarding/demo-people-form.png';

/**
 * Демо-скриншоты для шагов, где у нового пользователя элемент пустой (заявки,
 * авто, люди, форма). Ключ = `step.demo`. Картинки - статичные PNG с фейк-данными
 * на бренде проекта (в onboardingDemo, не из живого API).
 *
 * @type {Record<string, { src: string, alt: string, caption?: string }>}
 */
const DEMO = {
  applications: {
    src: demoApplications,
    alt: 'Пример списка заявок со статусами',
    caption: 'Пример: так выглядят ваши заявки и их статусы',
  },
  cars: {
    src: demoCars,
    alt: 'Пример списка автомобилей с номерами и статусами',
    caption: 'Пример: так выглядит список автомобилей',
  },
  employees: {
    src: demoEmployees,
    alt: 'Пример списка сотрудников с должностями',
    caption: 'Пример: так выглядит список сотрудников',
  },
  carForm: {
    src: demoCarForm,
    alt: 'Пример заполненной формы автомобиля: формат номера, номер, марка',
    caption: 'Пример заполнения: формат, номер и марка автомобиля',
  },
  peopleForm: {
    src: demoPeopleForm,
    alt: 'Пример заполненной формы сотрудника: ФИО, должность, документы',
    caption: 'Пример заполнения: гражданство, ФИО, должность и паспорт',
  },
};

/**
 * @param {string} key
 * @returns {{ src: string, alt: string, caption?: string } | null}
 */
export function getDemo(key) {
  return DEMO[key] || null;
}
