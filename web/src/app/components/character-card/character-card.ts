import {
  ChangeDetectionStrategy,
  Component,
  computed,
  input,
  signal,
} from '@angular/core';
import { DatePipe, DecimalPipe } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { ChartOptions } from 'chart.js';
import { BaseChartDirective } from 'ng2-charts';
import { Accordion, AccordionContent, AccordionHeader, AccordionPanel } from 'primeng/accordion';
import { Card } from 'primeng/card';
import { Slider } from 'primeng/slider';
import { Tab, TabList, TabPanel, TabPanels, Tabs } from 'primeng/tabs';
import { Tag } from 'primeng/tag';
import {
  buildChartData,
  CharacterLiveState,
  DPS_BUCKET_MS,
  ISK_WINDOW_MS,
  MINING_WINDOW_MS,
} from '../../models/character-live-state';

interface FeedAllEntry {
  timestamp: string;
  type: 'combat' | 'mining' | 'nav';
  direction?: 'in' | 'out';
  damage?: number;
  entity?: string;
  miss?: boolean;
  oreType?: string;
  amount?: number;
  residue?: boolean;
  critical?: boolean;
  from?: string;
  to?: string;
}

@Component({
  standalone: true,
  selector: 'app-character-card',
  templateUrl: './character-card.html',
  styleUrl: './character-card.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [
    Accordion,
    AccordionContent,
    AccordionHeader,
    AccordionPanel,
    BaseChartDirective,
    Card,
    DatePipe,
    DecimalPipe,
    FormsModule,
    RouterLink,
    Slider,
    Tab,
    TabList,
    TabPanel,
    TabPanels,
    Tabs,
    Tag,
  ],
})
export class CharacterCardComponent {
  readonly state = input.required<CharacterLiveState>();

  protected readonly feedTab = signal('all');
  protected readonly accordionValue = signal<string[]>(['events']);
  protected readonly chartWindowMinutes = signal(2);

  // Narrow intermediates — Angular's computed() uses === equality, so these only
  // propagate when the specific array reference changes, not on every event type.
  private readonly _dpsBuckets = computed(() => this.state().dpsBuckets);
  private readonly _killMarkers = computed(() => this.state().killMarkers);
  private readonly _capMarkers = computed(() => this.state().capMarkers);
  private readonly _criticalMarkers = computed(() => this.state().criticalMarkers);
  private readonly _combatFeed = computed(() => this.state().combatFeed);
  private readonly _miningFeed = computed(() => this.state().miningFeed);
  private readonly _navFeed = computed(() => this.state().navFeed);

  protected readonly windowedBuckets = computed(() => {
    const buckets = this._dpsBuckets();
    if (!buckets.length) return buckets;
    const windowMs = this.chartWindowMinutes() * 60_000;
    const latestTime = buckets[buckets.length - 1].time;
    return buckets.filter(b => b.time >= latestTime - windowMs);
  });

  protected readonly displayedChartData = computed(() =>
    buildChartData(this.windowedBuckets(), this._killMarkers(), this._capMarkers(), this._criticalMarkers()),
  );

  protected get chartWindowValue(): number { return this.chartWindowMinutes(); }
  protected set chartWindowValue(v: number) { this.chartWindowMinutes.set(v); }

  protected readonly allFeed = computed(() => {
    const entries: FeedAllEntry[] = [
      ...this._combatFeed().map(e => ({
        timestamp: e.timestamp, type: 'combat' as const,
        direction: e.direction, damage: e.damage, entity: e.entity, miss: e.miss,
      })),
      ...this._miningFeed().map(e => ({
        timestamp: e.timestamp, type: 'mining' as const,
        oreType: e.oreType, amount: e.amount,
        residue: e.residue, critical: e.critical,
      })),
      ...this._navFeed().map(e => ({
        timestamp: e.timestamp, type: 'nav' as const,
        from: e.from, to: e.to,
      })),
    ];
    return entries.sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime());
  });

  private readonly _entityStats = computed(() => this.state().entityStats);

  protected readonly entityRows = computed(() =>
    Object.values(this._entityStats()).sort(
      (a, b) => (b.kills * 100_000 + b.dmgOut + b.dmgIn) - (a.kills * 100_000 + a.dmgOut + a.dmgIn),
    ),
  );

  private static readonly MARKER_LABELS = new Set(['Kill', 'Cap Starved', 'Hold Full', 'Critical']);

  protected readonly lineChartOptions: ChartOptions<'line'> = {
    responsive: true,
    maintainAspectRatio: false,
    animation: false,
    plugins: {
      legend: { display: true, position: 'top', labels: { boxWidth: 12, padding: 8 } },
      tooltip: {
        mode: 'index',
        intersect: false,
        filter: item => item.parsed.y !== null && !isNaN(item.parsed.y),
        callbacks: {
          label: ctx => {
            if (CharacterCardComponent.MARKER_LABELS.has(ctx.dataset.label ?? '')) {
              return `  ● ${ctx.dataset.label}`;
            }
            return `${ctx.dataset.label}: ${ctx.parsed.y}`;
          },
        },
      },
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

  protected miningRateFor(): number {
    const s = this.state();
    const nonResidue = s.miningFeed.filter(e => !e.residue);
    if (!nonResidue.length) return 0;
    const latestMs = new Date(nonResidue[0].timestamp).getTime();
    const cutoff = latestMs - MINING_WINDOW_MS;
    return nonResidue
      .filter(e => new Date(e.timestamp).getTime() >= cutoff)
      .reduce((sum, e) => sum + e.amount, 0);
  }

  protected avgDpsFor(): number {
    const buckets = this.windowedBuckets();
    if (!buckets.length) return 0;
    const totalOut = buckets.reduce((sum, b) => sum + b.out, 0);
    return Math.round(totalOut / (buckets.length * (DPS_BUCKET_MS / 1000)));
  }

  protected avgInDpsFor(): number {
    const buckets = this.windowedBuckets();
    if (!buckets.length) return 0;
    const totalIn = buckets.reduce((sum, b) => sum + b.in, 0);
    return Math.round(totalIn / (buckets.length * (DPS_BUCKET_MS / 1000)));
  }

  protected residueLostFor(): number {
    return this.state().totalResidue;
  }

  protected criticalCountFor(): number {
    return this.state().totalCriticals;
  }

  protected critsPerHourFor(): number {
    const { criticalHistory } = this.state();
    if (!criticalHistory.length) return 0;
    const latestMs = new Date(criticalHistory[criticalHistory.length - 1].timestamp).getTime();
    const cutoff = latestMs - ISK_WINDOW_MS;
    return criticalHistory.filter(e => new Date(e.timestamp).getTime() >= cutoff).length;
  }

  protected iskPerHourFor(): number {
    const s = this.state();
    if (!s.bountyHistory.length) return 0;
    const latestMs = new Date(s.bountyHistory[s.bountyHistory.length - 1].timestamp).getTime();
    const cutoff = latestMs - ISK_WINDOW_MS;
    return s.bountyHistory
      .filter(e => new Date(e.timestamp).getTime() >= cutoff)
      .reduce((sum, e) => sum + e.isk, 0);
  }
}
