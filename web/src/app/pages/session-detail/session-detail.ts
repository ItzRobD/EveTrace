import { Component, computed, inject, OnInit, signal } from '@angular/core';
import { DatePipe, DecimalPipe } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { ActivatedRoute } from '@angular/router';
import { forkJoin } from 'rxjs';
import { ChartData, ChartOptions } from 'chart.js';
import { BaseChartDirective } from 'ng2-charts';
import { Tab, TabList, TabPanel, TabPanels, Tabs } from 'primeng/tabs';
import { Tag } from 'primeng/tag';
import { Skeleton } from 'primeng/skeleton';
import { TableModule } from 'primeng/table';
import { InputText } from 'primeng/inputtext';
import { Select } from 'primeng/select';
import { Session } from '../../models/session.model';
import { CombatEvent } from '../../models/combat-event.model';
import { KillEvent } from '../../models/kill-event.model';
import { MiningEvent } from '../../models/mining-event.model';
import { TravelEvent } from '../../models/travel-event.model';
import { CapEvent } from '../../models/cap-event.model';
import { ReloadEvent } from '../../models/reload-event.model';
import { ApiService } from '../../services/api.service';

interface DirectionOption {
  label: string;
  value: string | null;
}

@Component({
  standalone: true,
  selector: 'app-session-detail',
  templateUrl: './session-detail.html',
  styleUrl: './session-detail.scss',
  imports: [
    BaseChartDirective,
    DatePipe,
    DecimalPipe,
    FormsModule,
    InputText,
    RouterLink,
    Select,
    Skeleton,
    Tab,
    TableModule,
    TabList,
    TabPanel,
    TabPanels,
    Tabs,
    Tag,
  ],
})
export class SessionDetailComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly route = inject(ActivatedRoute);

  protected readonly session = signal<Session | null>(null);
  protected readonly loading = signal(true);
  protected readonly combatEvents = signal<CombatEvent[]>([]);
  protected readonly killEvents = signal<KillEvent[]>([]);
  protected readonly miningEvents = signal<MiningEvent[]>([]);
  protected readonly travelEvents = signal<TravelEvent[]>([]);
  protected readonly capEvents = signal<CapEvent[]>([]);
  protected readonly reloadEvents = signal<ReloadEvent[]>([]);

  protected readonly combatFilter = signal('');
  protected readonly directionFilter = signal<string | null>(null);

  protected readonly directionOptions: DirectionOption[] = [
    { label: 'All', value: null },
    { label: 'Outgoing', value: 'out' },
    { label: 'Incoming', value: 'in' },
  ];

  protected readonly combatStats = computed(() => {
    const events = this.combatEvents() ?? [];
    const kills = this.killEvents() ?? [];
    return {
      totalDamageOut: events
        .filter(e => e.Direction === 'out' && !e.IsMiss)
        .reduce((sum, e) => sum + e.Damage, 0),
      totalDamageIn: events
        .filter(e => e.Direction === 'in' && !e.IsMiss)
        .reduce((sum, e) => sum + e.Damage, 0),
      totalKills: kills.length,
      totalBounty: kills.reduce((sum, k) => sum + k.BountyIsk, 0),
    };
  });

  protected readonly damageChartData = computed<ChartData<'bar'>>(() => {
    const events = (this.combatEvents() ?? []).filter(e => !e.IsMiss);
    const buckets = new Map<number, { out: number; in: number }>();
    for (const e of events) {
      const minute = Math.floor(new Date(e.Timestamp).getTime() / 60000) * 60000;
      const bucket = buckets.get(minute) ?? { out: 0, in: 0 };
      if (e.Direction === 'out') bucket.out += e.Damage;
      else bucket.in += e.Damage;
      buckets.set(minute, bucket);
    }
    const sorted = [...buckets.entries()].sort(([a], [b]) => a - b);
    return {
      labels: sorted.map(([ms]) => new Date(ms).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })),
      datasets: [
        {
          label: 'Outgoing',
          data: sorted.map(([, v]) => v.out),
          backgroundColor: 'rgba(59,130,246,0.7)',
          borderColor: '#3b82f6',
          borderWidth: 1,
        },
        {
          label: 'Incoming',
          data: sorted.map(([, v]) => v.in),
          backgroundColor: 'rgba(239,68,68,0.7)',
          borderColor: '#ef4444',
          borderWidth: 1,
        },
      ],
    };
  });

  protected readonly damageChartOptions: ChartOptions<'bar'> = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: { position: 'top' },
      tooltip: { mode: 'index', intersect: false },
    },
    scales: {
      x: { stacked: false },
      y: { beginAtZero: true, title: { display: true, text: 'Damage' } },
    },
  };

  protected readonly miningStats = computed(() => {
    const events = this.miningEvents() ?? [];
    const oreMap = new Map<string, number>();
    let residueTotal = 0;
    for (const e of events) {
      if (e.IsResidue) {
        residueTotal += e.Amount;
      } else {
        const ore = e.OreType ?? 'Unknown';
        oreMap.set(ore, (oreMap.get(ore) ?? 0) + e.Amount);
      }
    }
    return {
      oreEntries: [...oreMap.entries()].sort(([, a], [, b]) => b - a),
      totalYield: [...oreMap.values()].reduce((s, v) => s + v, 0),
      residueTotal,
    };
  });

  protected readonly entityStats = computed(() => {
    const stats = new Map<string, { name: string; kills: number; dmgOut: number; dmgIn: number }>();
    for (const e of this.combatEvents() ?? []) {
      if (e.IsMiss || !e.Entity) continue;
      const prev = stats.get(e.Entity) ?? { name: e.Entity, kills: 0, dmgOut: 0, dmgIn: 0 };
      if (e.Direction === 'out') prev.dmgOut += e.Damage;
      else prev.dmgIn += e.Damage;
      stats.set(e.Entity, prev);
    }
    for (const k of this.killEvents() ?? []) {
      if (!k.Entity) continue;
      const prev = stats.get(k.Entity) ?? { name: k.Entity, kills: 0, dmgOut: 0, dmgIn: 0 };
      prev.kills += 1;
      stats.set(k.Entity, prev);
    }
    return [...stats.values()].sort(
      (a, b) => (b.kills * 100_000 + b.dmgOut + b.dmgIn) - (a.kills * 100_000 + a.dmgOut + a.dmgIn),
    );
  });

  protected readonly filteredCombatEvents = computed(() => {
    let events = this.combatEvents() ?? [];
    const dir = this.directionFilter();
    const filterText = this.combatFilter();
    if (dir) {
      events = events.filter(e => e.Direction === dir);
    }
    if (filterText.trim()) {
      const term = filterText.toLowerCase();
      events = events.filter(e => e.Entity.toLowerCase().includes(term));
    }
    return events;
  });

  protected characterName(): string {
    return this.session()?.SessionKey.split('/')[0] ?? '—';
  }

  ngOnInit(): void {
    const idParam = this.route.snapshot.paramMap.get('id');
    const sessionId = idParam ? parseInt(idParam, 10) : 0;
    if (!sessionId) return;

    forkJoin({
      session: this.api.getSession(sessionId),
      combat: this.api.getCombatEvents(sessionId),
      kills: this.api.getKillEvents(sessionId),
      mining: this.api.getMiningEvents(sessionId),
      travel: this.api.getTravelEvents(sessionId),
      cap: this.api.getCapEvents(sessionId),
      reload: this.api.getReloadEvents(sessionId),
    }).subscribe({
      next: data => {
        this.session.set(data.session);
        this.combatEvents.set(data.combat ?? []);
        this.killEvents.set(data.kills ?? []);
        this.miningEvents.set(data.mining ?? []);
        this.travelEvents.set(data.travel ?? []);
        this.capEvents.set(data.cap ?? []);
        this.reloadEvents.set(data.reload ?? []);
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }
}
