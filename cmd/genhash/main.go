// Команда genhash печатает хэш пароля в том же формате, что хранит система.
// Нужна при ручном восстановлении доступа: хэш вставляют в базу напрямую, когда
// войти в интерфейс нечем.
package main

import (
	"fmt"
	"os"

	"systemburo/internal/services"
)

func main() {
	// Пароль берётся аргументом; прежняя версия несла зашитый admin123 и печатала
	// хэш от него независимо от того, что просили.
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "использование: genhash <пароль>")
		os.Exit(2)
	}
	fmt.Println(services.HashPassword(os.Args[1]))
}
