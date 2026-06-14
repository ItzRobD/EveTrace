import type { ChartData, PointStyle } from 'chart.js';
import { LiveEvent } from './live-event.model';
import { chartMarker } from './chart-markers';

export const DPS_BUCKET_MS = 5_000;
export const DPS_WINDOW_MS = 1_800_000; // 30 min max storage; display window is user-controlled
export const MINING_WINDOW_MS = 60_000;
export const ISK_WINDOW_MS = 3_600_000;
export const MAX_BOUNTY_HISTORY = 500;
export const MAX_CRITICAL_HISTORY = 720; // 1 hr at 5-sec buckets
export const MAX_FEED_ENTRIES = 500;
export const CAP_ALERT_MS = 30_000;

export interface CombatFeedEntry {
  timestamp: string;
  direction: 'in' | 'out';
  damage: number;
  entity: string;
  miss: boolean;
}

export interface KillFeedEntry {
  timestamp: string;
  entity: string;
  bounty: number;
}

export interface MiningFeedEntry {
  timestamp: string;
  oreType: string;
  amount: number;
  residue: boolean;
  critical: boolean;
}

export interface EntityStat {
  name: string;
  kills: number;
  dmgOut: number;
  dmgIn: number;
}

export interface OreStat {
  oreType: string;
  cycles: number;
  amount: number;
}

export interface BountyEntry {
  timestamp: string;
  isk: number;
}

export interface DpsBucket {
  time: number;
  out: number;
  in: number;
  mining: number;
  residue: number;
}

export interface JumpEntry {
  timestamp: string;
}

export interface NavFeedEntry {
  timestamp: string;
  from: string;
  to: string;
}

export interface CharacterLiveState {
  characterName: string;
  sessionKey: string;
  sessionDbId: number | null;
  combatFeed: CombatFeedEntry[];
  killFeed: KillFeedEntry[];
  miningFeed: MiningFeedEntry[];
  bountyHistory: BountyEntry[];
  totalBountyISK: number;
  totalMined: number;
  totalResidue: number;
  totalCriticals: number;
  criticalHistory: { timestamp: string }[];
  jumpHistory: JumpEntry[];
  navFeed: NavFeedEntry[];
  dpsBuckets: DpsBucket[];
  killMarkers: number[];
  capMarkers: number[];
  criticalMarkers: number[];
  entityStats: Record<string, EntityStat>;
  oreStats: Record<string, OreStat>;
  capAlert: boolean;
  capAlertModule: string | null;
}

export const EMPTY_STATE: Omit<CharacterLiveState, 'characterName' | 'sessionKey' | 'sessionDbId'> = {
  combatFeed: [],
  killFeed: [],
  miningFeed: [],
  bountyHistory: [],
  totalBountyISK: 0,
  totalMined: 0,
  totalResidue: 0,
  totalCriticals: 0,
  criticalHistory: [],
  jumpHistory: [],
  navFeed: [],
  dpsBuckets: [],
  killMarkers: [],
  capMarkers: [],
  criticalMarkers: [],
  entityStats: {},
  oreStats: {},
  capAlert: false,
  capAlertModule: null,
};

export const EMPTY_CHART: ChartData<'line'> = { labels: [], datasets: [] };

function markerDataset(
  label: string,
  buckets: DpsBucket[],
  markerSet: Set<number>,
  color: string,
  pointStyle: PointStyle,
  yAxisID: string,
  yValues?: number[],
): ChartData<'line'>['datasets'][number] {
  return {
    label,
    data: buckets.map((b, i) => (markerSet.has(b.time) ? (yValues ? yValues[i] : 0) : NaN)),
    borderColor: color,
    backgroundColor: color,
    showLine: false,
    pointStyle,
    pointRadius: buckets.map(b => (markerSet.has(b.time) ? 9 : 0)),
    pointHoverRadius: 11,
    yAxisID,
  };
}

