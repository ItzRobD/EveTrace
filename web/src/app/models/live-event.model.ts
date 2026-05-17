export type EventType =
  | 'combat'
  | 'kill'
  | 'mining'
  | 'nav'
  | 'cap_starvation'
  | 'reload'
  | 'mining_full';

export interface LiveCombatPayload {
  Direction: 'in' | 'out';
  Damage: number;
  Entity: string;
  Weapon: string;
  HitType: string;
  Miss: boolean;
}

export interface LiveKillPayload {
  Entity: string;
  BountyISK: number;
}

export interface LiveMiningPayload {
  OreType: string;
  Amount: number;
  Residue: boolean;
  Critical: boolean;
}

export interface LiveNavPayload {
  From: string;
  To: string;
}

export interface LiveCapStarvationPayload {
  Module: string;
  Required: number;
  Available: number;
}

export interface LiveReloadPayload {
  Charge: string;
  Launcher: string;
  Seconds: number;
}

export interface LiveMiningFullPayload {
  Module: string;
}

export interface LiveEvent {
  Type: EventType;
  SessionID: string;
  Timestamp: string;
  Live: boolean;
  Combat: LiveCombatPayload | null;
  Kill: LiveKillPayload | null;
  Mining: LiveMiningPayload | null;
  Nav: LiveNavPayload | null;
  CapStarvation: LiveCapStarvationPayload | null;
  Reload: LiveReloadPayload | null;
  MiningFull: LiveMiningFullPayload | null;
}
