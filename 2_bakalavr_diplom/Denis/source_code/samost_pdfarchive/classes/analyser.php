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

/*
 * @package    plagiarism_samost
 * @copyright  2026 Mikhailov D.A., RTU MIREA
 * @license    http://www.gnu.org/copyleft/gpl.html GNU GPL v3 or later
 */

namespace plagiarism_samost;

defined('MOODLE_INTERNAL') || die();

class analyser {

    private int $shingle_size;

    private int $kgram_size;

    private int $window_size;

    public function __construct(int $shingle_size = 5, int $kgram_size = 5, int $window_size = 4) {
        $this->shingle_size = max(2, $shingle_size);
        $this->kgram_size   = max(2, $kgram_size);
        $this->window_size  = max(2, $window_size);
    }


    /**
     * Run pairwise analysis with group scope control.
     *
     *   'within' — compare only users within $target_userids against each other.
     *   'cross'  — compare each user in $target_userids against each user in
     *              $compare_userids (pairs within $target_userids are skipped,
     *              pairs within $compare_userids are skipped).
     *
     * @param  int   $cmid
     * @param  int[] $target_userids   Users whose we are processing.
     * @param  string $mode
     * @param  int[] $compare_userids  Users to compare against (crsooss only).
     * @return int  Number of pairs analysed.
     */
    public function run_for_cm_with_scope(
        int $cmid,
        array $target_userids,
        string $mode = 'within',
        array $compare_userids = []
    ): int {
        global $DB;

        $records = $DB->get_records('plagiarism_samost_files', ['cm' => $cmid]);

        $buckets = [];
        foreach ($records as $rec) {
            if (empty($rec->textcontent)) {
                continue;
            }
            $uid     = (int)$rec->userid;
            $content = (string)$rec->textcontent;
            if (!isset($buckets[$uid])) {
                $buckets[$uid] = ['code' => '', 'text' => ''];
            }
            $blocks = self::split_combined_content($content);
            foreach ($blocks as $block) {
                $payload = text_extractor::payload($block);
                if (text_extractor::is_code($block)) {
                    $buckets[$uid]['code'] .= ($buckets[$uid]['code'] !== '' ? "\n\n" : '') . $payload;
                } else {
                    $buckets[$uid]['text'] .= ($buckets[$uid]['text'] !== '' ? ' ' : '') . $payload;
                }
            }
        }

        $pairs = [];

        if ($mode === 'cross') {
            $target_set  = array_flip($target_userids);
            $compare_set = array_flip($compare_userids);
            foreach ($target_userids as $uid1) {
                foreach ($compare_userids as $uid2) {
                    if (!isset($buckets[$uid1]) || !isset($buckets[$uid2])) {
                        continue;
                    }
                    $a = min($uid1, $uid2);
                    $b = max($uid1, $uid2);
                    $pairs["$a-$b"] = [$a, $b];
                }
            }
	} else {
            $uids = array_values(array_filter($target_userids, fn($uid) => isset($buckets[$uid])));
            $n    = count($uids);
            for ($i = 0; $i < $n; $i++) {
                for ($j = $i + 1; $j < $n; $j++) {
                    $pairs[] = [$uids[$i], $uids[$j]];
                }
            }
            if (!empty($compare_userids)) {
                foreach ($uids as $uid1) {
                    foreach ($compare_userids as $uid2) {
                        if (!isset($buckets[$uid2])) {
                            continue;
                        }
                        $a = min($uid1, $uid2);
                        $b = max($uid1, $uid2);
                        $pairs["$a-$b"] = [$a, $b];
                    }
                }
            }
        }
        if (empty($pairs)) {
            return 0;
        }

        $affected_uids = array_unique(array_merge($target_userids, $compare_userids));
        if (!empty($affected_uids)) {
            [$in_sql, $in_params] = $DB->get_in_or_equal($affected_uids);
            $DB->execute(
                "DELETE FROM {plagiarism_samost_results}
                  WHERE cm = ?
                    AND (userid1 $in_sql OR userid2 $in_sql)",
                array_merge([(int)$cmid], $in_params, $in_params)
            );
        }

        $pairs_done = 0;
        $now        = time();

        foreach ($pairs as $pair) {
            [$uid1, $uid2] = $pair;

            $code1 = $buckets[$uid1]['code'] ?? '';
            $code2 = $buckets[$uid2]['code'] ?? '';
            $text1 = $buckets[$uid1]['text'] ?? '';
            $text2 = $buckets[$uid2]['text'] ?? '';

            $sim_code = null;
            if ($code1 !== '' && $code2 !== '') {
                $sim_code = round($this->winnowing_similarity($code1, $code2) * 100, 2);
            }

            $sim_text = null;
            if ($text1 !== '' && $text2 !== '') {
                $sim_text = round($this->jaccard_similarity($text1, $text2) * 100, 2);
            }

            $DB->insert_record('plagiarism_samost_results', (object)[
                'cm'              => $cmid,
                'userid1'         => $uid1,
                'userid2'         => $uid2,
                'similarity_code' => $sim_code,
                'similarity_text' => $sim_text,
                'timecalculated'  => $now,
            ]);

            $pairs_done++;
        }

        return $pairs_done;
    }

