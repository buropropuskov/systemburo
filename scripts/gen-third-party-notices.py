#!/usr/bin/env python3
"""Сборка перечня сторонних компонентов и условий их использования.

Результат - файл THIRD-PARTY-NOTICES.md в корне репозитория. Он передаётся
заказчику вместе с системой: разрешительные лицензии (MIT, Apache-2.0, BSD,
ISC) разрешают использовать чужой код при одном условии - сохранить в поставке
уведомление об авторских правах и текст самой лицензии. Без такого файла
условие не выполнено, и право пользоваться библиотекой формально утрачено.

Перечень собирается из репозитория, а не пишется руками: список зависимостей
меняется каждым обновлением, и переписанный по памяти файл разошёлся бы с
поставкой на первом же добавлении библиотеки.

Что берётся за состав поставки:

* интерфейс - производственное замыкание по frontend/package-lock.json, то есть
  всё, до чего дотягиваются dependencies. Пакеты, помеченные в файле замка как
  dev, отброшены: они участвуют только в сборке и в поставку не попадают;
* серверная часть - модули, которые компоновщик Go реально втягивает в двоичные
  файлы (go list -deps по ./cmd/...). Брать require из go.mod было бы неверно:
  там же лежат модули, нужные только тестам и генератору описания интерфейса.

Тексты лицензий не переводятся и не пересказываются - юридическую силу имеет
подлинник. Одинаковые тексты сводятся в один блок со списком компонентов:
у полутора сотен пакетов MIT совпадает дословно, различаясь только строкой
правообладателя, и полтораста копий одного абзаца читать невозможно.

Запуск (из корня репозитория, нужны установленный frontend/node_modules и
доступный кэш модулей Go):

    python3 scripts/gen-third-party-notices.py
    python3 scripts/gen-third-party-notices.py --check   # только проверить

Проверка ничего не пишет и возвращает ненулевой код, если файл в репозитории
разошёлся с состоянием зависимостей.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from collections import OrderedDict
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
NOTICES_PATH = REPO_ROOT / "THIRD-PARTY-NOTICES.md"

# Имена, под которыми пакеты кладут текст лицензии. Порядок значим: сначала
# точные имена, потом варианты написания. COPYING - традиция проектов GNU,
# LICENCE - британское написание, встречается у отдельных пакетов npm.
LICENSE_FILENAMES = (
    "LICENSE",
    "LICENSE.md",
    "LICENSE.txt",
    "LICENSE-MIT",
    "LICENSE-MIT.txt",
    "MIT-LICENSE",
    "MIT-LICENSE.txt",
    "LICENCE",
    "LICENCE.md",
    "LICENCE.txt",
    "COPYING",
    "COPYING.md",
    "COPYING.txt",
    "LICENSE.BSD",
    "LICENSE.APACHE2",
)

# Строка об авторских правах, а не всякая строка со словом copyright. Без
# требования года и знака в перечень попадали обрывки самого текста лицензии:
# перенос абзаца Apache-2.0 начинается словом "copyright notice that is included
# in", а в отказе от гарантий встречается "COPYRIGHT HOLDERS BE LIABLE" - оба
# выглядели правообладателем и печатались вместо него.
COPYRIGHT_RE = re.compile(
    r"^\s*(?:\(c\)|©)?\s*(?:Copyright|COPYRIGHT)\b(?=.*(?:\(c\)|©|\b(?:19|20)\d{2}\b))"
)

# Условие 4(d) лицензии Apache-2.0: если у компонента есть файл NOTICE, его
# содержимое обязано сопровождать поставку - названия лицензии тут мало.
NOTICE_FILENAMES = ("NOTICE", "NOTICE.txt", "NOTICE.md")

# Лицензии, не требующие ничего, кроме сохранения уведомления. Всё, что не
# попало сюда, выносится в отдельный раздел файла: такие условия читает человек,
# а не сверяет сценарий.
PERMISSIVE = {
    "MIT",
    "MIT-0",
    "ISC",
    "0BSD",
    "BSD-2-Clause",
    "BSD-3-Clause",
    "Apache-2.0",
    "BlueOak-1.0.0",
    "Unlicense",
    "CC0-1.0",
    "Python-2.0",
    "Zlib",
}


def read_text(path: Path) -> str:
    """Читает файл, не спотыкаясь о единичные битые байты в чужих лицензиях."""
    return path.read_text(encoding="utf-8", errors="replace")


def find_license_file(pkg_dir: Path) -> tuple[str, str] | None:
    """Возвращает (имя файла, текст) первой найденной лицензии пакета."""
    if not pkg_dir.is_dir():
        return None
    for name in LICENSE_FILENAMES:
        candidate = pkg_dir / name
        if candidate.is_file():
            return name, read_text(candidate).strip()
    # Регистр имени у части пакетов отличается (license, License.md). Перебор
    # каталога дороже прямой проверки, поэтому идёт вторым заходом.
    for entry in sorted(pkg_dir.iterdir()):
        if entry.is_file() and entry.name.lower().split(".")[0] in ("license", "licence", "copying"):
            return entry.name, read_text(entry).strip()
    return None


def find_notice_file(pkg_dir: Path) -> str:
    """Возвращает текст файла NOTICE компонента, если тот его приложил."""
    if not pkg_dir.is_dir():
        return ""
    for name in NOTICE_FILENAMES:
        candidate = pkg_dir / name
        if candidate.is_file():
            return read_text(candidate).strip()
    return ""


def extract_copyright(text: str, fallback: str = "") -> str:
    """Вытаскивает строки об авторских правах из текста лицензии.

    Именно они, а не название лицензии, и есть то, что требуется сохранить.
    """
    lines = []
    for line in text.splitlines():
        if COPYRIGHT_RE.match(line):
            cleaned = line.strip().rstrip(".")
            # Шаблонная строка из неподставленного текста Apache-2.0 -
            # правообладателя не называет, брать её нельзя.
            if "[yyyy] [name of copyright owner]" in cleaned:
                continue
            # Отказ от гарантий набран прописными целиком; строка оттуда
            # правообладателем не является.
            letters = [ch for ch in cleaned if ch.isalpha()]
            if letters and sum(ch.isupper() for ch in letters) / len(letters) > 0.8:
                continue
            if cleaned not in lines:
                lines.append(cleaned)
        if len(lines) >= 3:
            break
    if lines:
        return "; ".join(lines)
    return fallback


def author_of(meta: dict) -> str:
    author = meta.get("author")
    if isinstance(author, dict):
        return author.get("name", "")
    if isinstance(author, str):
        # "Имя <почта> (сайт)" - почта и сайт в уведомлении лишние.
        return re.sub(r"\s*[<(].*", "", author).strip()
    return ""


def spdx_of(meta: dict) -> str:
    """Приводит поле лицензии к одному виду: в npm оно бывает трёх форм."""
    lic = meta.get("license")
    if isinstance(lic, dict):
        return lic.get("type", "")
    if isinstance(lic, str):
        return lic
    licenses = meta.get("licenses")
    if isinstance(licenses, list) and licenses:
        return " OR ".join(
            item.get("type", "") if isinstance(item, dict) else str(item) for item in licenses
        )
    return ""


class Component:
    """Один сторонний компонент в поставке."""

    def __init__(
        self,
        name: str,
        version: str,
        spdx: str,
        holder: str,
        license_text: str,
        notice_text: str = "",
    ):
        self.name = name
        self.version = version
        self.spdx = spdx or "не указана"
        self.holder = holder or "не указан в поставке пакета"
        self.license_text = license_text
        self.notice_text = notice_text

    @property
    def title(self) -> str:
        return f"{self.name} {self.version}"


def collect_npm() -> list[Component]:
    lock_path = REPO_ROOT / "frontend" / "package-lock.json"
    lock = json.loads(read_text(lock_path))
    packages = lock.get("packages", {})

    components = []
    for key, entry in packages.items():
        # Пустой ключ - сам проект, dev - только сборка.
        if not key or entry.get("dev"):
            continue
        pkg_dir = REPO_ROOT / "frontend" / key
        meta_path = pkg_dir / "package.json"
        if not meta_path.is_file():
            # Сборки под чужую платформу (`@napi-rs/canvas-android-arm64` и
            # подобные) значатся в замке, но не устанавливаются и в поставку не
            # попадают: система работает в контейнере Linux. Отсутствие всего
            # остального означает, что зависимости просто не установлены.
            if entry.get("optional") and (entry.get("os") or entry.get("cpu")):
                continue
            raise SystemExit(
                f"нет каталога {pkg_dir.relative_to(REPO_ROOT)}: выполните npm ci в frontend/ "
                "и повторите - перечень собирается по установленным пакетам, а не по одному замку"
            )
        meta = json.loads(read_text(meta_path))

        found = find_license_file(pkg_dir)
        license_text = found[1] if found else ""
        # Пакет вправе положить текст лицензии, не назвав её: у `png-js` поля
        # license нет ни в замке, ни в описании, а MIT лежит файлом рядом.
        spdx = entry.get("license") or spdx_of(meta) or guess_spdx(license_text)
        holder = extract_copyright(license_text, author_of(meta))
        name = key.split("node_modules/")[-1]
        components.append(
            Component(
                name,
                entry.get("version", ""),
                spdx,
                holder,
                license_text,
                find_notice_file(pkg_dir),
            )
        )

    # Один и тот же пакет установлен по нескольким путям, когда соседям нужны
    # разные версии. Условия при этом одни, и повторять строку незачем; две
    # разные версии одного пакета остаются двумя строками - в поставку уходят
    # обе.
    unique = {}
    for component in components:
        unique.setdefault((component.name, component.version), component)

    return sorted(unique.values(), key=lambda c: (c.name.lower(), c.version))


def escape_module_path(path: str) -> str:
    """Кодирование пути модуля для кэша Go: заглавная буква -> !строчная."""
    return re.sub(r"[A-Z]", lambda m: "!" + m.group(0).lower(), path)


def collect_go() -> list[Component]:
    env = dict(os.environ)
    # go.mod требует более свежий тулчейн, чем бывает установлен локально;
    # без этого go list отказывается работать вместо того, чтобы его подтянуть.
    env.setdefault("GOTOOLCHAIN", "auto")

    def run_go(*args: str) -> str:
        result = subprocess.run(
            ["go", *args], cwd=REPO_ROOT, env=env, capture_output=True, text=True
        )
        if result.returncode != 0:
            raise SystemExit(f"go {' '.join(args)} завершилась ошибкой:\n{result.stderr.strip()}")
        return result.stdout

    mod_cache = Path(run_go("env", "GOMODCACHE").strip())
    listed = run_go(
        "list",
        "-deps",
        "-f",
        "{{if .Module}}{{.Module.Path}}\t{{.Module.Version}}{{end}}",
        "./cmd/...",
    )

    seen = OrderedDict()
    for line in listed.splitlines():
        if not line.strip():
            continue
        path, _, version = line.partition("\t")
        # Сам проект и модули стандартной библиотеки в перечень не идут.
        if path == "systemburo" or not version:
            continue
        seen[(path, version)] = None

    components = []
    for path, version in seen:
        pkg_dir = mod_cache / f"{escape_module_path(path)}@{version}"
        found = find_license_file(pkg_dir)
        license_text = found[1] if found else ""
        holder = extract_copyright(license_text)
        spdx = guess_spdx(license_text)
        components.append(
            Component(path, version, spdx, holder, license_text, find_notice_file(pkg_dir))
        )

    components.sort(key=lambda c: c.name.lower())
    return components


# Модули Go не объявляют лицензию машиночитаемо - в отличие от npm, поля для неё
# в go.mod нет. Определяем по подписи самого текста; неопознанное честно
# помечается как требующее чтения человеком, а не подставляется наугад.
SPDX_SIGNATURES = (
    ("Apache-2.0", "Apache License"),
    ("MPL-2.0", "Mozilla Public License Version 2.0"),
    ("ISC", "Permission to use, copy, modify, and/or distribute this software"),
    ("BSD-3-Clause", "Neither the name of"),
    ("BSD-2-Clause", "Redistributions in binary form must reproduce"),
    ("MIT", "Permission is hereby granted, free of charge"),
)


def guess_spdx(text: str) -> str:
    for spdx, signature in SPDX_SIGNATURES:
        if signature in text:
            return spdx
    return ""


def license_groups(components: list[Component]) -> list[tuple[str, list[Component]]]:
    """Сводит компоненты с дословно совпадающим текстом лицензии в один блок."""
    groups: OrderedDict[str, list[Component]] = OrderedDict()
    for component in components:
        if not component.license_text:
            continue
        groups.setdefault(component.license_text, []).append(component)
    ordered = sorted(groups.items(), key=lambda item: (-len(item[1]), item[1][0].name.lower()))
    return ordered


def md_escape(value: str) -> str:
    """Экранирует то, что порвало бы разметку таблицы."""
    return value.replace("|", "\\|").replace("\n", " ")


def render_table(components: list[Component]) -> list[str]:
    lines = [
        "| Компонент | Версия | Лицензия | Правообладатель |",
        "|---|---|---|---|",
    ]
    for component in components:
        lines.append(
            f"| `{md_escape(component.name)}` | {md_escape(component.version)} | "
            f"{md_escape(component.spdx)} | {md_escape(component.holder)} |"
        )
    return lines


def render(npm: list[Component], go: list[Component]) -> str:
    out: list[str] = []
    add = out.append

    add("# Сторонние компоненты и условия их использования")
    add("")
    add(
        "Система собрана с использованием библиотек с открытым исходным кодом. Их лицензии "
        "разрешают применение и распространение при одном условии: уведомление об авторских "
        "правах и текст лицензии должны сохраняться в поставке. Настоящий файл это условие "
        "и выполняет - он входит в состав поставки и передаётся вместе с системой."
    )
    add("")
    add(
        "Файл собран из репозитория сценарием `scripts/gen-third-party-notices.py` и правится "
        "только им же. Состав интерфейса определён по производственному замыканию зависимостей "
        "в `frontend/package-lock.json`, состав серверной части - по модулям, которые "
        "компоновщик втягивает в двоичные файлы. Замыкание берётся целиком, включая то, что "
        "сборщик мог отбросить при сборке: перечислить лишнее безопаснее, чем пропустить нужное."
    )
    add("")
    add("Тексты лицензий приведены на языке подлинника: юридическую силу имеет он, а не пересказ.")
    add("")

    nonpermissive = [c for c in npm + go if c.spdx.split(" OR ")[0].strip("() ") not in PERMISSIVE]

    add("## 1. Сводка")
    add("")
    add("| Показатель | Значение |")
    add("|---|---|")
    add(f"| Компонентов интерфейса | {len(npm)} |")
    add(f"| Компонентов серверной части | {len(go)} |")
    add(f"| Различных текстов лицензий | {len(license_groups(npm + go))} |")
    add("")

    if nonpermissive:
        add("## 2. Условия, требующие отдельного внимания")
        add("")
        add(
            "Перечисленное ниже не относится к разрешительным лицензиям вида MIT или Apache-2.0, "
            "по которым достаточно сохранить уведомление. Условия таких компонентов читаются "
            "отдельно."
        )
        add("")
        for line in render_table(nonpermissive):
            add(line)
        add("")

    section = 3 if nonpermissive else 2
    add(f"## {section}. Компоненты интерфейса")
    add("")
    for line in render_table(npm):
        add(line)
    add("")

    section += 1
    add(f"## {section}. Компоненты серверной части")
    add("")
    for line in render_table(go):
        add(line)
    add("")

    section += 1
    add(f"## {section}. Шрифты и графика")
    add("")
    add(
        "Шрифт `DejaVuSans.ttf` применяется при формировании файлов PDF на сервере. Его условия "
        "(Bitstream Vera Fonts Copyright и Arev Fonts Copyright) требуют, чтобы лицензия "
        "сопровождала файл шрифта, и лежат рядом с ним: `internal/export/fonts/LICENSE`."
    )
    add("")
    add(
        "Шрифты Montserrat и PT Sans (лицензия SIL Open Font License) в поставку не входят: "
        "интерфейс подключает их с внешней площадки, файлы шрифтов система не распространяет. "
        "Условие о сопровождении файла лицензией здесь не возникает."
    )
    add("")
    add(
        "Значки интерфейса и графика экрана входа нарисованы разработчиком системы, права на них "
        "принадлежат правообладателю системы, и в настоящий перечень они не входят."
    )
    add("")

    section += 1
    add(f"## {section}. Тексты лицензий")
    add("")
    add(
        "Тексты приведены дословно. Один блок - один текст: компоненты, лицензии которых "
        "совпадают буква в букву, перечислены общим списком перед текстом."
    )
    add("")

    groups = license_groups(npm + go)
    for index, (text, members) in enumerate(groups, start=1):
        spdx = members[0].spdx
        add(f"### {section}.{index}. {spdx}")
        add("")
        add("Компоненты: " + ", ".join(f"`{m.name}` {m.version}" for m in members))
        add("")
        add("```text")
        out.extend(text.splitlines())
        add("```")
        add("")

    with_notice = [c for c in npm + go if c.notice_text]
    if with_notice:
        section += 1
        add(f"## {section}. Уведомления компонентов")
        add("")
        add(
            "Лицензия Apache-2.0 требует передавать вместе с поставкой содержимое файла NOTICE "
            "тех компонентов, у которых он есть. Ниже приведены такие файлы дословно."
        )
        add("")
        for component in with_notice:
            add(f"### {component.title}")
            add("")
            add("```text")
            out.extend(component.notice_text.splitlines())
            add("```")
            add("")

    missing = [c for c in npm + go if not c.license_text]
    if missing:
        section += 1
        add(f"## {section}. Компоненты без вложенного текста лицензии")
        add("")
        add(
            "Перечисленные ниже пакеты названия лицензии не сопровождают её текстом. Лицензия "
            "указана по объявлению самого пакета; текст берётся из общедоступного описания "
            "соответствующей лицензии."
        )
        add("")
        for line in render_table(missing):
            add(line)
        add("")

    return "\n".join(out).rstrip() + "\n"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--check",
        action="store_true",
        help="не писать файл, а сверить его с текущим составом зависимостей",
    )
    args = parser.parse_args()

    rendered = render(collect_npm(), collect_go())

    if args.check:
        if not NOTICES_PATH.is_file():
            print("THIRD-PARTY-NOTICES.md отсутствует", file=sys.stderr)
            return 1
        if read_text(NOTICES_PATH) != rendered:
            print(
                "THIRD-PARTY-NOTICES.md разошёлся с составом зависимостей: "
                "выполните python3 scripts/gen-third-party-notices.py",
                file=sys.stderr,
            )
            return 1
        print("THIRD-PARTY-NOTICES.md соответствует составу зависимостей")
        return 0

    NOTICES_PATH.write_text(rendered, encoding="utf-8")
    print(f"записан {NOTICES_PATH.relative_to(REPO_ROOT)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
