import { inject, Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { map, Observable } from 'rxjs';
import { Character } from '../models/character.model';
import { Session } from '../models/session.model';
import { CombatEvent } from '../models/combat-event.model';
import { KillEvent } from '../models/kill-event.model';
import { MiningEvent } from '../models/mining-event.model';
import { TravelEvent } from '../models/travel-event.model';
import { CapEvent } from '../models/cap-event.model';
import { ReloadEvent } from '../models/reload-event.model';

interface ApiResponse<T> {
  data: T;
  count: number;
}

@Injectable({ providedIn: 'root' })
export class ApiService {
  private readonly http = inject(HttpClient);
  private readonly baseURL = '/api';

  private unwrap<T>(obs: Observable<ApiResponse<T>>): Observable<T> {
    return obs.pipe(map(r => r.data));
  }

  getCharacters(): Observable<Character[]> {
    return this.unwrap(this.http.get<ApiResponse<Character[]>>(`${this.baseURL}/characters`));
  }

  getCharacter(id: number): Observable<Character> {
    return this.unwrap(this.http.get<ApiResponse<Character>>(`${this.baseURL}/characters/${id}`));
  }

  deleteCharacter(id: number): Observable<void> {
    return this.http.delete<void>(`${this.baseURL}/characters/${id}`);
  }

  getCharacterSessions(id: number): Observable<Session[]> {
    return this.unwrap(this.http.get<ApiResponse<Session[]>>(`${this.baseURL}/characters/${id}/sessions`));
  }

  getSessions(): Observable<Session[]> {
    return this.unwrap(this.http.get<ApiResponse<Session[]>>(`${this.baseURL}/sessions`));
  }

  getSession(id: number): Observable<Session> {
    return this.unwrap(this.http.get<ApiResponse<Session>>(`${this.baseURL}/sessions/${id}`));
  }

  deleteSession(id: number): Observable<void> {
    return this.http.delete<void>(`${this.baseURL}/sessions/${id}`);
  }

  getCombatEvents(sessionId: number): Observable<CombatEvent[]> {
    return this.unwrap(this.http.get<ApiResponse<CombatEvent[]>>(`${this.baseURL}/sessions/${sessionId}/combat`));
  }

  getKillEvents(sessionId: number): Observable<KillEvent[]> {
    return this.unwrap(this.http.get<ApiResponse<KillEvent[]>>(`${this.baseURL}/sessions/${sessionId}/kills`));
  }

  getMiningEvents(sessionId: number): Observable<MiningEvent[]> {
    return this.unwrap(this.http.get<ApiResponse<MiningEvent[]>>(`${this.baseURL}/sessions/${sessionId}/mining`));
  }

  getTravelEvents(sessionId: number): Observable<TravelEvent[]> {
    return this.unwrap(this.http.get<ApiResponse<TravelEvent[]>>(`${this.baseURL}/sessions/${sessionId}/travel`));
  }

  getCapEvents(sessionId: number): Observable<CapEvent[]> {
    return this.unwrap(this.http.get<ApiResponse<CapEvent[]>>(`${this.baseURL}/sessions/${sessionId}/cap`));
  }

  getReloadEvents(sessionId: number): Observable<ReloadEvent[]> {
    return this.unwrap(this.http.get<ApiResponse<ReloadEvent[]>>(`${this.baseURL}/sessions/${sessionId}/reload`));
  }

  getStatus(): Observable<{ logDir: string; eventsProcessed: number; sessionsOpened: number; wsClients: number }> {
    return this.http.get<{ logDir: string; eventsProcessed: number; sessionsOpened: number; wsClients: number }>(
      `${this.baseURL}/status`,
    );
  }

  getLogDirPresets(): Observable<{ label: string; path: string }[]> {
    return this.http.get<{ presets: { label: string; path: string }[] }>(
      `${this.baseURL}/config/presets`,
    ).pipe(map(r => r.presets));
  }

  setLogDir(logDir: string): Observable<{ logDir: string; eventsProcessed: number; sessionsOpened: number; wsClients: number }> {
    return this.http.post<{ logDir: string; eventsProcessed: number; sessionsOpened: number; wsClients: number }>(
      `${this.baseURL}/config/logdir`,
      { logDir },
    );
  }

  replaySession(
    sessionId: number,
    speed = 20,
    maxGapMs = 500,
  ): Observable<{ message: string; events: number; speed: number; max_gap_ms: number; duration_secs: number }> {
    return this.http.post<{ message: string; events: number; speed: number; max_gap_ms: number; duration_secs: number }>(
      `${this.baseURL}/debug/replay/${sessionId}?speed=${speed}&max_gap_ms=${maxGapMs}`,
      null,
    );
  }

  cancelReplay(sessionId: number): Observable<void> {
    return this.http.delete<void>(`${this.baseURL}/debug/replay/${sessionId}`);
  }
}