export function buildChartData(
  buckets: DpsBucket[],
  killMarkers: number[],
  capMarkers: number[],
  criticalMarkers: number[],
): ChartData<'line'> {
  if (!buckets.length) return EMPTY_CHART;
  const dpsScale = DPS_BUCKET_MS / 1000;
  const miningScale = DPS_BUCKET_MS / 60_000;
  const hasMining = buckets.some(b => b.mining > 0);
  const hasResidue = buckets.some(b => b.residue > 0);
  const hasY2 = hasMining || hasResidue;
  const labels = buckets.map(b =>
    new Date(b.time).toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    }),
  );
  const datasets: ChartData<'line'>['datasets'] = [
    {
      label: 'Outgoing',
      data: buckets.map(b => Math.round(b.out / dpsScale)),
      borderColor: '#3b82f6',
      backgroundColor: 'rgba(59,130,246,0.1)',
      fill: true,
      tension: 0.3,
      pointRadius: 2,
      yAxisID: 'y',
    },
    {
      label: 'Incoming',
      data: buckets.map(b => Math.round(b.in / dpsScale)),
      borderColor: '#ef4444',
      backgroundColor: 'rgba(239,68,68,0.1)',
      fill: true,
      tension: 0.3,
      pointRadius: 2,
      yAxisID: 'y',
    },
  ];
  if (hasMining) {
    datasets.push({
      label: 'Mining',
      data: buckets.map(b => Math.round(b.mining / miningScale)),
      borderColor: '#eab308',
      backgroundColor: 'rgba(234,179,8,0.1)',
      fill: false,
      tension: 0.3,
      pointRadius: 2,
      yAxisID: 'y2',
    });
  }
  if (hasResidue) {
    datasets.push({
      label: 'Residue',
      data: buckets.map(b => Math.round(b.residue / miningScale)),
      borderColor: '#b45309',
      backgroundColor: 'rgba(180,83,9,0.15)',
      fill: false,
      tension: 0.3,
      pointRadius: 2,
      yAxisID: 'y2',
    });
  }

  const killSet = new Set(killMarkers);
  const capSet = new Set(capMarkers);
  const critSet = new Set(criticalMarkers);

  if (killSet.size) {
    // Sit kill markers on the Outgoing DPS line (mirrors how criticals sit on the
    // mining line) rather than pinned to y=0.
    const outValues = buckets.map(b => Math.round(b.out / dpsScale));
    datasets.push(markerDataset('Kill', buckets, killSet, '#f59e0b', chartMarker('kill'), 'y', outValues));
  }
  if (capSet.size) {
    datasets.push(markerDataset('Cap Starved', buckets, capSet, '#ef4444', chartMarker('cap'), 'y'));
  }
  if (critSet.size) {
    const miningValues = buckets.map(b => Math.round(b.mining / miningScale));
    datasets.push(markerDataset('Critical', buckets, critSet, '#22c55e', chartMarker('critical'), hasY2 ? 'y2' : 'y', miningValues));
  }

  return { labels, datasets };
}

export function eventSignature(event: LiveEvent): string {
  let payload = '';
  if (event.Type === 'combat' && event.Combat) {
    payload = `${event.Combat.Direction}|${event.Combat.Damage}|${event.Combat.Entity}`;
  } else if (event.Type === 'kill' && event.Kill) {
    payload = `${event.Kill.Entity}|${event.Kill.BountyISK}`;
  } else if (event.Type === 'mining' && event.Mining) {
    payload = `${event.Mining.OreType}|${event.Mining.Amount}`;
  }
  return `${event.SessionID}|${event.Timestamp}|${event.Type}|${payload}`;
}

// Append a new (always-latest) bucket and trim expired ones from the front.
// Avoids a full sort — buckets stay sorted because events arrive in order.
function appendBucket(buckets: DpsBucket[], newBucket: DpsBucket, cutoff: number): DpsBucket[] {
  const trimStart = buckets.findIndex(b => b.time >= cutoff);
  const trimmed = trimStart < 0 ? [] : trimStart === 0 ? buckets : buckets.slice(trimStart);
  return [...trimmed, newBucket];
}

