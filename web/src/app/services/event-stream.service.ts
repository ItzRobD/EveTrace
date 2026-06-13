import { Injectable, OnDestroy } from '@angular/core';
import { webSocket, WebSocketSubject } from 'rxjs/webSocket';
import { BehaviorSubject, EMPTY, Observable, Subject, defer } from 'rxjs';
import { catchError, repeat, share, takeUntil } from 'rxjs';
import { filter, map } from 'rxjs';
import {
  LiveEvent,
  LiveCombatPayload,
  LiveKillPayload,
  LiveMiningPayload,
} from '../models/live-event.model';
import { DiagnosticEvent } from '../models/diagnostic-event.model';

export type ConnectionStatus = 'connected' | 'disconnected' | 'reconnecting';
export type WSMessage = LiveEvent | DiagnosticEvent;

const RECONNECT_DELAY_MS = 3000;
const WS_URL = `ws://${window.location.host}/ws`;

@Injectable({ providedIn: 'root' })
export class EventStreamService implements OnDestroy {
  private socket$: WebSocketSubject<WSMessage> | null = null;
  private readonly destroy$ = new Subject<void>();
  private readonly statusSubject$ = new BehaviorSubject<ConnectionStatus>('disconnected');

  constructor() {
    // Hold a permanent subscription so the WebSocket stays connected regardless
    // of which page is active. Without this, share() drops to refCount 0 on
    // navigation, closes the socket, and the backend re-triggers any active
    // replay when the socket reconnects.
    this.events$.subscribe();
  }

  readonly status$: Observable<ConnectionStatus> = this.statusSubject$.asObservable();

  // defer ensures a fresh WebSocketSubject is created on each subscription
  // (including reconnects). repeat({ delay }) only resubscribes after the
  // inner observable completes, which happens when the socket closes — so
  // we never tear down a healthy connection on a timer.
  readonly events$: Observable<WSMessage> = defer(() => {
    this.socket$ = webSocket<WSMessage>({
      url: WS_URL,
      openObserver: {
        next: () => this.statusSubject$.next('connected'),
      },
      closeObserver: {
        next: () => this.statusSubject$.next('reconnecting'),
      },
    });
    return this.socket$.pipe(
      catchError(() => {
        this.statusSubject$.next('reconnecting');
        return EMPTY;
      }),
    );
  }).pipe(
    repeat({ delay: RECONNECT_DELAY_MS }),
    takeUntil(this.destroy$),
    share(),
  );

  readonly diagnostics$: Observable<DiagnosticEvent> = this.events$.pipe(
    filter((m): m is DiagnosticEvent => 'level' in m),
  );

  readonly liveEvents$: Observable<LiveEvent> = this.events$.pipe(
    filter((m): m is LiveEvent => 'Type' in m),
  );

  readonly combat$: Observable<LiveCombatPayload & { SessionID: string; Timestamp: string }> =
    this.liveEvents$.pipe(
      filter((e): e is LiveEvent & { Combat: LiveCombatPayload } =>
        e.Type === 'combat' && e.Combat !== null,
      ),
      map(e => ({ ...e.Combat, SessionID: e.SessionID, Timestamp: e.Timestamp })),
    );

  readonly kills$: Observable<LiveKillPayload & { SessionID: string; Timestamp: string }> =
    this.liveEvents$.pipe(
      filter((e): e is LiveEvent & { Kill: LiveKillPayload } =>
        e.Type === 'kill' && e.Kill !== null,
      ),
      map(e => ({ ...e.Kill, SessionID: e.SessionID, Timestamp: e.Timestamp })),
    );

  readonly mining$: Observable<LiveMiningPayload & { SessionID: string; Timestamp: string }> =
    this.liveEvents$.pipe(
      filter((e): e is LiveEvent & { Mining: LiveMiningPayload } =>
        e.Type === 'mining' && e.Mining !== null,
      ),
      map(e => ({ ...e.Mining, SessionID: e.SessionID, Timestamp: e.Timestamp })),
    );

  disconnect(): void {
    this.destroy$.next();
    this.socket$?.complete();
    this.statusSubject$.next('disconnected');
  }

  ngOnDestroy(): void {
    this.disconnect();
    this.destroy$.complete();
    this.statusSubject$.complete();
  }
}
