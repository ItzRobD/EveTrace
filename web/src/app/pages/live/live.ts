import {
  ChangeDetectionStrategy,
  Component,
  computed,
  inject,
  OnInit,
  OnDestroy,
  signal,
} from '@angular/core';
import { DatePipe, DecimalPipe } from '@angular/common';
import { RouterLink } from '@angular/router';
import { bufferTime, filter, Subject, takeUntil } from 'rxjs';
import { ChartData, ChartOptions } from 'chart.js';
import { BaseChartDirective } from 'ng2-charts';
import { Card } from 'primeng/card';
import { Select } from 'primeng/select';
import { Skeleton } from 'primeng/skeleton';
import { Tag } from 'primeng/tag';
import { Tab, TabList, TabPanel, TabPanels, Tabs } from 'primeng/tabs';
import { FormsModule } from '@angular/forms';
import { Button } from 'primeng/button';
import { Session } from '../../models/session.model';
import { ApiService } from '../../services/api.service';
import { EventStreamService } from '../../services/event-stream.service';
import { LiveEvent } from '../../models/live-event.model';

const DPS_BUCKET_MS = 5_000;
const DPS_WINDOW_MS = 120_000;
const MINING_WINDOW_MS = 60_000;
const ISK_WINDOW_MS = 3_600_000;
const MAX_COMBAT_FEED = 12;
const MAX_KILL_FEED = 8;
const MAX_BOUNTY_HISTORY = 500;
const CAP_ALERT_MS = 30_000;

interface CombatFeedEntry {
  timestamp: string;
  direction: 'in' | 'out';
  damage: number;
  entity: string;
  miss: boolean;
}

interface KillFeedEntry {
  timestamp: string;
  entity: string;
  bounty: number;
}

interface MiningFeedEntry {
  timestamp: string;
  oreType: string;
  amount: number;
}

interface BountyEntry {
  timestamp: string;
  isk: number;
}

interface DpsBucket {
  time: number;
  out: number;
  in: number;
  mining: number;
}

interface JumpEntry {
  timestamp: string;
}

interface NavFeedEntry {
  timestamp: string;
  from: string;
  to: string;
}

interface FeedAllEntry {
  timestamp: string;
  type: 'combat' | 'mining' | 'nav';
  // combat fields
  direction?: 'in' | 'out';
  damage?: number;
  entity?: string;
  miss?: boolean;
  // mining fields
  oreType?: string;
  amount?: number;
  // nav fields
  from?: string;
  to?: string;
}

interface CharacterLiveState {
  characterName: string;
  sessionKey: string;
  sessionDbId: number | null;
  combatFeed: CombatFeedEntry[];
  killFeed: KillFeedEntry[];
  miningFeed: MiningFeedEntry[];
  bountyHistory: BountyEntry[];
  totalBountyISK: number;
  totalMined: number;
  jumpHistory: JumpEntry[];
  navFeed: NavFeedEntry[];
  dpsBuckets: DpsBucket[];
  // Bucket-aligned timestamps for chart event markers.
  killMarkers: number[];
  capMarkers: number[];
  miningFullMarkers: number[];
  // Cached chart data — only rebuilt when buckets or markers change.
  chartData: ChartData<'line'>;
  capAlert: boolean;
  capAlertModule: string | null;
}

const EMPTY_CHART: ChartData<'line'> = { labels: [], datasets: [] };

function markerDataset(
  label: string,
  buckets: DpsBucket[],
  markerSet: Set<number>,
  color: string,
  pointStyle: string,
  yAxisID: string,
): ChartData<'line'>['datasets'][number] {
  return {
    label,
    data: buckets.map(b => (markerSet.has(b.time) ? 0 : NaN)),
    borderColor: color,
    backgroundColor: color,
    showLine: false,
    pointStyle: pointStyle as 'circle',
    pointRadius: buckets.map(b => (markerSet.has(b.time) ? 9 : 0)),
    pointHoverRadius: 11,
    yAxisID,
  };
}

