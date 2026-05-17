export interface CombatEvent {
  ID: number;
  SessionID: number;
  Timestamp: string;
  Direction: 'in' | 'out';
  Damage: number;
  Entity: string;
  Weapon: string | null;
  HitType: string | null;
  IsMiss: number;
}
