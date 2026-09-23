<?php
// This file is part of Moodle - http://moodle.org/
//
// Moodle is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Moodle is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with Moodle.  If not, see <http://www.gnu.org/licenses/>.

/**
 * @package    plagiarism_samost
 * @copyright  2026 Mikhailov D.A., RTU MIREA
 * @license    http://www.gnu.org/copyleft/gpl.html GNU GPL v3 or later
 */

$string['pluginname']           = 'Проверка самостоятельности';
$string['samost']               = 'Проверка самостоятельности';
$string['samost:viewreport']    = 'Просматривать отчёт о схожести работ';
$string['samost:runanalysis']   = 'Запускать анализ схожести';

// Страница настроек
$string['enabled']              = 'Включить плагин';
$string['enabled_help']         = 'При включении плагин анализирует работы студентов и отображает процент схожести.';
$string['threshold']            = 'Порог оповещения (%)';
$string['threshold_help']       = 'Пары с уровнем схожести выше этого значения будут выделены в отчёте.';
$string['shingle_size']         = 'Размер шингла (слов)';
$string['shingle_size_help']    = 'Количество последовательных слов в одном шингле при вычислении коэффициента Жаккара.';

// Форма настройки задания
$string['use_samost']           = 'Включить проверку самостоятельности';
$string['use_samost_help']      = 'Включить анализ схожести для этого задания.';

// Страница отчёта
$string['viewreport']           = 'Анализ сданных работ';
$string['report_heading']       = 'Антиплагиат: {$a}';
$string['report_notready']      = 'Анализ ещё не был запущен. Нажмите «Запустить анализ».';
$string['runanalysis']          = 'Запустить анализ';
$string['analysisqueued']       = 'Анализ поставлен в очередь. Результаты появятся в ближайшее время.';
$string['queue_waiting']        = 'Анализ в очереди — ожидается обработка {$a} студентов. Страница обновится автоматически через 15 секунд…';
$string['queue_hint']           = 'Обработка выполняется фоновым заданием. Не закрывайте страницу — она обновится сама.';
$string['task_process_queue']   = 'Самост: обработка очереди анализа';
$string['student1']             = 'Студент 1';
$string['student2']             = 'Студент 2';
$string['similarity']           = 'Схожесть (%)';
$string['actions']              = 'Действия';
$string['viewpair']             = 'Подробнее';
$string['noresults']            = 'Результатов нет.';
$string['high_similarity']      = 'Высокая схожесть';

// Страница детального сравнения пары
$string['pair_heading']         = 'Сравнение: {$a->user1} и {$a->user2}';
$string['similarity_score']     = 'Уровень схожести: {$a}%';
$string['text_a']               = 'Работа студента: {$a}';
$string['no_text']              = 'Не удалось извлечь текстовый контент из работы.';
$string['back_to_report']       = '← Назад к отчёту';

// Ошибки
$string['error_nopermission']   = 'У вас нет прав для просмотра этого отчёта.';
$string['error_invalidcm']      = 'Неверный идентификатор модуля курса.';
$string['error_notassign']      = 'Проверка самостоятельности работает только с элементом «Задание».';

$string['similarity_code'] = 'Схожесть кода (%)';
$string['similarity_text'] = 'Схожесть текста (%)';

// Вкладки
$string['tab_report'] = 'Отчёт';
$string['tab_info']   = 'Справка';

//  Уведомления
$string['no_submissions_in_group']    = 'В выбранной группе нет сданных работ.';
$string['not_enough_submissions']     = 'В выбранной группе только одна работа — сравнивать не с чем.';
$string['no_other_groups_submissions'] = 'В остальных группах нет сданных работ — не с кем сравнивать.';

