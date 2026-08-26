import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import EmployeeForm from '../EmployeeForm.vue';

// H-6 (#529): EmployeeForm уважает field-config выбранного шаблона.
// Ключи реестра: last_name/first_name/middle_name/passport/position/citizenship/patent/work_permission/target_tables.
// Дефолт хелперов = видимо+обязательно -> пустой конфиг не меняет поведение.
// patent: при fieldRequired=true форсируется обязательным; при дефолте - по isPatentRequired (гражданство).

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) })
}));
vi.mock('@/api/blacklist', () => ({
  checkPersonBlacklist: vi.fn().mockResolvedValue(null)
}));
vi.mock('@/stores/auth', () => ({
  useAuthStore: vi.fn().mockReturnValue({ token: 'test-token' })
}));
vi.mock('@/components/CreateApplication/ExistingEmployeesModal.vue', () => ({
  default: { name: 'ExistingEmployeesModal', template: '<div />' }
}));

beforeEach(() => {
  setActivePinia(createPinia());
});

const hasField = (w, text) => w.text().includes(text);

describe('EmployeeForm - потребление field-config (#529 H-6)', () => {
  it('без конфига: все поля видимы, звёздочки на обязательных', () => {
    const w = mount(EmployeeForm);
    expect(hasField(w, 'Фамилия')).toBe(true);
    expect(hasField(w, 'Имя')).toBe(true);
    expect(hasField(w, 'Отчество')).toBe(true);
    expect(hasField(w, 'Должность')).toBe(true);
    expect(hasField(w, 'Гражданство')).toBe(true);
    expect(hasField(w, 'Паспортные данные')).toBe(true);
    expect(hasField(w, 'Места прохода')).toBe(true);
  });

  it('fieldVisible: нет строки конфига -> true; visible:false -> false', () => {
    const w = mount(EmployeeForm, {
      props: { fieldConfig: { foo: { visible: false, required: true } } }
    });
    expect(w.vm.fieldVisible('missing')).toBe(true);
    expect(w.vm.fieldVisible('foo')).toBe(false);
  });

  it('fieldRequired: нет строки конфига -> true; required:false -> false', () => {
    const w = mount(EmployeeForm, {
      props: { fieldConfig: { foo: { visible: true, required: false } } }
    });
    expect(w.vm.fieldRequired('missing')).toBe(true);
    expect(w.vm.fieldRequired('foo')).toBe(false);
  });

  it('скрывает поле Фамилия при last_name.visible=false', () => {
    const w = mount(EmployeeForm, {
      props: { fieldConfig: { last_name: { visible: false, required: true } } }
    });
    expect(hasField(w, 'Фамилия')).toBe(false);
    expect(hasField(w, 'Имя')).toBe(true);
  });

  it('скрывает поле Имя при first_name.visible=false', () => {
    const w = mount(EmployeeForm, {
      props: { fieldConfig: { first_name: { visible: false, required: true } } }
    });
    expect(hasField(w, 'Имя')).toBe(false);
    expect(hasField(w, 'Фамилия')).toBe(true);
  });

  it('скрывает поле Отчество при middle_name.visible=false', () => {
    const w = mount(EmployeeForm, {
      props: { fieldConfig: { middle_name: { visible: false, required: true } } }
    });
    expect(hasField(w, 'Отчество')).toBe(false);
  });

  it('скрывает поле Должность при position.visible=false', () => {
    const w = mount(EmployeeForm, {
      props: { fieldConfig: { position: { visible: false, required: true } } }
    });
    expect(hasField(w, 'Должность')).toBe(false);
  });

  it('скрывает поле Гражданство при citizenship.visible=false', () => {
    const w = mount(EmployeeForm, {
      props: { fieldConfig: { citizenship: { visible: false, required: true } } }
    });
    expect(hasField(w, 'Гражданство')).toBe(false);
  });

  it('скрывает поле Паспортные данные при passport.visible=false', () => {
    const w = mount(EmployeeForm, {
      props: { fieldConfig: { passport: { visible: false, required: true } } }
    });
    expect(hasField(w, 'Паспортные данные')).toBe(false);
  });

  it('скрывает Места прохода при target_tables.visible=false', () => {
    const w = mount(EmployeeForm, {
      props: { fieldConfig: { target_tables: { visible: false, required: true } } }
    });
    expect(hasField(w, 'Места прохода')).toBe(false);
  });

  it('patent: при дефолтном конфиге effectivePatentRequired следует isPatentRequired (по гражданству)', () => {
    const w = mount(EmployeeForm);
    // Без выбранного гражданства isPatentRequired=false -> effectivePatentRequired=false
    expect(w.vm.isPatentRequired).toBe(false);
    expect(w.vm.effectivePatentRequired).toBe(false);
  });

  it('patent: при fieldRequired=true форсирует обязательность независимо от гражданства', () => {
    const w = mount(EmployeeForm, {
      props: { fieldConfig: { patent: { visible: true, required: true } } }
    });
    // isPatentRequired=false (нет гражданства с patent_required), но config.required=true
    expect(w.vm.isPatentRequired).toBe(false);
    expect(w.vm.effectivePatentRequired).toBe(true);
  });

  it('patent: при required=false и нет patent_required гражданства - effectivePatentRequired=false', () => {
    const w = mount(EmployeeForm, {
      props: { fieldConfig: { patent: { visible: true, required: false } } }
    });
    expect(w.vm.effectivePatentRequired).toBe(false);
  });

  it('скрывает блок "Иное разрешение на работы" при work_permission.visible=false', () => {
    const w = mount(EmployeeForm, {
      props: { fieldConfig: { work_permission: { visible: false, required: false } } }
    });
    expect(w.find('.completion__permission').exists()).toBe(false);
  });

  it('видимое необязательное поле не блокирует canAddEmployee (red H-6)', () => {
    // Все поля видимы, но required=false -> единственное правило это проверка ЧС
    // (blacklistInfo=null, мок). До фикса required-проверки гейтились лишь fieldVisible
    // и блокировали submit при пустом optional-поле.
    const optional = { visible: true, required: false };
    const w = mount(EmployeeForm, {
      props: { fieldConfig: {
        last_name: optional, first_name: optional, position: optional,
        citizenship: optional, passport: optional, patent: optional, target_tables: optional,
        pd_consent: optional,
      } }
    });
    expect(w.vm.canAddEmployee).toBe(true);
  });
});
