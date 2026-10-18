const DOCUMENT_FIELDS = [
    ['passport_series_number', 'passportSeriesNumber'],
    ['patent_number', 'patentNumber'],
    ['other_permission', 'otherPermission'],
];

// Серверный marker переносится отдельно от значений: null означает скрытое
// значение, а не предложение очистить документ при следующем сохранении.
export function employeeDocumentFields(employee, { fromForm = false, toForm = false } = {}) {
    const source = employee || {};
    const fields = {};
    for (const [apiKey, formKey] of DOCUMENT_FIELDS) {
        fields[toForm ? formKey : apiKey] = source.documents_hidden === true
            ? null
            : source[fromForm ? formKey : apiKey];
    }
    if (typeof source.documents_hidden === 'boolean') fields.documents_hidden = source.documents_hidden;
    if (toForm && (source.documents_hidden === true || source.sourceDocumentsHidden === true)) fields.sourceDocumentsHidden = true;
    return fields;
}

// Marker - метаданные ответа, не поле записи. Непрочитанные документы
// не отправляются ни пустыми строками, ни null, чтобы не очистить сохранённое.
export function employeeDocumentPayload(employee, options) {
    if (employee?.documents_hidden === true) return {};
    const fields = employeeDocumentFields(employee, options);
    delete fields.documents_hidden;
    return fields;
}

// Проверяем только новый сценарий переноса скрытой записи каталога. Дефолты
// совпадают с формой: паспорт обязателен, патент зависит от гражданства, если
// шаблон не усилил его обязательность. Остальные writers здесь не меняются.
export function employeeDocumentErrors(employees, config = {}) {
    const errors = [];
    const visible = (key) => config[key] ? config[key].visible === true : true;
    const required = (key) => visible(key) && (config[key] ? config[key].required === true : true);
    for (const employee of employees || []) {
        if (employee.documents_hidden !== true && employee.sourceDocumentsHidden !== true) continue;
        const documents = employeeDocumentPayload(employee, { fromForm: true });
        const present = (key) => typeof documents[key] === 'string' && documents[key].trim() !== '';
        const missing = [];
        if (required('passport') && !present('passport_series_number')) missing.push('паспортные данные');
        if (visible('patent') && (config.patent?.required === true || employee.citizenshipPatentRequired === true) && !present('patent_number') && !present('other_permission')) missing.push('патент или иное разрешение');
        if (config.work_permission && required('work_permission') && !present('other_permission')) missing.push('иное разрешение');
        if (missing.length) errors.push(`${[employee.lastName, employee.firstName].filter(Boolean).join(' ') || 'Сотрудник'}: введите ${missing.join(', ')} для новой заявки через правку строки. Справочник не изменится`);
    }
    return errors;
}
