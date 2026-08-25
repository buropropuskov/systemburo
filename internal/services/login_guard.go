package services

import (
	"sort"
	"sync"
	"time"
)

// loginFailureWindow - окно накопления неудачных попыток входа. Неудачи старше
// окна не учитываются (счётчик "прощает" давние ошибки), поэтому редкая опечатка
// раз в несколько минут не копится до блокировки.
const loginFailureWindow = 5 * time.Minute

// guardUsernamesPerIP - потолок числа логинов, которые запоминаются за одним IP.
// Перебор сотен имён с одного адреса упирается в потолок сознательно: столько
// состояния на один адрес держать незачем, лишнее истечёт само.
const guardUsernamesPerIP = 16

// guardMaxEntries - потолок числа адресов в памяти. Ступень пары живёт сутки,
// поэтому без потолка перебор с тысяч адресов раздувал бы карту весь этот срок.
// Сверх потолка вытесняются самые давние из незаблокированных: они уже ничего
// не стерегут, а действующая блокировка не теряется.
const guardMaxEntries = 20000

// loginGuard - счётчик НЕУДАЧНЫХ попыток входа в памяти. Ведёт два среза:
//
//   - по адресу: плоская блокировка после max неудач, любых логинов. Это сеть
//     против перебора имён с одного места. Длительность НЕ растёт по лестнице:
//     за одним внешним адресом сидит целый офис, и растущий кулдаун от чужих
//     опечаток запер бы всех разом на час без возможности сброса;
//   - по паре «адрес + введённый логин»: та же лестница сроков, что у блокировки
//     учётной записи. Пара не знает, существует ли логин, поэтому выдуманное имя
//     эскалирует наравне с настоящим - иначе по длительности блокировки можно
//     было бы отличать существующие учётки от несуществующих.
//
// Счётчик "осталось попыток" по той же причине показывается всегда и одинаково.
type loginGuard struct {
	mu       sync.Mutex
	entries  map[string]*loginAttempt
	max      int
	window   time.Duration
	blockDur time.Duration
}

type loginAttempt struct {
	failures     int
	lastFailure  time.Time
	blockedUntil time.Time
	// users - состояние по каждому логину, который падал с этого адреса. Нужно
	// и для лестницы по паре, и для адресного сброса администратором: без второго
	// админ снимает лок с учётки, а человек упирается в блокировку своего адреса.
	users map[string]*userAttempt
}

// userAttempt - срез счётчика по паре «адрес + логин». level повторяет ступень
// лестницы учётной записи, чтобы сроки совпадали для существующего и выдуманного логина.
type userAttempt struct {
	failures     int
	level        int
	lastFailure  time.Time
	blockedUntil time.Time
}

func newLoginGuard(max int, window, blockDur time.Duration) *loginGuard {
	g := &loginGuard{
		entries:  make(map[string]*loginAttempt),
		max:      max,
		window:   window,
		blockDur: blockDur,
	}
	go g.cleanup()
	return g
}

