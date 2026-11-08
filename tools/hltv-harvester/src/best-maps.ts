// Best team per map: reads each ranked team's HLTV map stats page and keeps, for every map,
// the team with the highest win rate over enough maps. Page scripts return unknown, checked here.

export type TeamMapRecord = { map: string; wins: number; draws: number; losses: number };

export type BestMapPick = { map: string; team_id: number; team: string; rank: number; maps: number; win_rate: number };

export const MAP_STATS_SCRIPT = `[...document.querySelectorAll('.two-grid .col')].map(c => ({
  name: c.querySelector('.map-pool-map-name')?.textContent?.trim() ?? '',
  rows: [...c.querySelectorAll('.stats-row')].map(r => [...r.querySelectorAll('span')].map(s => s.textContent?.trim() ?? ''))
})).filter(m => m.name !== '')`;

const WDL = /^(\d+)\s*\/\s*(\d+)\s*\/\s*(\d+)$/;

/** mapKey turns an HLTV map name ("Dust2", "Mirage - 63.6%") into the demo map key ("de_dust2"). */
export function mapKey(name: string): string {
  const base = name.split(" - ")[0].trim().toLowerCase().replace(/[^a-z0-9]/g, "");
  return base === "" ? "" : `de_${base}`;
}

export function parseTeamMapStats(raw: unknown): TeamMapRecord[] {
  if (!Array.isArray(raw)) return [];
  const out: TeamMapRecord[] = [];
  for (const item of raw) {
    if (typeof item !== "object" || item === null || Array.isArray(item)) continue;
    const name = "name" in item && typeof item.name === "string" ? item.name : "";
    const rows = "rows" in item && Array.isArray(item.rows) ? item.rows : [];
    const map = mapKey(name);
    for (const row of rows) {
      if (!Array.isArray(row) || row.length < 2 || !/wins/i.test(String(row[0]))) continue;
      const m = WDL.exec(String(row[row.length - 1]));
      if (map !== "" && m) out.push({ map, wins: Number(m[1]), draws: Number(m[2]), losses: Number(m[3]) });
    }
  }
  return out;
}

export type TeamStats = { id: number; name: string; rank: number; records: TeamMapRecord[] };

/** pickBestPerMap keeps, per map, the best win rate among teams with at least minMaps maps; ties go to more maps, then rank. */
export function pickBestPerMap(teams: TeamStats[], minMaps: number): BestMapPick[] {
  const best = new Map<string, BestMapPick>();
  for (const t of teams) {
    for (const r of t.records) {
      const maps = r.wins + r.draws + r.losses;
      if (maps < minMaps) continue;
      const pick: BestMapPick = { map: r.map, team_id: t.id, team: t.name, rank: t.rank, maps, win_rate: Math.round((r.wins / maps) * 1000) / 1000 };
      const cur = best.get(r.map);
      if (cur === undefined || better(pick, cur)) best.set(r.map, pick);
    }
  }
  return [...best.values()].sort((a, b) => a.map.localeCompare(b.map));
}

function better(a: BestMapPick, b: BestMapPick): boolean {
  if (a.win_rate !== b.win_rate) return a.win_rate > b.win_rate;
  if (a.maps !== b.maps) return a.maps > b.maps;
  return a.rank < b.rank;
}

/** teamSlug is the URL slug HLTV uses for a team name. */
export function teamSlug(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "") || "team";
}

export function isoDay(ms: number): string {
  return new Date(ms).toISOString().slice(0, 10);
}
