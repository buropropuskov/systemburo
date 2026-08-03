# -*- coding: utf-8 -*-
"""
Бастион-2: пакетное создание заявок из одной выгрузки.

Читает CSV, выгруженный из АРМ "Бюро пропусков", размножает последнюю запись
в пачку новых заявок и пишет файл, готовый к обратному импорту.

Меняются имя, табельный номер и корпоративный код персоны. Фамилия, фото,
организация, подразделение, должность, уровень доступа копируются как есть.
"""

import csv
import os
import re
import shutil
import sys
import tkinter as tk
from tkinter import filedialog, messagebox, ttk
from pathlib import Path

PRILOZHENIE = "Бастион-2 — пакетное создание заявок"
VERSIYA = "1.0"

KODIROVKI = ("cp1251", "utf-8-sig", "utf-8")
NERAZRYVNYJ_PROBEL = " "  # то же, что Alt+255
PREDEL_FIO = 20  # ограничение Бастиона на длину полей Ф.И.О.

CVETA = {
    "fon": "#f4f5f7",
    "panel": "#ffffff",
    "tekst": "#1f2328",
    "tusklyj": "#6b7280",
    "akcent": "#2563eb",
    "akcent_navedenie": "#1d4ed8",
    "ramka": "#d8dbdf",
    "uspeh": "#15803d",
    "oshibka": "#b91c1c",
}


# --------------------------------------------------------------------------
# Разбор и генерация. Ничего не знают про интерфейс, поэтому проверяемы отдельно.
# --------------------------------------------------------------------------

class OshibkaDannyh(Exception):
    """Понятная пользователю причина, по которой файл не подходит."""


def prochitat_vygruzku(put):
    """Разбирает выгрузку Бастиона.

    Экспорт содержит ДВЕ строки заголовка: технические имена полей и русские
    подписи к ним. Данные начинаются с третьей строки.
    """
    put = Path(put)
    if not put.exists():
        raise OshibkaDannyh("Файл не найден: %s" % put)

    for kodirovka in KODIROVKI:
        try:
            text = put.read_text(encoding=kodirovka)
        except UnicodeDecodeError:
            continue
        try:
            razdelitel = csv.Sniffer().sniff(text[:4096], delimiters=";,\t").delimiter
        except csv.Error:
            razdelitel = ";"
        vse = [r for r in csv.reader(text.splitlines(), delimiter=razdelitel) if r]
        if len(vse) < 2:
            raise OshibkaDannyh("В файле нет строк данных — выгрузи хотя бы одну заявку.")
        zagolovki = vse[0]
        podpisi = None
        pervaya = 1
        if len(vse) > 2 and vse[1][0].strip().lower() == "фамилия":
            podpisi = vse[1]
            pervaya = 2
        zapisi = [dict(zip(zagolovki, r)) for r in vse[pervaya:]]
        if not zapisi:
            raise OshibkaDannyh("После заголовков нет ни одной записи.")
        return {
            "zagolovki": zagolovki,
            "podpisi": podpisi,
            "zapisi": zapisi,
            "kodirovka": kodirovka,
            "razdelitel": razdelitel,
            "put": put,
        }
    raise OshibkaDannyh("Не удалось прочитать файл ни в одной из кодировок: %s"
                        % ", ".join(KODIROVKI))


def najti_kolonku(zagolovki, *varianty):
    nizhnie = dict((z.strip().lower(), z) for z in zagolovki)
    for v in varianty:
        if v.lower() in nizhnie:
            return nizhnie[v.lower()]
    return None


def kolonki_vygruzki(zagolovki):
    return {
        "familiya": najti_kolonku(zagolovki, "NAME", "PNAME", "Фамилия"),
        "imya": najti_kolonku(zagolovki, "FIRSTNAME", "Имя"),
        "otchestvo": najti_kolonku(zagolovki, "SECONDNAME", "Отчество"),
        "tabelnyj": najti_kolonku(zagolovki, "TABLENO", "Табельный номер"),
        "korp": najti_kolonku(zagolovki, "CORP_CODE", "Корп. код"),
        "korp_propuska": najti_kolonku(zagolovki, "PASSCC"),
        "foto": najti_kolonku(zagolovki, "PHOTO", "Фото"),
        # Организация в выгрузке разложена по восьми уровням иерархии, причём
        # верхние обычно пустые. Колонок несколько, поэтому список, а не одна.
        "organizaciya": [najti_kolonku(zagolovki, "WORG%d" % i) for i in range(8, 0, -1)]
                        + [najti_kolonku(zagolovki, "WORKPLACEORG", "ORGANIZATION")],
    }


