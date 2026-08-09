package services

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Отпечаток пустого бланка: по нему загруженный обратно файл узнаётся как бланк
// конкретного типа вложения, а не «какой-то xlsx». Импорт списка сверяет отпечаток
// с типом вложения, в который подают, и отказывает целиком, если это чужой бланк.
//
// Пишется в два места сразу: свойства документа переживают правку в Excel, но
// теряются при пересохранении частью редакторов и конвертеров; очень скрытый лист
// живёт наоборот. Совпасть достаточно одному.
const (
	blankFingerprintPrefix = "systemburo"
	// blankFingerprintSheet - лист с дублем отпечатка. Скрывается как veryHidden:
	// через интерфейс Excel такой лист не вернуть (нужен редактор VBA), поэтому
	// заполняющий бланк его не увидит и не снесёт вместе с «лишними» листами.
	blankFingerprintSheet = "systemburo_mark"
	blankFingerprintCell  = "A1"
)

// BlankFingerprint - отпечаток пустого бланка. ListStartRow нужен разбору: по нему
// видно, с какой строки в этом файле начинался список на момент выдачи, даже если
// шаблон с тех пор перенастроили.
type BlankFingerprint struct {
	UniqueAttachmentID int
	TemplateID         int
	ListStartRow       int
}

// String - текстовая форма отпечатка, она же то, что лежит в файле.
func (fp BlankFingerprint) String() string {
	return fmt.Sprintf("%s:ua=%d:tpl=%d:rows=%d",
		blankFingerprintPrefix, fp.UniqueAttachmentID, fp.TemplateID, fp.ListStartRow)
}

// ParseBlankFingerprint разбирает строку отпечатка. Второе значение - удалось ли:
// чужой файл, пустая строка и мусор в свойствах документа дают false, а не ошибку,
// потому что отсутствие отпечатка - обычный случай (бланк пересобрали руками), и
// разбор в этом случае падает на проверку структуры колонок.
func ParseBlankFingerprint(s string) (BlankFingerprint, bool) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) < 2 || parts[0] != blankFingerprintPrefix {
		return BlankFingerprint{}, false
	}
	var fp BlankFingerprint
	for _, part := range parts[1:] {
		key, raw, ok := strings.Cut(part, "=")
		if !ok {
			return BlankFingerprint{}, false
		}
		value, err := strconv.Atoi(raw)
		if err != nil {
			return BlankFingerprint{}, false
		}
		switch key {
		case "ua":
			fp.UniqueAttachmentID = value
		case "tpl":
			fp.TemplateID = value
		case "rows":
			fp.ListStartRow = value
		}
	}
	// Строка списка меньше первой - не «странное значение», а нечитаемый отпечаток:
	// разбор загруженного бланка отсчитывает от неё строки участников.
	if fp.UniqueAttachmentID <= 0 || fp.TemplateID <= 0 || fp.ListStartRow <= 0 {
		return BlankFingerprint{}, false
	}
	return fp, true
}

// StampBlankFingerprint вписывает отпечаток в уже открытый файл.
func StampBlankFingerprint(f *excelize.File, fp BlankFingerprint) error {
	mark := fp.String()
	// SetDocProps переписывает свойства ЦЕЛИКОМ по переданной структуре: непереданные
	// поля обнуляются, а не наследуются. Поэтому читаем текущие и меняем только
	// Category - иначе у шаблона пропадут заголовок, автор и тема, которые админ
	// вписал в Excel.
	props, err := f.GetDocProps()
	if err != nil {
		return fmt.Errorf("read blank doc props: %w", err)
	}
	if props == nil {
		props = &excelize.DocProperties{}
	}
	props.Category = mark
	if err := f.SetDocProps(props); err != nil {
		return fmt.Errorf("set blank fingerprint doc props: %w", err)
	}

	index, err := f.GetSheetIndex(blankFingerprintSheet)
	if err != nil {
		return fmt.Errorf("lookup blank fingerprint sheet: %w", err)
	}
	if index == -1 {
		if _, err := f.NewSheet(blankFingerprintSheet); err != nil {
			return fmt.Errorf("create blank fingerprint sheet: %w", err)
		}
	}
	if err := f.SetCellStr(blankFingerprintSheet, blankFingerprintCell, mark); err != nil {
		return fmt.Errorf("write blank fingerprint cell: %w", err)
	}
	if err := f.SetSheetVisible(blankFingerprintSheet, false, true); err != nil {
		return fmt.Errorf("hide blank fingerprint sheet: %w", err)
	}
	return nil
}

// ReadBlankFingerprint достаёт отпечаток из открытого файла: сначала из свойств
// документа, затем из скрытого листа. Ошибки чтения обоих источников означают ровно
// «отпечатка нет» - у произвольного .xlsx нашего листа не существует, и это не сбой.
func ReadBlankFingerprint(f *excelize.File) (BlankFingerprint, bool) {
	if props, err := f.GetDocProps(); err == nil && props != nil {
		if fp, ok := ParseBlankFingerprint(props.Category); ok {
			return fp, true
		}
	}
	if value, err := f.GetCellValue(blankFingerprintSheet, blankFingerprintCell); err == nil {
		if fp, ok := ParseBlankFingerprint(value); ok {
			return fp, true
		}
	}
	return BlankFingerprint{}, false
}
