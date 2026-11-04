package services

import (
	"crypto/sha256"
	"net/http"
	"strconv"
	"sync"
	"time"

	"systemburo/internal/apperr"
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
// Включает зарезервированные незавершёнными попытками записи. Состояние,
// влияющее на счётчик или ступень блокировки, не вытесняется ради нового адреса.
const guardMaxEntries = 20000

const guardCleanupInterval = 5 * time.Minute

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
	mu          sync.Mutex
	entries     map[string]*loginAttempt
	max         int
	window      time.Duration
	blockDur    time.Duration
	maxEntries  int
	now         func() time.Time
	nextCleanup time.Time
}

type loginAttempt struct {
	refs         int
	failures     int
	lastFailure  time.Time
	blockedUntil time.Time
	// users - состояние по SHA256 точных байтов логина, который падал с адреса. Нужно
	// и для лестницы по паре, и для адресного сброса администратором: без второго
	// админ снимает лок с учётки, а человек упирается в блокировку своего адреса.
	users map[[sha256.Size]byte]*userAttempt
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
		entries:    make(map[string]*loginAttempt),
		max:        max,
		window:     window,
		blockDur:   blockDur,
		maxEntries: guardMaxEntries,
		now:        time.Now,
	}
	return g
}

// loginGuardLease keeps an address resident until the attempt finishes, including
// when a concurrent success or administrative reset clears its counters.
type loginGuardLease struct {
	guard    *loginGuard
	ip       string
	entry    *loginAttempt
	released bool
}

func (g *loginGuard) reserve(ip string) (*loginGuardLease, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	lease := &loginGuardLease{guard: g, ip: ip}
	if ip == "" {
		return lease, nil
	}
	now := g.now()
	g.cleanupLocked(now)
	entry := g.entries[ip]
	if entry == nil {
		if len(g.entries) >= g.maxEntries {
			wait := g.nextCleanup.Sub(now)
			seconds := int64(wait / time.Second)
			if wait%time.Second > 0 {
				seconds++
			}
			if seconds < 1 {
				seconds = 1
			}
			return nil, apperr.New(http.StatusServiceUnavailable,
				"Сервис временно занят. Повтори попытку чуть позже.").
				WithHeader("Retry-After", strconv.FormatInt(seconds, 10))
		}
		entry = &loginAttempt{users: make(map[[sha256.Size]byte]*userAttempt)}
		g.entries[ip] = entry
	}
	entry.refs++
	lease.entry = entry
	return lease, nil
}

func (lease *loginGuardLease) release() {
	g := lease.guard
	g.mu.Lock()
	defer g.mu.Unlock()
	if lease.released {
		return
	}
	lease.released = true
	if lease.entry == nil {
		return
	}
	lease.entry.refs--
	if lease.entry.refs == 0 && lease.entry.failures == 0 && lease.entry.lastFailure.IsZero() &&
		lease.entry.blockedUntil.IsZero() && len(lease.entry.users) == 0 {
		delete(g.entries, lease.ip)
	}
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
	now := g.now()
	g.cleanupLocked(now)
	sec := 0
	if e.blockedUntil.After(now) {
		sec = guardSecondsUntil(e.blockedUntil, now)
	}
	if u := e.users[sha256.Sum256([]byte(username))]; u != nil && u.blockedUntil.After(now) {
		if s := guardSecondsUntil(u.blockedUntil, now); s > sec {
			sec = s
		}
	}
	return sec, sec > 0
}

// recordFailure учитывает неудачную попытку входа. Возвращает остаток попыток до
// блокировки и, если попытка исчерпала лимит, остаток её длительности. Остаток
// попыток - меньший из двух срезов: он не должен обещать больше, чем есть.
// Пустой ip не учитывается (remaining = max).
func (g *loginGuard) recordFailure(lease *loginGuardLease, username string) (remaining, blockedSec int, blocked bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if lease == nil || lease.guard != g || lease.released {
		panic("login failure requires an active guard reservation")
	}
	if lease.entry == nil {
		return g.max, 0, false
	}
	now := g.now()
	e := lease.entry

	// Срез по адресу: плоская блокировка.
	if g.cycleExpired(e.blockedUntil, e.lastFailure, now) {
		e.failures = 0
		e.blockedUntil = time.Time{}
	}
	e.failures++
	e.lastFailure = now
	if e.failures >= g.max {
		e.blockedUntil = now.Add(g.blockDur)
		blockedSec = guardSecondsUntil(e.blockedUntil, now)
	}
	remaining = g.max - e.failures
	if remaining < 0 {
		remaining = 0
	}

	// Срез по паре «адрес + логин»: лестница. Новый логин сверх потолка своего
	// среза не заводит - за него отвечает счётчик адреса, который к этому моменту
	// заведомо близок к блокировке.
	key := sha256.Sum256([]byte(username))
	u := e.users[key]
	if u == nil && username != "" && len(e.users) < guardUsernamesPerIP {
		u = &userAttempt{}
		e.users[key] = u
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
			if sec := guardSecondsUntil(u.blockedUntil, now); sec > blockedSec {
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
	g.resetLocked(ip)
}

func (g *loginGuard) resetLocked(ip string) {
	e := g.entries[ip]
	if e == nil {
		return
	}
	if e.refs == 0 {
		delete(g.entries, ip)
		return
	}
	*e = loginAttempt{refs: e.refs, users: make(map[[sha256.Size]byte]*userAttempt)}
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
	key := sha256.Sum256([]byte(username))
	for ip, e := range g.entries {
		if _, ok := e.users[key]; ok {
			g.resetLocked(ip)
			cleared++
		}
	}
	return cleared
}

func (g *loginGuard) cleanupLocked(now time.Time) {
	if !now.Before(g.nextCleanup) {
		g.pruneLocked(now)
		g.nextCleanup = now.Add(guardCleanupInterval)
	}
}

// pruneLocked removes only state that no longer affects failures or the ladder.
// In-flight attempts retain their address even when all counters have expired.
func (g *loginGuard) pruneLocked(now time.Time) {
	for ip, e := range g.entries {
		// Поднятая ступень пары живёт дольше окна: забыть её сразу значит
		// отдать перебирающему свежий цикл с первой ступени.
		for name, u := range e.users {
			idle := u.lastFailure.IsZero() || g.cycleExpired(u.blockedUntil, u.lastFailure, now)
			levelDone := u.level == 0 || now.Sub(u.lastFailure) > lockoutLevelDecay
			if idle && levelDone && !u.blockedUntil.After(now) {
				delete(e.users, name)
			}
		}
		blockDone := e.blockedUntil.IsZero() || !e.blockedUntil.After(now)
		stale := e.lastFailure.IsZero() || g.cycleExpired(e.blockedUntil, e.lastFailure, now)
		if e.refs == 0 && blockDone && stale && len(e.users) == 0 {
			delete(g.entries, ip)
		}
	}
}

func guardSecondsUntil(until, now time.Time) int {
	seconds := int(until.Sub(now) / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}

func secondsUntil(t time.Time) int {
	s := int(time.Until(t).Seconds())
	if s < 1 {
		s = 1
	}
	return s
}
