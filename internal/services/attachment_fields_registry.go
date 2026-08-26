package services

import "systemburo/internal/models"

// Реестр базовых полей вложений (feedback-0608-H / #529).
//
// Source of truth - КОД, а не БД: базовые поля жёстко завязаны на компоненты-формы
// подачи (DateRangeSection / EmployeeForm / VehicleForm / ItemsForm), каталог в БД
// разъехался бы с кодом. БД (attachment_field_config) хранит только оверрайды.
//
// Дефолты visible/required отражают ТЕКУЩЕЕ поведение форм (сверено со срезом H-1):
//   - common даты/время ЗАЛОЧЕНЫ (Locked): всегда видимы и обязательны, не
//     настраиваются - заявка без периода действия бессмысленна (решение владельца 10.06);
//   - роof/parking/notify - булевые чекбоксы, required для них не имеет смысла
//     (Requirable=false): значение есть всегда;
//   - people/cars/items - required совпадает со звёздочками и useFormValidation форм.

// PDConsentFieldKey -- ключ поля «согласие субъекта на обработку персональных данных».
// Заведён константой, потому что на него смотрят три разных места: форма подачи через
// merged-конфиг, проверка при подаче и дополнении заявки, разбор бланка (там поле
// исключено из построчной проверки - отметка ставится на весь список на сайте).
const PDConsentFieldKey = "pd_consent"

// Группы полей. common применяется ко всем типам вложения, остальные - к своему.
const (
	FieldGroupCommon = "common"
	FieldGroupPeople = "people"
	FieldGroupCars   = "cars"
	FieldGroupItems  = "items"
)

// FieldDef - описание одного базового поля вложения в реестре.
type FieldDef struct {
	Key             string // стабильный ключ, напр. "passport", "entry_date_from"
	Label           string // подпись для админ-модалки
	Group           string // FieldGroupCommon | People | Cars | Items
	DefaultVisible  bool
	DefaultRequired bool
	// Requirable=false для булевых чекбоксов (роof/parking/notify): значение всегда
	// есть, тумблер "обязательно" бессмыслен. Оверрайд required для них игнорируется.
	Requirable bool
	// Locked=true: поле нельзя настроить (всегда visible+required). Не показывается
	// в админ-модалке настройки, любые оверрайды из БД игнорируются. Для common
	// даты/времени - период действия обязателен у каждой заявки.
	Locked bool
}

// attachmentFieldRegistry - полный реестр. Порядок = порядок показа в UI.
var attachmentFieldRegistry = []FieldDef{
	// common - присутствует у всех типов вложения.
	// Дата/время залочены: период действия обязателен у каждой заявки, не настраивается.
	{Key: "entry_date_from", Label: "Дата въезда с", Group: FieldGroupCommon, DefaultVisible: true, DefaultRequired: true, Requirable: true, Locked: true},
	{Key: "entry_date_to", Label: "Дата въезда по", Group: FieldGroupCommon, DefaultVisible: true, DefaultRequired: true, Requirable: true, Locked: true},
	{Key: "entry_time_from", Label: "Время с", Group: FieldGroupCommon, DefaultVisible: true, DefaultRequired: true, Requirable: true, Locked: true},
	{Key: "entry_time_to", Label: "Время по", Group: FieldGroupCommon, DefaultVisible: true, DefaultRequired: true, Requirable: true, Locked: true},
	{Key: "roof_access", Label: "Доступ на крышу", Group: FieldGroupCommon, DefaultVisible: true, DefaultRequired: false, Requirable: false},
	{Key: "free_parking", Label: "Бесплатная парковка", Group: FieldGroupCommon, DefaultVisible: true, DefaultRequired: false, Requirable: false},

	// people
	{Key: "last_name", Label: "Фамилия", Group: FieldGroupPeople, DefaultVisible: true, DefaultRequired: true, Requirable: true},
	{Key: "first_name", Label: "Имя", Group: FieldGroupPeople, DefaultVisible: true, DefaultRequired: true, Requirable: true},
	{Key: "middle_name", Label: "Отчество", Group: FieldGroupPeople, DefaultVisible: true, DefaultRequired: false, Requirable: true},
	{Key: "passport", Label: "Паспортные данные", Group: FieldGroupPeople, DefaultVisible: true, DefaultRequired: true, Requirable: true},
	{Key: "position", Label: "Должность", Group: FieldGroupPeople, DefaultVisible: true, DefaultRequired: true, Requirable: true},
	{Key: "citizenship", Label: "Гражданство", Group: FieldGroupPeople, DefaultVisible: true, DefaultRequired: true, Requirable: true},
	{Key: "patent", Label: "Номер патента", Group: FieldGroupPeople, DefaultVisible: true, DefaultRequired: false, Requirable: true},
	{Key: "work_permission", Label: "Иное разрешение на работы", Group: FieldGroupPeople, DefaultVisible: true, DefaultRequired: false, Requirable: true},
	{Key: "target_tables", Label: "Места прохода", Group: FieldGroupPeople, DefaultVisible: true, DefaultRequired: true, Requirable: true},
	// Согласие субъекта на обработку его персональных данных (152-ФЗ). У сотрудника
	// вводят паспорт и патент, то есть данные третьего лица: отметка обязательна по
	// умолчанию, а не по настройке админа. Снять обязательность он всё же может -
	// у части заказчиков согласие собрано бумагой на весь подряд.
	{Key: PDConsentFieldKey, Label: "Согласие на обработку персональных данных", Group: FieldGroupPeople, DefaultVisible: true, DefaultRequired: true, Requirable: true},

	// cars
	{Key: "number", Label: "Номер ТС", Group: FieldGroupCars, DefaultVisible: true, DefaultRequired: true, Requirable: true},
	{Key: "mark", Label: "Марка ТС", Group: FieldGroupCars, DefaultVisible: true, DefaultRequired: true, Requirable: true},
	{Key: "unloading_places", Label: "Места разгрузки", Group: FieldGroupCars, DefaultVisible: true, DefaultRequired: true, Requirable: true},
	// «Проезд» (#1036): таблицы, в которых видна машина. DefaultRequired пока false —
	// обязательно по умолчанию (как «Места прохода» у сотрудников): машину нельзя
	// подать без выбора таблиц «Проезд». Админ может снять required в шаблоне полей.
	{Key: "passage_tables", Label: "Проезд", Group: FieldGroupCars, DefaultVisible: true, DefaultRequired: true, Requirable: true},
	// У машины вводят номер и марку, ФИО и документы владельца - нет; запись висит на
	// организации, компании или учётной записи заявителя. Отдельного согласия субъекта
	// такой набор не требует, а когда за машиной стоит физлицо, он покрыт общим
	// согласием заявителя при подаче. Поэтому поле выключено: включит администратор,
	// если юрист заказчика потребует - без правки кода.
	{Key: PDConsentFieldKey, Label: "Согласие на обработку персональных данных", Group: FieldGroupCars, DefaultVisible: false, DefaultRequired: false, Requirable: true},

	// items
	{Key: "item_name", Label: "Наименование ТМЦ", Group: FieldGroupItems, DefaultVisible: true, DefaultRequired: true, Requirable: true},
	{Key: "quantity", Label: "Количество", Group: FieldGroupItems, DefaultVisible: true, DefaultRequired: true, Requirable: true},
}

