let meta = #{
    name: "desktop-release-sync",
    description: "Verify ClipHub Studio desktop version, landing download URL, GitHub latest release, and the in-app updater all name the same installer",
    phases: [
        #{ title: "Collect", detail: "read package.json, landing, updater source, and GitHub latest" },
        #{ title: "Verify", detail: "adversarially confirm the four surfaces match" },
    ],
    when_to_use: "Before or after publishing a ClipHub Studio installer: landing, GitHub Releases, desktop/package.json, and the in-app update button must agree.",
};

let collect_schema = #{
    "type": "object",
    "required": ["desktop_version", "landing_url", "landing_tag", "updater_latest_api", "github_tag"],
    "properties": #{
        "desktop_version": #{ "type": "string" },
        "landing_url": #{ "type": "string" },
        "landing_tag": #{ "type": "string" },
        "updater_latest_api": #{ "type": "string" },
        "github_tag": #{ "type": "string" },
        "github_installer": #{ "type": "string" },
        "error": #{ "type": "string" },
    },
};

let verdict_schema = #{
    "type": "object",
    "required": ["aligned", "reason", "expected_tag"],
    "properties": #{
        "aligned": #{ "type": "boolean" },
        "reason": #{ "type": "string" },
        "expected_tag": #{ "type": "string" },
        "mismatches": #{ "type": "array", "items": #{ "type": "string" } },
    },
};

phase("Collect");
let collect_prompt = "";
collect_prompt += "You are collecting ClipHub Studio release identity from the real repo and GitHub. ";
collect_prompt += "Use read_file, grep, and run_terminal_command. Do not guess. ";
collect_prompt += "1. Read desktop/package.json and take the version string. ";
collect_prompt += "2. Read landing/app/page.tsx and extract DOWNLOAD_URL and RELEASE_VERSION. ";
collect_prompt += "3. Read desktop/src/app-update.ts and extract GITHUB_LATEST_RELEASE_URL. ";
collect_prompt += "4. Run: gh release view --repo rechedev9/cliphub --json tagName,assets ";
collect_prompt += "and take tagName plus the ClipHub.Studio.Setup.*.exe asset name. ";
collect_prompt += "Return those fields exactly. If a command fails, set github_tag to empty and put the error in error.";
let collected = agent(collect_prompt, #{
    label: "collect-release-identity",
    capability_mode: "execute",
    output_schema: collect_schema,
});

if collected == () || !collected.success || collected.output == () {
    complete(#{ aligned: false, reason: "collector failed", expected_tag: "" });
}

phase("Verify");
let verify_prompt = "";
verify_prompt += "Adversarially verify this ClipHub Studio release identity. Read the same files yourself. ";
verify_prompt += "Do not trust the collector. Facts to check: ";
verify_prompt += json_encode(collected.output);
verify_prompt += " Aligned means ALL of: desktop_version is semver X.Y.Z; landing_tag is v plus that version; ";
verify_prompt += "landing_url is https://github.com/rechedev9/cliphub/releases/download/vX.Y.Z/ClipHub.Studio.Setup.X.Y.Z.exe; ";
verify_prompt += "updater_latest_api is exactly https://api.github.com/repos/rechedev9/cliphub/releases/latest ";
verify_prompt += "(the in-app button must never hardcode a version); github_tag equals landing_tag; ";
verify_prompt += "github_installer is ClipHub.Studio.Setup.X.Y.Z.exe. ";
verify_prompt += "Set aligned=true only if every check passed with evidence you re-read. ";
verify_prompt += "expected_tag is vX.Y.Z from desktop/package.json.";
let verdict = agent(verify_prompt, #{
    label: "verify-release-identity",
    capability_mode: "execute",
    output_schema: verdict_schema,
});

if verdict == () || !verdict.success || verdict.output == () {
    complete(#{ aligned: false, reason: "verifier failed", expected_tag: collected.output.desktop_version });
}

complete(verdict.output);
