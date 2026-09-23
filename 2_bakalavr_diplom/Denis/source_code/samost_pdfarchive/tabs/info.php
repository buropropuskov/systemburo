<?php

/**
 * @package    plagiarism_samost
 * @copyright  2026 Mikhailov D.A., RTU MIREA
 * @license    http://www.gnu.org/copyleft/gpl.html GNU GPL v3 or later
 */

require_once(__DIR__ . '/../../../config.php');
require_once(__DIR__ . '/../lib.php');

$cmid = optional_param('cmid', 0, PARAM_INT);

if ($cmid > 0) {
    [$course, $cm] = get_course_and_cm_from_cmid($cmid);
    $context = context_module::instance($cmid);
    require_login($course, true, $cm);
    require_capability('plagiarism/samost:viewreport', $context);
    $PAGE->set_context($context);
    $PAGE->set_course($course);
    $PAGE->set_cm($cm);
    $PAGE->set_heading($course->fullname);
    $page_url = new moodle_url('/plagiarism/samost/tabs/info.php', ['cmid' => $cmid]);
} else {
    require_login();
    $context = context_system::instance();
    $PAGE->set_context($context);
    $PAGE->set_heading(get_string('tab_info', 'plagiarism_samost'));
    $page_url = new moodle_url('/plagiarism/samost/tabs/info.php');
}

$PAGE->set_url($page_url);
$PAGE->set_title(get_string('tab_info', 'plagiarism_samost'));
$PAGE->activityheader->disable();
$PAGE->requires->css('/plagiarism/samost/styles.css');

echo $OUTPUT->header();

if ($cmid > 0) {
    echo $OUTPUT->heading(get_string('report_heading', 'plagiarism_samost', format_string($cm->name)));
}

$currenttab = 'info';
require_once(__DIR__ . '/header.php');
echo '<div class="samost-info">';

echo '
<div class="samost-info-section samost-info-section--intro">
    <div class="samost-info-section-body">

        <p class="samost-info-intro-text">
            Система анализирует сданные работы попарно и вычисляет процент схожести —
            отдельно для файлов с <strong>программным кодом</strong> и отдельно для файлов с <strong>текстовым содержимым</strong>.
            Результат показывает насколько два файла похожи по структуре и содержанию,
            но <em>не является автоматическим вердиктом о списывании</em> — интерпретация остаётся за преподавателем.
        </p>

        <div class="samost-info-callouts">

            <div class="samost-info-callout samost-info-callout--warning">
                <div class="samost-info-callout-icon"><i class="fa fa-exclamation-triangle" aria-hidden="true"></i></div>
                <div>
                    <strong>Работы по шаблону дадут высокий процент схожести</strong>
                    <p>
                        Если студенты сдают документы с одинаковой структурой — титульным листом,
                        стандартными разделами, шаблонными формулировками — система найдёт много
                        совпадений даже при полностью самостоятельном содержании.
                        Такой результат не означает списывание: уникального текста в работе просто мало,
                        а совпадает шаблонная часть. Перед запуском анализа убедитесь,
                        что работы содержат достаточно авторского текста.
                    </p>
                </div>
            </div>

            <div class="samost-info-callout samost-info-callout--ok">
                <div class="samost-info-callout-icon"><i class="fa fa-check-circle" aria-hidden="true"></i></div>
                <div>
                    <strong>Лучше всего подходит для свободных текстовых работ</strong>
                    <p>
                        Анализ наиболее точен там, где студент пишет своими словами:
                        эссе, отчёты, пояснительные записки, описания алгоритмов.
                        Чем больше уникального авторского текста — тем значимее результат сравнения.
                    </p>
                </div>
            </div>

            <div class="samost-info-callout samost-info-callout--code">
                <div class="samost-info-callout-icon"><i class="fa fa-code" aria-hidden="true"></i></div>
                <div>
                    <strong>Для кода — устойчивость к переименованиям</strong>
                    <p>
                        Алгоритм Winnowing работает на уровне токенов, а не текста,
                        поэтому замена имён переменных, функций и классов не снижает схожесть.
                        Структурно одинаковый код будет распознан как похожий,
                        даже если внешне он выглядит иначе.
                        При этом стандартные заготовки и шаблоны кода (например типовой
                        main или boilerplate) также могут давать ложные совпадения.
                    </p>
                </div>
            </div>

        </div>

    </div>
