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
 * Admin settings form for plagiarism_samost.
 *
 * @package    plagiarism_samost
 * @copyright  2026 Mikhailov D.A., RTU MIREA
 * @license    http://www.gnu.org/copyleft/gpl.html GNU GPL v3 or later
 */

namespace plagiarism_samost\form;

defined('MOODLE_INTERNAL') || die();

require_once($CFG->libdir . '/formslib.php');

class admin_settings extends \moodleform {

    public function definition(): void {
        $mform = $this->_form;

        $mform->addElement('header', 'general', get_string('pluginname', 'plagiarism_samost'));

        $mform->addElement('advcheckbox', 'samost_enabled', get_string('enabled', 'plagiarism_samost'));
        $mform->addHelpButton('samost_enabled', 'enabled', 'plagiarism_samost');
        $mform->setDefault('samost_enabled', 0);

        $mform->addElement('text', 'samost_threshold', get_string('threshold', 'plagiarism_samost'), ['size' => 4]);
        $mform->addHelpButton('samost_threshold', 'threshold', 'plagiarism_samost');
        $mform->setType('samost_threshold', PARAM_INT);
        $mform->addRule('samost_threshold', null, 'numeric', null, 'client');
        $mform->setDefault('samost_threshold', 70);

        $mform->addElement('text', 'samost_shingle_size', get_string('shingle_size', 'plagiarism_samost'), ['size' => 4]);
        $mform->addHelpButton('samost_shingle_size', 'shingle_size', 'plagiarism_samost');
        $mform->setType('samost_shingle_size', PARAM_INT);
        $mform->addRule('samost_shingle_size', null, 'numeric', null, 'client');
        $mform->setDefault('samost_shingle_size', 7);

        $this->add_action_buttons();
    }

    public function validation($data, $files): array {
        $errors = parent::validation($data, $files);

        if (isset($data['samost_threshold'])) {
            $t = (int) $data['samost_threshold'];
            if ($t < 0 || $t > 100) {
                $errors['samost_threshold'] = 'Must be between 0 and 100.';
            }
        }

        if (isset($data['samost_shingle_size'])) {
            $s = (int) $data['samost_shingle_size'];
            if ($s < 2 || $s > 50) {
                $errors['samost_shingle_size'] = 'Must be between 2 and 50.';
            }
        }

        return $errors;
    }
}
