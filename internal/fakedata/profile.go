// Package fakedata наливает на проверочный стенд связный пласт вымышленных данных
// (#1682): справочники, пользователей, заявки, проходы. Пакет живёт отдельно от
// internal/services намеренно -- наливка не часть боевой логики системы и не должна
// быть доступна обработчикам.
package fakedata

import (
	"fmt"
	"sort"
	"strings"
)

// Profile -- заготовка объёма наливки. Значения задают верхнюю границу: шаг создаёт
// столько записей, сколько просит профиль, если у него хватает исходных данных.
type Profile struct {
	Name          string
	Organizations int
	Companies     int
	Users         int
	Employees     int
	Cars          int
	Applications  int
	Blacklists    int
	// DaysBack -- на сколько суток назад растягиваются даты заявок и проходов. Без
	// разброса по времени аналитика и отчёты показывают один день, и проверять на
	// таких данных нечего.
	DaysBack int
}

// Профили названы по объёму, а не по назначению: "проверить одну кнопку" и "посмотреть
// поведение списка на объёме" -- это про количество записей, и выбирать проще числами.
var profiles = map[string]Profile{
	"small": {
		Name: "small", Organizations: 3, Companies: 3, Users: 10,
		Employees: 30, Cars: 20, Applications: 30, Blacklists: 5, DaysBack: 30,
	},
	"medium": {
		Name: "medium", Organizations: 10, Companies: 10, Users: 50,
		Employees: 300, Cars: 200, Applications: 300, Blacklists: 20, DaysBack: 180,
	},
	"large": {
		Name: "large", Organizations: 50, Companies: 50, Users: 500,
		Employees: 3000, Cars: 2000, Applications: 5000, Blacklists: 100, DaysBack: 365,
	},
}

// DefaultProfile -- профиль, который берётся, когда -profile не задан.
const DefaultProfile = "medium"

// ProfileNames возвращает имена профилей в порядке возрастания объёма -- для справки
// команды, где перечисление вразнобой читается как небрежность.
func ProfileNames() []string {
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return profiles[names[i]].Applications < profiles[names[j]].Applications
	})
	return names
}

// ProfileByName отдаёт копию профиля по имени.
func ProfileByName(name string) (Profile, error) {
	p, ok := profiles[strings.TrimSpace(strings.ToLower(name))]
	if !ok {
		return Profile{}, fmt.Errorf("неизвестный профиль %q, доступны: %s", name, strings.Join(ProfileNames(), ", "))
	}
	return p, nil
}

// Overrides -- точечные переопределения профиля флагами команды. Ноль означает
// «оставить значение профиля»: отличить неуказанный флаг от осознанного нуля иначе
// нельзя, а наливка нуля организаций смысла не имеет.
type Overrides struct {
	Organizations int
	Companies     int
	Users         int
	Employees     int
	Cars          int
	Applications  int
	Blacklists    int
	DaysBack      int
}

// Apply накладывает переопределения на профиль.
func (p Profile) Apply(o Overrides) Profile {
	applyOne := func(dst *int, v int) {
		if v > 0 {
			*dst = v
		}
	}
	applyOne(&p.Organizations, o.Organizations)
	applyOne(&p.Companies, o.Companies)
	applyOne(&p.Users, o.Users)
	applyOne(&p.Employees, o.Employees)
	applyOne(&p.Cars, o.Cars)
	applyOne(&p.Applications, o.Applications)
	applyOne(&p.Blacklists, o.Blacklists)
	applyOne(&p.DaysBack, o.DaysBack)
	return p
}
