let meta = #{
    name: "name-tournament",
    description: "Creative naming tournament for the CS2 local reel pipeline (replacing TickCut)",
    phases: [
        #{ title: "Ideate", detail: "four creative tables in parallel" },
        #{ title: "Skeptic", detail: "adversarial screen of candidates" },
        #{ title: "Finals", detail: "rank survivors into a shortlist" },
    ],
    when_to_use: "When renaming the product away from a taken name and you want a multi-angle creative shortlist.",
};

let ideas_schema = #{
    "type": "object",
    "required": ["names"],
    "properties": #{
        "names": #{
            "type": "array",
            "minItems": 5,
            "maxItems": 8,
            "items": #{
                "type": "object",
                "required": ["name", "tagline", "why", "pronunciation"],
                "properties": #{
                    "name": #{ "type": "string" },
                    "tagline": #{ "type": "string" },
                    "why": #{ "type": "string" },
                    "pronunciation": #{ "type": "string" },
                },
            },
        },
    },
};

let skeptic_schema = #{
    "type": "object",
    "required": ["survivors"],
    "properties": #{
        "survivors": #{
            "type": "array",
            "maxItems": 12,
            "items": #{
                "type": "object",
                "required": ["name", "keep", "risk", "score"],
                "properties": #{
                    "name": #{ "type": "string" },
                    "keep": #{ "type": "boolean" },
                    "risk": #{ "type": "string" },
                    "score": #{ "type": "integer" },
                },
            },
        },
    },
};

let finals_schema = #{
    "type": "object",
    "required": ["finalists", "notes"],
    "properties": #{
        "finalists": #{
            "type": "array",
            "minItems": 5,
            "maxItems": 8,
            "items": #{
                "type": "object",
                "required": ["rank", "name", "tagline", "pitch", "domain_guess", "cons"],
                "properties": #{
                    "rank": #{ "type": "integer" },
                    "name": #{ "type": "string" },
                    "tagline": #{ "type": "string" },
                    "pitch": #{ "type": "string" },
                    "domain_guess": #{ "type": "string" },
                    "cons": #{ "type": "string" },
                },
            },
        },
        "notes": #{ "type": "string" },
    },
};

// Product brief shared by every table (cold agents need full context).
let brief = "";
brief += "PRODUCT: Windows-local deterministic CS2 demo/stream-to-vertical-reels pipeline. ";
brief += "Source of truth is the .dem (or stream VOD), not AI guesswork. ";
brief += "Flow: parse demo -> kill plan -> HLAE/CS2 capture -> FFmpeg/Lua effects -> publish pack (MP4, cover, caption). ";
brief += "Audience: CS2 creators, clippers, aspiring content makers; bilingual ES/EN ok. ";
brief += "Tone: sharp, competitive, craft/tooling — not toy, not enterprise SaaS. ";
brief += "CONSTRAINTS: current name TickCut is already taken elsewhere — do NOT propose TickCut, ForgeFrag, Frag-Forge variants, or near-collisions. ";
brief += "Prefer one word or tight compounds (2-3 syllables ideal). Avoid pure generic (Clipper, ShortsMaker). ";
brief += "Names should work as a desktop app brand and GitHub org. ";
brief += "Avoid trademarks of Valve, Faceit, Twitch, YouTube, Adobe. ";
brief += "Spanish market friendly pronunciation is a plus but English-first brand is fine.";

phase("Ideate");
log("Four creative tables: metal, cinema, radar, slang");

let tables = [
    #{
        angle: "metal-forge",
        prompt_extra: "Metaphor: metal, anvil, quench, edge, alloy, barrel, bolt — weapon-smith energy without sounding like a gun shop. Think blacksmith of frags.",
    },
    #{
        angle: "cinema-cut",
        prompt_extra: "Metaphor: edit bay, reel, splice, slate, director, take, cut — Hollywood of the deathmatch. Premium post-production for kills.",
    },
    #{
        angle: "radar-tactical",
        prompt_extra: "Metaphor: radar, crosshair, tick, demo, replay, angle, hold, peek — pure CS2 language made into a tool brand. Readable to players.",
    },
    #{
        angle: "creator-slang",
        prompt_extra: "Metaphor: clip culture, viral shorts, POV, ace, multi-kill, clutch — modern creator slang that still feels premium, not cringe TikTok.",
    },
];

let ideate_jobs = [];
for t in tables {
    let p = "";
    p += "You are a brand naming table for a CS2 creative tool. ";
    p += brief + " ";
    p += "YOUR ANGLE: " + t.angle + ". " + t.prompt_extra + " ";
    p += "Invent 5-8 original product names. For each: name, short tagline (ES or EN, max 8 words), why it fits, pronunciation guide. ";
    p += "No research tools required — pure creative judgment. Diversity inside the angle; no duplicates of each other. ";
    p += "Do not output TickCut or any forge+frag mashup.";
    ideate_jobs.push(#{
        prompt: p,
        label: "ideate:" + t.angle,
        capability_mode: "read-only",
        output_schema: ideas_schema,
    });
}

let ideate_results = parallel(ideate_jobs);

