<?php
/**
 * @package    plagiarism_samost
 * @copyright  2026 Mikhailov D.A., RTU MIREA
 * @license    http://www.gnu.org/copyleft/gpl.html GNU GPL v3 or later
 */

require_once(__DIR__ . '/../../config.php');
require_once(__DIR__ . '/lib.php');

require_login();
require_capability('moodle/site:config', context_system::instance());

$page   = optional_param('page', 0, PARAM_INT);
$action = optional_param('action', '', PARAM_ALPHA);
$fid    = optional_param('fid', 0, PARAM_INT);
$per_page = 15;

$PAGE->set_url(new moodle_url('/plagiarism/samost/feedback_list.php'));
$PAGE->set_context(context_system::instance());
$PAGE->set_title('Отзывы антиплагиата');
$PAGE->set_heading('Отзывы антиплагиата');
$PAGE->requires->css('/plagiarism/samost/styles.css');

if ($action === 'comment' && $fid > 0) {
    require_sesskey();
    $comment = trim(optional_param('admin_comment', '', PARAM_TEXT));
    $DB->set_field('plagiarism_samost_feedback', 'admin_comment', $comment, ['id' => $fid]);
    redirect(
        new moodle_url('/plagiarism/samost/feedback_list.php', ['page' => $page]),
        'Комментарий сохранён.',
        null,
        \core\output\notification::NOTIFY_SUCCESS
    );
}


$total  = $DB->count_records('plagiarism_samost_feedback');
$offset = $page * $per_page;

$feedbacks = $DB->get_records_sql(
    "SELECT f.*,
            u.firstname, u.lastname, u.firstnamephonetic, u.lastnamephonetic,
            u.middlename, u.alternatename, u.username,
            u1.firstname AS fn1, u1.lastname AS ln1,
            u2.firstname AS fn2, u2.lastname AS ln2
       FROM {plagiarism_samost_feedback} f
       JOIN {user} u  ON u.id  = f.userid
  LEFT JOIN {user} u1 ON u1.id = f.pair_userid1
  LEFT JOIN {user} u2 ON u2.id = f.pair_userid2
      ORDER BY f.timecreated DESC",
    [],
    $offset,
    $per_page
);

echo $OUTPUT->header();

echo '<div class="samost-fb-page">';

echo '<div class="samost-fb-header">';
echo '<span class="samost-fb-count">' . $total . ' ' . declension($total, 'отзыв', 'отзыва', 'отзывов') . '</span>';
echo '</div>';

