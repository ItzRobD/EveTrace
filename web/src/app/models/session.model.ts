export interface Session {
  ID: number;
  CharacterID: number;
  SessionKey: string;
  LogPath: string;
  StartedAt: string;
  Language: string;
  LastByteOffset: number;
}
