let meta = #{
    name: "desktop-efficiency",
    description: "Measure ClipHub Studio process-tree cost, persist a baseline, remasure after a change, and accept only raw-process wins",
    phases: [
        #{ title: "Measure", detail: "sample the desktop process tree and persist a baseline" },
        #{ title: "Change", detail: "apply a candidate code or config change" },
        #{ title: "Remeasure", detail: "sample the same scenario and duration" },
        #{ title: "Compare", detail: "accept only documented raw-process wins" },
    ],
    when_to_use: "When iterating on ClipHub Studio raw process cost (CPU, working set, private bytes) without launching HLAE, CS2, or a render.",
};

let measure_schema = #{
    "type": "object",
    "required": ["ok", "path", "scenario", "duration_seconds"],
    "properties": #{
        "ok": #{ "type": "boolean" },
        "path": #{ "type": "string" },
        "scenario": #{ "type": "string" },
        "duration_seconds": #{ "type": "number" },
        "error": #{ "type": "string" },
    },
};

let compare_schema = #{
    "type": "object",
    "required": ["ok", "accept", "verdict", "reason"],
    "properties": #{
        "ok": #{ "type": "boolean" },
        "accept": #{ "type": "boolean" },
        "verdict": #{ "type": "string" },
        "reason": #{ "type": "string" },
        "baseline_path": #{ "type": "string" },
        "candidate_path": #{ "type": "string" },
        "error": #{ "type": "string" },
    },
};

if args == () { pause("verification", "Pass args.scenario and either args.root_pid (iterate/measure) or args.baseline plus args.candidate (compare)."); }

let scenarios = ["foreground-idle", "background-idle", "stream-static", "stream-playback"];
let scenario = args.scenario;
if scenario == () || scenario == "" {
    pause("verification", "Pass args.scenario — one of foreground-idle, background-idle, stream-static, stream-playback.");
}
let scenario_ok = false;
for s in scenarios {
    if s == scenario { scenario_ok = true; }
}
if !scenario_ok {
    pause("verification", "args.scenario must be one of foreground-idle, background-idle, stream-static, stream-playback.");
}

let mode = args.mode;
if mode == () || mode == "" { mode = "iterate"; }

let seconds = args.seconds;
if seconds == () { seconds = 15; }

let forbidden = "Do not launch HLAE, CS2, cs2.exe, capture, FFmpeg, a render job, or zv record/shorts. Sample only the desktop app process tree (Electron and its owned children).";

if mode == "compare" {
    let baseline_path = args.baseline;
    let candidate_path = args.candidate;
    if baseline_path == () || baseline_path == "" || candidate_path == () || candidate_path == "" {
        pause("verification", "compare mode requires args.baseline and args.candidate report paths.");
    }
    phase("Compare");
    let compare_prompt = "";
    compare_prompt += "In the ClipHub repo, compare two existing desktop efficiency reports. ";
    compare_prompt += forbidden + " ";
    compare_prompt += "Run: node desktop/scripts/compare-efficiency.mjs --baseline ";
    compare_prompt += json_encode(baseline_path);
    compare_prompt += " --candidate ";
    compare_prompt += json_encode(candidate_path);
    compare_prompt += ". Read the JSON. Set ok=true if the command produced a verdict. ";
    compare_prompt += "Copy accept, verdict, and reason from that JSON. ";
    compare_prompt += "The compare/accept rule: accept only when verdict is improve — same scenario, same duration, ";
    compare_prompt += "cpu_p95_percent / working_set_peak_bytes / private_bytes_peak are all <= baseline and at least one is strictly lower. ";
    compare_prompt += "A mixed result is regress. Different scenario or duration is incomparable. GPU does not decide accept. ";
    compare_prompt += "Set baseline_path and candidate_path to the paths you compared.";
    let compared = agent(compare_prompt, #{
        label: "compare-reports",
        capability_mode: "execute",
        output_schema: compare_schema,
    });
    if compared == () || !compared.success || compared.output == () {
        complete(#{ ok: false, reason: "compare entry failed", scenario: scenario });
    }
    complete(#{
        ok: compared.output.ok,
        accept: compared.output.accept,
        verdict: compared.output.verdict,
        reason: compared.output.reason,
        scenario: scenario,
        baseline: compared.output.baseline_path,
        candidate: compared.output.candidate_path,
        sequence: "measure persist baseline change remasure same scenario/duration compare",
        rule: "accept only when verdict=improve on cpu_p95_percent, working_set_peak_bytes, private_bytes_peak",
    });
}

let root_pid = args.root_pid;
if root_pid == () {
    pause("verification", "Pass args.root_pid — the Electron main PID (or any live root to sample).");
}

let baseline_out = "desktop/e2e/artifacts/efficiency-" + scenario + "-baseline.json";
let candidate_out = "desktop/e2e/artifacts/efficiency-" + scenario + "-candidate.json";

