# Профиль сервера Minecraft: отчёт

`spark-report.pdf` — разбор health-снимка профилировщика spark с игрового сервера
(Paper 26.3, Minecraft 26.3, бесплатный хостинг Aternos, снимок от 18.09.2026).

Что в отчёте: железо физической ноды и реальная доля контейнера, TPS и длительность тиков
по трём минутным окнам, паузы сборщика мусора, население мира по типам сущностей,
сетевой трафик с прогнозом на месяц, разбор настроек `server.properties` и выводы.

## Как пересобрать

```
python3 src/extract.py                 # data/spark.bin -> data/facts.json
python3 src/report.py                  # facts.json -> report.html
google-chrome --headless=new --no-pdf-header-footer \
  --print-to-pdf=spark-report.pdf report.html
node src/check.mjs                     # проверка вёрстки на переполнение страниц
```

Исходные данные: `data/spark.bin` — сырой protobuf, скачанный с bytebin
(`https://bytebin.lucko.me/<id>`, где id берётся из ссылки вида `https://spark.lucko.me/<id>`).
Стек-трейсов в нём нет: health-снимок их не содержит, для них нужен полный профиль
через `spark profiler start` / `spark profiler stop`.
