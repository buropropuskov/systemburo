# -*- coding: utf-8 -*-
"""
Размножает одну строку экспорта Бастион-2 в пачку заявок.

Фамилия, фото, организация, подразделение, должность, уровень доступа
берутся из эталона без изменений. Меняются: имя, табельный номер и
корпоративный код персоны (он собирается из ФИО и обязан быть разным).

Скрипт кладётся в ту же папку, куда Бюро пропусков выгрузило CSV.
"""

import csv
import re
import shutil
import sys
from pathlib import Path

# ------------------------- НАСТРОЙКИ -------------------------

# Файл, который выгрузил Бастион.
ISHODNYJ_FAJL = "etalon.csv"

# Файл, который получится. Его и будем импортировать обратно.
GOTOVYJ_FAJL = "batch.csv"

# Сколько заявок сгенерировать. Первый прогон делай на 3 - на пробу.
SKOLKO = 10

# Шаблон имени. {n} - порядковый номер: "№ 2", "№ 3", ...
SHABLON_IMENI = "№ {n}"

# True - скрипт сам смотрит, что уже лежит в выгрузке, и продолжает с
# максимального номера и максимального табельного. Тогда выгружай из Бастиона
# ВСЕ уже созданные заявки, а не одну: где остановилась нумерация, видно
# только по ним. Две настройки ниже при этом не используются.
AVTO_PRODOLZHIT = True

# С какого номера начинать, если AVTO_PRODOLZHIT = False.
NACHINAT_S = 2

# Табельный номер ПЕРВОЙ генерируемой строки, если AVTO_PRODOLZHIT = False.
# Пусто - берётся из первой строки выгрузки и сдвигается на NACHINAT_S.
PERVYJ_TABELNYJ = ""

# -------------------- дальше менять не нужно --------------------

KODIROVKI = ("cp1251", "utf-8-sig", "utf-8")

# Пути считаем от папки, где лежит сам скрипт, а не от текущего каталога
# консоли - иначе при запуске двойным щелчком файл "не находится".
PAPKA = Path(__file__).resolve().parent


def prochitat(path):
    """Возвращает (заголовки, строку русских подписей или None, все записи, кодировку, разделитель).

    Экспорт Бастиона содержит ДВЕ строки заголовка: технические имена полей
    и русские подписи к ним. Данные начинаются с третьей строки.
    """
    for kod in KODIROVKI:
        try:
            text = path.read_text(encoding=kod)
        except UnicodeDecodeError:
            continue
        try:
            razdelitel = csv.Sniffer().sniff(text[:4096], delimiters=";,\t").delimiter
        except csv.Error:
            razdelitel = ";"
        vse = [r for r in csv.reader(text.splitlines(), delimiter=razdelitel) if r]
        if len(vse) < 2:
            sys.exit("В файле нет строк данных. Выгрузи из Бастиона хотя бы одну заявку.")
        zagolovki = vse[0]
        podpisi = None
        nomer_dannyh = 1
        if len(vse) > 2 and vse[1][0].strip().lower() == "фамилия":
            podpisi = vse[1]
            nomer_dannyh = 2
        zapisi = [dict(zip(zagolovki, r)) for r in vse[nomer_dannyh:]]
        return zagolovki, podpisi, zapisi, kod, razdelitel
    sys.exit("Не смог прочитать файл ни в одной из кодировок: %s" % ", ".join(KODIROVKI))


def hvostovoe_chislo(znachenie):
    """Число в конце строки: "0000022276" -> 22276, "№ 25" -> 25, мусор -> None."""
    sovpad = re.search(r"(\d+)\s*$", znachenie or "")
    return int(sovpad.group(1)) if sovpad else None


def najti_kolonku(zagolovki, *varianty):
    nizhnie = dict((z.strip().lower(), z) for z in zagolovki)
    for v in varianty:
        if v.lower() in nizhnie:
            return nizhnie[v.lower()]
    return None


def sleduyushchij_tabelnyj(znachenie, smeshchenie):
    """Наращивает цифровой хвост, сохраняя префикс и ведущие нули."""
    sovpad = re.search(r"(\d+)\s*$", znachenie or "")
    if not sovpad:
        return "%s%d" % (znachenie, smeshchenie) if znachenie else str(smeshchenie)
    cifry = sovpad.group(1)
    novye = str(int(cifry) + smeshchenie).rjust(len(cifry), "0")
    return znachenie[:sovpad.start(1)] + novye


def skopirovat_papku_foto():
    """Экспорт кладёт фото в подкаталог по имени файла, импорт там же его и ищет.

    Файл импорта называется иначе, значит нужен второй подкаталог с новым именем.
    """
    staroe = PAPKA / (Path(ISHODNYJ_FAJL).stem + ".CSV_PHOTO")
    novoe = PAPKA / (Path(GOTOVYJ_FAJL).stem + ".CSV_PHOTO")
    if not staroe.is_dir():
        print("ВНИМАНИЕ: не вижу папку %s - фото может не подтянуться." % staroe.name)
        return None
    if novoe.exists():
        shutil.rmtree(novoe)
    shutil.copytree(staroe, novoe)
    return novoe


