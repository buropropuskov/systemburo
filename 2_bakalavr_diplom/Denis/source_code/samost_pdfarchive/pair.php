<?php
/**
 * @package    plagiarism_samost
 * @copyright  2026 Mikhailov D.A., RTU MIREA
 * @license    http://www.gnu.org/copyleft/gpl.html GNU GPL v3 or later
 */

require_once(__DIR__ . '/../../config.php');
require_once(__DIR__ . '/lib.php');

use plagiarism_samost\text_extractor;
use plagiarism_samost\file_types;

$cmid = required_param('cmid', PARAM_INT);
$uid1 = required_param('uid1', PARAM_INT);
$uid2 = required_param('uid2', PARAM_INT);

[$course, $cm] = get_course_and_cm_from_cmid($cmid);
$context = context_module::instance($cmid);

require_login($course, true, $cm);
require_capability('plagiarism/samost:viewreport', $context);

$PAGE->set_url(new moodle_url('/plagiarism/samost/pair.php', [
    'cmid' => $cmid, 'uid1' => $uid1, 'uid2' => $uid2,
]));
$PAGE->set_context($context);
$PAGE->set_course($course);
$PAGE->set_title(get_string('pluginname', 'plagiarism_samost'));
$PAGE->set_heading($course->fullname);
$PAGE->activityheader->disable();

$action = optional_param('action', '', PARAM_ALPHA);
if ($action === 'feedback') {
    require_sesskey();
    $message = trim(optional_param('message', '', PARAM_TEXT));
    if (strlen($message) > 0) {
        // Read snapshot values passed as hidden fields from the form.
        $snap_code = optional_param('snap_code', null, PARAM_RAW);
        $snap_text = optional_param('snap_text', null, PARAM_RAW);
        $snap_code = ($snap_code !== null && $snap_code !== '') ? (float)$snap_code : null;
        $snap_text = ($snap_text !== null && $snap_text !== '') ? (float)$snap_text : null;

        $DB->insert_record('plagiarism_samost_feedback', (object)[
            'cm'                   => $cmid,
            'userid'               => (int)$USER->id,
            'message'              => $message,
            'pair_userid1'         => $uid1,
            'pair_userid2'         => $uid2,
            'snap_similarity_code' => $snap_code,
            'snap_similarity_text' => $snap_text,
            'timecreated'          => time(),
        ]);
    }
    redirect(
        new moodle_url('/plagiarism/samost/pair.php', ['cmid' => $cmid, 'uid1' => $uid1, 'uid2' => $uid2]),
        'Благодарим за отзыв! Обратная связь не предусмотрена, однако все отзывы анализируются и учитываются при доработке системы.',
        null,
        \core\output\notification::NOTIFY_SUCCESS
    );
}

$user1 = core_user::get_user($uid1, '*', MUST_EXIST);
$user2 = core_user::get_user($uid2, '*', MUST_EXIST);

$name1 = fullname($user1);
$name2 = fullname($user2);

$result = $DB->get_record_select(
    'plagiarism_samost_results',
    'cm = :cm AND ((userid1 = :u1 AND userid2 = :u2) OR (userid1 = :u2b AND userid2 = :u1b))',
    ['cm' => $cmid, 'u1' => $uid1, 'u2' => $uid2, 'u1b' => $uid1, 'u2b' => $uid2]
);

$sim_code = $result ? $result->similarity_code : null;
$sim_text = $result ? $result->similarity_text : null;


$records1 = $DB->get_records('plagiarism_samost_files', ['cm' => $cmid, 'userid' => $uid1]);
$records2 = $DB->get_records('plagiarism_samost_files', ['cm' => $cmid, 'userid' => $uid2]);

function samost_latest_upload(array $records): ?int {
    global $DB;
    $fileids = array_column($records, 'fileid');
    if (empty($fileids)) return null;
    [$in_sql, $in_params] = $DB->get_in_or_equal($fileids);
    return $DB->get_field_sql(
        "SELECT MAX(timemodified) FROM {files} WHERE id $in_sql AND filename != '.'",
        $in_params
    ) ?: null;
}

$upload_time1 = samost_latest_upload($records1);
$upload_time2 = samost_latest_upload($records2);

