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
 * Cron task — processes plagiarism_samost analysis queue.
 *
 * Picks up to 100 pending queue records per run (once per minute),
 * groups them by cm, runs the full analysis for each cm, and marks
 * all processed rows as done.
 *
 * @package    plagiarism_samost
 * @copyright  2026 Mikhailov D.A., RTU MIREA
 * @license    http://www.gnu.org/copyleft/gpl.html GNU GPL v3 or later
 */

namespace plagiarism_samost\task;

defined('MOODLE_INTERNAL') || die();

use plagiarism_samost\analyser;

class process_queue extends \core\task\scheduled_task {

    public function get_name(): string {
        return get_string('task_process_queue', 'plagiarism_samost');
    }

    /**
     * Main task body.
     *
     * Algorithm:
     *  1. Fetch up to BATCH_SIZE rows with status='queued', ordered by timecreated ASC.
     *  2. Group them by cm (course module).
     *  3. For each cm: extract text for every user in the group, run pairwise analysis.
     *  4. Mark processed rows as status='done'.
     *  5. Mark rows that threw an exception as status='error'.
     */
    public function execute(): void {
        global $DB, $CFG;

        $batch_size = 100;

        require_once($CFG->dirroot . '/mod/assign/locallib.php');

        $rows = $DB->get_records_select(
            'plagiarism_samost_queue',
            "status = 'queued'",
            [],
            'timecreated ASC',
            '*',
            0,
            $batch_size
        );

        if (empty($rows)) {
            mtrace('plagiarism_samost: queue is empty, nothing to do.');
            return;
        }

        mtrace('plagiarism_samost: processing ' . count($rows) . ' queued item(s).');

        $by_cm = [];
        foreach ($rows as $row) {
            $by_cm[$row->cm][] = $row;
        }

        $shingle_size = (int)(get_config('plagiarism_samost', 'shingle_size') ?: 7);
        $fs           = get_file_storage();

        foreach ($by_cm as $cmid => $cm_rows) {
            $row_ids = array_column($cm_rows, 'id');

            [$in_sql, $in_params] = $DB->get_in_or_equal($row_ids);
            $DB->execute(
                "UPDATE {plagiarism_samost_queue} SET status='processing', timemodified=? WHERE id $in_sql",
                array_merge([time()], $in_params)
            );

            try {
                [$course, $cm] = get_course_and_cm_from_cmid((int)$cmid);
                $context        = \context_module::instance((int)$cmid);

                $anal = new analyser($shingle_size);

                $userids_to_process = array_unique(array_column($cm_rows, 'userid'));

                $first_row         = reset($cm_rows);
                $scope_groupid     = (int)($first_row->groupid ?? 0);
                $scope_mode        = $first_row->mode ?? 'within';
		$scope_compare_ids = [];
                if (!empty($first_row->compare_userids)) {
                    $decoded = json_decode($first_row->compare_userids, true);
                    if (is_array($decoded)) {
                        $scope_compare_ids = array_map('intval', $decoded);
                    }
                }

                $all_userids_to_cache = $userids_to_process;
                if (!empty($scope_compare_ids)) {
                    $all_userids_to_cache = array_unique(array_merge($userids_to_process, $scope_compare_ids));
                }
                [$u_sql, $u_params] = $DB->get_in_or_equal($all_userids_to_cache);
                $DB->execute(
                    "DELETE FROM {plagiarism_samost_files} WHERE cm = ? AND userid $u_sql",
                    array_merge([(int)$cmid], $u_params)
                );

                [$u_sql2, $u_params2] = $DB->get_in_or_equal($all_userids_to_cache);
                $submissions = $DB->get_records_select(
                    'assign_submission',
                    "assignment = ? AND status = 'submitted' AND latest = 1 AND userid $u_sql2",
                    array_merge([$cm->instance], $u_params2)
                );

                foreach ($submissions as $submission) {
                    $files = $fs->get_area_files(
                        $context->id,
                        'assignsubmission_file',
                        'submission_files',
                        $submission->id,
                        'filename',
                        false
                    );
                    foreach ($files as $file) {
                        $anal->cache_file((int)$cmid, $submission->userid, $file);
                    }
                }

                $anal->run_for_cm_with_scope(
                    (int)$cmid,
                    $userids_to_process,
                    $scope_mode,
                    $scope_compare_ids
                );

                $DB->execute(
                    "UPDATE {plagiarism_samost_queue} SET status='done', timemodified=? WHERE id $in_sql",
                    array_merge([time()], $in_params)
                );

                mtrace("  cm={$cmid}: " . count($userids_to_process) . " user(s) analysed.");

            } catch (\Throwable $e) {
                $DB->execute(
                    "UPDATE {plagiarism_samost_queue} SET status='error', timemodified=? WHERE id $in_sql",
                    array_merge([time()], $in_params)
                );
                mtrace("  cm={$cmid}: ERROR — " . $e->getMessage());
            }
        }

        mtrace('plagiarism_samost: queue batch finished.');
    }
}
