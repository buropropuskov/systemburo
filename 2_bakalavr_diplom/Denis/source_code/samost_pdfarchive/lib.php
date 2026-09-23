<?php
/**
 * @package    plagiarism_samost
 * @copyright  2026 Mikhailov D.A., RTU MIREA
 * @license    http://www.gnu.org/copyleft/gpl.html GNU GPL v3 or later
 */

defined('MOODLE_INTERNAL') || die();

global $CFG;

require_once($CFG->dirroot . '/plagiarism/lib.php');

class plagiarism_plugin_samost extends plagiarism_plugin {

    public function get_settings_form($mform) {
        $mform->addElement(
            'advcheckbox',
            'samost_enabled',
            get_string('enabled', 'plagiarism_samost')
        );
        $mform->addHelpButton('samost_enabled', 'enabled', 'plagiarism_samost');
        $mform->setDefault('samost_enabled', 0);

        $mform->addElement(
            'text',
            'samost_threshold',
            get_string('threshold', 'plagiarism_samost'),
            ['size' => 4]
        );
        $mform->addHelpButton('samost_threshold', 'threshold', 'plagiarism_samost');
        $mform->setType('samost_threshold', PARAM_INT);
        $mform->setDefault('samost_threshold', 70);

        $mform->addElement(
            'text',
            'samost_shingle_size',
            get_string('shingle_size', 'plagiarism_samost'),
            ['size' => 4]
        );
        $mform->addHelpButton('samost_shingle_size', 'shingle_size', 'plagiarism_samost');
        $mform->setType('samost_shingle_size', PARAM_INT);
        $mform->setDefault('samost_shingle_size', 7);
    }

    public function save_settings($data) {
        set_config('enabled',      !empty($data->samost_enabled)      ? 1 : 0, 'plagiarism_samost');
        set_config('threshold',    (int) ($data->samost_threshold    ?? 70),   'plagiarism_samost');
        set_config('shingle_size', (int) ($data->samost_shingle_size ?? 7),    'plagiarism_samost');
        return true;
    }

    public function get_form_elements_module($mform, $context, $modulename = '') {
        if (!empty($modulename) && $modulename !== 'assign' && $modulename !== 'mod_assign') {
            return;
        }

        $mform->addElement('header', 'samost_header', get_string('pluginname', 'plagiarism_samost'));

        $mform->addElement(
            'advcheckbox',
            'samost_use',
            get_string('use_samost', 'plagiarism_samost')
        );
        $mform->addHelpButton('samost_use', 'use_samost', 'plagiarism_samost');
    }

    public function save_form_elements($data, $cmid) {
        global $DB;

        $value = !empty($data->samost_use) ? 1 : 0;

        $existing = $DB->get_record('plagiarism_samost_config', ['cm' => $cmid, 'name' => 'use_samost']);
        if ($existing) {
            $existing->value = $value;
            $DB->update_record('plagiarism_samost_config', $existing);
        } else {
            $DB->insert_record('plagiarism_samost_config', (object)[
                'cm'    => $cmid,
                'name'  => 'use_samost',
                'value' => $value,
            ]);
        }
    }

    /**
     * Returns JS to display button.
     *
     * @param  array $linkarray  Information about the submission:
     *                           - cmid      (int)
     *                           - userid    (int)
     *                           - file      (stored_file|null)
     *                           - content   (string|null)
     */
    public function get_links($linkarray) {
        global $PAGE, $DB;

        $cmid = (int)($linkarray['cmid'] ?? 0);
        if (!$cmid) return '';

        $pagetype = $PAGE->pagetype ?? '';
        if (!in_array($pagetype, ['mod-assign-grader', 'mod-assign-grading'], true)) return '';
        if (!get_config('plagiarism_samost', 'enabled')) return '';

        $config = $DB->get_record('plagiarism_samost_config', ['cm' => $cmid, 'name' => 'use_samost']);
        if (!$config || !$config->value) return '';

        $context = context_module::instance($cmid);
        if (!has_capability('plagiarism/samost:viewreport', $context)) return '';

        static $injected = [];
        if (!isset($injected[$cmid])) {
            $injected[$cmid] = true;
            $url = (new moodle_url('/plagiarism/samost/report.php', ['cmid' => $cmid]))->out(false);

            $label = get_string('viewreport', 'plagiarism_samost');
            
            $PAGE->requires->js_amd_inline("
                require([], function() {
                    function inject() {
                        var target = document.querySelector('.navitem.ms-sm-auto');
                        if (!target || document.getElementById('samost-nav-btn')) return;
                        var wrapper = document.createElement('div');
                        wrapper.className = 'navitem align-self-center me-2';
                        wrapper.innerHTML = '<a id=\"samost-nav-btn\" href=\"$url\" class=\"btn btn-primary btn-sm\">$label</a>';
                        target.parentNode.insertBefore(wrapper, target);
                    }
                    if (document.readyState === 'loading') {
                        document.addEventListener('DOMContentLoaded', inject);
                    } else {
                        inject();
                    }
                });
            ");
        }

        return '';
    }

    // TODO
    public function get_file_results($eventdata) {
        return true;
    }
}

function plagiarism_samost_coursemodule_standard_elements($formwrapper, $mform) {
    global $DB, $CFG;

    if ($formwrapper->get_current()->modulename !== 'assign') {
        return;
    }

    if (empty($CFG->enableplagiarism)) {
        return;
    }

    $mform->addElement('header', 'samost_header', get_string('pluginname', 'plagiarism_samost'));
    $mform->addElement(
        'advcheckbox',
        'samost_use',
        get_string('use_samost', 'plagiarism_samost')
    );
    $mform->addHelpButton('samost_use', 'use_samost', 'plagiarism_samost');

    $cmid = $formwrapper->get_current()->coursemodule ?? 0;
    if ($cmid) {
        $rec = $DB->get_record('plagiarism_samost_config', ['cm' => $cmid, 'name' => 'use_samost']);
        if ($rec) {
            $mform->setDefault('samost_use', (int) $rec->value);
        }
    }
}

function plagiarism_samost_coursemodule_edit_post_actions($data, $formwrapper) {
    global $DB;

    $cmid  = $data->coursemodule ?? 0;
    if (!$cmid) {
        return $data;
    }

    $value    = !empty($data->samost_use) ? 1 : 0;
    $existing = $DB->get_record('plagiarism_samost_config', ['cm' => $cmid, 'name' => 'use_samost']);

    if ($existing) {
        $existing->value = $value;
        $DB->update_record('plagiarism_samost_config', $existing);
    } else {
        $DB->insert_record('plagiarism_samost_config', (object)[
            'cm'    => $cmid,
            'name'  => 'use_samost',
            'value' => $value,
        ]);
    }

    return $data;
}