phase("Measure");
let measure_prompt = "";
measure_prompt += "In the ClipHub repo, persist a baseline desktop efficiency sample. ";
measure_prompt += forbidden + " ";
measure_prompt += "Run this exact sampler, no other media tools: ";
measure_prompt += "powershell -NoProfile -File .\\scripts\\measure-desktop-efficiency.ps1 -RootPid ";
measure_prompt += json_encode(root_pid);
measure_prompt += " -Scenario ";
measure_prompt += json_encode(scenario);
measure_prompt += " -Seconds ";
measure_prompt += json_encode(seconds);
measure_prompt += " -OutputPath ";
measure_prompt += json_encode(baseline_out);
measure_prompt += ". Then read that JSON and confirm schema_version, scenario, duration_seconds, summary.cpu_p95_percent, summary.working_set_peak_bytes, and a top-level roles object. ";
measure_prompt += "Set ok=true only if that file exists and those fields are present. Set path to the output path, scenario and duration_seconds from the file.";
let measured = agent(measure_prompt, #{
    label: "measure-baseline",
    capability_mode: "execute",
    output_schema: measure_schema,
});
if measured == () || !measured.success || measured.output == () || measured.output.ok != true {
    let err = "baseline sample failed";
    if measured != () && measured.output != () && measured.output.error != () { err = measured.output.error; }
    complete(#{ ok: false, reason: err, scenario: scenario });
}

if mode == "measure" {
    complete(#{
        ok: true,
        phase: "baseline-persisted",
        scenario: scenario,
        duration_seconds: measured.output.duration_seconds,
        baseline: measured.output.path,
        next: "change code or config, then remasure the same scenario and duration and compare with compare-efficiency.mjs",
        sequence: "measure persist baseline change remasure same scenario/duration compare",
        rule: "accept only when verdict=improve on cpu_p95_percent, working_set_peak_bytes, private_bytes_peak",
    });
}

phase("Change");
await_user("user", "Baseline persisted. Apply the candidate code or config change for this raw-process iteration. Do not launch HLAE, CS2, or a render. Resume to remasure the same scenario and duration.");

phase("Remeasure");
let remasure_prompt = "";
remasure_prompt += "In the ClipHub repo, remasure the same desktop efficiency scenario and duration as the persisted baseline. ";
remasure_prompt += forbidden + " ";
remasure_prompt += "Use the same scenario and the same requested duration as the baseline. Run: ";
remasure_prompt += "powershell -NoProfile -File .\\scripts\\measure-desktop-efficiency.ps1 -RootPid ";
remasure_prompt += json_encode(root_pid);
remasure_prompt += " -Scenario ";
remasure_prompt += json_encode(scenario);
remasure_prompt += " -Seconds ";
remasure_prompt += json_encode(seconds);
remasure_prompt += " -OutputPath ";
remasure_prompt += json_encode(candidate_out);
remasure_prompt += ". Then read that JSON and confirm schema_version, scenario, duration_seconds, summary.cpu_p95_percent, summary.working_set_peak_bytes, and a top-level roles object. ";
remasure_prompt += "Set ok=true only if that file exists and those fields are present. Set path to the output path, scenario and duration_seconds from the file.";
let remasured = agent(remasure_prompt, #{
    label: "remasure-candidate",
    capability_mode: "execute",
    output_schema: measure_schema,
});
if remasured == () || !remasured.success || remasured.output == () || remasured.output.ok != true {
    let remasure_err = "candidate remasure failed";
    if remasured != () && remasured.output != () && remasured.output.error != () { remasure_err = remasured.output.error; }
    complete(#{ ok: false, reason: remasure_err, scenario: scenario, baseline: measured.output.path });
}

phase("Compare");
let iterate_compare_prompt = "";
iterate_compare_prompt += "In the ClipHub repo, compare the persisted baseline against the remasured candidate. ";
iterate_compare_prompt += forbidden + " ";
iterate_compare_prompt += "Run: node desktop/scripts/compare-efficiency.mjs --baseline ";
iterate_compare_prompt += json_encode(baseline_out);
iterate_compare_prompt += " --candidate ";
iterate_compare_prompt += json_encode(candidate_out);
iterate_compare_prompt += ". Read the JSON. Set ok=true if the command produced a verdict. ";
iterate_compare_prompt += "Copy accept, verdict, and reason from that JSON. ";
iterate_compare_prompt += "The compare/accept rule: accept only when verdict is improve — same scenario, same duration, ";
iterate_compare_prompt += "cpu_p95_percent / working_set_peak_bytes / private_bytes_peak are all <= baseline and at least one is strictly lower. ";
iterate_compare_prompt += "A mixed result is regress. Different scenario or duration is incomparable. GPU does not decide accept. ";
iterate_compare_prompt += "Set baseline_path and candidate_path to the paths you compared.";
let iterate_compared = agent(iterate_compare_prompt, #{
    label: "compare-iteration",
    capability_mode: "execute",
    output_schema: compare_schema,
});
if iterate_compared == () || !iterate_compared.success || iterate_compared.output == () {
    complete(#{ ok: false, reason: "compare after remasure failed", scenario: scenario });
}
complete(#{
    ok: iterate_compared.output.ok,
    accept: iterate_compared.output.accept,
    verdict: iterate_compared.output.verdict,
    reason: iterate_compared.output.reason,
    scenario: scenario,
    duration_seconds: seconds,
    baseline: iterate_compared.output.baseline_path,
    candidate: iterate_compared.output.candidate_path,
    sequence: "measure persist baseline change remasure same scenario/duration compare",
    rule: "accept only when verdict=improve on cpu_p95_percent, working_set_peak_bytes, private_bytes_peak",
});