def znachenie_organizacii(zapis, kolonki):
    """Самый нижний заполненный уровень иерархии организаций."""
    for kolonka in kolonki["organizaciya"]:
        if kolonka and (zapis.get(kolonka) or "").strip():
            return zapis[kolonka].strip()
    return ""


def hvostovoe_chislo(znachenie):
    """Число в конце строки: "0000022276" -> 22276, "№ 25" -> 25, иначе None."""
    sovpad = re.search(r"(\d+)\s*$", znachenie or "")
    return int(sovpad.group(1)) if sovpad else None


def sdvinut_tabelnyj(znachenie, smeshchenie):
    """Наращивает цифровой хвост, сохраняя префикс и ведущие нули."""
    sovpad = re.search(r"(\d+)\s*$", znachenie or "")
    if not sovpad:
        return "%s%d" % (znachenie, smeshchenie) if znachenie else str(smeshchenie)
    cifry = sovpad.group(1)
    novye = str(int(cifry) + smeshchenie).rjust(len(cifry), "0")
    return znachenie[:sovpad.start(1)] + novye


def svodka(vygruzka):
    """Что интересного в прочитанном файле — для показа перед генерацией."""
    kol = kolonki_vygruzki(vygruzka["zagolovki"])
    zapisi = vygruzka["zapisi"]
    if not kol["imya"]:
        raise OshibkaDannyh("В файле нет колонки имени (FIRSTNAME) — "
                            "это не похоже на выгрузку Бюро пропусков.")

    nomera = [n for n in (hvostovoe_chislo(z.get(kol["imya"])) for z in zapisi)
              if n is not None]
    tabelnye = [(hvostovoe_chislo(z.get(kol["tabelnyj"])), z.get(kol["tabelnyj"]))
                for z in zapisi] if kol["tabelnyj"] else []
    tabelnye = [t for t in tabelnye if t[0] is not None]

    poslednyaya = zapisi[-1]
    return {
        "kolonki": kol,
        "vsego": len(zapisi),
        "familiya": poslednyaya.get(kol["familiya"], "") if kol["familiya"] else "",
        "organizaciya": znachenie_organizacii(poslednyaya, kol),
        "posledniy_nomer": max(nomera) if nomera else None,
        "posledniy_tabelnyj": max(tabelnye)[1] if tabelnye else None,
        "foto": poslednyaya.get(kol["foto"], "") if kol["foto"] else "",
        "obrazec": poslednyaya,
    }


