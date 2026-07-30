package services

import (
	"sync"
	"time"
)

// loginFailureWindow - окно накопления неудачных попыток входа. Неудачи старше
// окна не учитываются (счётчик "прощает" давние ошибки), поэтому редкая опечатка
// раз в несколько минут не копится до блокировки.
const loginFailureWindow = 5 * time.Minute

// guardUsernamesPerIP - потолок числа логинов, которые запоминаются за одним IP
// ради адресного сброса администратором. Перебор сотен имён с одного адреса
// упирается в потолок сознательно: такую запись снимать по просьбе одного
// пользователя незачем, она истечёт сама.
const guardUsernamesPerIP = 16

// loginGuard - единый per-IP счётчик НЕУДАЧНЫХ попыток входа. Считает попытки
// одинаково для существующих и несуществующих логинов, поэтому счётчик
// "осталось попыток" показывается всегда и не раскрывает, существует ли логин
// (иначе по наличию счётчика можно было бы перебирать имена). При исчерпании
// лимита блокирует IP на фиксированную длительность от момента блокировки -
// таймер честно тикает до нуля; после истечения даётся свежий цикл попыток.
//
// Длительность здесь ПЛОСКАЯ и не растёт по лестнице, в отличие от блокировки
// учётной записи: за одним внешним адресом сидит целый офис, и растущий кулдаун
// от чужих опечаток запер бы всех разом на час без возможности сброса. Лестница
// живёт на персональной учётке, которую администратор может разблокировать.
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
	// usernames - логины, с которыми с этого адреса были неудачи. Нужны, чтобы
	// сброс блокировки конкретному пользователю снимал и per-IP счётчик: иначе
	// администратор снимает лок с учётки, а человек упирается в блокировку адреса.
	usernames map[string]struct{}
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

// blockedSeconds возвращает остаток блокировки в секундах, если IP сейчас
// заблокирован. Иначе (0, false).
func (g *loginGuard) blockedSeconds(ip string) (int, bool) {
	if ip == "" {
		return 0, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	e := g.entries[ip]
	if e == nil {
		return 0, false
	}
	if e.blockedUntil.After(time.Now()) {
		return secondsUntil(e.blockedUntil), true
	}
	return 0, false
}

// recordFailure учитывает неудачную попытку входа с IP. Возвращает остаток попыток
// до блокировки и, если эта попытка исчерпала лимит, признак блокировки с остатком
// её длительности. username запоминается для адресного сброса администратором;
// пустой ip не учитывается (remaining = max).
func (g *loginGuard) recordFailure(ip, username string) (remaining, blockedSec int, blocked bool) {
	if ip == "" {
		return g.max, 0, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	e := g.entries[ip]
	if e == nil {
		e = &loginAttempt{usernames: make(map[string]struct{})}
		g.entries[ip] = e
	}
	if username != "" && len(e.usernames) < guardUsernamesPerIP {
		e.usernames[username] = struct{}{}
	}
	// Истёкшая блокировка или давняя последняя неудача -> свежий цикл.
	blockExpired := !e.blockedUntil.IsZero() && !e.blockedUntil.After(now)
	windowExpired := !e.lastFailure.IsZero() && now.Sub(e.lastFailure) > g.window
	if blockExpired || windowExpired {
		e.failures = 0
		e.blockedUntil = time.Time{}
	}
	e.failures++
	e.lastFailure = now
	if e.failures >= g.max {
		e.blockedUntil = now.Add(g.blockDur)
		return 0, secondsUntil(e.blockedUntil), true
	}
	return g.max - e.failures, 0, false
}

// reset снимает счётчик неудач IP (вызывается при успешном входе).
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
		if _, ok := e.usernames[username]; ok {
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
		now := time.Now()
		for ip, e := range g.entries {
			blockDone := e.blockedUntil.IsZero() || !e.blockedUntil.After(now)
			stale := e.lastFailure.IsZero() || now.Sub(e.lastFailure) > g.window
			if blockDone && stale {
				delete(g.entries, ip)
			}
		}
		g.mu.Unlock()
	}
}

func secondsUntil(t time.Time) int {
	s := int(time.Until(t).Seconds())
	if s < 1 {
		s = 1
	}
	return s
}
