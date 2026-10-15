"""Проверки отказов, которые раньше могли дать ложный успешный выпуск."""
from pathlib import Path
import subprocess
import json
import hashlib
import tempfile
import unittest
from unittest.mock import patch
from contextlib import ExitStack, redirect_stdout
from io import StringIO

import doc_tools
import build_docs
import docs_impact
import doc_facts


class CommandChecks(unittest.TestCase):
    def test_environment_duplicates_only_within_reference_table(self):
        reference = '# Приложение Б\n| `MODE` | x |\n# Приложение В\n'
        with patch.object(doc_facts, 'env_defaults', return_value={'MODE': 'x'}):
            problems = []
            doc_facts.check_env('| `MODE` | x |\n' + reference, problems)
            self.assertEqual(problems, [])
            doc_facts.check_env(reference.replace('# Приложение В', '| `MODE` | x |\n# Приложение В'), problems)
            self.assertEqual(len(problems), 1)

    def test_git_failure_is_not_an_empty_result(self):
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaises(RuntimeError):
                doc_tools.git(directory, 'rev-parse', '--verify', 'refs/heads/nonexistent-docs-check')

    def test_missing_source_fact_is_not_zero(self):
        with self.assertRaises(ValueError):
            doc_tools.number(r'ttl:\s*(\d+)', 'константа удалена')
        with self.assertRaises(ValueError):
            doc_tools.section('func Other() {}', r'^func AllModels')

    def test_model_list_survives_source_line_shift(self):
        code = '\n' * 250 + 'func AllModels() []interface{} {\n' \
               '\treturn []interface{}{\n\t\t&models.One{},\n\t\t&models.Two{},\n\t}\n}\n'
        part = doc_tools.section(code, r'^func AllModels\(\) \[\]interface\{\} \{')
        self.assertIn('&models.Two{}', part)

    def test_sql_tables_and_models_are_not_counted_twice(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'internal/database').mkdir(parents=True)
            (root / 'internal/models').mkdir(parents=True)
            (root / 'internal/database/partitions.go').write_text(
                'CREATE TABLE request_logs (\nCREATE TABLE pd_audit_logs (', encoding='utf-8')
            (root / 'internal/models/logs.go').write_text(
                'func (r *RequestLogs) TableName() string { return "request_logs" }', encoding='utf-8')
            self.assertEqual(doc_tools.schema_table_count(directory, '&models.RequestLogs{}'), 2)

    def test_staged_and_untracked_names_with_spaces_are_preserved(self):
        with tempfile.TemporaryDirectory() as directory:
            doc_tools.git(directory, 'init', '-q')
            path = Path(directory) / 'new file.go'
            path.write_text('package example\n', encoding='utf-8')
            with patch.object(docs_impact, 'REPO', directory):
                self.assertEqual(docs_impact.changed(''), ['new file.go'])
                doc_tools.git(directory, 'add', '--', path.name)
                self.assertEqual(docs_impact.changed(''), ['new file.go'])


