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

class text_extractor {

    const TYPE_TEXT = 'TEXT:';

    const TYPE_CODE = 'CODE:';

    public static function extract(\stored_file $file): string {
        $mimetype  = $file->get_mimetype();
        $filename  = strtolower($file->get_filename());
        $extension = pathinfo($filename, PATHINFO_EXTENSION);

        $kind = file_types::classify($file);

        if ($kind === 'code') {
            $raw = self::read_raw($file);
            return self::TYPE_CODE . self::tokenise_code($raw, $extension);
        }

        if ($kind === 'text') {
            if ($extension === 'docx' || $mimetype === file_types::DOCX_MIME) {
                return self::TYPE_TEXT . self::extract_docx($file);
            }
            if ($extension === 'pdf' || $mimetype === file_types::PDF_MIME) {
                $text = self::extract_pdf($file);
                return $text !== '' ? self::TYPE_TEXT . $text : '';
            }
            $raw = self::read_raw($file);
            return self::TYPE_TEXT . self::normalise($raw);
        }

        if ($kind === 'archive' && $extension === 'zip') {
            return self::extract_zip($file);
        }

        return ''; 
    }


    public static function is_code(string $content): bool {
        $prefix = self::TYPE_CODE;
        if ($prefix === '') {
            return false;
        }
        return strncmp($content, $prefix, strlen($prefix)) === 0;
    }


    public static function payload(string $content): string {
        foreach ([self::TYPE_CODE, self::TYPE_TEXT] as $prefix) {
            if ($prefix === '') {
                continue;
            }
            if (strncmp($content, $prefix, strlen($prefix)) === 0) {
                return substr($content, strlen($prefix));
            }
        }
        return $content;
    }

    /**
     * Transformations applied:
     *   1. Strip comments (// ... and /* ... and # ...)
     *   2. Replace string/char literals with STR
     *   3. Replace numeric literals with NUM
     *   4. Replace #include / import / using with placeholder tokens
     *   5. Keep language keywords as-is
     *   6. Replace all other identifiers (variable/function names) with VAR
     *
     * Example input (C):
     *   int main(int argc, char *argv[]) { printf("hello %d\n", argc); }
     * Example output:
     *   int VAR ( int VAR , char * VAR [ ] ) { VAR ( STR , VAR ) ; }
     *
     */
    public static function tokenise_code(string $code, string $extension = ''): string {
        $code = str_replace(["\r\n", "\r"], "\n", $code);
        $code = preg_replace('/\/\*.*?\*\//su', ' ', $code);
        $code = preg_replace('/\/\/[^\n]*/u', ' ', $code);

        if (in_array($extension, ['py', 'rb', 'sh'], true)) {
            $code = preg_replace('/#[^\n]*/u', ' ', $code);
        }

        $code = preg_replace('/"(?:[^"\\\\]|\\\\.)*"/u',   ' STR ', $code);
        $code = preg_replace("/\'(?:[^\'\\\\]|\\\\.)*\'/u", ' STR ', $code);
        $code = preg_replace('/`(?:[^`\\\\]|\\\\.)*`/u',   ' STR ', $code);

        $code = preg_replace('/\b\d+(\.\d+)?([eE][+-]?\d+)?\b/u', ' NUM ', $code);

        $code = preg_replace('/#\s*include\s*[<"][^>"]*[>"]/u', ' INCLUDE ', $code);

        $code = preg_replace('/\bimport\b[^\n;]*/u', ' IMPORT ', $code);
        $code = preg_replace('/\busing\b[^\n;]*/u',  ' USING ',  $code);

        $keywords = [
            'if','else','for','while','do','switch','case','default',
            'break','continue','return','goto','try','catch','finally',
            'throw','throws','raise','except','with','as','pass','yield',
            'int','long','short','char','float','double','bool','boolean',
            'void','string','byte','uint','ulong','auto','unsigned','signed',
            'class','struct','interface','enum','extends','implements',
            'new','delete','this','super','self',
            'public','private','protected','static','final','const',
            'abstract','virtual','override','readonly','volatile','extern',
            'def','lambda','async','await','typeof','instanceof',
            'null','true','false','undefined','nil','None','True','False',
            'var','let','function','func','package','defer','chan','map',
            'range','select','type','module','namespace','require',
            'echo','print','foreach','endforeach','array','list',
            'printf','scanf','cout','cin','malloc','free','sizeof',
            'INCLUDE','IMPORT','USING','STR','NUM',
        ];
        $kw_set = array_flip($keywords);

        $code = preg_replace_callback(
            '/\b[a-zA-Z_][a-zA-Z0-9_]*\b/u',
            function ($m) use ($kw_set) {
                return isset($kw_set[$m[0]]) ? $m[0] : 'VAR';
            },
            $code
        );

        $lines = explode("\n", $code);
        $result = [];
        foreach ($lines as $line) {
            preg_match('/^(\s*)(.*)/su', $line, $m);
            $indent  = $m[1];
            $content = trim(preg_replace('/[ \t]+/u', ' ', $m[2]));
            if ($content !== '') {
                $result[] = $indent . $content;
            } elseif (!empty($result)) {
                $result[] = '';
            }
        }

        return rtrim(implode("\n", $result));
    }

