<?php
/*
 * @package    plagiarism_samost
 * @copyright  2026 Mikhailov D.A., RTU MIREA
 * @license    http://www.gnu.org/copyleft/gpl.html GNU GPL v3 or later
 */

defined('MOODLE_INTERNAL') || die();

function xmldb_plagiarism_samost_upgrade($oldversion) {
    global $DB;
    $dbman = $DB->get_manager();

    if ($oldversion < 2026031001) {
        $table = new xmldb_table('plagiarism_samost_results');

        $field = new xmldb_field('similarity_code', XMLDB_TYPE_NUMBER, '5', null, false, null, '0', 'userid2');
        $field->setDecimals(2);
        if (!$dbman->field_exists($table, $field)) {
            $dbman->add_field($table, $field);
        }

        $field = new xmldb_field('similarity_text', XMLDB_TYPE_NUMBER, '5', null, false, null, '0', 'similarity_code');
        $field->setDecimals(2);
        if (!$dbman->field_exists($table, $field)) {
            $dbman->add_field($table, $field);
        }

        if ($dbman->field_exists($table, new xmldb_field('similarity'))) {
            $DB->execute("UPDATE {plagiarism_samost_results} SET similarity_code = similarity WHERE similarity_code = 0");
            $dbman->drop_field($table, new xmldb_field('similarity'));
        }

        upgrade_plugin_savepoint(true, 2026031001, 'plagiarism', 'samost');
    }

    if ($oldversion < 2026041002) {
        $table = new xmldb_table('plagiarism_samost_queue');

        if (!$dbman->table_exists($table)) {
            $table->add_field('id',           XMLDB_TYPE_INTEGER, '10',  XMLDB_UNSIGNED, XMLDB_NOTNULL, XMLDB_SEQUENCE);
            $table->add_field('cm',           XMLDB_TYPE_INTEGER, '10',  XMLDB_UNSIGNED, XMLDB_NOTNULL, null, '0');
            $table->add_field('userid',       XMLDB_TYPE_INTEGER, '10',  XMLDB_UNSIGNED, XMLDB_NOTNULL, null, '0');
            $table->add_field('status',       XMLDB_TYPE_CHAR,    '20',  null,           XMLDB_NOTNULL, null, 'queued');
            $table->add_field('timecreated',  XMLDB_TYPE_INTEGER, '10',  XMLDB_UNSIGNED, XMLDB_NOTNULL, null, '0');
            $table->add_field('timemodified', XMLDB_TYPE_INTEGER, '10',  XMLDB_UNSIGNED, XMLDB_NOTNULL, null, '0');

            $table->add_key('primary', XMLDB_KEY_PRIMARY, ['id']);

            $table->add_index('status_timecreated', XMLDB_INDEX_NOTUNIQUE, ['status', 'timecreated']);
            $table->add_index('cm_userid',          XMLDB_INDEX_NOTUNIQUE, ['cm', 'userid']);

            $dbman->create_table($table);
        }

        upgrade_plugin_savepoint(true, 2026041002, 'plagiarism', 'samost');
    }

    if ($oldversion < 2026041101) {
        $table = new xmldb_table('plagiarism_samost_queue');

        $field = new xmldb_field('groupid', XMLDB_TYPE_INTEGER, '10', null, false, null, '0', 'status');
        if (!$dbman->field_exists($table, $field)) {
            $dbman->add_field($table, $field);
        }

        $field = new xmldb_field('mode', XMLDB_TYPE_CHAR, '20', null, false, null, 'within', 'groupid');
        if (!$dbman->field_exists($table, $field)) {
            $dbman->add_field($table, $field);
        }

        $field = new xmldb_field('compare_userids', XMLDB_TYPE_TEXT, null, null, false, null, null, 'mode');
        if (!$dbman->field_exists($table, $field)) {
            $dbman->add_field($table, $field);
        }

        upgrade_plugin_savepoint(true, 2026041101, 'plagiarism', 'samost');
    }

    if ($oldversion < 2026041201) {
        $table = new xmldb_table('plagiarism_samost_feedback');

        if (!$dbman->table_exists($table)) {
            $table->add_field('id',                   XMLDB_TYPE_INTEGER, '10',  null, XMLDB_NOTNULL, XMLDB_SEQUENCE);
            $table->add_field('cm',                   XMLDB_TYPE_INTEGER, '10',  null, XMLDB_NOTNULL, null, '0');
            $table->add_field('userid',               XMLDB_TYPE_INTEGER, '10',  null, XMLDB_NOTNULL, null, '0');
            $table->add_field('message',              XMLDB_TYPE_TEXT,    null,  null, XMLDB_NOTNULL);
            $table->add_field('pair_userid1',         XMLDB_TYPE_INTEGER, '10',  null, false);
            $table->add_field('pair_userid2',         XMLDB_TYPE_INTEGER, '10',  null, false);
            $table->add_field('snap_similarity_code', XMLDB_TYPE_NUMBER,  '5',   null, false, null, null);
            $table->add_field('snap_similarity_text', XMLDB_TYPE_NUMBER,  '5',   null, false, null, null);
            $table->add_field('timecreated',          XMLDB_TYPE_INTEGER, '10',  null, XMLDB_NOTNULL, null, '0');

            $table->add_key('primary', XMLDB_KEY_PRIMARY, ['id']);
            $table->add_index('cm',     XMLDB_INDEX_NOTUNIQUE, ['cm']);
            $table->add_index('userid', XMLDB_INDEX_NOTUNIQUE, ['userid']);
            $table->add_index('pair',   XMLDB_INDEX_NOTUNIQUE, ['pair_userid1', 'pair_userid2']);

            $dbman->create_table($table);
        }

        upgrade_plugin_savepoint(true, 2026041201, 'plagiarism', 'samost');
    }

    if ($oldversion < 2026041204) {
        $table = new xmldb_table('plagiarism_samost_feedback');

        $field = new xmldb_field('pair_userid1', XMLDB_TYPE_INTEGER, '10', null, false, null, null, 'message');
        if (!$dbman->field_exists($table, $field)) {
            $dbman->add_field($table, $field);
        }

        $field = new xmldb_field('pair_userid2', XMLDB_TYPE_INTEGER, '10', null, false, null, null, 'pair_userid1');
        if (!$dbman->field_exists($table, $field)) {
            $dbman->add_field($table, $field);
        }

        $field = new xmldb_field('snap_similarity_code', XMLDB_TYPE_NUMBER, '5', null, false, null, null, 'pair_userid2');
        $field->setDecimals(2);
        if (!$dbman->field_exists($table, $field)) {
            $dbman->add_field($table, $field);
        }

        $field = new xmldb_field('snap_similarity_text', XMLDB_TYPE_NUMBER, '5', null, false, null, null, 'snap_similarity_code');
        $field->setDecimals(2);
        if (!$dbman->field_exists($table, $field)) {
            $dbman->add_field($table, $field);
        }

        if (!$dbman->index_exists($table, new xmldb_index('pair', XMLDB_INDEX_NOTUNIQUE, ['pair_userid1', 'pair_userid2']))) {
            $dbman->add_index($table, new xmldb_index('pair', XMLDB_INDEX_NOTUNIQUE, ['pair_userid1', 'pair_userid2']));
        }

        upgrade_plugin_savepoint(true, 2026041204, 'plagiarism', 'samost');
    }

    if ($oldversion < 2026041301) {
        $table = new xmldb_table('plagiarism_samost_feedback');
        $field = new xmldb_field('admin_comment', XMLDB_TYPE_TEXT, null, null, false, null, null, 'timecreated');
        if (!$dbman->field_exists($table, $field)) {
            $dbman->add_field($table, $field);
        }
        upgrade_plugin_savepoint(true, 2026041301, 'plagiarism', 'samost');
    }

    return true;
}
