package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"systemburo/internal/config"
	"systemburo/internal/crypto"
	"systemburo/internal/database"
)

// Перевод паспортных данных на другой ключ шифрования (#2253).
//
// Отдельной кнопки в интерфейсе нет намеренно: команда переписывает персональные
// данные во всей базе и требует на руках оба ключа. Это работа того, кто хранит
// ключ и делает резервные копии.
//
// Без флага -apply команда ничего не пишет, а показывает, сколько значений
// подлежит переводу.

const reencryptHelp = `Перевод паспортных данных на другой ключ шифрования.

Использование:
  server reencrypt -old-key <прежний ключ> [флаги]

Ключ, НА который переводим, берётся из DATA_ENCRYPTION_KEY в параметрах установки.
Прежний ключ передаётся флагом: он в параметрах уже не значится.

Флаги:
  -old-key   Прежний ключ, 64 шестнадцатеричных символа. Пустое значение означает,
             что данные лежат открытыми и шифрование включается впервые
  -apply     Выполнить перевод. Без него команда только считает
  -help      Эта справка

Порядок работы:
  1. Остановить приём заявок (режим технических работ) и снять резервную копию
  2. Прописать новый ключ в DATA_ENCRYPTION_KEY
  3. server reencrypt -old-key <прежний>            посмотреть объём
  4. server reencrypt -old-key <прежний> -apply     выполнить
  5. Поднять систему: контрольная запись подтвердит, что ключ подходит

Перевод идёт одной транзакцией. Прерывание отменяет всё: половина записей на
одном ключе и половина на другом не откроется целиком ни одним из них.
`

func runReencrypt(args []string) int {
	fs := flag.NewFlagSet("reencrypt", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, reencryptHelp) }
	oldKeyHex := fs.String("old-key", "", "прежний ключ, 64 шестнадцатеричных символа")
	apply := fs.Bool("apply", false, "выполнить перевод")
	help := fs.Bool("help", false, "справка")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *help {
		fmt.Print(reencryptHelp)
		return 0
	}

	oldKey, err := crypto.ParseHexKey(*oldKeyHex)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка: прежний ключ не разобран:", err)
		return 2
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка: параметры не загружены:", err)
		return 1
	}
	newKey, err := crypto.ParseHexKey(cfg.DataEncryptionKey)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка: DATA_ENCRYPTION_KEY не разобран:", err)
		return 2
	}

	if *oldKeyHex == cfg.DataEncryptionKey {
		fmt.Fprintln(os.Stderr, "Ошибка: прежний и новый ключ совпадают, переводить нечего.")
		fmt.Fprintln(os.Stderr, "Новый ключ берётся из DATA_ENCRYPTION_KEY - пропишите его до запуска команды.")
		return 2
	}

	db, err := openCleanupDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	results, err := database.Reencrypt(context.Background(), db, database.ReencryptOptions{
		OldKey: oldKey,
		NewKey: newKey,
		Apply:  *apply,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		return 1
	}

	printReencryptReport(results, *apply, newKey == nil)
	return 0
}

func printReencryptReport(results []database.ReencryptResult, applied bool, cleartext bool) {
	fmt.Println()
	if applied {
		fmt.Println("Перевод выполнен, контрольная запись обновлена.")
		if cleartext {
			fmt.Println("DATA_ENCRYPTION_KEY пуст: паспортные данные теперь хранятся открытыми.")
		}
	} else {
		fmt.Println("Ничего не изменено: это предварительный показ. Повторите с флагом -apply.")
	}
	fmt.Println()
	fmt.Println(padRight("Таблица", 26), padLeft("Строк", 10), padLeft("Значений", 12))

	var totalRows, totalValues int64
	for _, r := range results {
		fmt.Println(padRight(r.Table, 26), padLeft(fmt.Sprint(r.Rows), 10), padLeft(fmt.Sprint(r.Values), 12))
		totalRows += r.Rows
		totalValues += r.Values
	}
	fmt.Println(padRight("Итого", 26), padLeft(fmt.Sprint(totalRows), 10), padLeft(fmt.Sprint(totalValues), 12))
	fmt.Println()
}