</div>
';

echo '
<div class="samost-info-section">
    <div class="samost-info-section-head">
        <i class="fa fa-file-code-o" aria-hidden="true"></i>
        Поддерживаемые форматы файлов
    </div>
    <div class="samost-info-section-body">

        <div class="samost-info-subsection">
            <div class="samost-info-subtitle">
                <span class="samost-info-badge samost-info-badge--code">КОД</span>
                Файлы исходного кода — алгоритм Winnowing
            </div>
            <p class="samost-info-text">
                Файлы ниже обрабатываются как программный код.
                Для них применяется алгоритм отпечатков Winnowing,
                устойчивый к переименованию переменных и перестановке фрагментов.
            </p>
	    <p class="samost-info-tags-label">Поддерживаемые расширения:</p>
            <div class="samost-info-tags">
                <span class="samost-tag samost-tag--code">C / C++ / H</span>
                <span class="samost-tag samost-tag--code">Java</span>
                <span class="samost-tag samost-tag--code">Kotlin</span>
                <span class="samost-tag samost-tag--code">Scala</span>
                <span class="samost-tag samost-tag--code">Python</span>
                <span class="samost-tag samost-tag--code">Ruby</span>
                <span class="samost-tag samost-tag--code">PHP</span>
                <span class="samost-tag samost-tag--code">JavaScript / TS</span>
                <span class="samost-tag samost-tag--code">JSX / TSX / Vue</span>
                <span class="samost-tag samost-tag--code">C# / VB / F#</span>
                <span class="samost-tag samost-tag--code">Go</span>
                <span class="samost-tag samost-tag--code">Rust</span>
                <span class="samost-tag samost-tag--code">Swift</span>
                <span class="samost-tag samost-tag--code">SQL</span>
                <span class="samost-tag samost-tag--code">R</span>
                <span class="samost-tag samost-tag--code">Dart</span>
                <span class="samost-tag samost-tag--code">Lua / Shell</span>
                <span class="samost-tag samost-tag--code">ASM</span>
            </div>
        </div>

        <div class="samost-info-subsection">
            <div class="samost-info-subtitle">
                <span class="samost-info-badge samost-info-badge--text">ТЕКСТ</span>
                Текстовые документы — алгоритм шинглов Жаккара
            </div>
            <p class="samost-info-text">
                Документы ниже обрабатываются как естественный текст.
                Применяется метод шинглов с вычислением коэффициента Жаккара.
                Из DOCX извлекается чистый текст без форматирования.
            </p>
	    <p class="samost-info-tags-label">Поддерживаемые расширения:</p>
            <div class="samost-info-tags">
                <span class="samost-tag samost-tag--text">DOCX</span>
		<span class="samost-tag samost-tag--text">PDF</span>
                <span class="samost-tag samost-tag--text">TXT</span>
                <span class="samost-tag samost-tag--text">MD</span>
            </div>
        </div>

        <div class="samost-info-subsection">
            <div class="samost-info-subtitle">
                <span class="samost-info-badge samost-info-badge--archive">АРХИВ</span>
                ZIP-архивы — рекурсивная обработка
            </div>
            <p class="samost-info-text">
                ZIP-архивы распаковываются автоматически. Все вложенные файлы
                классифицируются и обрабатываются по соответствующему алгоритму.
                Служебные директории (<code>node_modules</code>, <code>__pycache__</code>,
                <code>.git</code>, <code>vendor</code> и др.) пропускаются.
            </p>
            <p class="samost-info-tags-label">Поддерживаемые расширения:</p>
            <div class="samost-info-tags">
                <span class="samost-tag samost-tag--archive">ZIP</span>
            </div>
        </div>

    </div>