export function applyEvent(
  existing: CharacterLiveState,
  event: LiveEvent,
): CharacterLiveState {
  const updated: CharacterLiveState = { ...existing };
  const eventMs = Date.parse(event.Timestamp);

  if (event.Type === 'combat' && event.Combat) {
    const entry: CombatFeedEntry = {
      timestamp: event.Timestamp,
      direction: event.Combat.Direction,
      damage: event.Combat.Damage,
      entity: event.Combat.Entity,
      miss: event.Combat.Miss,
    };
    updated.combatFeed = [entry, ...existing.combatFeed.slice(0, MAX_FEED_ENTRIES - 1)];

    if (!event.Combat.Miss && event.Combat.Entity) {
      const entityName = event.Combat.Entity;
      const prev = existing.entityStats[entityName] ?? { name: entityName, kills: 0, dmgOut: 0, dmgIn: 0 };
      const isOut = event.Combat.Direction === 'out';
      const damage = event.Combat.Damage;
      updated.entityStats = {
        ...existing.entityStats,
        [entityName]: {
          name: prev.name, kills: prev.kills,
          dmgOut: isOut ? prev.dmgOut + damage : prev.dmgOut,
          dmgIn: isOut ? prev.dmgIn : prev.dmgIn + damage,
        },
      };
    }

    if (!event.Combat.Miss) {
      const bucketTime = Math.floor(eventMs / DPS_BUCKET_MS) * DPS_BUCKET_MS;
      const isOut = event.Combat.Direction === 'out';
      const damage = event.Combat.Damage;
      const prevBucket = existing.dpsBuckets.find(b => b.time === bucketTime);
      if (prevBucket) {
        // Update existing bucket — array stays sorted, no trim needed
        updated.dpsBuckets = existing.dpsBuckets.map(b =>
          b.time !== bucketTime ? b : isOut ? { ...b, out: b.out + damage } : { ...b, in: b.in + damage }
        );
      } else {
        const lastTime = existing.dpsBuckets.at(-1)?.time ?? bucketTime;
        const cutoff = Math.max(bucketTime, lastTime) - DPS_WINDOW_MS;
        const newBucket: DpsBucket = { time: bucketTime, out: isOut ? damage : 0, in: isOut ? 0 : damage, mining: 0, residue: 0 };
        updated.dpsBuckets = appendBucket(existing.dpsBuckets, newBucket, cutoff);
      }
    }
  }

  if (event.Type === 'kill' && event.Kill) {
    updated.killFeed = [
      { timestamp: event.Timestamp, entity: event.Kill.Entity, bounty: event.Kill.BountyISK },
      ...existing.killFeed.slice(0, MAX_FEED_ENTRIES - 1),
    ];
    updated.totalBountyISK = existing.totalBountyISK + event.Kill.BountyISK;

    if (event.Kill.Entity) {
      const entityName = event.Kill.Entity;
      const prev = updated.entityStats[entityName] ?? { name: entityName, kills: 0, dmgOut: 0, dmgIn: 0 };
      updated.entityStats = {
        ...updated.entityStats,
        [entityName]: { ...prev, kills: prev.kills + 1 },
      };
    }
    updated.bountyHistory = [
      ...existing.bountyHistory,
      { timestamp: event.Timestamp, isk: event.Kill.BountyISK },
    ].slice(-MAX_BOUNTY_HISTORY);
    const killBucket = Math.floor(eventMs / DPS_BUCKET_MS) * DPS_BUCKET_MS;
    if (!existing.killMarkers.includes(killBucket)) {
      updated.killMarkers = [...existing.killMarkers, killBucket].slice(-50);
    }
  }

  // Normal mining yield (includes critical bonus)
  if (event.Type === 'mining' && event.Mining && !event.Mining.Residue) {
    const entry: MiningFeedEntry = {
      timestamp: event.Timestamp,
      oreType: event.Mining.OreType,
      amount: event.Mining.Amount,
      residue: false,
      critical: event.Mining.Critical,
    };
    updated.miningFeed = [entry, ...existing.miningFeed.slice(0, MAX_FEED_ENTRIES - 1)];
    updated.totalMined = existing.totalMined + event.Mining.Amount;

    const oreName = event.Mining.OreType;
    const prevOre = existing.oreStats[oreName] ?? { oreType: oreName, cycles: 0, amount: 0 };
    updated.oreStats = {
      ...existing.oreStats,
      [oreName]: { oreType: oreName, cycles: prevOre.cycles + 1, amount: prevOre.amount + event.Mining.Amount },
    };

    const mBucketTime = Math.floor(eventMs / DPS_BUCKET_MS) * DPS_BUCKET_MS;
    const prevMBucket = existing.dpsBuckets.find(b => b.time === mBucketTime);
    if (prevMBucket) {
      updated.dpsBuckets = existing.dpsBuckets.map(b =>
        b.time !== mBucketTime ? b : { ...b, mining: b.mining + event.Mining!.Amount }
      );
    } else {
      const lastTime = existing.dpsBuckets.at(-1)?.time ?? mBucketTime;
      const cutoff = Math.max(mBucketTime, lastTime) - DPS_WINDOW_MS;
      updated.dpsBuckets = appendBucket(existing.dpsBuckets, { time: mBucketTime, out: 0, in: 0, mining: event.Mining.Amount, residue: 0 }, cutoff);
    }

    if (event.Mining.Critical) {
      updated.totalCriticals = existing.totalCriticals + 1;
      updated.criticalHistory = [
        ...existing.criticalHistory,
        { timestamp: event.Timestamp },
      ].slice(-MAX_CRITICAL_HISTORY);
      const critBucket = Math.floor(eventMs / DPS_BUCKET_MS) * DPS_BUCKET_MS;
      if (!existing.criticalMarkers.includes(critBucket)) {
        updated.criticalMarkers = [...existing.criticalMarkers, critBucket].slice(-50);
      }
    }
  }

  // Residue loss — ore depleted from asteroid as inefficiency
  if (event.Type === 'mining' && event.Mining && event.Mining.Residue) {
    updated.miningFeed = [
      { timestamp: event.Timestamp, oreType: 'Residue', amount: event.Mining.Amount, residue: true, critical: false },
      ...existing.miningFeed.slice(0, MAX_FEED_ENTRIES - 1),
    ];
    updated.totalResidue = existing.totalResidue + event.Mining.Amount;

    const prevResidue = existing.oreStats['Residue'] ?? { oreType: 'Residue', cycles: 0, amount: 0 };
    updated.oreStats = {
      ...existing.oreStats,
      Residue: { oreType: 'Residue', cycles: prevResidue.cycles + 1, amount: prevResidue.amount + event.Mining.Amount },
    };

    const rBucketTime = Math.floor(eventMs / DPS_BUCKET_MS) * DPS_BUCKET_MS;
    const prevRBucket = existing.dpsBuckets.find(b => b.time === rBucketTime);
    if (prevRBucket) {
      updated.dpsBuckets = existing.dpsBuckets.map(b =>
        b.time !== rBucketTime ? b : { ...b, residue: b.residue + event.Mining!.Amount }
      );
    } else {
      const lastTime = existing.dpsBuckets.at(-1)?.time ?? rBucketTime;
      const cutoff = Math.max(rBucketTime, lastTime) - DPS_WINDOW_MS;
      updated.dpsBuckets = appendBucket(existing.dpsBuckets, { time: rBucketTime, out: 0, in: 0, mining: 0, residue: event.Mining.Amount }, cutoff);
    }
  }

  if (event.Type === 'nav' && event.Nav) {
    updated.jumpHistory = [
      ...existing.jumpHistory,
      { timestamp: event.Timestamp },
    ].slice(-MAX_BOUNTY_HISTORY);
    updated.navFeed = [
      { timestamp: event.Timestamp, from: event.Nav.From, to: event.Nav.To },
      ...existing.navFeed.slice(0, MAX_FEED_ENTRIES - 1),
    ];
  }

  if (event.Type === 'cap_starvation' && event.CapStarvation) {
    updated.capAlert = true;
    updated.capAlertModule = event.CapStarvation.Module;
    const capBucket = Math.floor(eventMs / DPS_BUCKET_MS) * DPS_BUCKET_MS;
    if (!existing.capMarkers.includes(capBucket)) {
      updated.capMarkers = [...existing.capMarkers, capBucket].slice(-50);
    }
  }

  return updated;
}