def sobrat_stroki(vygruzka, nastrojki):
    """Строит список новых записей. nastrojki — словарь из полей формы."""
    inf = svodka(vygruzka)
    kol = inf["kolonki"]
    obrazec = inf["obrazec"]

    skolko = nastrojki["skolko"]
    if skolko < 1:
        raise OshibkaDannyh("Количество заявок должно быть больше нуля.")

    shablon = nastrojki["shablon_imeni"]
    if "{n}" not in shablon:
        raise OshibkaDannyh('В шаблоне имени нет "{n}" — номер некуда подставить.')

    if nastrojki["avto"]:
        if inf["posledniy_nomer"] is None:
            raise OshibkaDannyh("Не удалось определить последний номер: "
                                "в именах нет цифр. Задай номер вручную.")
        if inf["posledniy_tabelnyj"] is None:
            raise OshibkaDannyh("Не удалось определить последний табельный: "
                                "в колонке нет цифр. Задай его вручную.")
        nachat_s = inf["posledniy_nomer"] + 1
        tabelnyj_baza, pervoe_smeshchenie = inf["posledniy_tabelnyj"], 1
    else:
        nachat_s = nastrojki["nachat_s"]
        if nastrojki["pervyj_tabelnyj"]:
            tabelnyj_baza, pervoe_smeshchenie = nastrojki["pervyj_tabelnyj"], 0
        else:
            tabelnyj_baza = obrazec.get(kol["tabelnyj"], "") if kol["tabelnyj"] else ""
            pervoe_smeshchenie = 1

    imya_obrazca = obrazec.get(kol["imya"], "")
    korp_obrazca = obrazec.get(kol["korp"], "") if kol["korp"] else ""

    preduprezhdeniya = []
    if kol["korp"] and korp_obrazca and imya_obrazca not in korp_obrazca:
        preduprezhdeniya.append(
            "Имя %r не найдено в корп. коде %r — код останется одинаковым у всех "
            "строк, и импорт может слить их в одну персону."
            % (imya_obrazca, korp_obrazca))

    stroki = []
    for poz, nomer in enumerate(range(nachat_s, nachat_s + skolko)):
        imya = shablon.format(n=nomer)
        if len(imya) > PREDEL_FIO:
            raise OshibkaDannyh("Имя %r длиннее %d символов — Бастион его обрежет."
                                % (imya, PREDEL_FIO))
        stroka = dict(obrazec)
        stroka[kol["imya"]] = imya
        if kol["otchestvo"] is not None and nastrojki["otchestvo"] is not None:
            stroka[kol["otchestvo"]] = nastrojki["otchestvo"]
        if kol["tabelnyj"]:
            stroka[kol["tabelnyj"]] = sdvinut_tabelnyj(tabelnyj_baza,
                                                       pervoe_smeshchenie + poz)
        if kol["korp"] and korp_obrazca and imya_obrazca:
            # Корп. код персоны собирается из Ф.И.О. и обязан быть уникальным:
            # мастер импорта сопоставляет строки с базой именно по нему.
            stroka[kol["korp"]] = korp_obrazca.replace(imya_obrazca, imya)
        if kol["korp_propuska"]:
            # Корп. код пропуска у каждого пропуска свой, дублировать нельзя.
            stroka[kol["korp_propuska"]] = ""
        stroki.append(stroka)

    return stroki, inf, preduprezhdeniya


def zapisat_fajl(vygruzka, stroki, kuda):
    kuda = Path(kuda)
    kuda.parent.mkdir(parents=True, exist_ok=True)
    zagolovki = vygruzka["zagolovki"]
    with kuda.open("w", encoding=vygruzka["kodirovka"], newline="") as fh:
        pisatel = csv.writer(fh, delimiter=vygruzka["razdelitel"], quoting=csv.QUOTE_ALL)
        pisatel.writerow(zagolovki)
        if vygruzka["podpisi"]:
            pisatel.writerow(vygruzka["podpisi"])
        for stroka in stroki:
            pisatel.writerow([stroka.get(k, "") for k in zagolovki])
    return kuda


def skopirovat_foto(ishodnyj, gotovyj):
    """Экспорт кладёт фото в подкаталог по имени файла, импорт там же его и ищет.

    Файл импорта называется иначе, значит нужен подкаталог с новым именем.
    """
    ishodnyj, gotovyj = Path(ishodnyj), Path(gotovyj)
    staraya = ishodnyj.with_suffix("") .parent / (ishodnyj.stem + ".CSV_PHOTO")
    novaya = gotovyj.parent / (gotovyj.stem + ".CSV_PHOTO")
    if not staraya.is_dir():
        return None, "Папка %s не найдена — фото при импорте может не подтянуться." % staraya.name
    if staraya.resolve() == novaya.resolve():
        return novaya, None
    if novaya.exists():
        shutil.rmtree(novaya)
    shutil.copytree(staraya, novaya)
    return novaya, None


# --------------------------------------------------------------------------
# Интерфейс
# --------------------------------------------------------------------------

