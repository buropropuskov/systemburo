#!/usr/bin/env python3
"""Проверка уязвимостей Go с протухающими исключениями.

govulncheck сам ничего исключать не умеет: он либо падает на находке, либо нет.
Уязвимость без выпущенного исправления держала бы проверку красной неделями, и
красный чек переставали бы читать - ровно то, чего проверка должна не допускать.
Trivy в соседнем шаге по той же причине запущен с ignore-unfixed.

Исключение здесь - не выключатель, а отсрочка с двумя предохранителями: оно живёт
до даты пересмотра, и оно перестаёт работать, как только у уязвимости появляется
исправленная версия. В обоих случаях проверка падает и требует вернуться к записи.

Запуск: python3 scripts/govulncheck_gate.py [путь-до-файла-исключений]
"""

import datetime as dt
import json
import pathlib
import subprocess
import sys

REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent
DEFAULT_EXCEPTIONS = REPO_ROOT / "scripts" / "govulncheck-exceptions.json"


def read_exceptions(path: pathlib.Path) -> dict[str, dict]:
    if not path.is_file():
        return {}
    data = json.loads(path.read_text(encoding="utf-8"))
    return {item["id"]: item for item in data.get("exceptions", [])}


def run_govulncheck() -> list[dict]:
    """Поток JSON-объектов от govulncheck. Ненулевой код - это найденные
    уязвимости, а не сбой запуска, поэтому он сам по себе не ошибка."""
    proc = subprocess.run(
        ["govulncheck", "-format", "json", "./..."],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
    )
    if not proc.stdout.strip():
        print(proc.stderr.strip() or "govulncheck не вернул вывода", file=sys.stderr)
        sys.exit(2)

    messages, decoder, raw = [], json.JSONDecoder(), proc.stdout.strip()
    pos = 0
    while pos < len(raw):
        obj, end = decoder.raw_decode(raw, pos)
        messages.append(obj)
        pos = end
        while pos < len(raw) and raw[pos] in " \n\r\t":
            pos += 1
    return messages


def collect(messages: list[dict]) -> dict[str, dict]:
    """Уязвимости, до которых дотягивается наш код, с модулем и версией, где они
    исправлены.

    Считаются только находки, где в трассе есть вызываемые функции. Находка на
    уровне модуля означает лишь, что уязвимый пакет лежит в графе зависимостей, а
    его уязвимые места никто не зовёт; сам govulncheck такие держит отдельно и
    сборку ими не валит - иначе проверка краснела бы от чужого кода, который у нас
    не исполняется."""
    osvs = {m["osv"]["id"]: m["osv"] for m in messages if "osv" in m}
    found: dict[str, dict] = {}
    for message in messages:
        finding = message.get("finding")
        if not finding:
            continue
        trace = finding.get("trace") or []
        if not any(step.get("function") for step in trace):
            continue
        osv_id = finding["osv"]
        entry = found.setdefault(
            osv_id,
            {
                "id": osv_id,
                "module": trace[0].get("module", ""),
                "fixed": finding.get("fixed_version", ""),
                "summary": osvs.get(osv_id, {}).get("summary", ""),
            },
        )
        if not entry["fixed"]:
            entry["fixed"] = finding.get("fixed_version", "")
    return found


def main() -> int:
    path = pathlib.Path(sys.argv[1]) if len(sys.argv) > 1 else DEFAULT_EXCEPTIONS
    exceptions = read_exceptions(path)
    found = collect(run_govulncheck())
    today = dt.date.today()

    blocking, excused = [], []
    for osv_id, vuln in sorted(found.items()):
        rule = exceptions.get(osv_id)
        if rule is None:
            blocking.append((vuln, "исключения нет"))
            continue
        if vuln["fixed"]:
            blocking.append((vuln, f"вышло исправление {vuln['fixed']} - обновитесь и уберите запись"))
            continue
        until = dt.date.fromisoformat(rule["until"])
        if until < today:
            blocking.append((vuln, f"срок записи истёк {rule['until']}"))
            continue
        excused.append((vuln, rule, until))

    for vuln, rule, until in excused:
        print(f"отложено до {rule['until']} ({(until - today).days} дн.): {vuln['id']} {vuln['module']}")
        print(f"  {vuln['summary']}")
        print(f"  {rule['why']}")

    stale = sorted(set(exceptions) - set(found))
    for osv_id in stale:
        print(f"запись про {osv_id} больше ничего не отсрочивает: уязвимость не найдена, уберите её из перечня")

    for vuln, reason in blocking:
        print(f"уязвимость {vuln['id']} в {vuln['module']}: {reason}")
        print(f"  {vuln['summary']}")

    if blocking or stale:
        return 1
    print(f"уязвимостей, требующих действий, нет (отложено записей: {len(excused)})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
