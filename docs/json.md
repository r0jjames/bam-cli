# JSON output

Every read command accepts `--json` and prints one JSON document to stdout. `watch --json` and `run --watch --json` print NDJSON: one event per line, ending with `{"type":"done","build":{...}}`.

Rules:

- every entity has `key` and `url`
- times are RFC3339 UTC strings or `null`
- durations are integers in `*_ms` fields
- lists are `[]`, never `null`
- secret values are `"********"`
- errors are plain text on stderr; the exit code gives the class

Within a major version fields are only added, never renamed, retyped or removed.

## Build

    {"key":"PROJ-BUILD-45","url":"...","plan_key":"PROJ-BUILD","branch":"","number":45,"state":"failed",
     "reason":"Manual run by jdoe","custom_build":false,"labels":[],"queued_at":null,
     "started_at":"2026-09-12T11:56:20Z","finished_at":"2026-09-12T12:00:00Z","queue_duration_ms":0,
     "duration_ms":220000,"agent":"","revisions":[],"stages":[{"name":"Test","state":"failed",
     "duration_ms":31000,"jobs":[{"key":"PROJ-BUILD-INT-45","url":"...","name":"Integration",
     "state":"failed","duration_ms":31000}]}],"failed_tests":[]}

`state` is one of `queued`, `running`, `success`, `failed`, `stopped`, `skipped`, `not_built`, `unknown`.

A running build carries the server's duration estimate as well:

    "progress":{"percent":0.55,"average_ms":180000,"elapsed_ms":99000,"remaining_ms":81000,"stage":"Deploy"}

`percent` is a number between 0 and 1, `average_ms` the plan's average build duration, and `stage` the stage running now. The whole `progress` object is absent when the build is not running, or when the Bamboo server does not report progress. `stage` is absent when the server does not name one.

## Plan

`plan show --json` includes a `"variables_error"` string field when the plan's variables could not be read (for example, when the Bamboo server does not expose the variables endpoint). The field is omitted entirely when reading the variables succeeds.

## Events

    {"type":"state","time":"...","build_key":"PROJ-BUILD-45","state":"running","progress":{"percent":0.55,"average_ms":180000,"elapsed_ms":99000,"remaining_ms":81000}}
    {"type":"stage","time":"...","build_key":"PROJ-BUILD-45","name":"Test","state":"running"}
    {"type":"job","time":"...","build_key":"PROJ-BUILD-45","name":"Integration","key":"PROJ-BUILD-INT-45","state":"failed"}
    {"type":"done","time":"...","build_key":"PROJ-BUILD-45","state":"failed","build":{...}}

`state`, `stage` and `job` events carry `progress` while the server reports it. `done` and error events never do: a finished build has a duration, not an estimate.