</div>
';


echo '
<div class="samost-info-section">
    <div class="samost-info-section-head">
        <i class="fa fa-code" aria-hidden="true"></i>
        Алгоритм Winnowing — сравнение исходного кода
    </div>
    <div class="samost-info-section-body">

        <p class="samost-info-text">
            Winnowing — алгоритм выбора локальных минимумов из набора хэшей.
            Он устойчив к переименованию переменных, изменению пробелов и перестановке независимых блоков.
        </p>

        <div class="samost-info-steps">
            <div class="samost-info-step">
                <div class="samost-info-step-num">1</div>
                <div>
                    <strong>Токенизация.</strong>
                    Код разбивается на токены — ключевые слова, идентификаторы, операторы.
                    Пробелы, комментарии и имена переменных нормализуются.
                </div>
            </div>
            <div class="samost-info-step">
                <div class="samost-info-step-num">2</div>
                <div>
                    <strong>K-граммы.</strong>
                    Из токенов формируются скользящие окна длиной k.
                    Каждое окно хэшируется.
                </div>
            </div>
            <div class="samost-info-step">
                <div class="samost-info-step-num">3</div>
                <div>
                    <strong>Отпечатки.</strong>
                    В каждом окне размером w выбирается минимальный хэш —
                    это и есть «отпечаток» документа.
                </div>
            </div>
            <div class="samost-info-step">
                <div class="samost-info-step-num">4</div>
                <div>
                    <strong>Коэффициент Жаккара по отпечаткам.</strong>
                    Схожесть = |A ∩ B| / |A ∪ B|, где A и B — множества отпечатков двух работ.
                </div>
            </div>
        </div>

        <div class="samost-info-example">
            <div class="samost-info-example-title">Пример</div>
            <div class="samost-info-example-cols">
                <div>
                    <div class="samost-info-example-label">Студент А</div>
                    <pre class="samost-info-code">def sort(arr):
    for i in range(len(arr)):
        for j in range(len(arr)-i-1):
            if arr[j] > arr[j+1]:
                arr[j], arr[j+1] = arr[j+1], arr[j]</pre>
                </div>
                <div>
                    <div class="samost-info-example-label">Студент Б</div>
                    <pre class="samost-info-code">def bubble(data):
    n = len(data)
    for i in range(n):
        for j in range(n-i-1):
            if data[j] > data[j+1]:
                data[j], data[j+1] = data[j+1], data[j]</pre>
                </div>
            </div>
            <div class="samost-info-example-note">
                <i class="fa fa-info-circle" aria-hidden="true"></i>
                Несмотря на разные имена функции и переменных,
                структура токенов идентична — алгоритм выявит высокую схожесть.
            </div>
        </div>

    </div>
</div>
';


