import { ChartData } from 'chart.js';
import { LiveEvent } from './live-event.model';

export const DPS_BUCKET_MS = 5_000;
export const DPS_WINDOW_MS = 1_800_000; // 30 min max storage; display window is user-controlled
export const MINING_WINDOW_MS = 60_000;
export const ISK_WINDOW_MS = 3_600_000;
export const FEED_WINDOW_MS = 10 * 60 * 1_000; // 10-min rolling feed history
export const MAX_BOUNTY_HISTORY = 500;
export const MAX_CRITICAL_HISTORY = 720; // 1 hr at 5-sec buckets
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
  chartData: ChartData<'line'>;
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
  chartData: { labels: [], datasets: [] },
  capAlert: false,
  capAlertModule: null,
};

export const EMPTY_CHART: ChartData<'line'> = { labels: [], datasets: [] };

function markerDataset(
  label: string,
  buckets: DpsBucket[],
  markerSet: Set<number>,
  color: string,
  pointStyle: string,
  yAxisID: string,
  yValues?: number[],
): ChartData<'line'>['datasets'][number] {
  return {
    label,
    data: buckets.map((b, i) => (markerSet.has(b.time) ? (yValues ? yValues[i] : 0) : NaN)),
    borderColor: color,
    backgroundColor: color,
    showLine: false,
    pointStyle: pointStyle as 'circle',
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
    datasets.push(markerDataset('Kill', buckets, killSet, '#f59e0b', 'star', 'y'));
  }
  if (capSet.size) {
    datasets.push(markerDataset('Cap Starved', buckets, capSet, '#ef4444', 'crossRot', 'y'));
  }
  if (critSet.size) {
    const miningValues = buckets.map(b => Math.round(b.mining / miningScale));
    datasets.push(markerDataset('Critical', buckets, critSet, '#22c55e', 'triangle', hasY2 ? 'y2' : 'y', miningValues));
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

export function applyEvent(
  existing: CharacterLiveState,
  event: LiveEvent,
): CharacterLiveState {
  const updated: CharacterLiveState = { ...existing };

  if (event.Type === 'combat' && event.Combat) {
    const entry: CombatFeedEntry = {
      timestamp: event.Timestamp,
      direction: event.Combat.Direction,
      damage: event.Combat.Damage,
      entity: event.Combat.Entity,
      miss: event.Combat.Miss,
    };
    const combatCutoff = new Date(event.Timestamp).getTime() - FEED_WINDOW_MS;
    updated.combatFeed = [entry, ...existing.combatFeed]
      .filter(e => new Date(e.timestamp).getTime() >= combatCutoff);

    if (!event.Combat.Miss) {
      const bucketTime =
        Math.floor(new Date(event.Timestamp).getTime() / DPS_BUCKET_MS) * DPS_BUCKET_MS;
      const latestTime = Math.max(bucketTime, ...existing.dpsBuckets.map(b => b.time));
      const cutoff = latestTime - DPS_WINDOW_MS;
      const isOut = event.Combat.Direction === 'out';
      const prevBucket = existing.dpsBuckets.find(b => b.time === bucketTime);
      let buckets: DpsBucket[] = prevBucket
        ? existing.dpsBuckets.map(b => {
            if (b.time !== bucketTime) return b;
            return isOut
              ? { ...b, out: b.out + event.Combat!.Damage }
              : { ...b, in: b.in + event.Combat!.Damage };
          })
        : [
            ...existing.dpsBuckets,
            {
              time: bucketTime,
              out: isOut ? event.Combat.Damage : 0,
              in: isOut ? 0 : event.Combat.Damage,
              mining: 0,
              residue: 0,
            },
          ];
      buckets = buckets.filter(b => b.time >= cutoff).sort((a, b) => a.time - b.time);
      updated.dpsBuckets = buckets;
    }
  }

  if (event.Type === 'kill' && event.Kill) {
    const killCutoff = new Date(event.Timestamp).getTime() - FEED_WINDOW_MS;
    updated.killFeed = [
      { timestamp: event.Timestamp, entity: event.Kill.Entity, bounty: event.Kill.BountyISK },
      ...existing.killFeed,
    ].filter(e => new Date(e.timestamp).getTime() >= killCutoff);
    updated.totalBountyISK = existing.totalBountyISK + event.Kill.BountyISK;
    updated.bountyHistory = [
      ...existing.bountyHistory,
      { timestamp: event.Timestamp, isk: event.Kill.BountyISK },
    ].slice(-MAX_BOUNTY_HISTORY);
    const killBucket = Math.floor(new Date(event.Timestamp).getTime() / DPS_BUCKET_MS) * DPS_BUCKET_MS;
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
    const miningCutoff = new Date(event.Timestamp).getTime() - FEED_WINDOW_MS;
    updated.miningFeed = [entry, ...existing.miningFeed]
      .filter(e => new Date(e.timestamp).getTime() >= miningCutoff);
    updated.totalMined = existing.totalMined + event.Mining.Amount;

    const mBucketTime =
      Math.floor(new Date(event.Timestamp).getTime() / DPS_BUCKET_MS) * DPS_BUCKET_MS;
    const latestTime = Math.max(mBucketTime, ...existing.dpsBuckets.map(b => b.time));
    const cutoff = latestTime - DPS_WINDOW_MS;
    const prevMBucket = existing.dpsBuckets.find(b => b.time === mBucketTime);
    const baseBuckets = updated.dpsBuckets.length ? updated.dpsBuckets : existing.dpsBuckets;
    let mBuckets: DpsBucket[] = prevMBucket
      ? baseBuckets.map(b => {
          if (b.time !== mBucketTime) return b;
          return { ...b, mining: b.mining + event.Mining!.Amount };
        })
      : [
          ...baseBuckets,
          { time: mBucketTime, out: 0, in: 0, mining: event.Mining.Amount, residue: 0 },
        ];
    mBuckets = mBuckets.filter(b => b.time >= cutoff).sort((a, b) => a.time - b.time);
    updated.dpsBuckets = mBuckets;

    if (event.Mining.Critical) {
      updated.totalCriticals = existing.totalCriticals + 1;
      updated.criticalHistory = [
        ...existing.criticalHistory,
        { timestamp: event.Timestamp },
      ].slice(-MAX_CRITICAL_HISTORY);
      const critBucket = Math.floor(new Date(event.Timestamp).getTime() / DPS_BUCKET_MS) * DPS_BUCKET_MS;
      if (!existing.criticalMarkers.includes(critBucket)) {
        updated.criticalMarkers = [...existing.criticalMarkers, critBucket].slice(-50);
      }
    }
  }

  // Residue loss — ore depleted from asteroid as inefficiency
  if (event.Type === 'mining' && event.Mining && event.Mining.Residue) {
    const entry: MiningFeedEntry = {
      timestamp: event.Timestamp,
      oreType: 'Residue',
      amount: event.Mining.Amount,
      residue: true,
      critical: false,
    };
    const residueCutoff = new Date(event.Timestamp).getTime() - FEED_WINDOW_MS;
    updated.miningFeed = [entry, ...existing.miningFeed]
      .filter(e => new Date(e.timestamp).getTime() >= residueCutoff);

    updated.totalResidue = existing.totalResidue + event.Mining.Amount;

    const rBucketTime =
      Math.floor(new Date(event.Timestamp).getTime() / DPS_BUCKET_MS) * DPS_BUCKET_MS;
    const baseBuckets = updated.dpsBuckets.length ? updated.dpsBuckets : existing.dpsBuckets;
    const latestTime = Math.max(rBucketTime, ...baseBuckets.map(b => b.time));
    const cutoff = latestTime - DPS_WINDOW_MS;
    const prevRBucket = baseBuckets.find(b => b.time === rBucketTime);
    let rBuckets: DpsBucket[] = prevRBucket
      ? baseBuckets.map(b => {
          if (b.time !== rBucketTime) return b;
          return { ...b, residue: b.residue + event.Mining!.Amount };
        })
      : [
          ...baseBuckets,
          { time: rBucketTime, out: 0, in: 0, mining: 0, residue: event.Mining.Amount },
        ];
    rBuckets = rBuckets.filter(b => b.time >= cutoff).sort((a, b) => a.time - b.time);
    updated.dpsBuckets = rBuckets;
  }

  if (event.Type === 'nav' && event.Nav) {
    const navCutoff = new Date(event.Timestamp).getTime() - FEED_WINDOW_MS;
    updated.jumpHistory = [
      ...existing.jumpHistory,
      { timestamp: event.Timestamp },
    ].slice(-MAX_BOUNTY_HISTORY);
    updated.navFeed = [
      { timestamp: event.Timestamp, from: event.Nav.From, to: event.Nav.To },
      ...existing.navFeed,
    ].filter(e => new Date(e.timestamp).getTime() >= navCutoff);
  }

  if (event.Type === 'cap_starvation' && event.CapStarvation) {
    updated.capAlert = true;
    updated.capAlertModule = event.CapStarvation.Module;
    const capBucket = Math.floor(new Date(event.Timestamp).getTime() / DPS_BUCKET_MS) * DPS_BUCKET_MS;
    if (!existing.capMarkers.includes(capBucket)) {
      updated.capMarkers = [...existing.capMarkers, capBucket].slice(-50);
    }
  }

  return updated;
}