let pool = [];
let seen = #{};
for r in ideate_results {
    if r == () || !r.success || r.output == () || r.output.names == () {
        log("ideate slot failed or empty — skipping");
        continue;
    }
    for n in r.output.names {
        if n == () || n.name == () { continue; }
        let key = n.name;
        key.trim();
        if key == "" { continue; }
        // fingerprint() is case-sensitive enough for brand dedupe without
        // needing string lowercasing (Rhai has no portable lower() here).
        let dedupe = fingerprint(key);
        if seen[dedupe] == true { continue; }
        seen[dedupe] = true;
        pool.push(n);
    }
}

if pool.len() == 0 {
    complete(#{ summary: "No names generated.", finalists: [] });
}

log("Pool size: " + pool.len().to_string());

// Serialize candidates for the skeptic (bounded).
let max_pool = 28;
if pool.len() > max_pool {
    log("Truncating pool from " + pool.len().to_string() + " to " + max_pool.to_string());
    let trimmed = [];
    let i = 0;
    for n in pool {
        if i >= max_pool { break; }
        trimmed.push(n);
        i += 1;
    }
    pool = trimmed;
}

let pool_json = json_encode(pool);

phase("Skeptic");
log("Adversarial brand screen on the pooled candidates");

let skeptic_prompt = "";
skeptic_prompt += "You are a ruthless brand skeptic for a CS2 desktop product. ";
skeptic_prompt += brief + " ";
skeptic_prompt += "CANDIDATES JSON: " + pool_json + " ";
skeptic_prompt += "For EACH candidate name, decide keep=true/false. ";
skeptic_prompt += "Reject if: too close to TickCut; sounds like existing major tools (Premiere, OBS, Medal, Shadowplay, Faceit, HLTV, CSGO.Demos); ";
skeptic_prompt += "unpronounceable; offensive double meaning in ES/EN; generic SEO spam; trademark-looking of known orgs. ";
skeptic_prompt += "score 1-10 (10=best brand for THIS product). ";
skeptic_prompt += "Return survivors array covering every name you evaluated (include keep=false ones with risk notes). ";
skeptic_prompt += "Be strict: keep only names you'd defend to a founder.";

let skeptic = agent(skeptic_prompt, #{
    label: "skeptic-brand",
    capability_mode: "read-only",
    output_schema: skeptic_schema,
});

let kept = [];
if skeptic != () && skeptic.success && skeptic.output != () && skeptic.output.survivors != () {
    for s in skeptic.output.survivors {
        if s != () && s.keep == true && s.name != () {
            kept.push(s);
        }
    }
}

if kept.len() == 0 {
    // Fail open to top of pool with low confidence so the user still gets options.
    log("Skeptic kept none — falling back to first 8 raw pool names for finals");
    let i = 0;
    for n in pool {
        if i >= 8 { break; }
        kept.push(#{ name: n.name, keep: true, risk: "skeptic empty — unverified", score: 5 });
        i += 1;
    }
}

log("Survivors: " + kept.len().to_string());

// Attach original taglines from pool for finals.
let enriched = [];
for s in kept {
    let tag = "";
    let why = "";
    for n in pool {
        if fingerprint(s.name) == fingerprint(n.name) {
            if n.tagline != () { tag = n.tagline; }
            if n.why != () { why = n.why; }
            break;
        }
    }
    enriched.push(#{
        name: s.name,
        score: s.score,
        risk: s.risk,
        tagline: tag,
        why: why,
    });
}

let kept_json = json_encode(enriched);

phase("Finals");
log("Ranking shortlist for founder decision");

let finals_prompt = "";
finals_prompt += "You are the creative director closing a naming tournament. ";
finals_prompt += brief + " ";
finals_prompt += "SURVIVORS JSON: " + kept_json + " ";
finals_prompt += "Pick 5-8 finalists ranked 1..N (best first). ";
finals_prompt += "For each: rank, name, tagline (improve if weak), pitch (2 sentences, Spanish preferred), domain_guess (e.g. name.app / name.dev plausibility — guess only), cons (honest downside). ";
finals_prompt += "Maximize diversity of vibes across the shortlist (not 8 metal synonyms). ";
finals_prompt += "notes: one paragraph for the founder on how to choose.";

let finals = agent(finals_prompt, #{
    label: "finals-rank",
    capability_mode: "read-only",
    output_schema: finals_schema,
});

if finals == () || !finals.success || finals.output == () {
    complete(#{
        summary: "Finals agent failed; survivors only",
        survivors: enriched,
        finalists: [],
    });
}

// Write a markdown report for the user session.
let report = "# Name tournament — shortlist\n\n";
report += finals.output.notes + "\n\n";
report += "| Rank | Name | Tagline | Cons |\n";
report += "|------|------|---------|------|\n";
for f in finals.output.finalists {
    let rank = f.rank;
    let name = f.name;
    let tag = f.tagline;
    let cons = f.cons;
    report += "| " + rank.to_string() + " | **" + name + "** | " + tag + " | " + cons + " |\n";
}
report += "\n## Pitches\n\n";
for f in finals.output.finalists {
    report += "### " + f.rank.to_string() + ". " + f.name + "\n";
    report += f.pitch + "\n\n";
    report += "- Domain guess: " + f.domain_guess + "\n";
    report += "- Cons: " + f.cons + "\n\n";
}

let path = write_scratch_file("name-tournament.md", report);

complete(#{
    summary: "Name tournament finished — " + finals.output.finalists.len().to_string() + " finalists",
    path: path,
    finalists: finals.output.finalists,
    notes: finals.output.notes,
    pool_size: pool.len(),
    survivors: kept.len(),
});
