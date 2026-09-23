<?php

/**
 * @package    plagiarism_samost
 * @copyright  2026 Mikhailov D.A., RTU MIREA
 * @license    http://www.gnu.org/copyleft/gpl.html GNU GPL v3 or later
 */

require_once(__DIR__ . '/../../config.php');
require_once(__DIR__ . '/lib.php');

$cmid             = required_param('cmid', PARAM_INT);
$action           = optional_param('action', '', PARAM_ALPHA);
$groupid          = optional_param('group', -1, PARAM_INT);
$mode             = optional_param('mode',  '',  PARAM_ALPHA);
$include_archived = optional_param('archived', -1, PARAM_INT);

[$course, $cm] = get_course_and_cm_from_cmid($cmid);
$context = context_module::instance($cmid);

require_login($course);
require_capability('plagiarism/samost:viewreport', $context);

if ($cm->modname !== 'assign') {
    throw new moodle_exception('error_notassign', 'plagiarism_samost');
}

if ($groupid === -1) {
    $groupid = (int)get_user_preferences('plagiarism_samost_group_' . $cmid, 0);
}
if ($mode === '') {
    $mode = get_user_preferences('plagiarism_samost_mode_' . $cmid, 'within');
}
if (!in_array($mode, ['within', 'cross'])) {
    $mode = 'within';
}
if ($include_archived === -1) {
    $include_archived = (int)get_user_preferences('plagiarism_samost_archived_' . $cmid, 0);
}
$include_archived = (int)(bool)$include_archived;

$PAGE->set_url(new moodle_url('/plagiarism/samost/report.php', ['cmid' => $cmid]));
$PAGE->set_context($context);
$PAGE->set_course($course);
$PAGE->set_cm($cm);
$PAGE->set_title(get_string('pluginname', 'plagiarism_samost'));
$PAGE->set_heading($course->fullname);
$PAGE->activityheader->disable();
$PAGE->requires->css('/plagiarism/samost/styles.css');


if ($action === 'feedback') {
    require_sesskey();
    $message    = optional_param('message', '', PARAM_TEXT);
    $pair_uid1  = optional_param('pair_uid1', 0, PARAM_INT);
    $pair_uid2  = optional_param('pair_uid2', 0, PARAM_INT);
    $snap_code  = optional_param('snap_code', '', PARAM_RAW);
    $snap_text  = optional_param('snap_text', '', PARAM_RAW);

    $message = trim($message);
    if (strlen($message) > 0) {
        $record = (object)[
            'cm'          => (int)$cmid,
            'userid'      => (int)$USER->id,
            'message'     => $message,
            'timecreated' => time(),
        ];
        if ($pair_uid1 && $pair_uid2) {
            $record->pair_userid1         = $pair_uid1;
            $record->pair_userid2         = $pair_uid2;
            $record->snap_similarity_code = is_numeric($snap_code) ? (float)$snap_code : null;
            $record->snap_similarity_text = is_numeric($snap_text) ? (float)$snap_text : null;
        }
        $DB->insert_record('plagiarism_samost_feedback', $record);
    }

    $redirect_uid1 = optional_param('redirect_uid1', 0, PARAM_INT);
    $redirect_uid2 = optional_param('redirect_uid2', 0, PARAM_INT);
    if ($redirect_uid1 && $redirect_uid2) {
        $back = new moodle_url('/plagiarism/samost/pair.php', [
            'cmid' => $cmid, 'uid1' => $redirect_uid1, 'uid2' => $redirect_uid2,
        ]);
    } else {
        $back = new moodle_url('/plagiarism/samost/report.php', ['cmid' => $cmid]);
    }
    redirect($back, 'Благодарим за отзыв! Обратная связь не предусмотрена, однако все отзывы анализируются и учитываются при доработке системы.', null, \core\output\notification::NOTIFY_SUCCESS);
}