    /**
     * Run full pairwise analysis for all submissions in an assignment.
     *
     * Reads cached content from plagiarism_samost_files, selects the right
     * algorithm per content type, stores results in plagiarism_samost_results.
     *
     * @param  int $cmid
     * @return int  Number of pairs analysed.
     */
    public function run_for_cm(int $cmid): int {
        global $DB;

        $records = $DB->get_records('plagiarism_samost_files', ['cm' => $cmid]);

        $buckets = [];
        foreach ($records as $rec) {
            if (empty($rec->textcontent)) {
                continue;
            }
            $uid     = $rec->userid;
            $content = (string) $rec->textcontent;
            if (!isset($buckets[$uid])) {
                $buckets[$uid] = ['code' => '', 'text' => ''];
            }

            $blocks = self::split_combined_content($content);
            foreach ($blocks as $block) {
                $payload = text_extractor::payload($block);
                if (text_extractor::is_code($block)) {
                    $buckets[$uid]['code'] .= ($buckets[$uid]['code'] !== '' ? "\n\n" : '') . $payload;
                } else {
                    $buckets[$uid]['text'] .= ($buckets[$uid]['text'] !== '' ? ' ' : '') . $payload;
                }
            }
        }

        $userids = array_keys($buckets);
        $n       = count($userids);

        if ($n < 2) {
            return 0;
        }

        $DB->delete_records('plagiarism_samost_results', ['cm' => $cmid]);

        $pairs_done = 0;
        $now        = time();

        for ($i = 0; $i < $n; $i++) {
            for ($j = $i + 1; $j < $n; $j++) {
                $uid1 = $userids[$i];
                $uid2 = $userids[$j];

                $code1 = $buckets[$uid1]['code'];
                $code2 = $buckets[$uid2]['code'];
                $text1 = $buckets[$uid1]['text'];
                $text2 = $buckets[$uid2]['text'];

                $sim_code = null;
                if ($code1 !== '' && $code2 !== '') {
                    $sim_code = round($this->winnowing_similarity($code1, $code2) * 100, 2);
                }

                $sim_text = null;
                if ($text1 !== '' && $text2 !== '') {
                    $sim_text = round($this->jaccard_similarity($text1, $text2) * 100, 2);
                }

                $DB->insert_record('plagiarism_samost_results', (object)[
                    'cm'              => $cmid,
                    'userid1'         => $uid1,
                    'userid2'         => $uid2,
                    'similarity_code' => $sim_code,
                    'similarity_text' => $sim_text,
                    'timecalculated'  => $now,
                ]);

                $pairs_done++;
            }
        }

        return $pairs_done;
    }