// groupForAttachmentType возвращает type-specific группу полей для типа вложения.
func groupForAttachmentType(attachmentType string) string {
	switch attachmentType {
	case "people":
		return FieldGroupPeople
	case "cars":
		return FieldGroupCars
	case "items":
		return FieldGroupItems
	default:
		return ""
	}
}

// FieldRegistryFor возвращает базовые поля, применимые к типу вложения:
// common + type-specific группа. Порядок сохраняется (common впереди).
// Неизвестный тип -> только common.
func FieldRegistryFor(attachmentType string) []FieldDef {
	typeGroup := groupForAttachmentType(attachmentType)
	out := make([]FieldDef, 0, len(attachmentFieldRegistry))
	for _, f := range attachmentFieldRegistry {
		if f.Group == FieldGroupCommon || f.Group == typeGroup {
			out = append(out, f)
		}
	}
	return out
}

// fieldDefByKey строит индекс ключ->FieldDef для применимых полей типа.
func fieldDefByKey(attachmentType string) map[string]FieldDef {
	defs := FieldRegistryFor(attachmentType)
	idx := make(map[string]FieldDef, len(defs))
	for _, d := range defs {
		idx[d.Key] = d
	}
	return idx
}

// MergeFieldConfig мержит реестр типа вложения с оверрайдами из БД.
// Где оверрайда нет - берётся дефолт реестра. Для не-Requirable полей
// (булевые чекбоксы) required всегда false независимо от оверрайда.
// Залоченные поля (Locked) всегда visible+required, оверрайды для них игнорируются.
func MergeFieldConfig(attachmentType string, overrides []models.AttachmentFieldConfig) []models.MergedField {
	byKey := make(map[string]models.AttachmentFieldConfig, len(overrides))
	for _, o := range overrides {
		byKey[o.FieldKey] = o
	}

	defs := FieldRegistryFor(attachmentType)
	out := make([]models.MergedField, 0, len(defs))
	for _, d := range defs {
		visible, required := d.DefaultVisible, d.DefaultRequired
		switch {
		case d.Locked:
			// Залоченные (дата/время) - всегда видимы и обязательны, оверрайд не применяется.
			visible, required = true, true
		default:
			if o, ok := byKey[d.Key]; ok {
				visible = o.Visible
				required = o.Required
			}
			if !d.Requirable {
				required = false
			}
		}
		out = append(out, models.MergedField{
			Key:        d.Key,
			Label:      d.Label,
			Group:      d.Group,
			Visible:    visible,
			Required:   required,
			Requirable: d.Requirable,
			Locked:     d.Locked,
		})
	}
	return out
}