class PdfChecks(unittest.TestCase):
    def test_page_map_ignores_contents_entry_without_dotted_leader(self):
        text = ('СОДЕРЖАНИЕ\n1. Назначение ...... 3\n'
                '2. Длинный заголовок4\f'
                '3\n1. Назначение\nОсновной текст\f'
                '4\n2. Длинный заголовок\nПродолжение\f')
        with patch.object(build_docs, 'executable', return_value='pdftotext'), \
             patch.object(build_docs.subprocess, 'run', return_value=
                          subprocess.CompletedProcess([], 0, text, '')):
            self.assertEqual(build_docs.pagemap_from_pdf({}, [
                (1, '1. Назначение'), (1, '2. Длинный заголовок')]),
                {'1. Назначение': 2, '2. Длинный заголовок': 3})

    def test_page_map_rejects_document_without_body_heading(self):
        with patch.object(build_docs, 'executable', return_value='pdftotext'), \
             patch.object(build_docs.subprocess, 'run', return_value=
                          subprocess.CompletedProcess([], 0, 'СОДЕРЖАНИЕ\n1. Назначение ... 3', '')):
            with self.assertRaisesRegex(RuntimeError, 'первый заголовок'):
                build_docs.pagemap_from_pdf({}, [(1, '1. Назначение')])

    def test_tail_break_must_advance_past_existing_split(self):
        metadata = [{'number': '1', 'head_probes': [],
                     'row_probes': [['first'], ['second'], ['third']]}]
        for text, expected in [('Продолжение таблицы 1 missing', None),
                               ('Продолжение таблицы 1 second', 2),
                               ('Продолжение таблицы 1 second third', None)]:
            with self.subTest(text=text), \
                 patch.object(build_docs, 'executable', return_value='pdftotext'), \
                 patch.object(build_docs.subprocess, 'run', return_value=
                              subprocess.CompletedProcess([], 0, text, '')):
                self.assertEqual(build_docs.next_break_in_tail({}, metadata, '1', [1]), expected)

    def test_empty_changelog_form_is_not_a_data_continuation(self):
        from docx import Document
        document = Document()
        with patch.object(build_docs, 'changelog', return_value=[
            ('1.0', '01.01.2026', 'First'), ('1.1', '02.01.2026', 'Second')
        ]):
            build_docs.change_log(document, {}, {'И.1': [1]})
        captions = [paragraph.text for paragraph in document.paragraphs
                    if 'таблицы И.1' in paragraph.text]
        self.assertEqual(captions, ['Продолжение таблицы И.1', 'Окончание таблицы И.1'])

    def test_wrapped_table_header_after_running_notice_is_detected(self):
        rows = build_docs.page_body_lines('Не для распространения\n№ Показатель Допустимое Фактически\nзначение\n27 Доступность\n8\n')
        self.assertTrue(build_docs.starts_with_table_header(
            rows, {'№ Показатель Допустимое значение Фактически'}))
        self.assertFalse(build_docs.starts_with_table_header(
            ['Продолжение таблицы 4.2', *rows], {'№ Показатель Допустимое значение Фактически'}))

    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.pdf = self.root / 'Проверка.pdf'
        self.pdf.write_bytes(b'%PDF-old')
        (self.root / 'Проверка.docx').write_bytes(b'fixture')
        (self.root / 'Проверка.md').write_text('# Проверка\n', encoding='utf-8')
        self.paths = patch.object(build_docs, 'doc_path', side_effect=lambda cfg, ext: str(self.root / ('Проверка' + ext)))
        self.paths.start()
        self.addCleanup(self.paths.stop)

    def test_failed_conversion_does_not_accept_existing_pdf(self):
        failure = subprocess.CompletedProcess([], 1, '', 'conversion failed')
        with patch.object(build_docs.subprocess, 'run', return_value=failure):
            with self.assertRaises(RuntimeError):
                build_docs.to_pdf({})
        self.assertEqual(self.pdf.read_bytes(), b'%PDF-old')

    def test_zero_exit_without_new_pdf_is_failure(self):
        with patch.object(build_docs.subprocess, 'run', return_value=subprocess.CompletedProcess([], 0, '', '')):
            with self.assertRaises(RuntimeError):
                build_docs.to_pdf({})
        self.assertEqual(self.pdf.read_bytes(), b'%PDF-old')

    def test_invalid_output_does_not_replace_existing_pdf(self):
        def convert(command, **kwargs):
            (Path(command[command.index('--outdir') + 1]) / self.pdf.name).write_bytes(b'not a PDF')
            return subprocess.CompletedProcess(command, 0, '', '')
        with patch.object(build_docs.subprocess, 'run', side_effect=convert):
            with self.assertRaises(RuntimeError):
                build_docs.to_pdf({})
        self.assertEqual(self.pdf.read_bytes(), b'%PDF-old')

    def test_success_replaces_pdf_with_fresh_output(self):
        def convert(command, **kwargs):
            (Path(command[command.index('--outdir') + 1]) / self.pdf.name).write_bytes(b'%PDF-new')
            return subprocess.CompletedProcess(command, 0, '', '')
        with patch.object(build_docs.subprocess, 'run', side_effect=convert):
            self.assertEqual(build_docs.to_pdf({}), str(self.pdf))
        self.assertEqual(self.pdf.read_bytes(), b'%PDF-new')

    def test_profile_is_outside_document_output_directory(self):
        def convert(command, **kwargs):
            output = Path(command[command.index('--outdir') + 1])
            profile = next(arg for arg in command if arg.startswith('-env:UserInstallation='))
            self.assertFalse(profile.split('=', 1)[1].startswith(output.as_uri() + '/'))
            (output / self.pdf.name).write_bytes(b'%PDF-new')
            return subprocess.CompletedProcess(command, 0, '', '')
        with patch.object(build_docs.subprocess, 'run', side_effect=convert):
            build_docs.to_pdf({})

    def test_manifest_records_exact_output_hashes(self):
        with patch.object(build_docs, 'git', return_value=subprocess.CompletedProcess([], 0, 'revision\n', '')):
            build_docs.write_build_manifest({'key': 'pilot'}, 14)
        record = json.loads((self.root / 'Проверка.build.json').read_text(encoding='utf-8'))
        self.assertEqual(record['code_revision'], 'revision')
        self.assertEqual(record['pdf_sha256'], hashlib.sha256(b'%PDF-old').hexdigest())
        self.assertEqual(record['pages'], 14)

    def test_final_layout_can_fix_many_tables_without_immediate_count_reduction(self):
        metadata = [{'number': str(n), 'row_probes': [['first'], ['second']]}
                    for n in range(8)]
        repaired = []
        searches = []
        def find_break(*args):
            searches.append(True)
            return (None, None) if len(searches) == 1 else (str(len(repaired)), [1])
        def verdict(cfg, number, **kwargs):
            repaired.append(number)
            return 'ok'
        def report(*args, **kwargs):
            # Сдвиг следующей таблицы оставляет число разрывов прежним.
            return ([1], []) if len(repaired) < 8 else ([], [])
        with ExitStack() as stack:
            stack.enter_context(patch.object(build_docs, 'build', return_value=([(1, 'Title')], metadata)))
            stack.enter_context(patch.object(build_docs, 'to_pdf', return_value=str(self.pdf)))
            stack.enter_context(patch.object(build_docs, 'find_first_unsplit_break', side_effect=find_break))
            stack.enter_context(patch.object(build_docs, 'split_verdict', side_effect=verdict))
            stack.enter_context(patch.object(build_docs, 'report_silent_breaks', side_effect=report))
            stack.enter_context(patch.object(build_docs, 'pagemap_from_pdf', return_value={'Title': 1}))
            stack.enter_context(patch.object(build_docs.subprocess, 'run', return_value=
                subprocess.CompletedProcess([], 0, b'Pages: 1', b'')))
            manifest = stack.enter_context(patch.object(build_docs, 'write_build_manifest'))
            stack.enter_context(redirect_stdout(StringIO()))
            build_docs.build_one('deploy')
        self.assertEqual(repaired, [str(n) for n in range(8)])
        manifest.assert_called_once()

    def test_unresolved_final_break_cannot_publish_a_manifest(self):
        with ExitStack() as stack:
            stack.enter_context(patch.object(build_docs, 'build', return_value=([(1, 'Title')], [])))
            stack.enter_context(patch.object(build_docs, 'to_pdf', return_value=str(self.pdf)))
            stack.enter_context(patch.object(build_docs, 'find_first_unsplit_break', return_value=(None, None)))
            stack.enter_context(patch.object(build_docs, 'report_silent_breaks', return_value=([3], [])))
            stack.enter_context(patch.object(build_docs, 'pagemap_from_pdf', return_value={'Title': 1}))
            manifest = stack.enter_context(patch.object(build_docs, 'write_build_manifest'))
            stack.enter_context(redirect_stdout(StringIO()))
            with self.assertRaisesRegex(RuntimeError, 'разрывы таблиц'):
                build_docs.build_one('deploy')
        manifest.assert_not_called()


if __name__ == '__main__':
    unittest.main()
