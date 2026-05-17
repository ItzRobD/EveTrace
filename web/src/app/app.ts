import { Component, inject, OnDestroy, OnInit } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { toSignal } from '@angular/core/rxjs-interop';
import { map, Subscription } from 'rxjs';
import { BreakpointObserver } from '@angular/cdk/layout';
import { Drawer } from 'primeng/drawer';
import { Button } from 'primeng/button';
import { Tag } from 'primeng/tag';
import { Tooltip } from 'primeng/tooltip';
import { EventStreamService, ConnectionStatus } from './services/event-stream.service';

interface NavItem {
  label: string;
  icon: string;
  route: string;
}

const MOBILE_BREAKPOINT = '(max-width: 768px)';

@Component({
  selector: 'app-root',
  imports: [RouterOutlet, RouterLink, RouterLinkActive, Drawer, Button, Tag, Tooltip],
  templateUrl: './app.html',
  styleUrl: './app.scss',
})
export class App implements OnInit, OnDestroy {
  private readonly eventStream = inject(EventStreamService);
  private readonly breakpoints = inject(BreakpointObserver);
  private eventsSubscription?: Subscription;

  protected readonly connectionStatus = toSignal(this.eventStream.status$, {
    initialValue: 'disconnected' as ConnectionStatus,
  });

  protected readonly isMobile = toSignal(
    this.breakpoints.observe(MOBILE_BREAKPOINT).pipe(map(r => r.matches)),
    { initialValue: false },
  );

  protected sidebarCollapsed = false;
  protected drawerOpen = false;

  protected readonly navItems: NavItem[] = [
    { label: 'Live', icon: 'pi pi-bolt', route: '/live' },
    { label: 'Characters', icon: 'pi pi-users', route: '/characters' },
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

  ngOnInit(): void {
    this.eventsSubscription = this.eventStream.events$.subscribe();
  }

  ngOnDestroy(): void {
    this.eventsSubscription?.unsubscribe();
  }
}