echo '
<div class="samost-info-section">
    <div class="samost-info-section-head">
        <i class="fa fa-file-text-o" aria-hidden="true"></i>
        Метод шинглов — сравнение текстовых документов
    </div>
    <div class="samost-info-section-body">

        <p class="samost-info-text">
            Шинглы (shingles) — метод сравнения текстов на основе пересечения
            наборов последовательных слов. Нечувствителен к регистру и знакам препинания.
        </p>

        <div class="samost-info-steps">
            <div class="samost-info-step">
                <div class="samost-info-step-num">1</div>
                <div>
                    <strong>Нормализация.</strong>
                    Текст приводится к нижнему регистру,
                    знаки препинания и лишние пробелы убираются.
                </div>
            </div>
            <div class="samost-info-step">
                <div class="samost-info-step-num">2</div>
                <div>
                    <strong>Шинглы.</strong>
                    Из слов формируются скользящие окна длиной N.
                    Каждое окно — один шингл.
                </div>
            </div>
            <div class="samost-info-step">
                <div class="samost-info-step-num">3</div>
                <div>
                    <strong>Коэффициент Жаккара.</strong>
                    Схожесть = |A ∩ B| / |A ∪ B|,
                    где A и B — множества шинглов двух текстов.
                </div>
            </div>
        </div>

       <div class="samost-info-example">
    <div class="samost-info-example-title">Пример (размер шингла = 3 слова)</div>
    <div class="samost-info-example-cols">
        <div>
            <div class="samost-info-example-label">Студент А</div>
            <p class="samost-info-quote">
                «Сортировка пузырьком является одним из простейших алгоритмов сортировки массивов.»
            </p>
            <div class="samost-info-shingles">
                <span class="samost-shingle samost-shingle--match">сортировка пузырьком является</span>
                <span class="samost-shingle samost-shingle--match">пузырьком является одним</span>
                <span class="samost-shingle samost-shingle--match">является одним из</span>
                <span class="samost-shingle">одним из простейших</span>
                <span class="samost-shingle">из простейших алгоритмов</span>
                <span class="samost-shingle">простейших алгоритмов сортировки</span>
                <span class="samost-shingle">алгоритмов сортировки массивов</span>
            </div>
        </div>
        <div>
            <div class="samost-info-example-label">Студент Б</div>
            <p class="samost-info-quote">
                «Сортировка пузырьком является одним из известных методов упорядочивания элементов.»
            </p>
            <div class="samost-info-shingles">
                <span class="samost-shingle samost-shingle--match">сортировка пузырьком является</span>
                <span class="samost-shingle samost-shingle--match">пузырьком является одним</span>
                <span class="samost-shingle samost-shingle--match">является одним из</span>
                <span class="samost-shingle">одним из известных</span>
                <span class="samost-shingle">из известных методов</span>
                <span class="samost-shingle">известных методов упорядочивания</span>
                <span class="samost-shingle">методов упорядочивания элементов</span>
            </div>
        </div>
    </div>
    <div class="samost-info-example-note">
        <i class="fa fa-info-circle" aria-hidden="true"></i>
        Выделены совпадающие шинглы. Итого: 3 общих из 11 уникальных → Жаккар = 3/11 ≈ <strong>27%</strong>.
        Именно эти совпадения подсвечиваются на странице сравнения пары.
    </div>
</div>

    </div>
</div>
';


echo '
<div class="samost-info-section">
    <div class="samost-info-section-head">
        <i class="fa fa-sitemap" aria-hidden="true"></i>
        Режимы сравнения
    </div>
    <div class="samost-info-section-body">
        <div class="samost-info-modes">
            <div class="samost-info-mode">
                <div class="samost-mode-icon samost-mode-icon--within">
                    <i class="fa fa-compress" aria-hidden="true"></i>
                </div>
                <div>
                    <strong>Внутри группы</strong>
                    <p class="samost-info-text" style="margin-top:4px">
                        Студенты выбранной группы сравниваются между собой попарно.
                        Подходит для проверки списывания внутри одного потока.
                        Количество пар: N × (N−1) / 2, где N — число сданных работ.
                    </p>
                </div>
            </div>
            <div class="samost-info-mode">
                <div class="samost-mode-icon samost-mode-icon--cross">
                    <i class="fa fa-random" aria-hidden="true"></i>
                </div>
                <div>
                    <strong>Группа с другими группами</strong>
                    <p class="samost-info-text" style="margin-top:4px">
                        Каждый студент выбранной группы сравнивается со всеми студентами
                        остальных групп. Студенты внутри каждой группы между собой не сравниваются.
                        Используется для выявления заимствований между потоками.
                    </p>
                </div>
            </div>
        </div>
    </div>
</div>
';

echo '</div>'; 

echo $OUTPUT->footer();