def main():
    istochnik = PAPKA / ISHODNYJ_FAJL
    if not istochnik.exists():
        sys.exit("Не вижу файл %s рядом со скриптом." % ISHODNYJ_FAJL)

    zagolovki, podpisi, zapisi, kodirovka, razdelitel = prochitat(istochnik)
    etalon = zapisi[0]

    k_familiya = najti_kolonku(zagolovki, "NAME", "PNAME", "Фамилия")
    k_imya = najti_kolonku(zagolovki, "FIRSTNAME", "Имя")
    k_tabelnyj = najti_kolonku(zagolovki, "TABLENO", "Табельный номер")
    k_korp = najti_kolonku(zagolovki, "CORP_CODE", "Корп. код")
    k_korp_propuska = najti_kolonku(zagolovki, "PASSCC")
    k_foto = najti_kolonku(zagolovki, "PHOTO", "Фото")

    if not k_imya:
        sys.exit("В экспорте нет колонки имени (FIRSTNAME). Колонки: %s" % ", ".join(zagolovki))

    imya_etalona = etalon.get(k_imya, "")
    korp_etalona = etalon.get(k_korp, "") if k_korp else ""

    if AVTO_PRODOLZHIT:
        # Продолжаем с того места, где остановилась выгрузка: максимальный
        # номер в имени и максимальный табельный берём из самих записей.
        nomera = [hvostovoe_chislo(z.get(k_imya)) for z in zapisi]
        nomera = [n for n in nomera if n is not None]
        if not nomera:
            sys.exit("Не смог разобрать номера в именах - в колонке %s нет цифр." % k_imya)
        nachinat_s = max(nomera) + 1

        tabelnye = [(hvostovoe_chislo(z.get(k_tabelnyj)), z.get(k_tabelnyj)) for z in zapisi]
        tabelnye = [t for t in tabelnye if t[0] is not None]
        if not tabelnye:
            sys.exit("Не смог разобрать табельные - в колонке %s нет цифр." % k_tabelnyj)
        tabelnyj_baza, pervoe_smeshchenie = max(tabelnye)[1], 1
        print("Прочитано записей: %d, последняя - № %d, табельный %s"
              % (len(zapisi), max(nomera), tabelnyj_baza))
    else:
        nachinat_s = NACHINAT_S
        # Явно заданный табельный - это номер первой строки, смещение нулевое.
        # Взятый из выгрузки относится к позиции 1, поэтому двигаем на NACHINAT_S.
        if PERVYJ_TABELNYJ:
            tabelnyj_baza, pervoe_smeshchenie = PERVYJ_TABELNYJ, 0
        else:
            tabelnyj_baza, pervoe_smeshchenie = etalon.get(k_tabelnyj, ""), NACHINAT_S - 1

    if k_korp and korp_etalona and imya_etalona not in korp_etalona:
        print("ВНИМАНИЕ: имя %r не нашлось в корп. коде %r - код останется прежним "
              "у всех строк, и импорт может слить их в одну персону."
              % (imya_etalona, korp_etalona))

    stroki = []
    for poz, nomer in enumerate(range(nachinat_s, nachinat_s + SKOLKO)):
        imya = SHABLON_IMENI.format(n=nomer)
        if len(imya) > 20:
            sys.exit("Имя длиннее 20 символов, Бастион обрежет: %s" % imya)
        stroka = dict(etalon)
        stroka[k_imya] = imya
        if k_tabelnyj:
            stroka[k_tabelnyj] = sleduyushchij_tabelnyj(tabelnyj_baza, pervoe_smeshchenie + poz)
        if k_korp and korp_etalona and imya_etalona:
            # Корп. код персоны собирается из ФИО и должен быть уникальным:
            # мастер импорта сопоставляет строки с базой именно по нему.
            stroka[k_korp] = korp_etalona.replace(imya_etalona, imya)
        if k_korp_propuska:
            # Корп. код пропуска у эталона свой; продублировать его на сто
            # записей нельзя, пусть Бастион сгенерирует новые.
            stroka[k_korp_propuska] = ""
        stroki.append(stroka)

    with (PAPKA / GOTOVYJ_FAJL).open("w", encoding=kodirovka, newline="") as fh:
        pisatel = csv.writer(fh, delimiter=razdelitel, quoting=csv.QUOTE_ALL)
        pisatel.writerow(zagolovki)
        if podpisi:
            pisatel.writerow(podpisi)
        for stroka in stroki:
            pisatel.writerow([stroka.get(k, "") for k in zagolovki])

    papka_foto = skopirovat_papku_foto()

    print("Готово: %d строк -> %s" % (len(stroki), GOTOVYJ_FAJL))
    print("Кодировка %s, разделитель '%s', строк заголовка %d"
          % (kodirovka, razdelitel, 2 if podpisi else 1))
    print("Фамилия у всех: %s" % (etalon.get(k_familiya) if k_familiya else "колонки нет"))
    print("Имена:          %s .. %s" % (stroki[0][k_imya], stroki[-1][k_imya]))
    if k_tabelnyj:
        print("Табельные:      %s .. %s" % (stroki[0][k_tabelnyj], stroki[-1][k_tabelnyj]))
    if k_korp:
        print("Корп. коды:     %r .. %r" % (stroki[0][k_korp], stroki[-1][k_korp]))
    if k_foto:
        print("Фото на всех:   %s" % (etalon.get(k_foto) or "(пусто)"))
    if papka_foto:
        print("Папка с фото:   %s" % papka_foto.name)


main()
