import demoApplications from '@/assets/onboarding/demo-applications.png';
import demoCars from '@/assets/onboarding/demo-cars.png';
import demoEmployees from '@/assets/onboarding/demo-employees.png';
import demoCenterList from '@/assets/onboarding/demo-center-list.png';
import demoApprovers from '@/assets/onboarding/demo-approvers.png';
import demoVote from '@/assets/onboarding/demo-detail-actions.png';
import demoApplicationDetail from '@/assets/onboarding/demo-application-detail.png';
import demoFactTable from '@/assets/onboarding/demo-fact-table.png';

/**
 * Демо-скриншоты для шагов, где у нового пользователя элемент пустой (заявки,
 * авто, люди, форма). Ключ = `step.demo`. Картинки - статичные PNG с фейк-данными
 * на бренде проекта (в onboardingDemo, не из живого API).
 *
 * @type {Record<string, { src: string, width: number, height: number, alt: string, caption?: string }>}
 */
const DEMO = {
  factTable: {
    src: demoFactTable,
    width: 368, height: 345,
    alt: 'Учебный пример таблицы «Автомобили по факту»: въезд, выезд и данные пропуска',
    caption: 'Учебный пример. Это изображение: кнопки внутри него не меняют данные.',
  },
  applications: {
    src: demoApplications,
    width: 1420, height: 470,
    alt: 'Пример списка заявок со статусами',
    caption: 'Пример: так выглядят ваши заявки и их статусы',
  },
  cars: {
    src: demoCars,
    width: 1028, height: 300,
    alt: 'Пример списка автомобилей с номерами и статусами',
    caption: 'Пример: так выглядит список автомобилей',
  },
  employees: {
    src: demoEmployees,
    width: 1028, height: 300,
    alt: 'Пример списка сотрудников с должностями',
    caption: 'Пример: так выглядит список сотрудников',
  },
  centerList: {
    src: demoCenterList,
    width: 3088, height: 916,
    alt: 'Пример списка заявок в центре: группы по датам, статусы, метки ожидания',
    caption: 'Пример: так выглядит центр заявок, когда заявки есть',
  },
  approvers: {
    src: demoApprovers,
    width: 826, height: 600,
    alt: 'Пример блока согласования: статус заявки и список ответственных',
    caption: 'Пример: так выглядит блок согласования в карточке',
  },
  applicationDetail: {
    src: demoApplicationDetail,
    width: 1600, height: 483,
    alt: 'Пример карточки заявки: номер и дата сверху, состав слева, согласование справа',
    caption: 'Пример: так выглядит карточка вашей заявки',
  },
  vote: {
    src: demoVote,
    width: 694, height: 134,
    alt: 'Кнопки согласования и отказа в карточке заявки',
    caption: 'Пример: так выглядят кнопки решения по заявке',
  },
};

/**
 * @param {string} key
 * @returns {{ src: string, width: number, height: number, alt: string, caption?: string } | null}
 */
export function getDemo(key) {
  return DEMO[key] || null;
}