if ($action === 'run' && has_capability('plagiarism/samost:runanalysis', $context)) {
    require_sesskey();

    if (!in_array($mode, ['within', 'cross'])) {
        $mode = 'within';
    }

    $all_submissions = $DB->get_records_select(
        'assign_submission',
        "assignment = :assignment AND status = 'submitted' AND latest = 1",
        ['assignment' => $cm->instance],
        '',
        'userid'
    );
    $all_userids_raw = array_map(fn($s) => (int)$s->userid, $all_submissions);

    $enrolled     = get_enrolled_users($context, '', 0, 'u.id');
    $enrolled_ids = array_map(fn($u) => (int)$u->id, $enrolled);

    $all_userids = array_values(array_intersect($all_userids_raw, $enrolled_ids));

    if ($groupid > 0) {
        $group_members  = groups_get_members($groupid, 'u.id');
        $group_userids  = array_map(fn($u) => (int)$u->id, $group_members);
        $target_userids = array_values(array_intersect($all_userids, $group_userids));
    } else {
        $target_userids = array_values($all_userids);
        $mode = 'within';
    }


    if (empty($target_userids)) {
        redirect(
            new moodle_url('/plagiarism/samost/report.php', [
                'cmid' => $cmid, 'group' => $groupid, 'mode' => $mode,
            ]),
            get_string('no_submissions_in_group', 'plagiarism_samost'),
            null,
            \core\output\notification::NOTIFY_WARNING
        );
    }

    if ($mode === 'within' && count($target_userids) < 2) {
        redirect(
            new moodle_url('/plagiarism/samost/report.php', [
                'cmid' => $cmid, 'group' => $groupid, 'mode' => $mode,
            ]),
            get_string('not_enough_submissions', 'plagiarism_samost'),
            null,
            \core\output\notification::NOTIFY_WARNING
        );
    }

    $compare_userids_json = null;

    if ($mode === 'cross') {
        // cross: сравниваем target с другими группами (+ архивные если включено).
        $compare_pool  = $include_archived ? $all_userids_raw : $all_userids;
        $other_userids = array_values(array_diff($compare_pool, $target_userids));
        if (empty($other_userids)) {
            redirect(
                new moodle_url('/plagiarism/samost/report.php', [
                    'cmid' => $cmid, 'group' => $groupid,
                    'mode' => $mode, 'archived' => $include_archived,
                ]),
                get_string('no_other_groups_submissions', 'plagiarism_samost'),
                null,
                \core\output\notification::NOTIFY_WARNING
            );
        }
        $compare_userids_json = json_encode($other_userids);
    }

    if ($mode === 'within' && $include_archived && $groupid > 0) {
        // within + archived: target сравниваются между собой И с архивными.
        // Архивные = сдали задание, но НЕ зачислены сейчас.
        // Архивные в очередь НЕ попадают — они только в compare_userids.
        $archived_userids = array_values(array_diff($all_userids_raw, $enrolled_ids));
        if (!empty($archived_userids)) {
            $compare_userids_json = json_encode($archived_userids);
        }
    }

    $now = time();

    foreach ($target_userids as $uid) {
        $existing = $DB->record_exists_select(
            'plagiarism_samost_queue',
            "cm = ? AND userid = ? AND status IN ('queued','processing')",
            [(int)$cmid, $uid]
        );
        if ($existing) {
            continue;
        }
        $DB->insert_record('plagiarism_samost_queue', (object)[
            'cm'              => (int)$cmid,
            'userid'          => $uid,
            'status'          => 'queued',
            'groupid'         => $groupid,
            'mode'            => $mode,
            'compare_userids' => $compare_userids_json,
            'timecreated'     => $now,
            'timemodified'    => $now,
        ]);
    }

    set_user_preference('plagiarism_samost_group_'    . $cmid, $groupid);
    set_user_preference('plagiarism_samost_mode_'     . $cmid, $mode);
    set_user_preference('plagiarism_samost_archived_' . $cmid, $include_archived);

    redirect(
        new moodle_url('/plagiarism/samost/report.php', [
            'cmid' => $cmid, 'group' => $groupid,
            'mode' => $mode, 'archived' => $include_archived,
        ]),
        get_string('analysisqueued', 'plagiarism_samost'),
        null,
        \core\output\notification::NOTIFY_SUCCESS
    );
}

$pending_count = $DB->count_records_select(
    'plagiarism_samost_queue',
    "cm = ? AND status IN ('queued','processing')",
    [(int)$cmid]
);

$threshold = (int)(get_config('plagiarism_samost', 'threshold') ?: 70);
$results   = $DB->get_records(
    'plagiarism_samost_results',
    ['cm' => $cmid],
    'COALESCE(similarity_code, similarity_text) DESC'
);

$userids = [];
foreach ($results as $r) {
    $userids[$r->userid1] = $r->userid1;
    $userids[$r->userid2] = $r->userid2;
}

