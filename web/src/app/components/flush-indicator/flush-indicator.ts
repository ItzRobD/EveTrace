import {
  ChangeDetectionStrategy,
  Component,
  computed,
  inject,
  OnDestroy,
  OnInit,
  signal,
} from '@angular/core';
import { DecimalPipe } from '@angular/common';
import { interval, startWith, Subscription, switchMap } from 'rxjs';
import { Button } from 'primeng/button';
import { ProgressBar } from 'primeng/progressbar';
import { ApiService, StatusResponse } from '../../services/api.service';

// Compact header indicator for un-flushed parse progress. It only appears while
// events are buffered: a small indeterminate strobe bar signals "working", a count
// shows how many events are pending, and hovering reveals a popover with a flush
// control. Deliberately unobtrusive so it doesn't distract during a reparse.
@Component({
  standalone: true,
  selector: 'app-flush-indicator',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [DecimalPipe, Button, ProgressBar],
  template: `
    @if (status(); as s) {
      @if (s.pendingEvents > 0) {
        <div class="flush-indicator" [class.is-flushing]="flushing()">
          <p-progressbar
            mode="indeterminate"
            [showValue]="false"
            styleClass="flush-progress"
          />
          <span class="flush-count">{{ s.pendingEvents | number }} Events</span>

          <div class="flush-pop">
            <div class="flush-pop-text">
              <strong>{{ s.pendingEvents | number }}</strong> event(s) buffered
              <div class="muted">next write in {{ countdown() }}</div>
            </div>
            <p-button
              [label]="flushing() ? 'Flushing…' : 'Flush now'"
              icon="pi pi-save"
              size="small"
              [loading]="flushing()"
              (onClick)="flushNow()"
            />
          </div>
        </div>
      }
    }
  `,
  styles: [`
    .flush-indicator {
      position: relative;
      display: inline-flex;
      flex-direction: column; // strobe bar on top, count underneath
      align-items: center;
      gap: 2px;
      cursor: default;
    }

    // ── Strobe bar size knobs ── adjust width / height here.
    // PrimeNG puts the styleClass on the same element as .p-progressbar, so we
    // size .flush-progress directly (not a descendant).
    :host ::ng-deep .flush-progress {
      width: 54px;
      height: 3px; // bar thickness
      background: var(--p-surface-300);
      border-radius: 5px;
      overflow: hidden;

      .p-progressbar-value {
        background: #4da8c0;
      }
    }

    .flush-count {
      font-size: 0.7rem;
      font-weight: 600;
      color: var(--eve-warn); // event-count text stays warn/yellow
      font-variant-numeric: tabular-nums;
      white-space: nowrap;
    }

    .flush-pop {
      position: absolute;
      top: 100%;
      right: 0;
      margin-top: 0.4rem;
      z-index: 100;
      display: flex;
      flex-direction: column;
      gap: 0.5rem;
      padding: 0.6rem 0.7rem;
      min-width: 13rem;
      background: var(--p-surface-100);
      border: 1px solid var(--p-surface-border);
      border-radius: var(--p-border-radius-sm, 2px);
      box-shadow: 0 4px 14px rgba(0, 0, 0, 0.35);

      // Hidden until the indicator (or the popover itself) is hovered.
      visibility: hidden;
      opacity: 0;
      transform: translateY(-2px);
      transition: opacity 0.12s ease, transform 0.12s ease, visibility 0.12s;
    }

    .flush-indicator:hover .flush-pop {
      visibility: visible;
      opacity: 1;
      transform: translateY(0);
    }

    .flush-pop-text {
      font-size: 0.78rem;
      color: var(--p-text-color);
      strong { color: var(--p-text-color); }
      .muted { color: var(--p-text-muted-color); font-size: 0.72rem; margin-top: 0.15rem; }
    }
  `],
})
export class FlushIndicatorComponent implements OnInit, OnDestroy {
  private readonly api = inject(ApiService);

  protected readonly status = signal<StatusResponse | null>(null);
  protected readonly flushing = signal(false);

  // Local 1-second clock drives the countdown between status polls.
  private readonly nowMs = signal(Date.now());
  private flushAtMs = 0;

  protected readonly countdown = computed(() => {
    const remaining = Math.max(0, this.flushAtMs - this.nowMs());
    const totalSec = Math.round(remaining / 1000);
    const m = Math.floor(totalSec / 60);
    const s = totalSec % 60;
    return `${m}:${s.toString().padStart(2, '0')}`;
  });

  private pollSub?: Subscription;
  private tickSub?: Subscription;

  ngOnInit(): void {
    this.pollSub = interval(5000)
      .pipe(
        startWith(0),
        switchMap(() => this.api.getStatus()),
      )
      .subscribe({
        next: s => this.applyStatus(s),
        error: () => {},
      });
    this.tickSub = interval(1000).subscribe(() => this.nowMs.set(Date.now()));
  }

  ngOnDestroy(): void {
    this.pollSub?.unsubscribe();
    this.tickSub?.unsubscribe();
  }

  protected flushNow(): void {
    this.flushing.set(true);
    this.api.flushEvents().subscribe({
      next: r => {
        this.applyStatus(r.status);
        this.flushing.set(false);
      },
      error: () => this.flushing.set(false),
    });
  }

  private applyStatus(s: StatusResponse): void {
    this.status.set(s);
    this.flushAtMs = Date.now() + s.secondsToNextFlush * 1000;
  }
}
