let meta = #{
    name: "cli-operator",
    description: "CLI-first ClipHub path: inspect capabilities, probe playability, report the next zv argv",
    phases: [
        #{ title: "Doctor", detail: "inspect zv capabilities" },
        #{ title: "Probe", detail: "classify playdemo tick-0 safety" },
    ],
    when_to_use: "When forging a CS2 reel from the CLI without Studio.",
};

let doctor_schema = #{
    "type": "object",
    "required": ["record_ready", "hlae_path", "pin_ok", "notes"],
    "properties": #{
        "record_ready": #{ "type": "boolean" },
        "hlae_path": #{ "type": "string" },
        "pin_ok": #{ "type": "boolean" },
        "notes": #{ "type": "string" },
    },
};

let probe_schema = #{
    "type": "object",
    "required": ["class", "first_full_packet_tick", "tickrate", "next"],
    "properties": #{
        "class": #{ "type": "string" },
        "first_full_packet_tick": #{ "type": "number" },
        "tickrate": #{ "type": "number" },
        "next": #{ "type": "string" },
    },
};

if args == () { pause("verification", "Pass args.demo — path to a .dem file."); }
let demo = args.demo;
if demo == () || demo == "" { pause("verification", "Pass args.demo — path to a .dem file."); }

phase("Doctor");
let doctor_prompt = "";
doctor_prompt += "In the ClipHub repo, first run powershell -NoProfile -File .\\scripts\\operator-session.ps1. ";
doctor_prompt += "Then run .\\bin\\zv.exe capabilities --format json. Do not launch CS2 or HLAE. ";
doctor_prompt += "The only allowed HLAE path is %APPDATA%\\cliphub-studio\\tools\\hlae\\2.192.1\\HLAE.exe. ";
doctor_prompt += "C:\\HLAE\\HLAE.exe and C:\\HLAE-2.191.1\\HLAE.exe are wrong. ";
doctor_prompt += "Set hlae_path to the path you will pass as --hlae on record. ";
doctor_prompt += "Set pin_ok=true only if that path contains hlae\\2.192.1\\HLAE.exe and the file exists. ";
doctor_prompt += "Set record_ready from record.ready. Explain any mismatch in notes.";
let doctor = agent(doctor_prompt, #{
    label: "doctor",
    capability_mode: "execute",
    output_schema: doctor_schema,
});
if doctor == () || !doctor.success || doctor.output == () {
    complete(#{ ok: false, reason: "capabilities inspect failed" });
}

phase("Probe");
let probe_prompt = "";
probe_prompt += "In the ClipHub repo, run: .\\bin\\zv.exe demo probe --demo ";
probe_prompt += json_encode(demo);
probe_prompt += " --out playability.json --format json. ";
probe_prompt += "Do not launch CS2 or HLAE. Read the JSON class, first_full_packet_tick, tickrate. ";
probe_prompt += "If class is playable, set next to the staged argv starting with demo players. ";
probe_prompt += "If class is unplayable_start, corrupt, or unknown, set next to STOP do not launch CS2.";
let probe = agent(probe_prompt, #{
    label: "probe",
    capability_mode: "execute",
    output_schema: probe_schema,
});
if probe == () || !probe.success || probe.output == () {
    complete(#{ ok: false, reason: "demo probe failed", doctor: doctor.output });
}

let pin_ok = doctor.output.pin_ok;
let next = probe.output.next;
if pin_ok != true {
    next = "STOP: pin HLAE 2.192.1 via scripts\\operator-session.ps1 and pass --hlae; autodetect on this PC is C:\\HLAE-2.191.1";
}
complete(#{
    ok: pin_ok == true,
    record_ready: doctor.output.record_ready,
    hlae_path: doctor.output.hlae_path,
    pin_ok: pin_ok,
    class: probe.output.class,
    first_full_packet_tick: probe.output.first_full_packet_tick,
    next: next,
});