// blockedSeconds возвращает остаток блокировки в секундах, если адрес или пара
// «адрес + логин» сейчас заперты. Из двух сроков берётся больший: меньший обещал
// бы вход раньше, чем он откроется.
func (g *loginGuard) blockedSeconds(ip, username string) (int, bool) {
	if ip == "" {
		return 0, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	e := g.entries[ip]
	if e == nil {
		return 0, false
	}
	now := time.Now()
	sec := 0
	if e.blockedUntil.After(now) {
		sec = secondsUntil(e.blockedUntil)
	}
	if u := e.users[username]; u != nil && u.blockedUntil.After(now) {
		if s := secondsUntil(u.blockedUntil); s > sec {
			sec = s
		}
	}
	return sec, sec > 0
}

// recordFailure учитывает неудачную попытку входа. Возвращает остаток попыток до
// блокировки и, если попытка исчерпала лимит, остаток её длительности. Остаток
// попыток - меньший из двух срезов: он не должен обещать больше, чем есть.
// Пустой ip не учитывается (remaining = max).
func (g *loginGuard) recordFailure(ip, username string) (remaining, blockedSec int, blocked bool) {
	if ip == "" {
		return g.max, 0, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	e := g.entries[ip]
	if e == nil {
		if len(g.entries) >= guardMaxEntries {
			g.pruneLocked(now)
		}
		e = &loginAttempt{users: make(map[string]*userAttempt)}
		g.entries[ip] = e
	}

	// Срез по адресу: плоская блокировка.
	if g.cycleExpired(e.blockedUntil, e.lastFailure, now) {
		e.failures = 0
		e.blockedUntil = time.Time{}
	}
	e.failures++
	e.lastFailure = now
	if e.failures >= g.max {
		e.blockedUntil = now.Add(g.blockDur)
		blockedSec = secondsUntil(e.blockedUntil)
	}
	remaining = g.max - e.failures
	if remaining < 0 {
		remaining = 0
	}

	// Срез по паре «адрес + логин»: лестница. Новый логин сверх потолка своего
	// среза не заводит - за него отвечает счётчик адреса, который к этому моменту
	// заведомо близок к блокировке.
	u := e.users[username]
	if u == nil && username != "" && len(e.users) < guardUsernamesPerIP {
		u = &userAttempt{}
		e.users[username] = u
	}
	if u != nil {
		if g.cycleExpired(u.blockedUntil, u.lastFailure, now) {
			u.failures = 0
			u.blockedUntil = time.Time{}
		}
		// Ступень затухает по тому же правилу, что у учётной записи.
		if !u.lastFailure.IsZero() && now.Sub(u.lastFailure) > lockoutLevelDecay {
			u.level = 0
		}
		u.failures++
		u.lastFailure = now
		if u.failures >= g.max {
			u.blockedUntil = now.Add(stepDuration(g.blockDur, u.level))
			u.level++
			u.failures = 0
			if sec := secondsUntil(u.blockedUntil); sec > blockedSec {
				blockedSec = sec
			}
		}
		if left := g.max - u.failures; left < remaining {
			remaining = left
		}
	}

	if blockedSec > 0 {
		return 0, blockedSec, true
	}
	return remaining, 0, false
}

// cycleExpired сообщает, пора ли начинать счёт заново: блокировка отбыта либо
// последняя неудача старше окна.
func (g *loginGuard) cycleExpired(blockedUntil, lastFailure, now time.Time) bool {
	blockExpired := !blockedUntil.IsZero() && !blockedUntil.After(now)
	windowExpired := !lastFailure.IsZero() && now.Sub(lastFailure) > g.window
	return blockExpired || windowExpired
}

// reset снимает счётчик неудач адреса (вызывается при успешном входе).
func (g *loginGuard) reset(ip string) {
	if ip == "" {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.entries, ip)
}

// resetUser снимает счётчики со всех адресов, где падал этот логин. Зовётся, когда
// администратор разблокировал учётку: без этого лок с учётки снят, а человек всё
// ещё упирается в блокировку своего адреса и считает, что сброс не сработал.
// Возвращает число снятых записей.
func (g *loginGuard) resetUser(username string) int {
	if username == "" {
		return 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	cleared := 0
	for ip, e := range g.entries {
		if _, ok := e.users[username]; ok {
			delete(g.entries, ip)
			cleared++
		}
	}
	return cleared
}

func (g *loginGuard) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		g.mu.Lock()
		g.pruneLocked(time.Now())
		g.mu.Unlock()
	}
}

// pruneLocked выбрасывает отработавшие записи и, если после этого адресов всё
// ещё больше потолка, самые давние из незаблокированных. Вызывается под мьютексом.
func (g *loginGuard) pruneLocked(now time.Time) {
	for ip, e := range g.entries {
		// Пара живёт дольше окна: в ней лестница, и забыть её сразу значит
		// отдать перебирающему свежий цикл с первой ступени.
		for name, u := range e.users {
			idle := u.lastFailure.IsZero() || now.Sub(u.lastFailure) > lockoutLevelDecay
			if idle && !u.blockedUntil.After(now) {
				delete(e.users, name)
			}
		}
		blockDone := e.blockedUntil.IsZero() || !e.blockedUntil.After(now)
		stale := e.lastFailure.IsZero() || now.Sub(e.lastFailure) > g.window
		if blockDone && stale && len(e.users) == 0 {
			delete(g.entries, ip)
		}
	}
	if len(g.entries) <= guardMaxEntries {
		return
	}
	type aged struct {
		ip   string
		seen time.Time
	}
	free := make([]aged, 0, len(g.entries))
	for ip, e := range g.entries {
		if e.blockedUntil.After(now) {
			continue
		}
		blocked := false
		for _, u := range e.users {
			if u.blockedUntil.After(now) {
				blocked = true
				break
			}
		}
		if !blocked {
			free = append(free, aged{ip, e.lastFailure})
		}
	}
	sort.Slice(free, func(i, j int) bool { return free[i].seen.Before(free[j].seen) })
	for _, item := range free {
		if len(g.entries) <= guardMaxEntries {
			break
		}
		delete(g.entries, item.ip)
	}
}

func secondsUntil(t time.Time) int {
	s := int(time.Until(t).Seconds())
	if s < 1 {
		s = 1
	}
	return s
}
