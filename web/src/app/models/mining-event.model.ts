export interface MiningEvent {
  ID: number;
  SessionID: number;
  Timestamp: string;
  OreType: string | null;
  Amount: number;
  IsResidue: number;
  IsCritical: number;
}

export interface MiningFullEvent {
  ID: number;
  SessionID: number;
  Timestamp: string;
  Module: string;
}
