export interface CapEvent {
  ID: number;
  SessionID: number;
  Timestamp: string;
  Module: string;
  CapRequired: number;
  CapAvailable: number;
}
