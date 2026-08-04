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
		{"набор ключей", "host=db user=postgres dbname=auto_registry sslmode=disable", "auto_registry"},
		{"пусто", "", ""},
		{"без имени базы", "host=db user=postgres", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, fakedata.DatabaseName(c.dsn))
		})
	}
}
