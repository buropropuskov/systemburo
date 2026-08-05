package fakedata_test

import (
	"testing"

	"systemburo/internal/fakedata"

	"github.com/stretchr/testify/require"
)

func TestDatabaseName(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
		want string
	}{
		{"url", "postgres://postgres:postgres@db:5432/auto_registry?sslmode=disable", "auto_registry"},
		{"url без параметров", "postgresql://user@host/systemburo", "systemburo"},
		{"пусто", "", ""},
		// Набор ключей со значениями до команды не доходит: config.Validate не пускает
		// систему стартовать с таким DATABASE_URL. Пустое имя тут безопаснее выдумки --
		// подтверждение с ним не совпадёт, и обход отметки не откроется.
		{"набор ключей", "host=db user=postgres dbname=auto_registry", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, fakedata.DatabaseName(c.dsn))
		})
	}
}
