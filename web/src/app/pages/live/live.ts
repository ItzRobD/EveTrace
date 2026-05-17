import {
  ChangeDetectionStrategy,
  Component,
  computed,
  DestroyRef,
  inject,
  OnInit,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { DatePipe, DecimalPipe } from '@angular/common';
import { RouterLink } from '@angular/router';
import { bufferTime, filter } from 'rxjs';
import { ChartData, ChartOptions } from 'chart.js';
import { BaseChartDirective } from 'ng2-charts';
import { Card } from 'primeng/card';
import { Select } from 'primeng/select';
import { Skeleton } from 'primeng/skeleton';
import { Tag } from 'primeng/tag';
import { FormsModule } from '@angular/forms';
import { Button } from 'primeng/button';
import { Session } from '../../models/session.model';
import { ApiService } from '../../services/api.service';
import { EventStreamService } from '../../services/event-stream.service';
import { LiveEvent } from '../../models/live-event.model';

const DPS_BUCKET_MS = 5_000;
const DPS_WINDOW_MS = 120_000;
const MINING_WINDOW_MS = 60_000;
const MAX_COMBAT_FEED = 12;
const MAX_KILL_FEED = 8;
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

interface DpsBucket {
  time: number;
  out: number;
  in: number;
}

interface CharacterLiveState {
  characterName: string;
  sessionKey: string;
  sessionDbId: number | null;
  combatFeed: CombatFeedEntry[];
  killFeed: KillFeedEntry[];
  miningFeed: MiningFeedEntry[];
  dpsBuckets: DpsBucket[];
  // Cached chart data — only rebuilt when dpsBuckets reference changes.
  chartData: ChartData<'line'>;
  capAlert: boolean;
  capAlertModule: string | null;
}

const EMPTY_CHART: ChartData<'line'> = { labels: [], datasets: [] };

function buildChartData(buckets: DpsBucket[]): ChartData<'line'> {
  if (!buckets.length) return EMPTY_CHART;
  const scale = DPS_BUCKET_MS / 1000;
  return {
    labels: buckets.map(b =>
      new Date(b.time).toLocaleTimeString([], {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      }),
    ),
    datasets: [
      {
        label: 'Outgoing',
        data: buckets.map(b => Math.round(b.out / scale)),
        borderColor: '#3b82f6',
        backgroundColor: 'rgba(59,130,246,0.1)',
        fill: true,
        tension: 0.3,
        pointRadius: 2,
      },
      {
        label: 'Incoming',
        data: buckets.map(b => Math.round(b.in / scale)),
        borderColor: '#ef4444',
        backgroundColor: 'rgba(239,68,68,0.1)',
        fill: true,
        tension: 0.3,
        pointRadius: 2,
      },
    ],
  };
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
    Tag,
  ],
})
export class LiveComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly eventStream = inject(EventStreamService);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly loading = signal(true);
  protected readonly liveState = signal<Map<string, CharacterLiveState>>(new Map());
  protected readonly activeChars = computed(() => [...this.liveState().values()]);

  protected readonly allSessions = signal<Session[]>([]);
  protected readonly replaySessionId = signal<number | null>(null);
  protected readonly replaySpeed = signal(20);
  protected readonly replayActive = signal(false);
  protected readonly replayInfo = signal('');

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
      y: { beginAtZero: true, title: { display: true, text: 'DPS' } },
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

    this.eventStream.events$
      .pipe(
        bufferTime(100),
        filter(batch => batch.length > 0),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe(batch => this.processBatch(batch));
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
        dpsBuckets: [],
        chartData: EMPTY_CHART,
        capAlert: false,
        capAlertModule: null,
      };

      const updated = this.applyEvent(existing, event);

      // Rebuild chart data only when the buckets array reference changed
      // (avoids unnecessary Chart.js repaints for non-combat events).
      const chartData =
        updated.dpsBuckets !== existing.dpsBuckets
          ? buildChartData(updated.dpsBuckets)
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
    }

    if (event.Type === 'mining' && event.Mining && !event.Mining.Residue) {
      updated.miningFeed = [
        { timestamp: event.Timestamp, oreType: event.Mining.OreType, amount: event.Mining.Amount },
        ...existing.miningFeed,
      ].slice(0, MAX_KILL_FEED);
    }

    if (event.Type === 'cap_starvation' && event.CapStarvation) {
      updated.capAlert = true;
      updated.capAlertModule = event.CapStarvation.Module;
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

    this.liveState.set(new Map());
    this.replayActive.set(true);
    this.replayInfo.set('');

    if (this.replayTimer !== null) {
      clearTimeout(this.replayTimer);
      this.replayTimer = null;
    }

    this.api.replaySession(id, this.replaySpeed(), this.replayMaxGap()).subscribe({
      next: res => {
        this.replayInfo.set(`Replaying ${res.events} events at ${res.speed}×`);
        const ms = (res.duration_secs + 2) * 1000;
        this.replayTimer = setTimeout(() => {
          this.replayActive.set(false);
          this.replayInfo.set('Replay complete');
          this.replayTimer = null;
        }, ms);
      },
      error: () => {
        this.replayActive.set(false);
        this.replayInfo.set('Failed to start replay');
      },
    });
  }

  protected stopReplay(): void {
    const id = this.replaySessionId();
    if (!id) return;

    if (this.replayTimer !== null) {
      clearTimeout(this.replayTimer);
      this.replayTimer = null;
    }

    this.api.cancelReplay(id).subscribe({
      next: () => {
        this.replayActive.set(false);
        this.replayInfo.set('Replay cancelled');
      },
      error: () => {
        this.replayActive.set(false);
        this.replayInfo.set('Replay already finished');
      },
    });
  }

  protected miningRateFor(state: CharacterLiveState): number {
    const cutoff = Date.now() - MINING_WINDOW_MS;
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

  protected avgInDpsFor(state: CharacterLiveState): number {
    if (!state.dpsBuckets.length) return 0;
    const totalIn = state.dpsBuckets.reduce((sum, b) => sum + b.in, 0);
    const windowSecs = state.dpsBuckets.length * (DPS_BUCKET_MS / 1000);
    return Math.round(totalIn / windowSecs);
  }
}
