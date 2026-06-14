import { Injectable, effect, signal } from '@angular/core';

export type ThemeMode = 'dark' | 'light';

const STORAGE_KEY = 'evetrace-theme';

// Toggles the `.app-dark` class on <html>, which PrimeNG's darkModeSelector and
// our scheme-scoped CSS variables key off. Defaults to dark (EVE-faithful) and
// persists the choice to localStorage.
@Injectable({ providedIn: 'root' })
export class ThemeService {
  readonly mode = signal<ThemeMode>(this.initialMode());

  constructor() {
    effect(() => this.apply(this.mode()));
  }

  toggle(): void {
    this.mode.update(m => (m === 'dark' ? 'light' : 'dark'));
  }

  private initialMode(): ThemeMode {
    return localStorage.getItem(STORAGE_KEY) === 'light' ? 'light' : 'dark';
  }

  private apply(mode: ThemeMode): void {
    document.documentElement.classList.toggle('app-dark', mode === 'dark');
    localStorage.setItem(STORAGE_KEY, mode);
  }
}
