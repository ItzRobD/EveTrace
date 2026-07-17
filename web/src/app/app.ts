import { Component, computed, inject, OnDestroy, OnInit, signal } from '@angular/core';
import { Router, RouterLink, RouterLinkActive, RouterOutlet, NavigationEnd } from '@angular/router';
import { toSignal } from '@angular/core/rxjs-interop';
import { map, Subscription, filter } from 'rxjs';
import { BreakpointObserver } from '@angular/cdk/layout';
import { Drawer } from 'primeng/drawer';
import { Button } from 'primeng/button';
import { Tag } from 'primeng/tag';
import { Tooltip } from 'primeng/tooltip';
import { Dialog } from 'primeng/dialog';
import { ConfirmDialog } from 'primeng/confirmdialog';
import { ConfirmationService } from 'primeng/api';
import { EventStreamService, ConnectionStatus } from './services/event-stream.service';
import { ThemeService } from './services/theme.service';
import { ApiService } from './services/api.service';
import { FlushIndicatorComponent } from './components/flush-indicator/flush-indicator';

interface NavItem {
  label: string;
  icon: string;
  route: string;
}

const MOBILE_BREAKPOINT = '(max-width: 768px)';

@Component({
  selector: 'app-root',
  imports: [RouterOutlet, RouterLink, RouterLinkActive, Drawer, Button, Tag, Tooltip, Dialog, ConfirmDialog, FlushIndicatorComponent],
  providers: [ConfirmationService],
  templateUrl: './app.html',
  styleUrl: './app.scss',
})
export class App implements OnInit, OnDestroy {
  private readonly eventStream = inject(EventStreamService);
  private readonly theme = inject(ThemeService);
  private readonly breakpoints = inject(BreakpointObserver);
  private readonly api = inject(ApiService);
  private readonly confirm = inject(ConfirmationService);

  protected readonly themeMode = this.theme.mode;
  protected readonly themeIcon = computed(() =>
    this.themeMode() === 'dark' ? 'pi pi-moon' : 'pi pi-sun',
  );

  protected toggleTheme(): void {
    this.theme.toggle();
  }
  private readonly router = inject(Router);
  private eventsSubscription?: Subscription;

  protected readonly connectionStatus = toSignal(this.eventStream.status$, {
    initialValue: 'disconnected' as ConnectionStatus,
  });

  protected readonly isMobile = toSignal(
    this.breakpoints.observe(MOBILE_BREAKPOINT).pipe(map(r => r.matches)),
    { initialValue: false },
  );

  protected readonly isPopout = toSignal(
    this.router.events.pipe(
      filter(event => event instanceof NavigationEnd),
      map(() => this.router.url.includes('/popout/')),
    ),
    { initialValue: this.router.url.includes('/popout/') },
  );

  protected sidebarCollapsed = false;
  protected drawerOpen = false;

  protected noLogDirVisible = false;
  protected noLogDirMessage = '';

  /** Set once the user confirms Quit — swaps the app for a "stopped" overlay. */
  protected readonly shuttingDown = signal(false);

  protected readonly navItems: NavItem[] = [
    { label: 'Live', icon: 'pi pi-gauge', route: '/live' },
    { label: 'Replay', icon: 'pi pi-history', route: '/replay' },
    { label: 'Characters', icon: 'pi pi-users', route: '/characters' },
    { label: 'Config', icon: 'pi pi-cog', route: '/config' },
  ];

  protected onHamburgerClick(): void {
    if (this.isMobile()) {
      this.drawerOpen = true;
    } else {
      this.sidebarCollapsed = false;
    }
  }

  protected statusSeverity(
    status: ConnectionStatus,
  ): 'success' | 'danger' | 'warn' | 'secondary' {
    const map: Record<ConnectionStatus, 'success' | 'danger' | 'warn' | 'secondary'> = {
      connected: 'success',
      disconnected: 'danger',
      reconnecting: 'warn',
    };
    return map[status];
  }

  protected confirmQuit(): void {
    this.confirm.confirm({
      message: 'Shut down EveTrace? Any buffered events will be saved first.',
      header: 'Quit EveTrace',
      icon: 'pi pi-power-off',
      acceptLabel: 'Shut down',
      rejectLabel: 'Cancel',
      accept: () => {
        // Show the stopped overlay immediately so the WebSocket dropping doesn't
        // flash "reconnecting". The request only ever returns 202; the server
        // then exits, so we don't wait on a response body to switch views.
        this.shuttingDown.set(true);
        this.api.shutdown().subscribe({ error: () => {} });
      },
    });
  }

  protected goToConfig(): void {
    this.noLogDirVisible = false;
    this.router.navigate(['/config']);
    this.drawerOpen = false;
  }

  ngOnInit(): void {
    this.eventsSubscription = this.eventStream.events$.subscribe(msg => {
      if ('level' in msg && msg.code === 'no_log_dir') {
        this.noLogDirMessage = msg.message;
        this.noLogDirVisible = true;
      }
    });
  }

  ngOnDestroy(): void {
    this.eventsSubscription?.unsubscribe();
  }
}
