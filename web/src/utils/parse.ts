// utils/parse.ts — parsing helpers for sizes, times, dates

/** Parse a size string (e.g. "3.14 TiB") → GiB as number, or null */
export function parseSize(str: unknown): number | null {
  if (!str) return null;
  const m = String(str).match(/([\d.]+)\s*(B|[KMGTP]i?B)/i);
  if (!m) return null;
  // Decimal-labelled units (kb/mb/gb/tb) map to the binary factors — tracker
  // software computes 1024-based sizes but often mislabels them.
  const factors: Record<string, number> = {
    b: 1 / (1024 ** 3), kib: 1 / (1024 ** 2), mib: 1 / 1024,
    gib: 1, tib: 1024, pib: 1048576,
    kb: 1 / (1024 ** 2), mb: 1 / 1024, gb: 1, tb: 1024, pb: 1048576,
  };
  return parseFloat(m[1]) * (factors[m[2].toLowerCase()] ?? 1);
}

type DurUnit = 'year' | 'month' | 'week' | 'day' | 'hour' | 'minute' | 'second' | null;

const DUR_SECONDS: Record<Exclude<DurUnit, null>, number> = {
  year: 365 * 86400, month: 30 * 86400, week: 7 * 86400,
  day: 86400, hour: 3600, minute: 60, second: 1,
};

/**
 * Map a unit token to its duration unit. Case matters where tracker layouts
 * rely on it: "M" is months against "m" minutes in English, and "S" is
 * semanas (weeks) against "s" seconds on Portuguese sites. A bare lowercase
 * "m" is ambiguous and gets resolved by position in parseSeedTime.
 */
function classifyDurUnit(raw: string): DurUnit {
  switch (raw) {
    case 'y': case 'Y': case 'a': case 'A':
    case 'ano': case 'anos': case 'year': case 'years':
      return 'year';
    case 'M': case 'mo': case 'mes': case 'meses': case 'month': case 'months':
      return 'month';
    case 'w': case 'W': case 'S':
    case 'sem': case 'semana': case 'semanas': case 'week': case 'weeks':
      return 'week';
    case 'd': case 'D': case 'dia': case 'dias': case 'day': case 'days':
      return 'day';
    case 'h': case 'H': case 'hora': case 'horas': case 'hour': case 'hours':
      return 'hour';
    case 'min': case 'mins':
    case 'minuto': case 'minutos': case 'minute': case 'minutes':
      return 'minute';
    case 's': case 'seg': case 'segs': case 'segundo': case 'segundos':
    case 'sec': case 'secs': case 'second': case 'seconds':
      return 'second';
  }
  return null;
}

/**
 * Parse seed time string → total seconds.
 * Splits the string into value/unit pairs so every occurrence of a unit
 * counts, and understands the Portuguese unit letters UNIT3D renders in
 * pt-BR ("178a 2m 4d 20h 52m 38s" = 178 years, 2 months … 52 minutes).
 * Also accepts a plain integer/float as raw seconds.
 * Mirrors parse.SeedTimeToSeconds on the Go side.
 */
export function parseSeedTime(str: unknown): number | null {
  if (str == null || str === '') return null;
  const s = String(str).trim();
  if (!s) return null;
  // Plain number → raw seconds
  if (/^\d+(\.\d+)?$/.test(s)) return Math.round(parseFloat(s));

  const tokens = [...s.matchAll(/(\d+(?:\.\d+)?)\s*([A-Za-zÀ-ÿ]+)/g)];
  if (!tokens.length) return null;

  const units = tokens.map((t) => classifyDurUnit(t[2]));
  // A bare lowercase "m" means minutes in English layouts and months in
  // Portuguese ones. UNIT3D renders units in descending order, so an "m" with
  // a day, week or hour after it occupies the month slot; later ones are
  // minutes.
  tokens.forEach((t, i) => {
    if (units[i] !== null || t[2] !== 'm') return;
    units[i] = 'minute';
    for (let j = i + 1; j < units.length; j++) {
      if (units[j] === 'day' || units[j] === 'week' || units[j] === 'hour') {
        units[i] = 'month';
        break;
      }
    }
  });

  let total = 0, found = false;
  tokens.forEach((t, i) => {
    const u = units[i];
    if (!u) return;
    found = true;
    total += parseFloat(t[1]) * DUR_SECONDS[u];
  });
  return found ? Math.round(total) : null;
}

/** Calculate account age in whole days from a YYYY-MM-DD string */
export function memberDays(d: string): number | null {
  if (!d) return null;
  const j = new Date(d);
  if (isNaN(j.getTime())) return null;
  return Math.floor((Date.now() - j.getTime()) / 86_400_000);
}

/** Format account age in days to "1Y 2M 3W 4D" */
export function memberDur(d: string): string {
  const days = memberDays(d);
  if (days === null || days < 0) return '—';
  if (days === 0) return '0D';
  const Y = Math.floor(days / 365), r1 = days % 365;
  const M = Math.floor(r1 / 30),   r2 = r1 % 30;
  const W = Math.floor(r2 / 7),    D = r2 % 7;
  return ([[Y,'Y'],[M,'M'],[W,'W'],[D,'D']] as [number,string][])
    .filter(([v]) => v)
    .map(([v, u]) => `${v}${u}`)
    .join(' ') || '0D';
}

/**
 * Parse account-age target: plain number (days) or "Y M W D" format.
 * Returns total days.
 */
export function parseAgeDays(str: string): number | null {
  if (!str) return null;
  const s = String(str).trim();
  if (/^\d+$/.test(s)) return parseInt(s);
  let total = 0;
  const yr = s.match(/(\d+)\s*Y/i); if (yr) total += parseInt(yr[1]) * 365;
  const mo = s.match(/(\d+)\s*M/);  if (mo) total += parseInt(mo[1]) * 30;
  const wk = s.match(/(\d+)\s*W/i); if (wk) total += parseInt(wk[1]) * 7;
  const dd = s.match(/(\d+)\s*D/i); if (dd) total += parseInt(dd[1]);
  return total > 0 ? total : null;
}

/** Build a favicon URL from a tracker URL */
export function getFaviconUrl(trackerUrl: string): string {
  try {
    const u = new URL(trackerUrl);
    return `${u.protocol}//${u.host}/favicon.ico`;
  } catch { return ''; }
}