function buildChartData(
  buckets: DpsBucket[],
  killMarkers: number[],
  capMarkers: number[],
  miningFullMarkers: number[],
): ChartData<'line'> {
  if (!buckets.length) return EMPTY_CHART;
  const dpsScale = DPS_BUCKET_MS / 1000;
  const miningScale = DPS_BUCKET_MS / 60_000; // convert bucket total → units/min
  const hasMining = buckets.some(b => b.mining > 0);
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

  const killSet = new Set(killMarkers);
  const capSet = new Set(capMarkers);
  const fullSet = new Set(miningFullMarkers);

  if (killSet.size) {
    datasets.push(markerDataset('Kill', buckets, killSet, '#f59e0b', 'star', 'y'));
  }
  if (capSet.size) {
    datasets.push(markerDataset('Cap Starved', buckets, capSet, '#ef4444', 'crossRot', 'y'));
  }
  if (fullSet.size) {
    datasets.push(markerDataset('Hold Full', buckets, fullSet, '#06b6d4', 'rectRot', hasMining ? 'y2' : 'y'));
  }

  return { labels, datasets };
}

function eventSignature(event: LiveEvent): string {
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

@Component({
  standalone: true,
  selector: 'app-live',
  templateUrl: './live.html',
  styleUrl: './live.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [
    BaseChartDirective,
    Button,
    Card,
    DatePipe,
    DecimalPipe,
    FormsModule,
    RouterLink,
    Select,
    Skeleton,
    Tab,
    TabList,
    TabPanel,
    TabPanels,
    Tabs,
    Tag,
  ],
})
export class LiveComponent implements OnInit, OnDestroy {
  private readonly api = inject(ApiService);
  private readonly eventStream = inject(EventStreamService);

  protected readonly loading = signal(true);
  protected readonly liveState = signal<Map<string, CharacterLiveState>>(new Map());
  protected readonly activeChars = computed(() => [...this.liveState().values()]);

  protected readonly allSessions = signal<Session[]>([]);
  protected readonly replaySessionId = signal<number | null>(null);
  protected readonly replaySpeed = signal(20);
  protected readonly replayActive = signal(false);
  protected readonly streamActive = signal(false);
  protected readonly replayInfo = signal('');
  protected readonly feedTabsMap = signal<Map<string, string>>(new Map());

  private readonly stopStream$ = new Subject<void>();

  protected readonly speedOptions = [
    { label: '5×', value: 5 },
    { label: '10×', value: 10 },
    { label: '20×', value: 20 },
    { label: '50×', value: 50 },
    { label: '100×', value: 100 },
  ];

  protected readonly maxGapOptions = [
    { label: 'No gaps', value: 0 },
    { label: '200ms', value: 200 },
    { label: '500ms', value: 500 },
    { label: '1s', value: 1000 },
    { label: '2s', value: 2000 },
  ];

  protected readonly replayMaxGap = signal(500);

  private sessionsByKey = new Map<string, Session>();
  private fetchedKeys = new Set<string>();

  protected readonly lineChartOptions: ChartOptions<'line'> = {
    responsive: true,
    maintainAspectRatio: false,
    animation: false,
    plugins: {
      legend: { display: true, position: 'top', labels: { boxWidth: 12, padding: 8 } },
      tooltip: { mode: 'index', intersect: false },
    },
    scales: {
      x: { ticks: { maxRotation: 0, autoSkip: true, maxTicksLimit: 8 } },
      y: { beginAtZero: true, position: 'left', title: { display: true, text: 'DPS' } },
      y2: {
        beginAtZero: true,
        position: 'right',
        title: { display: true, text: 'Mining / min' },
        grid: { drawOnChartArea: false },
      },
    },
  };