$users = !empty($userids)
    ? $DB->get_records_list('user', 'id', $userids, '', 'id,firstname,lastname,firstnamephonetic,lastnamephonetic,middlename,alternatename,username')
    : [];

$user_groups = [];
if (!empty($userids)) {
    foreach ($userids as $uid) {
        $groups = groups_get_all_groups($course->id, $uid, $cm->groupingid);
        if (!empty($groups)) {
            $user_groups[$uid] = implode(', ', array_map(fn($g) => format_string($g->name), $groups));
        }
    }
}

$all_groups = groups_get_all_groups($course->id, 0, $cm->groupingid);

echo $OUTPUT->header();
echo $OUTPUT->heading(
    get_string('report_heading', 'plagiarism_samost', format_string($cm->name))
);

$currenttab = 'report';
require_once(__DIR__ . '/tabs/header.php');


if (has_capability('plagiarism/samost:runanalysis', $context)) {

    $report_base = new moodle_url('/plagiarism/samost/report.php', ['cmid' => $cmid]);

    $selected_group = ($groupid > 0 && isset($all_groups[$groupid])) ? $all_groups[$groupid] : null;
    if (!$selected_group && !empty($all_groups)) {
        $selected_group = reset($all_groups);
        $groupid = (int)$selected_group->id;
    }
    $selected_label = $selected_group ? format_string($selected_group->name) : '';

    $group_items_html = '';
    if (!empty($all_groups)) {
        foreach ($all_groups as $group) {
            $active  = ($groupid === (int)$group->id) ? ' samost-item-active' : '';
            $gname   = format_string($group->name);
            $letter  = mb_strtoupper(mb_substr($gname, 0, 1, 'UTF-8'), 'UTF-8');
            $group_items_html .= '<a href="' . (new moodle_url($report_base, ['group' => $group->id]))->out(false) . '"'
                . ' class="samost-item' . $active . '">'
                . '<span class="samost-avatar">' . $letter . '</span>'
                . '<span>' . $gname . '</span>'
                . '</a>';
        }
    }

    $btn_avatar_inner = mb_strtoupper(mb_substr($selected_label, 0, 1, 'UTF-8'), 'UTF-8');

    $runurl = new moodle_url('/plagiarism/samost/report.php', [
        'cmid'     => $cmid,
        'action'   => 'run',
        'group'    => $groupid,
        'mode'     => $mode,
        'archived' => $include_archived,
        'sesskey'  => sesskey(),
    ]);

    $mode_label      = ($mode === 'cross') ? 'Группа с другими группами' : 'Внутри группы';
    $mode_icon_class = ($mode === 'cross') ? 'samost-mode-icon samost-mode-icon--cross' : 'samost-mode-icon samost-mode-icon--within';
    $mode_icon_fa    = ($mode === 'cross') ? 'fa-random' : 'fa-compress';
    $within_active   = ($mode === 'within') ? ' samost-item-active' : '';
    $cross_active    = ($mode === 'cross')  ? ' samost-item-active' : '';


    echo '<div class="samost-toolbar">';


    echo '<div>';
    echo '<span class="samost-label">Изолированные группы</span>';
    echo '<div class="samost-group-pos" id="samost-group-pos">';
    echo '<button type="button" class="samost-btn" id="samost-group-btn">';
    echo '<span class="samost-avatar">' . $btn_avatar_inner . '</span>';
    echo '<span class="samost-btn-value">' . $selected_label . '</span>';
    echo '<i class="fa fa-chevron-down samost-btn-chevron" aria-hidden="true"></i>';
    echo '</button>';
    echo '<div class="samost-menu" id="samost-group-menu">' . $group_items_html . '</div>';
    echo '</div></div>';

    echo '<div>';
    echo '<span class="samost-label">Режим сравнения</span>';
    echo '<div class="samost-group-pos" id="samost-mode-pos">';
    echo '<button type="button" class="samost-btn" id="samost-mode-btn">';
    echo '<span class="' . $mode_icon_class . '" id="samost-mode-icon"><i class="fa ' . $mode_icon_fa . '" aria-hidden="true"></i></span>';
    echo '<span class="samost-btn-value" id="samost-mode-label">' . $mode_label . '</span>';
    echo '<i class="fa fa-chevron-down samost-btn-chevron" aria-hidden="true"></i>';
    echo '</button>';
    echo '<div class="samost-menu" id="samost-mode-menu" style="min-width:290px;">';
    echo '<a href="#" class="samost-item' . $within_active . '" data-mode="within" data-label="Внутри группы" data-icon="fa-compress" data-iconclass="samost-mode-icon--within">';
    echo '<span class="samost-mode-icon samost-mode-icon--within"><i class="fa fa-compress" aria-hidden="true"></i></span>';
    echo '<span><strong>Внутри группы</strong><span class="samost-item-desc">Студенты выбранной группы сравниваются между собой</span></span>';
    echo '</a>';
    echo '<div class="samost-divider"></div>';
    echo '<a href="#" class="samost-item' . $cross_active . '" data-mode="cross" data-label="Группа с другими группами" data-icon="fa-random" data-iconclass="samost-mode-icon--cross">';
    echo '<span class="samost-mode-icon samost-mode-icon--cross"><i class="fa fa-random" aria-hidden="true"></i></span>';
    echo '<span><strong>Группа с другими группами</strong><span class="samost-item-desc">Студенты группы сравниваются со студентами остальных групп</span></span>';
    echo '</a>';
    echo '</div></div></div>';

    $archived_checked  = $include_archived ? 'checked' : '';
    $archived_style    = '';
    echo '<div class="samost-archived-wrap" id="samost-archived-wrap" ' . $archived_style . '>';
    echo '<label class="samost-archived-toggle">';
    echo '<input type="checkbox" id="samost-archived-chk" ' . $archived_checked . '>';
    echo '<span class="samost-toggle-slider"></span>';
    echo '<span class="samost-toggle-text">Архивные студенты</span>';
    echo '</label>';
    echo '</div>';

    echo '<a href="' . $runurl->out(false) . '" class="samost-run-btn" id="samost-run-link">';
    echo '<i class="fa fa-play" aria-hidden="true"></i> ';
    echo get_string('runanalysis', 'plagiarism_samost');
    echo '</a>';

    echo '<div style="flex:1;"></div>';
    echo '<button type="button" class="samost-feedback-btn" id="samost-feedback-open">';
    echo '<i class="fa fa-comment-o" aria-hidden="true"></i> Оставить отзыв';
    echo '</button>';

    echo '</div>'; 

    $feedback_url = new moodle_url('/plagiarism/samost/report.php', [
        'cmid'    => $cmid,
        'action'  => 'feedback',
        'sesskey' => sesskey(),
    ]);
    echo '
<div class="samost-modal-overlay" id="samost-modal-overlay">
    <div class="samost-modal" role="dialog" aria-modal="true" aria-labelledby="samost-modal-title">
        <div class="samost-modal-header">
            <span id="samost-modal-title" class="samost-modal-title">Оставить отзыв</span>
            <button type="button" class="samost-modal-close" id="samost-modal-close" aria-label="Закрыть">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
            </button>
        </div>
        <form method="post" action="' . $feedback_url->out(false) . '" class="samost-modal-body">
            <label class="samost-modal-label" for="samost-feedback-text">Ваш отзыв</label>
            <textarea
                id="samost-feedback-text"
                name="message"
                class="samost-modal-textarea"
                placeholder="Опишите ваш опыт использования системы: что работает корректно, какие функции требуют доработки, с какими трудностями вы столкнулись."
                rows="5"
                maxlength="2000"
                required
            ></textarea>
            <div class="samost-modal-footer">
                <button type="button" class="samost-modal-cancel" id="samost-modal-cancel">Отмена</button>
                <button type="submit" class="samost-modal-submit">
                    <i class="fa fa-paper-plane-o" aria-hidden="true"></i> Отправить
                </button>
            </div>
        </form>
    </div>
</div>';

    $PAGE->requires->js_amd_inline("
        require([], function() {
            function makeDropdown(btnId, menuId, posId) {
                var btn  = document.getElementById(btnId);
                var menu = document.getElementById(menuId);
                var pos  = document.getElementById(posId);
                if (!btn || !menu || !pos) return null;
                function open()   { menu.classList.add('samost-visible'); btn.classList.add('samost-open'); }
                function close()  { menu.classList.remove('samost-visible'); btn.classList.remove('samost-open'); }
                function toggle() { menu.classList.contains('samost-visible') ? close() : open(); }
                btn.addEventListener('click', function(e) { e.stopPropagation(); toggle(); });
                document.addEventListener('click', function(e) { if (!pos.contains(e.target)) close(); });
                document.addEventListener('keydown', function(e) { if (e.key === 'Escape') close(); });
                return { close: close };
            }

            makeDropdown('samost-group-btn', 'samost-group-menu', 'samost-group-pos');
            var modeDd    = makeDropdown('samost-mode-btn', 'samost-mode-menu', 'samost-mode-pos');
            var modeItems = document.querySelectorAll('#samost-mode-menu .samost-item');
            var runLink   = document.getElementById('samost-run-link');

            modeItems.forEach(function(item) {
                item.addEventListener('click', function(e) {
                    e.preventDefault();
                    var newMode      = item.getAttribute('data-mode');
                    var newLabel     = item.getAttribute('data-label');
                    var newIcon      = item.getAttribute('data-icon');
                    var newIconClass = item.getAttribute('data-iconclass');

                    modeItems.forEach(function(i) { i.classList.remove('samost-item-active'); });
                    item.classList.add('samost-item-active');

                    document.getElementById('samost-mode-label').textContent = newLabel;
                    var modeIconEl = document.getElementById('samost-mode-icon');
                    modeIconEl.className = 'samost-mode-icon ' + newIconClass;
                    modeIconEl.innerHTML = '<i class=\"fa ' + newIcon + '\" aria-hidden=\"true\"></i>';

                    var archivedWrap = document.getElementById('samost-archived-wrap');
                    if (archivedWrap) {
                        archivedWrap.style.opacity = '1';
                        archivedWrap.style.pointerEvents = 'auto';
                    }
                    if (runLink) {
                        var href = runLink.getAttribute('href');
                        href = href.replace(/([?&]mode=)[^&]*/, '\$1' + newMode);
                        runLink.setAttribute('href', href);
                    }
                    if (modeDd) modeDd.close();
                });
            });

            var archivedChk = document.getElementById('samost-archived-chk');
            if (archivedChk) {
                archivedChk.addEventListener('change', function() {
                    if (runLink) {
                        var href = runLink.getAttribute('href');
                        href = href.replace(/([?&]archived=)[^&]*/, '\$1' + (archivedChk.checked ? '1' : '0'));
                        runLink.setAttribute('href', href);
                    }
                });
            }
        });
    ");

    $PAGE->requires->js_amd_inline("
        require([], function() {
            var overlay = document.getElementById('samost-modal-overlay');
            var openBtn = document.getElementById('samost-feedback-open');
            var closeBtn = document.getElementById('samost-modal-close');
            var cancelBtn = document.getElementById('samost-modal-cancel');
            var textarea = document.getElementById('samost-feedback-text');

            if (!overlay || !openBtn) return;

            function openModal() {
                overlay.classList.add('samost-modal-visible');
                setTimeout(function() { if (textarea) textarea.focus(); }, 50);
            }
            function closeModal() {
                overlay.classList.remove('samost-modal-visible');
            }

            openBtn.addEventListener('click', openModal);
            if (closeBtn)  closeBtn.addEventListener('click', closeModal);
            if (cancelBtn) cancelBtn.addEventListener('click', closeModal);

            overlay.addEventListener('click', function(e) {
                if (e.target === overlay) closeModal();
            });
            document.addEventListener('keydown', function(e) {
                if (e.key === 'Escape') closeModal();
            });
        });
    ");
}

if ($pending_count > 0) {

    $PAGE->requires->js_amd_inline("
        window.setTimeout(function() { window.location.reload(); }, 15000);
    ");

    $spinner = html_writer::tag(
        'span', '',
        ['class' => 'spinner-border spinner-border-sm me-2', 'role' => 'status', 'aria-hidden' => 'true']
    );
    echo html_writer::div(
        $spinner . get_string('queue_waiting', 'plagiarism_samost', $pending_count),
        'alert alert-info d-flex align-items-center mb-3'
    );

} else if (!empty($results)) {

    $has_code = false;
    $has_text = false;
    foreach ($results as $r) {
        if ($r->similarity_code !== null) {
            $has_code = true;
        }
        if ($r->similarity_text !== null) {
            $has_text = true;
        }
        if ($has_code && $has_text) {
            break;
        }
    }

    $badge_html = function(?float $val) use ($threshold): string {
        if ($val === null) {
            return '<span class="samost-badge samost-badge--empty">—</span>';
        }
        $pct     = number_format($val, 1) . '%';
        $is_high = $val >= $threshold;
        $cls     = $is_high ? 'samost-badge samost-badge--high' : 'samost-badge samost-badge--ok';
        $icon    = $is_high ? '⚠ ' : '';
        return '<span class="' . $cls . '">' . $icon . $pct . '</span>';
    };

    $avatar_html = function(string $name): string {
        $parts = preg_split('/\s+/', trim($name));
        $init  = '';
        foreach (array_slice($parts, 0, 2) as $p) {
            $init .= mb_strtoupper(mb_substr($p, 0, 1, 'UTF-8'), 'UTF-8');
        }
        return '<span class="samost-row-avatar">' . htmlspecialchars($init ?: '?') . '</span>';
    };

    $head_cols  = '<th>' . get_string('student1', 'plagiarism_samost') . '</th>';
    $head_cols .= '<th>' . get_string('student2', 'plagiarism_samost') . '</th>';
    if ($has_code) $head_cols .= '<th>' . get_string('similarity_code', 'plagiarism_samost') . '</th>';
    if ($has_text) $head_cols .= '<th>' . get_string('similarity_text', 'plagiarism_samost') . '</th>';
    $head_cols .= '<th></th>';

    echo '<div class="samost-table-wrap">';
    echo '<table class="samost-table">';
    echo '<thead><tr>' . $head_cols . '</tr></thead>';
    echo '<tbody>';

    $target_group_members = [];
    if ($groupid > 0) {
        $gm = groups_get_members($groupid, 'u.id');
        $target_group_members = array_keys($gm);
    }

    foreach ($results as $result) {
        if ($groupid > 0 && !empty($target_group_members)) {
            $uid1_in_target = in_array((int)$result->userid1, $target_group_members);
            $uid2_in_target = in_array((int)$result->userid2, $target_group_members);
            if (!$uid1_in_target && $uid2_in_target) {
                [$result->userid1, $result->userid2] = [$result->userid2, $result->userid1];
            }
        }

        $name1 = isset($users[$result->userid1]) ? fullname($users[$result->userid1]) : "User {$result->userid1}";
        $name2 = isset($users[$result->userid2]) ? fullname($users[$result->userid2]) : "User {$result->userid2}";

        $username1 = isset($users[$result->userid1]) ? $users[$result->userid1]->username : '';
        $username2 = isset($users[$result->userid2]) ? $users[$result->userid2]->username : '';

        $group1 = !empty($user_groups[$result->userid1]) ? '<span class="samost-row-group">' . htmlspecialchars($user_groups[$result->userid1]) . '</span>' : '';
        $group2 = !empty($user_groups[$result->userid2]) ? '<span class="samost-row-group">' . htmlspecialchars($user_groups[$result->userid2]) . '</span>' : '';

        $uname1 = $username1 ? '<span class="samost-row-username">' . htmlspecialchars($username1) . '</span>' : '';
        $uname2 = $username2 ? '<span class="samost-row-username">' . htmlspecialchars($username2) . '</span>' : '';

        $detail_url = new moodle_url('/plagiarism/samost/pair.php', [
            'cmid' => $cmid, 'uid1' => $result->userid1, 'uid2' => $result->userid2,
        ]);

        $is_high_row = ($result->similarity_code >= $threshold) || ($result->similarity_text >= $threshold);
        $row_class   = $is_high_row ? ' samost-row--high' : '';

        echo '<tr class="samost-row' . $row_class . '">';
        echo '<td><div class="samost-row-user">' . $avatar_html($name1) . '<div><span class="samost-row-name">' . htmlspecialchars($name1) . '</span>' . $uname1 . $group1 . '</div></div></td>';
        echo '<td><div class="samost-row-user">' . $avatar_html($name2) . '<div><span class="samost-row-name">' . htmlspecialchars($name2) . '</span>' . $uname2 . $group2 . '</div></div></td>';
        if ($has_code) echo '<td>' . $badge_html($result->similarity_code) . '</td>';
        if ($has_text) echo '<td>' . $badge_html($result->similarity_text) . '</td>';
        echo '<td><a href="' . $detail_url->out(false) . '" class="samost-view-btn">' . get_string('viewpair', 'plagiarism_samost') . '</a></td>';
        echo '</tr>';
    }

    echo '</tbody></table></div>';

} else {
    echo $OUTPUT->notification(get_string('report_notready', 'plagiarism_samost'), 'info');
}

echo $OUTPUT->footer();
