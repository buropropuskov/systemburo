<?php
/*
 * @package    plagiarism_samost
 * @copyright  2026 Mikhailov D.A., RTU MIREA
 * @license    http://www.gnu.org/copyleft/gpl.html GNU GPL v3 or later
 */
defined('MOODLE_INTERNAL') || die();

$tabs = [];

if (!empty($cmid)) {
    $tabs[] = new tabobject(
        'report',
        new moodle_url('/plagiarism/samost/report.php', ['cmid' => $cmid]),
        get_string('tab_report', 'plagiarism_samost')
    );
}

$tabs[] = new tabobject(
    'info',
    new moodle_url('/plagiarism/samost/tabs/info.php', !empty($cmid) ? ['cmid' => $cmid] : []),
    get_string('tab_info', 'plagiarism_samost')
);

$currenttab = $currenttab ?? 'report';
print_tabs([$tabs], $currenttab);