  ngOnInit(): void {
    this.api.getSessions().subscribe({
      next: sessions => {
        const list = sessions ?? [];
        for (const s of list) {
          this.sessionsByKey.set(s.SessionKey, s);
        }
        this.allSessions.set(list);
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }

  ngOnDestroy(): void {
    this.stopStream$.next();
    this.stopStream$.complete();
  }

  private startStream(): void {
    // Reset the stop subject so a fresh takeUntil gate is in place.
    this.stopStream$.next();

    this.eventStream.events$
      .pipe(
        bufferTime(100),
        filter(batch => batch.length > 0),
        takeUntil(this.stopStream$),
      )
      .subscribe(batch => this.processBatch(batch));

    this.streamActive.set(true);
  }

  private stopStream(): void {
    this.stopStream$.next();
    this.streamActive.set(false);
    this.liveState.set(new Map());
  }

  protected goLive(): void {
    this.liveState.set(new Map());
    this.replayActive.set(false);
    this.replayInfo.set('');
    this.startStream();
  }

  private processBatch(events: LiveEvent[]): void {
    const now = Date.now(); // used only for cap alert timeouts
    const updatedMap = new Map(this.liveState());
    const capTimeoutChars: string[] = [];

    // Deduplicate within the batch: identical events arriving in the same 100ms
    // window are artifacts of duplicate DB rows (e.g. from --from-start restarts).
    const seen = new Set<string>();

    for (const event of events) {
      const charName = event.SessionID.split('/')[0];
      if (!charName) continue;

      const sig = eventSignature(event);
      if (seen.has(sig)) continue;
      seen.add(sig);

      const session = this.sessionsByKey.get(event.SessionID);
      if (!session && !this.fetchedKeys.has(event.SessionID)) {
        this.fetchedKeys.add(event.SessionID);
        this.fetchSessionForKey(event.SessionID, charName);
      }

      const existing: CharacterLiveState = updatedMap.get(charName) ?? {
        characterName: charName,
        sessionKey: event.SessionID,
        sessionDbId: session?.ID ?? null,
        combatFeed: [],
        killFeed: [],
        miningFeed: [],
        bountyHistory: [],
        totalBountyISK: 0,
        totalMined: 0,
        jumpHistory: [],
        navFeed: [],
        dpsBuckets: [],
        killMarkers: [],
        capMarkers: [],
        miningFullMarkers: [],
        chartData: EMPTY_CHART,
        capAlert: false,
        capAlertModule: null,
      };

      const updated = this.applyEvent(existing, event);

      // Rebuild chart data when buckets or any marker array changes.
      const chartData =
        updated.dpsBuckets !== existing.dpsBuckets ||
        updated.killMarkers !== existing.killMarkers ||
        updated.capMarkers !== existing.capMarkers ||
        updated.miningFullMarkers !== existing.miningFullMarkers
          ? buildChartData(updated.dpsBuckets, updated.killMarkers, updated.capMarkers, updated.miningFullMarkers)
          : existing.chartData;

      updatedMap.set(charName, { ...updated, chartData });

      if (event.Type === 'cap_starvation') {
        capTimeoutChars.push(charName);
      }
    }

    this.liveState.set(updatedMap);

    for (const charName of capTimeoutChars) {
      setTimeout(() => {
        this.liveState.update(m => {
          const m2 = new Map(m);
          const s = m2.get(charName);
          if (s?.capAlert) m2.set(charName, { ...s, capAlert: false, capAlertModule: null });
          return m2;
        });
      }, CAP_ALERT_MS);
    }
  }

  private applyEvent(
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
      updated.combatFeed = [entry, ...existing.combatFeed].slice(0, MAX_COMBAT_FEED);

      if (!event.Combat.Miss) {
        const bucketTime =
          Math.floor(new Date(event.Timestamp).getTime() / DPS_BUCKET_MS) * DPS_BUCKET_MS;
        // Anchor the rolling window to the latest event time seen, not wall
        // clock. This keeps DPS accurate during replay (original timestamps)
        // and behaves identically to Date.now() during live play.
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
              },
            ];
        buckets = buckets.filter(b => b.time >= cutoff).sort((a, b) => a.time - b.time);
        updated.dpsBuckets = buckets;
      }
    }

    if (event.Type === 'kill' && event.Kill) {
      updated.killFeed = [
        { timestamp: event.Timestamp, entity: event.Kill.Entity, bounty: event.Kill.BountyISK },
        ...existing.killFeed,
      ].slice(0, MAX_KILL_FEED);
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

    if (event.Type === 'mining' && event.Mining && event.Mining.Residue) {
      const fullBucket = Math.floor(new Date(event.Timestamp).getTime() / DPS_BUCKET_MS) * DPS_BUCKET_MS;
      if (!existing.miningFullMarkers.includes(fullBucket)) {
        updated.miningFullMarkers = [...existing.miningFullMarkers, fullBucket].slice(-50);
      }
    }

    if (event.Type === 'mining' && event.Mining && !event.Mining.Residue) {
      updated.miningFeed = [
        { timestamp: event.Timestamp, oreType: event.Mining.OreType, amount: event.Mining.Amount },
        ...existing.miningFeed,
      ].slice(0, MAX_KILL_FEED);
      updated.totalMined = existing.totalMined + event.Mining.Amount;

      // Add mining yield to the shared DPS/mining bucket for the chart.
      const mBucketTime =
        Math.floor(new Date(event.Timestamp).getTime() / DPS_BUCKET_MS) * DPS_BUCKET_MS;
      const latestTime = Math.max(mBucketTime, ...existing.dpsBuckets.map(b => b.time));
      const cutoff = latestTime - DPS_WINDOW_MS;
      const prevMBucket = existing.dpsBuckets.find(b => b.time === mBucketTime);
      let mBuckets: DpsBucket[] = prevMBucket
        ? (updated.dpsBuckets.length ? updated.dpsBuckets : existing.dpsBuckets).map(b => {
            if (b.time !== mBucketTime) return b;
            return { ...b, mining: b.mining + event.Mining!.Amount };
          })
        : [
            ...(updated.dpsBuckets.length ? updated.dpsBuckets : existing.dpsBuckets),
            { time: mBucketTime, out: 0, in: 0, mining: event.Mining.Amount },
          ];
      mBuckets = mBuckets.filter(b => b.time >= cutoff).sort((a, b) => a.time - b.time);
      updated.dpsBuckets = mBuckets;
    }

    if (event.Type === 'nav' && event.Nav) {
      updated.jumpHistory = [
        ...existing.jumpHistory,
        { timestamp: event.Timestamp },
      ].slice(-MAX_BOUNTY_HISTORY);
      updated.navFeed = [
        { timestamp: event.Timestamp, from: event.Nav.From, to: event.Nav.To },
        ...existing.navFeed,
      ].slice(0, MAX_KILL_FEED);
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

  private fetchSessionForKey(sessionKey: string, charName: string): void {
    this.api.getSessions().subscribe({
      next: sessions => {
        for (const s of sessions ?? []) {
          this.sessionsByKey.set(s.SessionKey, s);
        }
        const session = this.sessionsByKey.get(sessionKey);
        if (session) {
          this.liveState.update(map => {
            const newMap = new Map(map);
            const existing = newMap.get(charName);
            if (existing && existing.sessionDbId === null) {
              newMap.set(charName, { ...existing, sessionDbId: session.ID! });
            }
            return newMap;
          });
        }
      },
    });
  }

  private replayTimer: ReturnType<typeof setTimeout> | null = null;

  protected startReplay(): void {
    const id = this.replaySessionId();
    if (!id) return;

    this.replayActive.set(true);
    this.replayInfo.set('');

    if (this.replayTimer !== null) {
      clearTimeout(this.replayTimer);
      this.replayTimer = null;
    }

    this.startStream();

    this.api.replaySession(id, this.replaySpeed(), this.replayMaxGap()).subscribe({
      next: res => {
        this.replayInfo.set(`Replaying ${res.events} events at ${res.speed}×`);
        const ms = (res.duration_secs + 2) * 1000;
        this.replayTimer = setTimeout(() => {
          this.replayActive.set(false);
          this.replayInfo.set('Replay complete');
          this.replayTimer = null;
          this.stopStream();
        }, ms);
      },
      error: () => {
        this.replayActive.set(false);
        this.replayInfo.set('Failed to start replay');
        this.stopStream();
      },
    });
  }

  protected stopReplay(): void {
    const id = this.replaySessionId();

    if (this.replayTimer !== null) {
      clearTimeout(this.replayTimer);
      this.replayTimer = null;
    }

    this.stopStream();
    this.replayActive.set(false);

    if (!id) return;
    this.api.cancelReplay(id).subscribe({
      next: () => this.replayInfo.set('Replay cancelled'),
      error: () => this.replayInfo.set('Replay already finished'),
    });
  }

  protected getFeedTab(charName: string): string {
    return this.feedTabsMap().get(charName) ?? 'all';
  }

  protected setFeedTab(charName: string, tab: string | number | undefined): void {
    if (tab == null) return;
    const m = new Map(this.feedTabsMap());
    m.set(charName, String(tab));
    this.feedTabsMap.set(m);
  }

  protected allFeedFor(state: CharacterLiveState): FeedAllEntry[] {
    const entries: FeedAllEntry[] = [
      ...state.combatFeed.map(e => ({
        timestamp: e.timestamp, type: 'combat' as const,
        direction: e.direction, damage: e.damage, entity: e.entity, miss: e.miss,
      })),
      ...state.miningFeed.map(e => ({
        timestamp: e.timestamp, type: 'mining' as const,
        oreType: e.oreType, amount: e.amount,
      })),
      ...state.navFeed.map(e => ({
        timestamp: e.timestamp, type: 'nav' as const,
        from: e.from, to: e.to,
      })),
    ];
    return entries
      .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
      .slice(0, 20);
  }

  protected miningRateFor(state: CharacterLiveState): number {
    if (!state.miningFeed.length) return 0;
    // miningFeed is newest-first; anchor on the latest event to work during replay.
    const latestMs = new Date(state.miningFeed[0].timestamp).getTime();
    const cutoff = latestMs - MINING_WINDOW_MS;
    return state.miningFeed
      .filter(e => new Date(e.timestamp).getTime() >= cutoff)
      .reduce((s, e) => s + e.amount, 0);
  }

  protected avgDpsFor(state: CharacterLiveState): number {
    if (!state.dpsBuckets.length) return 0;
    const totalOut = state.dpsBuckets.reduce((sum, b) => sum + b.out, 0);
    const windowSecs = state.dpsBuckets.length * (DPS_BUCKET_MS / 1000);
    return Math.round(totalOut / windowSecs);
  }

  protected jumpsPerHourFor(state: CharacterLiveState): number {
    if (!state.jumpHistory.length) return 0;
    const latestMs = new Date(state.jumpHistory[state.jumpHistory.length - 1].timestamp).getTime();
    const cutoff = latestMs - ISK_WINDOW_MS;
    const count = state.jumpHistory.filter(e => new Date(e.timestamp).getTime() >= cutoff).length;
    return Math.round(count);
  }

  protected iskPerHourFor(state: CharacterLiveState): number {
    if (!state.bountyHistory.length) return 0;
    const latestMs = new Date(state.bountyHistory[state.bountyHistory.length - 1].timestamp).getTime();
    const cutoff = latestMs - ISK_WINDOW_MS;
    return state.bountyHistory
      .filter(e => new Date(e.timestamp).getTime() >= cutoff)
      .reduce((s, e) => s + e.isk, 0);
  }

  protected avgInDpsFor(state: CharacterLiveState): number {
    if (!state.dpsBuckets.length) return 0;
    const totalIn = state.dpsBuckets.reduce((sum, b) => sum + b.in, 0);
    const windowSecs = state.dpsBuckets.length * (DPS_BUCKET_MS / 1000);
    return Math.round(totalIn / windowSecs);
  }
}
