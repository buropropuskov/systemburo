package services

import "runtime"

// argon2Sem ограничивает число одновременных вычислений Argon2id. Каждое такое
// вычисление держит ~19 МБ рабочей памяти и грузит ядро; без лимита login-storm
// при 1-2k онлайн умножает 19 МБ на число параллельных логинов и валит процесс в
// OOM. Слот занимается только на время самого IDKey, поэтому лишние логины ждут
// очереди вместо одновременного выделения памяти.
var argon2Sem = make(chan struct{}, defaultArgon2Concurrency())

func defaultArgon2Concurrency() int {
	if n := runtime.GOMAXPROCS(0); n > 0 {
		return n
	}
	return 1
}

// SetArgon2Concurrency перенастраивает лимит одновременных Argon2-вычислений.
// n<=0 - по числу ядер (GOMAXPROCS). Вызывать на старте до приёма трафика:
// переустановка канала не синхронизирована с работающими withArgon2Slot.
func SetArgon2Concurrency(n int) {
	if n <= 0 {
		n = defaultArgon2Concurrency()
	}
	argon2Sem = make(chan struct{}, n)
}

// withArgon2Slot выполняет fn, заняв слот семафора, и освобождает его после.
// Канал захватывается в локальную переменную, чтобы захват и освобождение шли
// через один и тот же экземпляр семафора даже при гонке с SetArgon2Concurrency.
func withArgon2Slot(fn func()) {
	sem := argon2Sem
	sem <- struct{}{}
	defer func() { <-sem }()
	fn()
}