function samost_split_buckets(array $records): array {
    $code = '';
    $text = '';
    foreach ($records as $rec) {
        if (empty($rec->textcontent)) continue;
        $content = (string) $rec->textcontent;

        // Handle combined "CODE:...\n\nTEXT:..." from ZIP archives.
        $has_code = strpos($content, 'CODE:') !== false;
        $has_text_prefix = strpos($content, 'TEXT:') !== false;

        if ($has_code && $has_text_prefix) {
            // ZIP combined block — extract each part separately.
            if (preg_match('/CODE:(.*?)(?=\n\nTEXT:|$)/su', $content, $m) && trim($m[1]) !== '') {
                $code .= ($code !== '' ? "\n\n" : '') . trim($m[1]);
            }
            if (preg_match('/TEXT:(.*?)$/su', $content, $m) && trim($m[1]) !== '') {
                $text .= ($text !== '' ? ' ' : '') . trim($m[1]);
            }
        } elseif ($has_code) {
            $payload = text_extractor::payload($content);
            $code .= ($code !== '' ? "\n\n" : '') . $payload;
        } else {
            $payload = text_extractor::payload($content);
            $text .= ($text !== '' ? ' ' : '') . $payload;
        }
    }
    return ['code' => $code, 'text' => $text];
}

$b1 = samost_split_buckets($records1);
$b2 = samost_split_buckets($records2);

function samost_normalise_word(string $w): string {
    $w = mb_strtolower($w, 'UTF-8');
    $w = preg_replace('/[^\p{L}\p{N}]/u', '', $w);
    return $w;
}

function samost_shingle_strings(string $text, int $size): array {
    $raw_words = preg_split('/\s+/u', $text, -1, PREG_SPLIT_NO_EMPTY);
    $words = array_values(array_filter(array_map('samost_normalise_word', $raw_words), fn($w) => $w !== ''));
    $set = [];
    for ($i = 0; $i <= count($words) - $size; $i++) {
        $set[implode(' ', array_slice($words, $i, $size))] = true;
    }
    return $set;
}

function samost_highlight(string $text, array $common_shingles, int $size): string {
    if (empty($text)) return '';

    $all_tokens   = preg_split('/(\s+)/u', $text, -1, PREG_SPLIT_DELIM_CAPTURE);
    $word_indices = [];
    foreach ($all_tokens as $idx => $tok) {
        if ($idx % 2 === 0) $word_indices[] = $idx;
    }

    $n = count($word_indices);
    $mask = array_fill(0, $n, false);

    for ($i = 0; $i <= $n - $size; $i++) {
        $norm = array_filter(
            array_map('samost_normalise_word', array_map(fn($k) => $all_tokens[$word_indices[$k]], range($i, $i + $size - 1))),
            fn($w) => $w !== ''
        );
        if (isset($common_shingles[implode(' ', $norm)])) {
            for ($k = 0; $k < $size; $k++) $mask[$i + $k] = true;
        }
    }

    $html = '';
    $in_mark = false;
    foreach ($word_indices as $wi => $tok_idx) {
        $word  = $all_tokens[$tok_idx];
        $space = $all_tokens[$tok_idx + 1] ?? '';
        if ($mask[$wi] && !$in_mark)       { $html .= '<mark>'; $in_mark = true; }
        elseif (!$mask[$wi] && $in_mark)   { $html .= '</mark>'; $in_mark = false; }
        $html .= htmlspecialchars($word, ENT_QUOTES, 'UTF-8') . htmlspecialchars($space, ENT_QUOTES, 'UTF-8');
    }
    if ($in_mark) $html .= '</mark>';
    return $html;
}


function samost_initials(string $name): string {
    $parts = preg_split('/\s+/', trim($name), -1, PREG_SPLIT_NO_EMPTY);
    $init  = '';
    foreach (array_slice($parts, 0, 2) as $p) {
        $init .= mb_strtoupper(mb_substr($p, 0, 1, 'UTF-8'), 'UTF-8');
    }
    return $init ?: '?';
}


function samost_meter(?float $val, int $threshold, string $label): string {
    if ($val === null) return '';
    $pct     = number_format($val, 1);
    $is_high = $val >= $threshold;
    $color   = $is_high ? 'var(--danger, #ca3120)' : 'var(--success, #1CA374)';
    $bg      = $is_high ? 'rgba(202,49,32,.07)'    : 'rgba(28,163,116,.07)';
    $border  = $is_high ? 'rgba(202,49,32,.25)'    : 'rgba(28,163,116,.25)';
    $icon    = $is_high ? '⚠' : '✓';
    $bar_w   = min(100, round($val));

    return '
<div class="samost-meter" style="background:' . $bg . '; border:1.5px solid ' . $border . ';">
    <div class="samost-meter-label">' . htmlspecialchars($label) . '</div>
    <div class="samost-meter-value" style="color:' . $color . ';">'
        . '<span class="samost-meter-icon">' . $icon . '</span>'
        . $pct . '%'
    . '</div>
    <div class="samost-meter-bar-track">
        <div class="samost-meter-bar-fill" style="width:' . $bar_w . '%;background:' . $color . ';"></div>
    </div>
</div>';
}


