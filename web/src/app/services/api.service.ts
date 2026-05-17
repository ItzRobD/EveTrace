import { inject, Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { Character } from '../models/character.model';
import { Session } from '../models/session.model';
import { CombatEvent } from '../models/combat-event.model';
import { KillEvent } from '../models/kill-event.model';
import { MiningEvent } from '../models/mining-event.model';
import { TravelEvent } from '../models/travel-event.model';
import { CapEvent } from '../models/cap-event.model';
import { ReloadEvent } from '../models/reload-event.model';

@Injectable({ providedIn: 'root' })
export class ApiService {
  private readonly http = inject(HttpClient);
  private readonly baseURL = '/api';

  getCharacters(): Observable<Character[]> {
    return this.http.get<Character[]>(`${this.baseURL}/characters`);
  }

  getCharacter(id: number): Observable<Character> {
    return this.http.get<Character>(`${this.baseURL}/characters/${id}`);
  }

  deleteCharacter(id: number): Observable<void> {
    return this.http.delete<void>(`${this.baseURL}/characters/${id}`);
  }

  getCharacterSessions(id: number): Observable<Session[]> {
    return this.http.get<Session[]>(`${this.baseURL}/characters/${id}/sessions`);
  }

  getSessions(): Observable<Session[]> {
    return this.http.get<Session[]>(`${this.baseURL}/sessions`);
  }

  getSession(id: number): Observable<Session> {
    return this.http.get<Session>(`${this.baseURL}/sessions/${id}`);
  }

  deleteSession(id: number): Observable<void> {
    return this.http.delete<void>(`${this.baseURL}/sessions/${id}`);
  }

  getCombatEvents(sessionId: number): Observable<CombatEvent[]> {
    return this.http.get<CombatEvent[]>(`${this.baseURL}/sessions/${sessionId}/combat`);
  }

  getKillEvents(sessionId: number): Observable<KillEvent[]> {
    return this.http.get<KillEvent[]>(`${this.baseURL}/sessions/${sessionId}/kills`);
  }

  getMiningEvents(sessionId: number): Observable<MiningEvent[]> {
    return this.http.get<MiningEvent[]>(`${this.baseURL}/sessions/${sessionId}/mining`);
  }

  getTravelEvents(sessionId: number): Observable<TravelEvent[]> {
    return this.http.get<TravelEvent[]>(`${this.baseURL}/sessions/${sessionId}/travel`);
  }

  getCapEvents(sessionId: number): Observable<CapEvent[]> {
    return this.http.get<CapEvent[]>(`${this.baseURL}/sessions/${sessionId}/cap`);
  }

  getReloadEvents(sessionId: number): Observable<ReloadEvent[]> {
    return this.http.get<ReloadEvent[]>(`${this.baseURL}/sessions/${sessionId}/reload`);
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
