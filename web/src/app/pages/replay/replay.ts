import {
  ChangeDetectionStrategy,
  Component,
  computed,
  inject,
  OnDestroy,
  OnInit,
  signal,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { bufferTime, filter, Subject, takeUntil } from 'rxjs';
import { Button } from 'primeng/button';
import { Select } from 'primeng/select';
import { Skeleton } from 'primeng/skeleton';
import { CharacterCardComponent } from '../../components/character-card/character-card';
import { ApiService } from '../../services/api.service';
import { EventStreamService } from '../../services/event-stream.service';
import { LiveEvent } from '../../models/live-event.model';
import { Session } from '../../models/session.model';
import {
  applyEvent,
  buildChartData,
  CAP_ALERT_MS,
  CharacterLiveState,
  EMPTY_STATE,
} from '../../models/character-live-state';

@Component({
  standalone: true,
  selector: 'app-replay',
  templateUrl: './replay.html',
  styleUrl: './replay.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [Button, CharacterCardComponent, FormsModule, Select, Skeleton],
})
export class ReplayComponent implements OnInit, OnDestroy {
  private readonly api = inject(ApiService);
  private readonly eventStream = inject(EventStreamService);

  protected readonly loading = signal(true);
  protected readonly allSessions = signal<Session[]>([]);
  protected readonly selectedSessionId = signal<number | null>(null);
  protected readonly replaySpeed = signal(20);
  protected readonly replayMaxGap = signal(500);
  protected readonly replayActive = signal(false);
  protected readonly replayPaused = signal(false);
  protected readonly replayInfo = signal('');

  protected readonly sessionsWithEvents = computed(() =>
    this.allSessions().filter(s => s.EventCount > 0),
  );

  protected readonly liveState = signal<Map<string, CharacterLiveState>>(new Map());
  protected readonly activeChars = computed(() => [...this.liveState().values()]);

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

  private readonly stopStream$ = new Subject<void>();
  private replayTimer: ReturnType<typeof setTimeout> | null = null;
  private sessionsByKey = new Map<string, Session>();
  private fetchedKeys = new Set<string>();

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

  protected onSessionChange(id: number | null): void {
    this.selectedSessionId.set(id);
    // If a replay is already running, stop it so the new selection starts clean.
    if (this.replayActive()) {
      this.stopReplay();
    }
  }

  protected startReplay(): void {
    const id = this.selectedSessionId();
    if (!id) return;

    // Always tear down any existing stream + state before starting fresh.
    this.clearReplay();

    this.replayActive.set(true);
    this.replayInfo.set('');
    this.fetchedKeys.clear();

    // Subscribe to event stream for this replay's duration.
    this.eventStream.events$
      .pipe(
        bufferTime(100),
        filter(batch => batch.length > 0),
        takeUntil(this.stopStream$),
      )
      .subscribe(batch => this.processBatch(batch));

    this.api.replaySession(id, this.replaySpeed(), this.replayMaxGap()).subscribe({
      next: res => {
        if (!res.events) {
          this.stopStream$.next();
          this.replayActive.set(false);
          this.replayInfo.set('No events found for this session');
          return;
        }
        this.replayInfo.set(`Replaying ${res.events} events at ${res.speed}×`);
        const ms = (res.duration_secs + 2) * 1000;
        this.replayTimer = setTimeout(() => {
          this.replayTimer = null;
          this.stopStream$.next();
          this.replayActive.set(false);
          this.replayInfo.set('Replay complete');
        }, ms);
      },
      error: () => {
        this.stopStream$.next();
        this.replayActive.set(false);
        this.replayInfo.set('Failed to start replay');
      },
    });
  }

  protected pauseReplay(): void {
    this.replayPaused.set(true);
    this.replayInfo.set('Paused');
  }

  protected resumeReplay(): void {
    this.replayPaused.set(false);
    this.replayInfo.set('');
  }

  protected stopReplay(): void {
    const id = this.selectedSessionId();

    this.clearReplay();
    this.replayActive.set(false);
    this.replayPaused.set(false);

    if (!id) return;
    this.api.cancelReplay(id).subscribe({
      next: () => this.replayInfo.set('Replay cancelled'),
      error: () => this.replayInfo.set('Replay already finished'),
    });
  }

  private clearReplay(): void {
    if (this.replayTimer !== null) {
      clearTimeout(this.replayTimer);
      this.replayTimer = null;
    }
    this.stopStream$.next();
    this.liveState.set(new Map());
  }

  private processBatch(events: LiveEvent[]): void {
    if (this.replayPaused()) return;
    const updatedMap = new Map(this.liveState());
    const capTimeoutChars: string[] = [];

    for (const event of events) {
      const charName = event.SessionID.split('/')[0];
      if (!charName) continue;

      const session = this.sessionsByKey.get(event.SessionID);
      if (!session && !this.fetchedKeys.has(event.SessionID)) {
        this.fetchedKeys.add(event.SessionID);
        this.fetchSessionForKey(event.SessionID, charName);
      }

      const existing: CharacterLiveState = updatedMap.get(charName) ?? {
        characterName: charName,
        sessionKey: event.SessionID,
        sessionDbId: session?.ID ?? null,
        ...EMPTY_STATE,
      };

      const updated = applyEvent(existing, event);

      const chartData =
        updated.dpsBuckets !== existing.dpsBuckets ||
        updated.killMarkers !== existing.killMarkers ||
        updated.capMarkers !== existing.capMarkers ||
        updated.criticalMarkers !== existing.criticalMarkers
          ? buildChartData(updated.dpsBuckets, updated.killMarkers, updated.capMarkers, updated.criticalMarkers)
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
}