function samost_section(
    string $title,
    string $badge_html,
    string $html1,
    string $html2,
    string $name1,
    string $name2,
    bool   $is_code,
    bool   $has_legend,
    ?int   $upload_time1 = null,
    ?int   $upload_time2 = null
): void {
    $box_class = $is_code ? 'samost-codebox' : 'samost-textbox';

    echo '<div class="samost-section">';
    echo '<div class="samost-section-head">';
    echo '<span class="samost-section-title">' . htmlspecialchars($title) . '</span>';
    if ($badge_html) echo $badge_html;
    echo '</div>';

    echo '<div class="samost-cols">';
    foreach ([[$name1, $html1, $upload_time1], [$name2, $html2, $upload_time2]] as [$name, $html, $utime]) {
        echo '<div class="samost-col">';
        echo '<div class="samost-col-header">'
            . '<span class="samost-col-avatar">' . samost_initials($name) . '</span>'
            . '<div>'
            . '<span class="samost-col-name">' . htmlspecialchars($name) . '</span>'
            . ($utime ? '<span class="samost-col-date">' . userdate($utime, get_string('strftimedatetimeshort', 'langconfig')) . '</span>' : '')
            . '</div>'
            . '</div>';
        if (empty($html)) {
            echo '<div class="samost-empty">— нет файлов данного типа —</div>';
        } else {
            echo '<div class="' . $box_class . '">' . $html . '</div>';
        }
        echo '</div>';
    }
    echo '</div>';

    if ($has_legend) {
        echo '<div class="samost-legend">'
            . '<span class="samost-legend-swatch"></span>'
            . 'Совпадающие фрагменты (шинглы)'
            . '</div>';
    }

    echo '</div>';
}


$shingle_size = (int)(get_config('plagiarism_samost', 'shingle_size') ?: 5);
$threshold    = (int)(get_config('plagiarism_samost', 'threshold') ?: 70);

$code_html1 = $b1['code'] !== '' ? html_writer::tag('pre', s($b1['code']), ['style' => 'margin:0;white-space:pre-wrap;']) : '';
$code_html2 = $b2['code'] !== '' ? html_writer::tag('pre', s($b2['code']), ['style' => 'margin:0;white-space:pre-wrap;']) : '';


$text_html1 = '';
$text_html2 = '';
$has_text_highlight = false;
if ($b1['text'] !== '' || $b2['text'] !== '') {
    $shingles1 = samost_shingle_strings($b1['text'], $shingle_size);
    $shingles2 = samost_shingle_strings($b2['text'], $shingle_size);
    $common    = array_intersect_key($shingles1, $shingles2);
    $has_text_highlight = !empty($common);
    $text_html1 = $b1['text'] !== '' ? samost_highlight($b1['text'], $common, $shingle_size) : '';
    $text_html2 = $b2['text'] !== '' ? samost_highlight($b2['text'], $common, $shingle_size) : '';
}

$has_code_section = ($b1['code'] !== '' || $b2['code'] !== '');
$has_text_section = ($b1['text'] !== '' || $b2['text'] !== '');


$PAGE->requires->css('/plagiarism/samost/styles.css');
echo $OUTPUT->header();

$timecalc = $result ? userdate($result->timecalculated, get_string('strftimedatetime', 'langconfig')) : '';

echo '<div class="samost-pair-header">';
echo '<div class="samost-pair-header-top">';
echo '<a href="' . (new moodle_url('/plagiarism/samost/report.php', ['cmid' => $cmid]))->out(false) . '" class="samost-back-btn">';
echo '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"/></svg>';
echo get_string('back_to_report', 'plagiarism_samost');
echo '</a>';

$feedback_url = new moodle_url('/plagiarism/samost/pair.php', [
    'cmid'    => $cmid,
    'uid1'    => $uid1,
    'uid2'    => $uid2,
    'action'  => 'feedback',
    'sesskey' => sesskey(),
]);
echo '<button type="button" class="samost-feedback-btn" id="samost-pair-feedback-open">';
echo '<i class="fa fa-comment-o" aria-hidden="true"></i> Оставить отзыв';
echo '</button>';
echo '</div>';

echo '<div class="samost-pair-title">';
echo '<span class="samost-pair-avatar samost-pair-avatar-1">' . samost_initials($name1) . '</span>';
echo '<span class="samost-pair-name">' . htmlspecialchars($name1) . '</span>';
echo '<span class="samost-pair-sep">—</span>';
echo '<span class="samost-pair-avatar samost-pair-avatar-2">' . samost_initials($name2) . '</span>';
echo '<span class="samost-pair-name">' . htmlspecialchars($name2) . '</span>';
if ($timecalc) {
    echo '<span class="samost-pair-date">' . htmlspecialchars($timecalc) . '</span>';
}
echo '</div>';

echo '<div class="samost-meters">';
echo samost_meter($sim_code, $threshold, get_string('similarity_code', 'plagiarism_samost'));
echo samost_meter($sim_text, $threshold, get_string('similarity_text', 'plagiarism_samost'));
echo '</div>';