    public function compute_similarity(string $content_a, string $content_b): float {
        $is_code_a = text_extractor::is_code($content_a);
        $is_code_b = text_extractor::is_code($content_b);

        $payload_a = text_extractor::payload($content_a);
        $payload_b = text_extractor::payload($content_b);

        if ($is_code_a && $is_code_b) {
            // Winnowing.
            return $this->winnowing_similarity($payload_a, $payload_b);
        }

        // Shingles.
        return $this->jaccard_similarity($payload_a, $payload_b);
    }

    // Winnowing similarity (code)
    public function winnowing_similarity(string $tokens_a, string $tokens_b): float {
        $fp_a = text_extractor::winnowing_fingerprints($tokens_a, $this->kgram_size, $this->window_size);
        $fp_b = text_extractor::winnowing_fingerprints($tokens_b, $this->kgram_size, $this->window_size);

        if (empty($fp_a) || empty($fp_b)) {
            return 0.0;
        }

        $intersection = count(array_intersect_key($fp_a, $fp_b));
        $union        = count($fp_a) + count($fp_b) - $intersection;

        return $union > 0 ? $intersection / $union : 0.0;
    }

    // Jaccard shingle similarity (text)

    public function jaccard_similarity(string $text_a, string $text_b): float {
        $shingles_a = $this->build_shingle_set($text_a);
        $shingles_b = $this->build_shingle_set($text_b);

        if (empty($shingles_a) || empty($shingles_b)) {
            return 0.0;
        }

        $intersection = count(array_intersect_key($shingles_a, $shingles_b));
        $union        = count($shingles_a) + count($shingles_b) - $intersection;

        return $union > 0 ? $intersection / $union : 0.0;
    }

    public function cache_file(int $cmid, int $userid, \stored_file $file): string {
        global $DB;

        $content = text_extractor::extract($file);

        if ($content === '') {
            return ''; // unsupported type — skip
        }

        $existing = $DB->get_record('plagiarism_samost_files', [
            'cm'     => $cmid,
            'userid' => $userid,
            'fileid' => $file->get_id(),
        ]);

        $now = time();
        if ($existing) {
            $existing->textcontent  = $content;
            $existing->timemodified = $now;
            $DB->update_record('plagiarism_samost_files', $existing);
        } else {
            $DB->insert_record('plagiarism_samost_files', (object)[
                'cm'           => $cmid,
                'userid'       => $userid,
                'fileid'       => $file->get_id(),
                'textcontent'  => $content,
                'timecreated'  => $now,
                'timemodified' => $now,
            ]);
        }

        return $content;
    }

    private static function split_combined_content(string $content): array {
        $has_code = strpos($content, text_extractor::TYPE_CODE) !== false;
        $has_text = strpos($content, text_extractor::TYPE_TEXT) !== false;

        if ($has_code && $has_text) {
            $blocks = [];
            if (preg_match('/CODE:(.*?)(?=\n\nTEXT:|$)/su', $content, $m) && trim($m[1]) !== '') {
                $blocks['code'] = text_extractor::TYPE_CODE . trim($m[1]);
            }
            if (preg_match('/TEXT:(.*?)$/su', $content, $m) && trim($m[1]) !== '') {
                $blocks['text'] = text_extractor::TYPE_TEXT . trim($m[1]);
            }
            return $blocks ?: ['0' => $content];
        }

        return ['0' => $content];
    }

    private function build_shingle_set(string $text): array {
        $words = preg_split('/\s+/u', $text, -1, PREG_SPLIT_NO_EMPTY);
        $count = count($words);
        $shingles = [];

        for ($i = 0; $i <= $count - $this->shingle_size; $i++) {
            $shingle          = implode(' ', array_slice($words, $i, $this->shingle_size));
            $shingles[md5($shingle)] = true;
        }

        return $shingles;
    }
}