    /**
     * Build a Winnowing fingerprint set from a token stream.
     *
     *   1. Split token stream into k-grams (windows of k tokens).
     *   2. Hash each k-gram.
     *   3. Slide a window of size w over the hash sequence.
     *   4. Select the minimum hash in each window.
     *   5. Deduplicate consecutive identical minimums.
     *
     */
    public static function winnowing_fingerprints(
        string $token_stream,
        int $k = 5,
        int $w = 4
    ): array {
        $tokens = preg_split('/\s+/u', $token_stream, -1, PREG_SPLIT_NO_EMPTY);
        $n      = count($tokens);

        if ($n < $k) {
            return [crc32($token_stream) => true];
        }

        $hashes = [];
        for ($i = 0; $i <= $n - $k; $i++) {
            $kgram     = implode(' ', array_slice($tokens, $i, $k));
            $hashes[]  = crc32($kgram);
        }

        $m = count($hashes);
        if ($m < $w) {
            return array_fill_keys($hashes, true);
        }

        $fingerprints = [];
        $prev_min_pos = -1;

        for ($i = 0; $i <= $m - $w; $i++) {
            $min_pos = $i;
            for ($j = $i + 1; $j < $i + $w; $j++) {
                if ($hashes[$j] < $hashes[$min_pos]) {
                    $min_pos = $j;
                }
            }
            if ($min_pos !== $prev_min_pos) {
                $fingerprints[$hashes[$min_pos]] = true;
                $prev_min_pos = $min_pos;
            }
        }

        return $fingerprints;
    }

    public static function normalise(string $text): string {
        $text = mb_strtolower($text, 'UTF-8');
        $text = preg_replace('/[^\p{L}\p{N}\s]/u', ' ', $text);
        $text = preg_replace('/\s+/u', ' ', $text);
        return trim($text);
    }

    private static function extract_zip(\stored_file $file): string {
        $tmp = make_request_directory() . '/' . clean_filename($file->get_filename());
        $file->copy_content_to($tmp);

        $zip = new \ZipArchive();
        if ($zip->open($tmp) !== true) {
            @unlink($tmp);
            return '';
        }

        $code_parts = [];
        $text_parts = [];

        for ($i = 0; $i < $zip->numFiles; $i++) {
            $stat = $zip->statIndex($i);
            $path = $stat['name'];

            if (substr($path, -1) === '/') {
                continue;
            }

            if (file_types::should_skip_archive_path($path)) {
                continue;
            }

            $filename = basename($path);
            $ext      = strtolower(pathinfo($filename, PATHINFO_EXTENSION));
            $kind     = file_types::classify_filename($filename);

            if ($kind === 'unknown' || $kind === 'archive') {
                continue;
            }

            $raw_bytes = $zip->getFromIndex($i);
            if ($raw_bytes === false || $raw_bytes === '') {
                continue;
            }

            if (!mb_check_encoding($raw_bytes, 'UTF-8')) {
                $raw_bytes = mb_convert_encoding($raw_bytes, 'UTF-8', 'auto');
            }

            if ($kind === 'code') {
                $tokens = self::tokenise_code($raw_bytes, $ext);
                if ($tokens !== '') {
                    $code_parts[] = $tokens;
                }
            } elseif ($kind === 'text') {
                if ($ext === 'docx') {
                    $docx_tmp = make_request_directory() . '/' . clean_filename($filename);
                    file_put_contents($docx_tmp, $raw_bytes);
                    $text = self::extract_docx_from_path($docx_tmp);
                    @unlink($docx_tmp);
                } elseif ($ext === 'pdf') {
                    $pdf_tmp = make_request_directory() . '/' . clean_filename($filename);
                    file_put_contents($pdf_tmp, $raw_bytes);
                    $text = self::extract_pdf_from_path($pdf_tmp);
                    @unlink($pdf_tmp);
                } else {
                    $text = self::normalise($raw_bytes);
                }
                if ($text !== '') {
                    $text_parts[] = $text;
                }
            }
        }

        $zip->close();
        @unlink($tmp);

        $result = '';
        if (!empty($code_parts)) {
            $result .= self::TYPE_CODE . implode("\n\n", $code_parts);
        }
        if (!empty($text_parts)) {
            if ($result !== '') {
                $result .= "\n\n";
            }
            $result .= self::TYPE_TEXT . implode(' ', $text_parts);
        }

        return $result;
    }