echo '</div>'; // .samost-pair-header

$sim_code_fmt = $sim_code !== null ? number_format((float)$sim_code, 1) . '%' : '—';
$sim_text_fmt = $sim_text !== null ? number_format((float)$sim_text, 1) . '%' : '—';
$sim_code_raw = $sim_code !== null ? (float)$sim_code : '';
$sim_text_raw = $sim_text !== null ? (float)$sim_text : '';

echo '
<div class="samost-modal-overlay" id="samost-pair-modal-overlay">
    <div class="samost-modal" role="dialog" aria-modal="true" aria-labelledby="samost-pair-modal-title">
        <div class="samost-modal-header">
            <span id="samost-pair-modal-title" class="samost-modal-title">Оставить отзыв по сравнению</span>
            <button type="button" class="samost-modal-close" id="samost-pair-modal-close" aria-label="Закрыть">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
            </button>
        </div>
        <div class="samost-modal-pair-info">
            <div class="samost-modal-pair-names">
                <span>' . htmlspecialchars($name1) . '</span>
                <span class="samost-modal-pair-vs">—</span>
                <span>' . htmlspecialchars($name2) . '</span>
            </div>
            <div class="samost-modal-pair-scores">
                <span class="samost-modal-score-label">Код:</span>
                <span class="samost-modal-score-val">' . $sim_code_fmt . '</span>
                <span class="samost-modal-score-label" style="margin-left:12px">Текст:</span>
                <span class="samost-modal-score-val">' . $sim_text_fmt . '</span>
            </div>
        </div>
        <form method="post" action="' . $feedback_url->out(false) . '" class="samost-modal-body">
            <input type="hidden" name="pair_uid1" value="' . (int)$uid1 . '">
            <input type="hidden" name="pair_uid2" value="' . (int)$uid2 . '">
            <input type="hidden" name="snap_code" value="' . s($sim_code_raw) . '">
            <input type="hidden" name="snap_text" value="' . s($sim_text_raw) . '">
            <input type="hidden" name="redirect_uid1" value="' . (int)$uid1 . '">
            <input type="hidden" name="redirect_uid2" value="' . (int)$uid2 . '">
            <label class="samost-modal-label" for="samost-pair-feedback-text">Опишите проблему или замечание</label>
            <textarea
                id="samost-pair-feedback-text"
                name="message"
                class="samost-modal-textarea"
                placeholder="Опишите Ваш опыт использования системы: что работает корректно, какие функции требуют доработки, с какими трудностями вы столкнулись."
                rows="5"
                maxlength="2000"
                required
            ></textarea>
            <div class="samost-modal-footer">
                <button type="button" class="samost-modal-cancel" id="samost-pair-modal-cancel">Отмена</button>
                <button type="submit" class="samost-modal-submit">
                    <i class="fa fa-paper-plane-o" aria-hidden="true"></i> Отправить
                </button>
            </div>
        </form>
    </div>
</div>';

$PAGE->requires->js_amd_inline("
    require([], function() {
        var overlay   = document.getElementById('samost-pair-modal-overlay');
        var openBtn   = document.getElementById('samost-pair-feedback-open');
        var closeBtn  = document.getElementById('samost-pair-modal-close');
        var cancelBtn = document.getElementById('samost-pair-modal-cancel');
        var textarea  = document.getElementById('samost-pair-feedback-text');
        if (!overlay || !openBtn) return;
        function openModal()  { overlay.classList.add('samost-modal-visible'); setTimeout(function(){ if(textarea) textarea.focus(); }, 50); }
        function closeModal() { overlay.classList.remove('samost-modal-visible'); }
        openBtn.addEventListener('click', openModal);
        if (closeBtn)  closeBtn.addEventListener('click', closeModal);
        if (cancelBtn) cancelBtn.addEventListener('click', closeModal);
        overlay.addEventListener('click', function(e) { if (e.target === overlay) closeModal(); });
        document.addEventListener('keydown', function(e) { if (e.key === 'Escape') closeModal(); });
    });
");

if ($has_code_section) {
    samost_section(
        get_string('similarity_code', 'plagiarism_samost'),
        '',
        $code_html1,
        $code_html2,
        $name1,
        $name2,
        true,
        false,
        $upload_time1,
        $upload_time2
    );
}

if ($has_text_section) {
    samost_section(
        get_string('similarity_text', 'plagiarism_samost'),
        '',
        $text_html1,
        $text_html2,
        $name1,
        $name2,
        false,
        $has_text_highlight,
        $upload_time1,
        $upload_time2
    );
}

if (!$has_code_section && !$has_text_section) {
    echo $OUTPUT->notification(get_string('no_text', 'plagiarism_samost'), 'warning');
}

echo $OUTPUT->footer();
