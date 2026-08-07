package main

import (
	"flag"
	"fmt"
	"os"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// Генерация пары ключей VAPID для Web Push (#974). Разовая операция при развёртывании:
// ключи не хранятся в БД, а идут в переменные окружения VAPID_PUBLIC_KEY/VAPID_PRIVATE_KEY.
// В отличие от cleanup/storage/archive/fake команда не читает config.Load() и не
// открывает соединение с базой - генерация ключевой пары не зависит ни от того, ни от
// другого.

const vapidHelp = `Генерация пары ключей VAPID для Web Push.

Использование:
  server vapid [флаги]

Флаги:
  -help   Эта справка

Выводит готовые строки для файла параметров - вставить как есть:
  VAPID_PUBLIC_KEY=...
  VAPID_PRIVATE_KEY=...

Публичный ключ уходит в браузер (PushManager.subscribe), приватный остаётся на сервере
и никогда не передаётся клиенту - храните его как секрет, им подписываются push-запросы
от имени сервера. VAPID_SUBJECT (контакт бюро, mailto: или https://) команда не
генерирует - его задают отдельно, вручную.
`

// runVAPID выполняет подкоманду и возвращает код возврата процесса.
func runVAPID(args []string) int {
	fs := flag.NewFlagSet("vapid", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, vapidHelp) }
	help := fs.Bool("help", false, "справка")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *help {
		fmt.Print(vapidHelp)
		return 0
	}

	privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	fmt.Println("Пара ключей VAPID сгенерирована. Добавьте в файл параметров:")
	fmt.Println()
	fmt.Println("VAPID_PUBLIC_KEY=" + publicKey)
	fmt.Println("VAPID_PRIVATE_KEY=" + privateKey)
	fmt.Println()
	fmt.Println("Приватный ключ храните как секрет. Оба поля пустыми выключают push:")
	fmt.Println("система работает как раньше, без ошибок в логе.")
	return 0
}
