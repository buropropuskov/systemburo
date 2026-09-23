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
 * English language strings for plagiarism_samost.
 *
 * @package    plagiarism_samost
 * @copyright  2026 Mikhailov D.A., RTU MIREA
 * @license    http://www.gnu.org/copyleft/gpl.html GNU GPL v3 or later
 */

$string['pluginname']           = 'Samost – Independence Checker';
$string['samost']               = 'Samost';
$string['samost:viewreport']    = 'View similarity comparison report';
$string['samost:runanalysis']   = 'Run similarity analysis';
$string['enabled']              = 'Enable Samost plugin';
$string['enabled_help']         = 'When enabled, Samost will analyse student submissions and display similarity percentages.';
$string['threshold']            = 'Alert threshold (%)';
$string['threshold_help']       = 'Pairs with similarity above this value will be highlighted in the report.';
$string['shingle_size']         = 'Shingle size (words)';
$string['shingle_size_help']    = 'Number of consecutive words per shingle used in Jaccard comparison. Recommended: 5–10.';
$string['use_samost']           = 'Enable independence check';
$string['use_samost_help']      = 'Enable Samost similarity analysis for this assignment.';
$string['viewreport']           = 'View comparison';
$string['report_heading']       = 'Similarity report: {$a}';
$string['report_notready']      = 'Analysis has not been run yet. Click "Run analysis" to start.';
$string['runanalysis']          = 'Run analysis';
$string['analysisqueued']       = 'Analysis queued. Results will appear shortly.';
$string['queue_waiting']        = 'Analysis is queued — {$a} user(s) pending. This page will refresh automatically in 15 seconds…';
$string['queue_hint']           = 'Processing runs as a background task (once per minute, up to 100 records per run). The page will refresh itself.';
$string['task_process_queue']   = 'Samost: process analysis queue';
$string['student1']             = 'Student 1';
$string['student2']             = 'Student 2';
$string['similarity']           = 'Similarity (%)';
$string['actions']              = 'Actions';
$string['viewpair']             = 'View details';
$string['noresults']            = 'No results found.';
$string['high_similarity']      = 'High similarity';
$string['pair_heading']         = 'Comparison: {$a->user1} vs {$a->user2}';
$string['similarity_score']     = 'Similarity score: {$a}%';
$string['text_a']               = 'Submission by {$a}';
$string['no_text']              = 'No text content could be extracted from this submission.';
$string['back_to_report']       = '← Back to report';
$string['error_nopermission']   = 'You do not have permission to view this report.';
$string['error_invalidcm']      = 'Invalid course module.';
$string['error_notassign']      = 'Samost only works with Assignment activities.';
$string['similarity_code'] = 'Code similarity (%)';
$string['similarity_text'] = 'Text similarity (%)';
$string['tab_report'] = 'Report';
$string['tab_info']   = 'Help';
$string['no_submissions_in_group']     = 'No submitted work found in the selected group.';
$string['not_enough_submissions']      = 'Only one submission in the selected group — nothing to compare against.';
$string['no_other_groups_submissions'] = 'No submissions found in other groups — nothing to compare against.';