class Prilozhenie(tk.Tk):

    def __init__(self):
        super().__init__()
        self.vygruzka = None

        self.title("%s  %s" % (PRILOZHENIE, VERSIYA))
        self.configure(bg=CVETA["fon"])
        self.minsize(720, 640)
        self._nastroit_stili()
        self._sobrat_interfejs()
        self._obnovit_dostupnost()

    # -- оформление --------------------------------------------------------

    def _nastroit_stili(self):
        stil = ttk.Style(self)
        if "clam" in stil.theme_names():
            stil.theme_use("clam")
        obychnyj = ("Segoe UI", 10)
        stil.configure(".", background=CVETA["fon"], foreground=CVETA["tekst"],
                       font=obychnyj)
        stil.configure("Panel.TFrame", background=CVETA["panel"])
        stil.configure("Panel.TLabel", background=CVETA["panel"])
        stil.configure("Zagolovok.TLabel", background=CVETA["fon"],
                       font=("Segoe UI Semibold", 15))
        stil.configure("Podzagolovok.TLabel", background=CVETA["fon"],
                       foreground=CVETA["tusklyj"], font=("Segoe UI", 9))
        stil.configure("Razdel.TLabel", background=CVETA["panel"],
                       font=("Segoe UI Semibold", 10))
        stil.configure("Tusklyj.TLabel", background=CVETA["panel"],
                       foreground=CVETA["tusklyj"], font=("Segoe UI", 9))
        stil.configure("Svodka.TLabel", background=CVETA["panel"],
                       foreground=CVETA["tusklyj"], font=("Consolas", 9))
        stil.configure("TEntry", fieldbackground="white", padding=4)
        stil.configure("TCheckbutton", background=CVETA["panel"])
        stil.configure("TRadiobutton", background=CVETA["panel"])
        stil.configure("Obzor.TButton", padding=(10, 4))

    def _panel(self, roditel, zagolovok):
        """Белая карточка с заголовком — визуальная группа полей."""
        obolochka = tk.Frame(roditel, bg=CVETA["ramka"], padx=1, pady=1)
        obolochka.pack(fill="x", pady=(0, 12))
        vnutri = ttk.Frame(obolochka, style="Panel.TFrame", padding=(16, 12, 16, 14))
        vnutri.pack(fill="x")
        ttk.Label(vnutri, text=zagolovok, style="Razdel.TLabel").grid(
            row=0, column=0, columnspan=3, sticky="w", pady=(0, 8))
        vnutri.columnconfigure(1, weight=1)
        return vnutri

    # -- сборка ------------------------------------------------------------

    def _sobrat_interfejs(self):
        korpus = ttk.Frame(self, padding=(20, 18, 20, 14))
        korpus.pack(fill="both", expand=True)

        ttk.Label(korpus, text="Пакетное создание заявок",
                  style="Zagolovok.TLabel").pack(anchor="w")
        ttk.Label(korpus,
                  text="Выгрузи из Бюро пропусков последнюю заявку, укажи сколько "
                       "нужно новых — получишь файл для обратного импорта.",
                  style="Podzagolovok.TLabel").pack(anchor="w", pady=(2, 14))

        self._panel_ishodnika(korpus)
        self._panel_chto_sozdavat(korpus)
        self._panel_numeracii(korpus)
        self._panel_rezultata(korpus)
        self._panel_zhurnala(korpus)
        self._nizhnyaya_polosa(korpus)

    def _panel_ishodnika(self, roditel):
        panel = self._panel(roditel, "1. Выгрузка из Бастиона")
        self.pole_ishodnika = ttk.Entry(panel)
        self.pole_ishodnika.grid(row=1, column=0, columnspan=2, sticky="ew", padx=(0, 8))
        ttk.Button(panel, text="Обзор…", style="Obzor.TButton",
                   command=self._vybrat_ishodnik).grid(row=1, column=2, sticky="e")
        self.nadpis_svodki = ttk.Label(panel, text="Файл не выбран", style="Svodka.TLabel",
                                       justify="left")
        self.nadpis_svodki.grid(row=2, column=0, columnspan=3, sticky="w", pady=(10, 0))

    def _panel_chto_sozdavat(self, roditel):
        panel = self._panel(roditel, "2. Что создавать")

        ttk.Label(panel, text="Сколько новых заявок",
                  style="Panel.TLabel").grid(row=1, column=0, sticky="w", pady=3)
        self.pole_kolichestva = ttk.Spinbox(panel, from_=1, to=9999, width=8)
        self.pole_kolichestva.set(35)
        self.pole_kolichestva.grid(row=1, column=1, sticky="w", pady=3)

        ttk.Label(panel, text="Шаблон имени",
                  style="Panel.TLabel").grid(row=2, column=0, sticky="w", pady=3)
        self.pole_shablona = ttk.Entry(panel, width=18)
        self.pole_shablona.insert(0, "№ {n}")
        self.pole_shablona.grid(row=2, column=1, sticky="w", pady=3)
        ttk.Label(panel, text="{n} — порядковый номер",
                  style="Tusklyj.TLabel").grid(row=2, column=2, sticky="w", padx=(8, 0))

        self.otchestvo_alt255 = tk.BooleanVar(value=True)
        ttk.Checkbutton(panel, text="Отчество — символ Alt+255 (неразрывный пробел)",
                        variable=self.otchestvo_alt255,
                        command=self._obnovit_dostupnost).grid(
            row=3, column=0, columnspan=3, sticky="w", pady=(8, 0))
        ttk.Label(panel, text="Снимешь галочку — отчество останется как в выгрузке.",
                  style="Tusklyj.TLabel").grid(row=4, column=0, columnspan=3, sticky="w")

    def _panel_numeracii(self, roditel):
        panel = self._panel(roditel, "3. Нумерация")

        self.rezhim_numeracii = tk.StringVar(value="avto")
        ttk.Radiobutton(panel, text="Продолжить автоматически от последней записи",
                        variable=self.rezhim_numeracii, value="avto",
                        command=self._obnovit_dostupnost).grid(
            row=1, column=0, columnspan=3, sticky="w")
        ttk.Radiobutton(panel, text="Задать вручную",
                        variable=self.rezhim_numeracii, value="ruchnoj",
                        command=self._obnovit_dostupnost).grid(
            row=2, column=0, columnspan=3, sticky="w", pady=(4, 6))

        self.nadpis_nomera = ttk.Label(panel, text="Начать с номера", style="Panel.TLabel")
        self.nadpis_nomera.grid(row=3, column=0, sticky="w", padx=(24, 0), pady=3)
        self.pole_nomera = ttk.Spinbox(panel, from_=1, to=999999, width=8)
        self.pole_nomera.set(1)
        self.pole_nomera.grid(row=3, column=1, sticky="w", pady=3)

        self.nadpis_tabelnogo = ttk.Label(panel, text="Первый табельный",
                                          style="Panel.TLabel")
        self.nadpis_tabelnogo.grid(row=4, column=0, sticky="w", padx=(24, 0), pady=3)
        self.pole_tabelnogo = ttk.Entry(panel, width=18)
        self.pole_tabelnogo.grid(row=4, column=1, sticky="w", pady=3)
        ttk.Label(panel, text="пусто — продолжить от выгрузки",
                  style="Tusklyj.TLabel").grid(row=4, column=2, sticky="w", padx=(8, 0))

    def _panel_rezultata(self, roditel):
        panel = self._panel(roditel, "4. Куда сохранить")
        self.pole_rezultata = ttk.Entry(panel)
        self.pole_rezultata.grid(row=1, column=0, columnspan=2, sticky="ew", padx=(0, 8))
        ttk.Button(panel, text="Обзор…", style="Obzor.TButton",
                   command=self._vybrat_rezultat).grid(row=1, column=2, sticky="e")
        ttk.Label(panel, text="Папку с фотографиями рядом с файлом программа создаст сама.",
                  style="Tusklyj.TLabel").grid(row=2, column=0, columnspan=3,
                                               sticky="w", pady=(8, 0))

    def _panel_zhurnala(self, roditel):
        obolochka = tk.Frame(roditel, bg=CVETA["ramka"], padx=1, pady=1)
        obolochka.pack(fill="both", expand=True, pady=(0, 12))
        self.zhurnal = tk.Text(obolochka, height=8, wrap="word", bd=0,
                               bg=CVETA["panel"], fg=CVETA["tekst"],
                               font=("Consolas", 9), padx=12, pady=10,
                               state="disabled")
        self.zhurnal.pack(side="left", fill="both", expand=True)
        polosa = ttk.Scrollbar(obolochka, command=self.zhurnal.yview)
        polosa.pack(side="right", fill="y")
        self.zhurnal.configure(yscrollcommand=polosa.set)
        self.zhurnal.tag_configure("uspeh", foreground=CVETA["uspeh"])
        self.zhurnal.tag_configure("oshibka", foreground=CVETA["oshibka"])
        self.zhurnal.tag_configure("tusklyj", foreground=CVETA["tusklyj"])

    def _nizhnyaya_polosa(self, roditel):
        polosa = ttk.Frame(roditel)
        polosa.pack(fill="x")
        self.nadpis_statusa = ttk.Label(polosa, text="Готов к работе",
                                        style="Podzagolovok.TLabel")
        self.nadpis_statusa.pack(side="left")
        self.knopka_sozdat = tk.Button(
            polosa, text="Создать файл", command=self._sozdat,
            bg=CVETA["akcent"], fg="white", activebackground=CVETA["akcent_navedenie"],
            activeforeground="white", relief="flat", cursor="hand2",
            font=("Segoe UI Semibold", 10), padx=22, pady=9, bd=0)
        self.knopka_sozdat.pack(side="right")
        self.knopka_otkryt = tk.Button(
            polosa, text="Открыть папку", command=self._otkryt_papku,
            bg=CVETA["fon"], fg=CVETA["tekst"], relief="flat", cursor="hand2",
            font=("Segoe UI", 10), padx=14, pady=9, bd=0, state="disabled")
        self.knopka_otkryt.pack(side="right", padx=(0, 8))

    # -- действия ----------------------------------------------------------

    def _pisat(self, soobshchenie, metka=None):
        self.zhurnal.configure(state="normal")
        self.zhurnal.insert("end", soobshchenie + "\n", metka or ())
        self.zhurnal.see("end")
        self.zhurnal.configure(state="disabled")

    def _status(self, tekst):
        self.nadpis_statusa.configure(text=tekst)
        self.update_idletasks()

    def _vybrat_ishodnik(self):
        put = filedialog.askopenfilename(
            title="Выгрузка из Бюро пропусков",
            filetypes=[("Файлы CSV", "*.csv"), ("Все файлы", "*.*")])
        if not put:
            return
        self.pole_ishodnika.delete(0, "end")
        self.pole_ishodnika.insert(0, put)
        if not self.pole_rezultata.get().strip():
            po_umolchaniyu = Path(put).with_name("batch.csv")
            self.pole_rezultata.insert(0, str(po_umolchaniyu))
        self._prochitat_ishodnik()

    def _vybrat_rezultat(self):
        put = filedialog.asksaveasfilename(
            title="Куда сохранить файл для импорта", defaultextension=".csv",
            initialfile="batch.csv",
            filetypes=[("Файлы CSV", "*.csv"), ("Все файлы", "*.*")])
        if put:
            self.pole_rezultata.delete(0, "end")
            self.pole_rezultata.insert(0, put)

    def _prochitat_ishodnik(self):
        put = self.pole_ishodnika.get().strip()
        try:
            self.vygruzka = prochitat_vygruzku(put)
            inf = svodka(self.vygruzka)
        except OshibkaDannyh as oshibka:
            self.vygruzka = None
            self.nadpis_svodki.configure(text="Файл не подходит: %s" % oshibka)
            self._pisat("Ошибка: %s" % oshibka, "oshibka")
            self._obnovit_dostupnost()
            return

        stroki_svodki = [
            "записей в файле:   %d" % inf["vsego"],
            "фамилия:           %s" % (inf["familiya"] or "—"),
            "организация:       %s" % (inf["organizaciya"] or "—"),
            "последний номер:   %s" % (inf["posledniy_nomer"]
                                       if inf["posledniy_nomer"] is not None else "не найден"),
            "последний табельный: %s" % (inf["posledniy_tabelnyj"] or "не найден"),
            "фото:              %s" % (inf["foto"] or "—"),
        ]
        self.nadpis_svodki.configure(text="\n".join(stroki_svodki))
        if inf["posledniy_nomer"] is not None:
            self.pole_nomera.set(inf["posledniy_nomer"] + 1)
        self._pisat("Прочитан файл %s — записей %d, кодировка %s."
                    % (Path(put).name, inf["vsego"], self.vygruzka["kodirovka"]))
        self._obnovit_dostupnost()

    def _obnovit_dostupnost(self):
        ruchnoj = self.rezhim_numeracii.get() == "ruchnoj"
        sostoyanie = "normal" if ruchnoj else "disabled"
        for vidzhet in (self.pole_nomera, self.pole_tabelnogo):
            vidzhet.configure(state=sostoyanie)
        for nadpis in (self.nadpis_nomera, self.nadpis_tabelnogo):
            nadpis.configure(foreground=CVETA["tekst"] if ruchnoj else CVETA["tusklyj"])
        self.knopka_sozdat.configure(state="normal" if self.vygruzka else "disabled")

    def _sobrat_nastrojki(self):
        try:
            skolko = int(self.pole_kolichestva.get())
        except ValueError:
            raise OshibkaDannyh("Количество заявок должно быть числом.")
        try:
            nachat_s = int(self.pole_nomera.get())
        except ValueError:
            nachat_s = 1
        return {
            "skolko": skolko,
            "shablon_imeni": self.pole_shablona.get(),
            "otchestvo": NERAZRYVNYJ_PROBEL if self.otchestvo_alt255.get() else None,
            "avto": self.rezhim_numeracii.get() == "avto",
            "nachat_s": nachat_s,
            "pervyj_tabelnyj": self.pole_tabelnogo.get().strip(),
        }

    def _sozdat(self):
        if not self.vygruzka:
            return
        kuda = self.pole_rezultata.get().strip()
        if not kuda:
            messagebox.showwarning(PRILOZHENIE, "Укажи, куда сохранить файл.")
            return
        if Path(kuda).resolve() == self.vygruzka["put"].resolve():
            messagebox.showerror(PRILOZHENIE,
                                 "Результат нельзя писать поверх исходной выгрузки.")
            return

        self._status("Готовлю файл…")
        try:
            nastrojki = self._sobrat_nastrojki()
            stroki, inf, preduprezhdeniya = sobrat_stroki(self.vygruzka, nastrojki)
            gotovyj = zapisat_fajl(self.vygruzka, stroki, kuda)
            papka_foto, zamechanie = skopirovat_foto(self.vygruzka["put"], gotovyj)
        except OshibkaDannyh as oshibka:
            self._pisat("Ошибка: %s" % oshibka, "oshibka")
            self._status("Не получилось")
            messagebox.showerror(PRILOZHENIE, str(oshibka))
            return
        except Exception as oshibka:  # неожиданное — показать, а не проглотить
            self._pisat("Сбой: %s" % oshibka, "oshibka")
            self._status("Не получилось")
            messagebox.showerror(PRILOZHENIE, "Неожиданная ошибка:\n%s" % oshibka)
            return

        kol = inf["kolonki"]
        self._pisat("")
        self._pisat("Создано записей: %d  →  %s" % (len(stroki), gotovyj.name), "uspeh")
        self._pisat("  имена:       %s .. %s" % (stroki[0][kol["imya"]],
                                                 stroki[-1][kol["imya"]]))
        if kol["tabelnyj"]:
            self._pisat("  табельные:   %s .. %s" % (stroki[0][kol["tabelnyj"]],
                                                     stroki[-1][kol["tabelnyj"]]))
        if kol["korp"]:
            self._pisat("  корп. коды:  %s .. %s" % (stroki[0][kol["korp"]],
                                                     stroki[-1][kol["korp"]]))
        if papka_foto:
            self._pisat("  папка с фото: %s" % papka_foto.name, "tusklyj")
        for zamechanie_stroka in preduprezhdeniya + ([zamechanie] if zamechanie else []):
            self._pisat("  внимание: %s" % zamechanie_stroka, "oshibka")

        self.itogovaya_papka = gotovyj.parent
        self.knopka_otkryt.configure(state="normal")
        self._status("Готово — файл создан")

    def _otkryt_papku(self):
        papka = getattr(self, "itogovaya_papka", None)
        if not papka:
            return
        try:
            if sys.platform.startswith("win"):
                os.startfile(str(papka))
            elif sys.platform == "darwin":
                os.system('open "%s"' % papka)
            else:
                os.system('xdg-open "%s"' % papka)
        except Exception as oshibka:
            messagebox.showerror(PRILOZHENIE, "Не удалось открыть папку:\n%s" % oshibka)


def vklyuchit_chetkij_shrift():
    """Без этого на мониторах с масштабированием окно выглядит размытым."""
    if not sys.platform.startswith("win"):
        return
    try:
        import ctypes
        ctypes.windll.shcore.SetProcessDpiAwareness(1)
    except Exception:
        try:
            ctypes.windll.user32.SetProcessDPIAware()
        except Exception:
            pass


if __name__ == "__main__":
    vklyuchit_chetkij_shrift()
    Prilozhenie().mainloop()