if (empty($feedbacks)) {
    echo $OUTPUT->notification('Отзывов пока нет.', 'info');
} else {
    foreach ($feedbacks as $fb) {
        $author     = fullname($fb);
        $username   = $fb->username;
        $date       = userdate($fb->timecreated, get_string('strftimedatetime', 'langconfig'));
        $is_pair    = !empty($fb->pair_userid1) && !empty($fb->pair_userid2);
        $has_comment = !empty($fb->admin_comment);

        $comment_url = new moodle_url('/plagiarism/samost/feedback_list.php', [
            'action'  => 'comment',
            'fid'     => $fb->id,
            'page'    => $page,
            'sesskey' => sesskey(),
        ]);

        echo '<div class="samost-fb-card' . ($has_comment ? ' samost-fb-card--commented' : '') . '">';

        echo '<div class="samost-fb-card-header">';
        echo '<div class="samost-fb-author">';
        echo '<span class="samost-fb-avatar">' . samost_fb_initials($author) . '</span>';
        echo '<div>';
        echo '<span class="samost-fb-author-name">' . htmlspecialchars($author) . '</span>';
        echo '<span class="samost-fb-author-username">' . htmlspecialchars($username) . '</span>';
        echo '</div>';
        echo '</div>';
        echo '<div class="samost-fb-meta">';
        if ($is_pair) {
            $pair_name1 = trim($fb->fn1 . ' ' . $fb->ln1);
            $pair_name2 = trim($fb->fn2 . ' ' . $fb->ln2);
            echo '<span class="samost-fb-source samost-fb-source--pair">';
            echo '<i class="fa fa-code-fork" aria-hidden="true"></i> ';
            echo htmlspecialchars($pair_name1) . ' — ' . htmlspecialchars($pair_name2);
            if ($fb->snap_similarity_code !== null || $fb->snap_similarity_text !== null) {
                echo '<span class="samost-fb-snaps">';
                if ($fb->snap_similarity_code !== null) {
                    echo '<span class="samost-fb-snap">Код: ' . number_format((float)$fb->snap_similarity_code, 1) . '%</span>';
                }
                if ($fb->snap_similarity_text !== null) {
                    echo '<span class="samost-fb-snap">Текст: ' . number_format((float)$fb->snap_similarity_text, 1) . '%</span>';
                }
                echo '</span>';
            }
            echo '</span>';
        } else {
            echo '<span class="samost-fb-source samost-fb-source--report">';
            echo '<i class="fa fa-bar-chart" aria-hidden="true"></i> Из отчёта';
            echo '</span>';
        }
        echo '<span class="samost-fb-date">' . htmlspecialchars($date) . '</span>';
        if ($is_pair) {
            $source_url = new moodle_url('/plagiarism/samost/pair.php', [
                'cmid' => $fb->cm,
                'uid1' => $fb->pair_userid1,
                'uid2' => $fb->pair_userid2,
            ]);
            $source_label = 'Открыть сравнение';
        } else {
            $source_url   = new moodle_url('/plagiarism/samost/report.php', ['cmid' => $fb->cm]);
            $source_label = 'Открыть отчёт';
        }
        echo '<a href="' . $source_url->out(false) . '" class="samost-fb-source-link" target="_blank">';
        echo '<i class="fa fa-external-link" aria-hidden="true"></i> ' . $source_label;
        echo '</a>';

        echo '</div>';
        echo '</div>';

        echo '<div class="samost-fb-message">' . nl2br(htmlspecialchars($fb->message)) . '</div>';

        if ($has_comment) {
            echo '<div class="samost-fb-admin-comment">';
            echo '<span class="samost-fb-admin-comment-label"><i class="fa fa-lock" aria-hidden="true"></i> Заметка</span>';
            echo '<div class="samost-fb-admin-comment-text">' . nl2br(htmlspecialchars($fb->admin_comment)) . '</div>';
            echo '</div>';
        }
        echo '<details class="samost-fb-comment-details">';
        echo '<summary class="samost-fb-comment-toggle">';
        echo $has_comment ? '<i class="fa fa-pencil" aria-hidden="true"></i> Изменить заметку' : '<i class="fa fa-plus" aria-hidden="true"></i> Добавить заметку';
        echo '</summary>';
        echo '<form method="post" action="' . $comment_url->out(false) . '" class="samost-fb-comment-form">';
        echo '<textarea name="admin_comment" class="samost-fb-comment-textarea" rows="3" placeholder="Личная заметка — видна только вам...">' . htmlspecialchars($fb->admin_comment ?? '') . '</textarea>';
        echo '<div class="samost-fb-comment-actions">';
        echo '<button type="submit" class="samost-fb-save-btn"><i class="fa fa-check" aria-hidden="true"></i> Сохранить</button>';
        echo '</div>';
        echo '</form>';
        echo '</details>';

        echo '</div>'; // .samost-fb-card
    }

    echo $OUTPUT->paging_bar($total, $page, $per_page, new moodle_url('/plagiarism/samost/feedback_list.php'));
}

echo '</div>'; // .samost-fb-page

echo $OUTPUT->footer();

function samost_fb_initials(string $name): string {
    $parts = preg_split('/\s+/', trim($name));
    $init  = '';
    foreach (array_slice($parts, 0, 2) as $p) {
        $init .= mb_strtoupper(mb_substr($p, 0, 1, 'UTF-8'), 'UTF-8');
    }
    return $init ?: '?';
}

function declension(int $n, string $one, string $few, string $many): string {
    $n = abs($n) % 100;
    $n1 = $n % 10;
    if ($n > 10 && $n < 20) return $many;
    if ($n1 > 1 && $n1 < 5)  return $few;
    if ($n1 === 1)             return $one;
    return $many;
}
