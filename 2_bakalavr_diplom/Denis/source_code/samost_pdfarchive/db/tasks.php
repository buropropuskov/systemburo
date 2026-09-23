<?php

defined('MOODLE_INTERNAL') || die();

$tasks = [
    [
        'classname'   => '\plagiarism_samost\task\process_queue',
        'blocking'    => 0,
        'minute'      => '*',
        'hour'        => '*',
        'day'         => '*',
        'month'       => '*',
        'dayofweek'   => '*',
    ],
];
