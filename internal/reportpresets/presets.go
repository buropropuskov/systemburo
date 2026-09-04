// Package reportpresets отдаёт готовые наборы конструктора отчётов.
//
// Источник один - frontend/src/components/statistics/reportPresets.json: по нему
// строится галерея во вкладке «Отчёты», и он же копируется сюда командой
// `make sync-presets`, чтобы бэкенд заводил в базе системные шаблоны с теми же
// названиями и описаниями. Расхождение копии с источником ловит тест
// TestReportPresets_FrontAndBackInSync - иначе список снова разъедется, как это
// произошло после переезда галереи из базы во фронт (#632 -> #2315).
package reportpresets

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed presets.json
var presetsJSON []byte

// Preset — готовый набор отчёта. Form непрозрачен для бэкенда: его применяет
// конструктор на фронте, сюда он попадает как есть и ложится в config шаблона.
type Preset struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	ResultHint  string          `json:"resultHint"`
	Form        json.RawMessage `json:"form"`
}

// All возвращает наборы в порядке источника: он же порядок карточек в галерее.
func All() ([]Preset, error) {
	var presets []Preset
	if err := json.Unmarshal(presetsJSON, &presets); err != nil {
		return nil, fmt.Errorf("parse report presets: %w", err)
	}
	return presets, nil
}

// Raw отдаёт исходный JSON — нужен тесту синхронности с фронтом.
func Raw() []byte {
	return presetsJSON
}
