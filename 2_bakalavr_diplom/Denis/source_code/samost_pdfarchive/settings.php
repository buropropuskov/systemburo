<?php
/**
 * @package    plagiarism_samost
 * @copyright  2026 Mikhailov D.A., RTU MIREA
 * @license    http://www.gnu.org/copyleft/gpl.html GNU GPL v3 or later
 */

require_once(__DIR__ . '/../../config.php');
require_once(__DIR__ . '/lib.php');
require_once($CFG->libdir . '/adminlib.php');

admin_externalpage_setup('plagiarismsamost');

$plugin = new plagiarism_plugin_samost();

$mform = new \plagiarism_samost\form\admin_settings();

if ($mform->is_cancelled()) {
    redirect(new moodle_url('/admin/index.php'));
} elseif ($data = $mform->get_data()) {
    $plugin->save_settings($data);
    redirect(
        $PAGE->url,
        get_string('changessaved'),
        null,
        \core\output\notification::NOTIFY_SUCCESS
    );
}

$mform->set_data([
    'samost_enabled'      => get_config('plagiarism_samost', 'enabled'),
    'samost_threshold'    => get_config('plagiarism_samost', 'threshold') ?: 70,
    'samost_shingle_size' => get_config('plagiarism_samost', 'shingle_size') ?: 7,
]);

echo $OUTPUT->header();
echo $OUTPUT->heading(get_string('pluginname', 'plagiarism_samost'));
$mform->display();
echo $OUTPUT->footer();

