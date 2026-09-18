# Сервер Minecraft: факты из профиля spark

`spark-facts.pdf` — двухстраничная подборка фактов о игровом сервере, посчитанных по
health-снимку профилировщика spark (Paper 26.3, Minecraft 26.3, хостинг Aternos, 18.09.2026).

Внутри, например: 129 рамок с предметами против 85 монстров, 841 загруженный чанк = 30 футбольных
полей, 3007 остановок сборщика мусора в сутки, 4,2% процессора ноды, 84 ГиБ трафика в месяц
и 1906 блоков от нуля координат до игрока.

## Как пересобрать

```
python3 src/extract.py                 # data/spark.bin -> data/facts.json
python3 src/facts_pdf.py               # facts.json -> facts.html
google-chrome --headless=new --no-pdf-header-footer \
  --print-to-pdf=spark-facts.pdf facts.html
node src/check.mjs                     # ловит карточки, вылезшие за край страницы
```

`data/spark.bin` — сырой protobuf профиля, скачанный с bytebin: ссылка вида
`https://spark.lucko.me/<id>` соответствует `https://bytebin.lucko.me/<id>`.
Стек-трейсов в нём нет, это health-снимок; для них нужен полный профиль через
`spark profiler start` и `spark profiler stop`.

`src/extract.py` разбирает protobuf без схемы: внутри обобщённый парсер полей, потому что
готовых .proto-файлов spark под рукой нет. Имена сущностей и моделей железа приходится
вытаскивать аккуратно — часть строк при наивном разборе выглядит как вложенные сообщения
и теряется (именно так сначала пропали 178 из 548 сущностей, включая все 129 рамок).
