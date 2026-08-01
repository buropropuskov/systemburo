package services

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Кэш байтов шаблона (#1615, B4). База ему не нужна - он про файл на диске, поэтому
// тест живёт здесь, а не в internal/handlers.

func TestTemplateCache_ReadsOnceUntilFileChanges(t *testing.T) {
	svc := &attachmentBlankService{templateCache: make(map[string]cachedTemplate)}
	path := filepath.Join(t.TempDir(), "template.xlsx")
	require.NoError(t, os.WriteFile(path, []byte("первая версия"), 0o600))

	first, err := svc.loadTemplateFile(path)
	require.NoError(t, err)
	assert.Equal(t, "первая версия", string(first))

	// Файл на месте, содержимое не менялось - отдаётся кэш. Проверяем не по числу
	// чтений (его не видно), а по тому, что подменённое мимо кэша содержимое с теми
	// же приметами файла не подхватывается.
	info, err := os.Stat(path)
	require.NoError(t, err)
	// Подменяем содержимое, сохранив размер и время изменения: для файловой системы
	// это «тот же файл». Кэш обязан отдать прежние байты - иначе он не кэш.
	require.NoError(t, os.WriteFile(path, []byte("подмена ровно"), 0o600))
	require.NoError(t, os.Chtimes(path, info.ModTime(), info.ModTime()))

	second, err := svc.loadTemplateFile(path)
	require.NoError(t, err)
	assert.Equal(t, "первая версия", string(second), "приметы файла не изменились - читать заново незачем")

	// Замена файла на месте: администратор загрузил новый шаблон под тем же путём.
	// Держать кэш на инварианте «путь не переиспользуется» нельзя - система молча
	// раздавала бы старые бланки до перезапуска процесса.
	require.NoError(t, os.WriteFile(path, []byte("вторая версия шаблона"), 0o600))
	later := time.Now().Add(time.Second)
	require.NoError(t, os.Chtimes(path, later, later))

	third, err := svc.loadTemplateFile(path)
	require.NoError(t, err)
	assert.Equal(t, "вторая версия шаблона", string(third), "изменённый файл обязан перечитаться")
}

// Файл того же размера, но с другим временем изменения - тоже другой файл.
func TestTemplateCache_InvalidatesOnMtimeAloneChange(t *testing.T) {
	svc := &attachmentBlankService{templateCache: make(map[string]cachedTemplate)}
	path := filepath.Join(t.TempDir(), "same-size.xlsx")
	require.NoError(t, os.WriteFile(path, []byte("AAAA"), 0o600))

	first, err := svc.loadTemplateFile(path)
	require.NoError(t, err)
	require.Equal(t, "AAAA", string(first))

	require.NoError(t, os.WriteFile(path, []byte("BBBB"), 0o600))
	later := time.Now().Add(2 * time.Second)
	require.NoError(t, os.Chtimes(path, later, later))

	second, err := svc.loadTemplateFile(path)
	require.NoError(t, err)
	assert.Equal(t, "BBBB", string(second), "размер совпал, но файл другой - решает время изменения")
}

// Разные пути не путаются между собой.
func TestTemplateCache_KeepsPathsApart(t *testing.T) {
	svc := &attachmentBlankService{templateCache: make(map[string]cachedTemplate)}
	dir := t.TempDir()
	one := filepath.Join(dir, "one.xlsx")
	two := filepath.Join(dir, "two.xlsx")
	require.NoError(t, os.WriteFile(one, []byte("шаблон один"), 0o600))
	require.NoError(t, os.WriteFile(two, []byte("шаблон два"), 0o600))

	gotOne, err := svc.loadTemplateFile(one)
	require.NoError(t, err)
	gotTwo, err := svc.loadTemplateFile(two)
	require.NoError(t, err)

	assert.Equal(t, "шаблон один", string(gotOne))
	assert.Equal(t, "шаблон два", string(gotTwo))
}

// Удалённый шаблон обязан давать честную ошибку, а не бланк из памяти процесса.
func TestTemplateCache_ForgetsRemovedFile(t *testing.T) {
	svc := &attachmentBlankService{templateCache: make(map[string]cachedTemplate)}
	path := filepath.Join(t.TempDir(), "gone.xlsx")
	require.NoError(t, os.WriteFile(path, []byte("шаблон"), 0o600))

	_, err := svc.loadTemplateFile(path)
	require.NoError(t, err)

	require.NoError(t, os.Remove(path))
	_, err = svc.loadTemplateFile(path)
	require.Error(t, err, "файла нет - генерация обязана упасть, а не выдать кэш")

	svc.templateCacheMu.Lock()
	_, stillCached := svc.templateCache[path]
	svc.templateCacheMu.Unlock()
	assert.False(t, stillCached, "запись пропавшего файла не должна занимать память до перезапуска")
}

// Массовый прогон читает шаблон из многих горутин - карта обязана это переживать.
// Гонку ловит -race, здесь проверяем ещё и что все получили одинаковые байты.
func TestTemplateCache_ConcurrentReads(t *testing.T) {
	svc := &attachmentBlankService{templateCache: make(map[string]cachedTemplate)}
	path := filepath.Join(t.TempDir(), "concurrent.xlsx")
	require.NoError(t, os.WriteFile(path, []byte("общий шаблон"), 0o600))

	var wg sync.WaitGroup
	results := make([][]byte, 16)
	errs := make([]error, 16)
	for i := range results {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = svc.loadTemplateFile(path)
		}(i)
	}
	wg.Wait()

	for i := range results {
		require.NoError(t, errs[i])
		assert.Equal(t, "общий шаблон", string(results[i]))
	}
}
