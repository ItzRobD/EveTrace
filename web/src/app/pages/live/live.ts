import {
  ChangeDetectionStrategy,
  Component,
  computed,
  inject,
  OnDestroy,
  OnInit,
  signal,
} from '@angular/core';
import { bufferTime, filter, Subject, takeUntil } from 'rxjs';
import { Skeleton } from 'primeng/skeleton';
import { Button } from 'primeng/button';
import { CharacterCardComponent } from '../../components/character-card/character-card';
import { ApiService } from '../../services/api.service';
import { EventStreamService } from '../../services/event-stream.service';
import { LiveEvent } from '../../models/live-event.model';
import { Session } from '../../models/session.model';
import {
  applyEvent,
  CAP_ALERT_MS,
  CharacterLiveState,
  EMPTY_STATE,
} from '../../models/character-live-state';

@Component({
  standalone: true,
  selector: 'app-live',
  templateUrl: './live.html',
  styleUrl: './live.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [Button, CharacterCardComponent, Skeleton],
})
export class LiveComponent implements OnInit, OnDestroy {
  private readonly api = inject(ApiService);
  private readonly eventStream = inject(EventStreamService);

  protected readonly loading = signal(true);
  protected readonly liveState = signal<Map<string, CharacterLiveState>>(new Map());
  protected readonly hiddenChars = signal<Set<string>>(new Set());

  protected readonly allChars = computed(() => [...this.liveState().values()]);
  protected readonly activeChars = computed(() => {
    const hidden = this.hiddenChars();
    return this.allChars().filter(s => !hidden.has(s.characterName));
  });
  protected readonly showFilter = computed(() => this.allChars().length > 1);

  private readonly destroy$ = new Subject<void>();
  private sessionsByKey = new Map<string, Session>();
  private fetchedKeys = new Set<string>();

  ngOnInit(): void {
    this.api.getSessions().subscribe({
      next: sessions => {
        for (const s of sessions ?? []) {
          this.sessionsByKey.set(s.SessionKey, s);
        }
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });

    this.eventStream.events$
      .pipe(
        bufferTime(100),
        filter(batch => batch.length > 0),
        takeUntil(this.destroy$),
      )
      .subscribe(batch => this.processBatch(batch));
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  protected toggleChar(name: string): void {
    this.hiddenChars.update(set => {
      const next = new Set(set);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  }

  private processBatch(events: LiveEvent[]): void {
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

      updatedMap.set(charName, updated);

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