    /**
     *
     * Returns empty string if:
     *  - pdftotext is not installed
     *  - PDF has no text layer (scan)
     *  - Extraction fails for any reason
     *
     */
    private static function extract_pdf(\stored_file $file): string {
        $tmp = make_request_directory() . '/' . clean_filename($file->get_filename());
        $file->copy_content_to($tmp);
        $text = self::extract_pdf_from_path($tmp);
        @unlink($tmp);
        return $text;
    }
 
    private static function extract_pdf_from_path(string $path): string {
        $which = shell_exec('which pdftotext 2>/dev/null');
        if (empty(trim((string)$which))) {
            return '';
        }
 
        $out_path = $path . '.txt';
 
        // -nopgbrk: no form-feed chars between pages
        // -enc UTF-8: force UTF-8 output
        // -q: quiet
        $cmd = sprintf(
            'pdftotext -nopgbrk -enc UTF-8 -q %s %s 2>/dev/null',
            escapeshellarg($path),
            escapeshellarg($out_path)
        );
        shell_exec($cmd);
 
        if (!file_exists($out_path)) {
            return '';
        }
 
        $text = file_get_contents($out_path);
        @unlink($out_path);
 
        if ($text === false || trim($text) === '') {
            return ''; // scan or empty PDF — no text layer
        }
 
        if (!mb_check_encoding($text, 'UTF-8')) {
            $text = mb_convert_encoding($text, 'UTF-8', 'auto');
        }
 
        return self::normalise($text);
    }

    private static function extract_docx_from_path(string $path): string {
        $zip = new \ZipArchive();
        if ($zip->open($path) !== true) {
            return '';
        }
        $xml = $zip->getFromName('word/document.xml');
        $zip->close();

        if ($xml === false) {
            return '';
        }

        libxml_use_internal_errors(true);
        $dom = new \DOMDocument();
        $dom->loadXML($xml);
        libxml_clear_errors();

        $texts = [];
        $nodes = $dom->getElementsByTagNameNS(
            'http://schemas.openxmlformats.org/wordprocessingml/2006/main', 't'
        );
        foreach ($nodes as $node) {
            $texts[] = $node->nodeValue;
        }
        return self::normalise(implode(' ', $texts));
    }

    private static function read_raw(\stored_file $file): string {
        $content = $file->get_content();
        if (!mb_check_encoding($content, 'UTF-8')) {
            $content = mb_convert_encoding($content, 'UTF-8', 'auto');
        }
        return $content;
    }

    private static function extract_docx(\stored_file $file): string {
        $tmp = make_request_directory() . '/' . clean_filename($file->get_filename());
        $file->copy_content_to($tmp);

        $zip = new \ZipArchive();
        if ($zip->open($tmp) !== true) {
            @unlink($tmp);
            return '';
        }

        $xml = $zip->getFromName('word/document.xml');
        $zip->close();
        @unlink($tmp);

        if ($xml === false) {
            return '';
        }

        libxml_use_internal_errors(true);
        $dom = new \DOMDocument();
        $dom->loadXML($xml);
        libxml_clear_errors();

        $texts = [];
        $nodes = $dom->getElementsByTagNameNS(
            'http://schemas.openxmlformats.org/wordprocessingml/2006/main',
            't'
        );
        foreach ($nodes as $node) {
            $texts[] = $node->nodeValue;
        }

        return self::normalise(implode(' ', $texts));
    }
}
