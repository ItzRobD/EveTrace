import { Routes } from '@angular/router';

export const routes: Routes = [
  { path: '', redirectTo: '/live', pathMatch: 'full' },
  {
    path: 'characters',
    loadComponent: () =>
      import('./pages/characters/characters').then(m => m.CharactersComponent),
  },
  {
    path: 'sessions/:id',
    loadComponent: () =>
      import('./pages/session-detail/session-detail').then(m => m.SessionDetailComponent),
  },
  {
    path: 'live',
    loadComponent: () =>
      import('./pages/live/live').then(m => m.LiveComponent),
  },
  {
    path: 'replay',
    loadComponent: () =>
      import('./pages/replay/replay').then(m => m.ReplayComponent),
  },
  {
    path: 'popout/:name',
    loadComponent: () =>
      import('./pages/popout-character/popout-character').then(m => m.PopoutCharacterComponent),
  },
  {
    path: 'config',
    loadComponent: () =>
      import('./pages/config/config').then(m => m.ConfigComponent),
  },
];
