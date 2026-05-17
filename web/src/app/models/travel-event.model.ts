export interface TravelEvent {
  ID: number;
  SessionID: number;
  Timestamp: string;
  FromSystem: string | null;
  ToSystem: string;
}
