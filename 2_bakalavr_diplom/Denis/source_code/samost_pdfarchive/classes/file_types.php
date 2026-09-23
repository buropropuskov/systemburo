<?php
/**
 * @package    plagiarism_samost
 * @copyright  2026 Mikhailov D.A., RTU MIREA
 * @license    http://www.gnu.org/copyleft/gpl.html GNU GPL v3 or later
 */

namespace plagiarism_samost;

defined('MOODLE_INTERNAL') || die();

class file_types {

    const CODE_EXTENSIONS = [
        'c', 'cpp', 'h', 'hpp', 'cc', 'cxx',
        'java', 'kt', 'scala', 'groovy',
        'py', 'rb', 'php', 'pl', 'lua', 'sh', 'bash',
        'js', 'ts', 'jsx', 'tsx', 'vue',
        'cs', 'vb', 'fs',
        'go', 'rs', 'swift', 'zig',
        'sql', 'r',
        'dart', 'm', 'asm', 's',
    ];

    const TEXT_EXTENSIONS = [
        'txt', 'md', 'pdf',
    ];

    const TEXT_MIMES = [
        'text/plain',
        'text/markdown',
    ];

    const DOCX_MIME = 'application/vnd.openxmlformats-officedocument.wordprocessingml.document';

    const PDF_MIME = 'application/pdf';
    const ARCHIVE_EXTENSIONS = ['zip'];


    const ARCHIVE_SKIP_PATTERNS = [
        // Directories
        '__pycache__', 'node_modules', '.git', '.svn', '.hg',
        'vendor', 'venv', '.venv', 'env', 'target', 'build', 'dist',
        // Files
        '.DS_Store', 'Thumbs.db', '.gitignore', '.gitkeep',
        'package-lock.json', 'yarn.lock', 'composer.lock',
    ];

    public static function is_code_extension(string $extension): bool {
        return in_array(strtolower($extension), self::CODE_EXTENSIONS, true);
    }

    public static function is_text_extension(string $extension, string $mimetype = ''): bool {
        if (in_array(strtolower($extension), self::TEXT_EXTENSIONS, true)) {
            return true;
        }
        if ($extension === 'docx' || $mimetype === self::DOCX_MIME) {
            return true;
        }
        if ($extension === 'pdf' || $mimetype === self::PDF_MIME) {
            return true;
        }
        if (in_array($mimetype, self::TEXT_MIMES, true)) {
            return true;
        }
        return false;
    }

    public static function is_archive_extension(string $extension): bool {
        return in_array(strtolower($extension), self::ARCHIVE_EXTENSIONS, true);
    }

    public static function should_skip_archive_path(string $path): bool {
        $parts = explode('/', str_replace('\\', '/', $path));
        foreach ($parts as $part) {
            if (in_array($part, self::ARCHIVE_SKIP_PATTERNS, true)) {
                return true;
            }
	    if (strlen($part) > 1 && $part[0] === '.') {
                return true;
            }
        }
        return false;
    }

    public static function classify(\stored_file $file): string {
        $ext  = strtolower(pathinfo($file->get_filename(), PATHINFO_EXTENSION));
        $mime = $file->get_mimetype();

        if (self::is_archive_extension($ext)) {
            return 'archive';
        }
        if (self::is_code_extension($ext)) {
            return 'code';
        }
        if (self::is_text_extension($ext, $mime)) {
            return 'text';
        }
        return 'unknown';
    }

    public static function classify_filename(string $filename): string {
        $ext = strtolower(pathinfo($filename, PATHINFO_EXTENSION));
        if (self::is_archive_extension($ext)) {
            return 'archive';
        }
        if (self::is_code_extension($ext)) {
            return 'code';
        }
        if (in_array($ext, self::TEXT_EXTENSIONS, true) || $ext === 'docx') {
            return 'text';
        }
        return 'unknown';
    }
}
