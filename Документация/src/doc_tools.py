"""Общие операции документационных проверок без зависимости от shell."""
import re
import subprocess
from pathlib import Path


def git(repo, *args, allowed=(0,)):
    result = subprocess.run(
        ['git', '-c', 'core.quotepath=false', *args], cwd=repo,
        capture_output=True, encoding='utf-8', errors='strict', timeout=120,
    )
    if result.returncode not in allowed:
        raise RuntimeError('git %s: %s' % (' '.join(args), result.stderr.strip()))
    return result


def source(repo, path):
    return (Path(repo) / path).read_text(encoding='utf-8')


def section(body, start, end=r'^}'):
    found = re.search(start, body, re.M)
    if not found:
        raise ValueError('В коде не найдено начало раздела: ' + start)
    tail = body[found.end():]
    finish = re.search(end, tail, re.M)
    if not finish:
        raise ValueError('В коде не найден конец раздела: ' + end)
    return tail[:finish.start()]


def number(pattern, body):
    found = re.search(pattern, body, re.M)
    if not found:
        raise ValueError('В коде не найден числовой факт: ' + pattern)
    return int(found.group(1))


def public_api(repo):
    return section(source(repo, 'internal/router/router.go'),
                   r'^\s*api := e.Group[^\n]*', r'^\s*protected := api.Group')


def admin_views(repo):
    paths = list((Path(repo) / 'frontend/src/views/admin').glob('*.vue'))
    paths += [Path(repo) / p for p in (
        'frontend/src/views/AdminSettings.vue', 'frontend/src/views/RequestsView.vue',
        'frontend/src/views/FeedbackPage.vue', 'frontend/src/components/TableConstructor.vue')]
    for path in paths:
        if not path.is_file():
            raise FileNotFoundError(path)
    return {p.name for p in paths if p.name != 'AdminPageShell.vue'}


def schema_table_count(repo, models):
    """Базовые таблицы схемы; динамические партиции не входят в показатель."""
    names = set(re.findall(r'&models\.(\w+)\s*{', models))
    extra = set()
    for path in (Path(repo) / 'internal/database').glob('*.go'):
        if not path.name.endswith('_test.go'):
            extra.update(re.findall(r'CREATE TABLE (?:IF NOT EXISTS )?([a-z_]+)\s*\(',
                                   path.read_text(encoding='utf-8')))
    # SQL-журнал может позднее перейти в AutoMigrate — не посчитать его дважды.
    explicit = {}
    for path in (Path(repo) / 'internal/models').glob('*.go'):
        if not path.name.endswith('_test.go'):
            explicit.update(re.findall(r'func\s+\((?:\w+\s+)?\*?(\w+)\)\s+TableName\(\)\s+string\s*{\s*return\s+"([^"]+)"',
                                       path.read_text(encoding='utf-8')))
    extra -= {explicit[name] for name in names if name in explicit}
    return len(names) + len(extra)


def collect_metrics(repo):
    root = Path(repo)
    paths = [root / p for p in git(repo, 'ls-files', '-z').stdout.split('\0') if p]
    code = [p for p in paths if p.suffix in ('.go', '.vue', '.js', '.ts', '.css', '.scss')
            and 'docs' != p.relative_to(root).parts[0]]
    contents = {p: p.read_text(encoding='utf-8') for p in code}
    go = [p for p in code if p.suffix == '.go'
          and p.relative_to(root).parts[0] in ('cmd', 'internal')]
    go_tests = [p for p in go if p.name.endswith('_test.go')]
    fe = root / 'frontend/src'
    specs = [p for p in code if p.is_relative_to(fe)
             and re.search(r'\.spec\.(js|ts)$', p.name)]
    e2e = [p for p in paths if p.is_relative_to(root / 'frontend/e2e')
           and '.spec.' in p.name]
    contents.update({p: p.read_text(encoding='utf-8') for p in e2e if p not in contents})
    count_matches = lambda pattern, files: sum(len(re.findall(pattern, contents[p], re.M)) for p in files)
    lines = lambda files: sum(len(contents[p].splitlines()) for p in files)
    migration = source(repo, 'internal/database/migrate.go')
    models = section(migration, r'^func AllModels\(\) \[\]interface\{\} \{')
    catalog = section(source(repo, 'internal/services/permission_catalog.go'), r'^func staticCatalog\(\)[^{]+{')
    verbs = section(source(repo, 'internal/services/permission_service.go'), r'^var tableVerbs[^\n]*')
    types = section(migration, r'^\s*types := \[\]models.UserType{', r'^\s*}\s*$')
    router = source(repo, 'internal/router/router.go')
    report = source(repo, 'internal/services/daily_pass_report_service.go')
    m = {
        'go_tests': count_matches(r'^func Test', go_tests),
        'go_test_files': len(go_tests),
        'go_db_files': sum('testutil.' in contents[p] for p in go_tests if p.is_relative_to(root / 'internal')),
        'vitest': count_matches(r'^\s*it(?:\.each)?\s*\(', specs),
        'vitest_files': len(specs), 'e2e': count_matches(r'^\s*test\s*\(', e2e), 'e2e_files': len(e2e),
        'db_tables': schema_table_count(repo, models),
        'api_methods': len(re.findall(r'\.(?:GET|POST|PUT|PATCH|DELETE)\(', router)),
        'api_groups': len(re.findall(r'\.Group\(', router)),
        'public_routes': len(re.findall(r'api\.(?:GET|POST|PUT|PATCH|DELETE)\(', public_api(repo))),
        'perm_keys': len(re.findall(r'(?:^|{)\s*Key:', catalog, re.M)),
        'perm_cats': len(set(re.findall(r'Category:\s*(Cat\w+)', catalog))),
        'table_verbs': len(re.findall(r'^\s*{?"', verbs, re.M)),
        'gin': len(re.findall(r'USING gin', migration)),
        'user_types': len(re.findall(r'Code:', types)),
        'perm_ttl_sec': number(r'ttl:\s*(\d+)\s*\* time.Second', source(repo, 'internal/services/permission_resolver.go')),
        'shift_hour': number(r'passReportBoundaryHour\s*=\s*(\d+)', report),
        'shift_minute': number(r'passReportBoundaryMinute\s*=\s*(\d+)', report),
        'code_files': len(code), 'code_lines': lines(code),
        'be_lines': lines([p for p in go if p not in go_tests]),
        'vue_lines': lines([p for p in code if p.is_relative_to(fe) and p.suffix == '.vue']),
        'views': sum(p.is_relative_to(fe / 'views') and p.suffix == '.vue' for p in code),
        'components': sum(p.is_relative_to(fe / 'components') and p.suffix == '.vue' for p in code),
    }
    for name in ('services', 'handlers', 'models'):
        m[name] = sum(p.parent == root / 'internal' / name and not p.name.endswith('_test.go') for p in go)
    m['tests_total'] = m['go_tests'] + m['vitest'] + m['e2e']
    return m
