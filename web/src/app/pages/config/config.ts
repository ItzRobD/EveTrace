import {
  ChangeDetectionStrategy,
  Component,
  inject,
  OnDestroy,
  OnInit,
  signal,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { DecimalPipe } from '@angular/common';
import { interval, Subscription, switchMap, startWith } from 'rxjs';
import { toSignal } from '@angular/core/rxjs-interop';
import { map } from 'rxjs';
import { InputText } from 'primeng/inputtext';
import { Button } from 'primeng/button';
import { DatePicker } from 'primeng/datepicker';
import { Tag } from 'primeng/tag';
import { ApiService, StatusResponse } from '../../services/api.service';
import { EventStreamService, ConnectionStatus } from '../../services/event-stream.service';

@Component({
  standalone: true,
  selector: 'app-config',
  templateUrl: './config.html',
  styleUrl: './config.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [Button, DecimalPipe, FormsModule, InputText, Tag, DatePicker],
})
export class ConfigComponent implements OnInit, OnDestroy {
  private readonly api = inject(ApiService);
  private readonly eventStream = inject(EventStreamService);

  protected readonly status = signal<StatusResponse | null>(null);
  protected readonly logDirInput = signal('');
  protected readonly minDateValue = signal<Date | null>(null);
  protected readonly idleTimeoutValue = signal<number>(30);
  protected readonly saving = signal(false);
  protected readonly savingMinDate = signal(false);
  protected readonly savingIdleTimeout = signal(false);
  protected readonly saveError = signal('');
  protected readonly saveMinDateError = signal('');
  protected readonly saveIdleTimeoutError = signal('');
  protected readonly flushingBuffer = signal(false);
  protected readonly flushBufferMsg = signal('');
  protected readonly presets = signal<{ label: string; path: string }[]>([]);

  protected readonly connectionStatus = toSignal(this.eventStream.status$, {
    initialValue: 'disconnected' as ConnectionStatus,
  });

  protected readonly statusSeverity = toSignal(
    this.eventStream.status$.pipe(
      map((s): 'success' | 'danger' | 'warn' => {
        if (s === 'connected') return 'success';
        if (s === 'reconnecting') return 'warn';
        return 'danger';
      }),
    ),
    { initialValue: 'danger' as 'success' | 'danger' | 'warn' },
  );

  private pollSub?: Subscription;

  ngOnInit(): void {
    this.pollSub = interval(5000)
      .pipe(startWith(0), switchMap(() => this.api.getStatus()))
      .subscribe({
        next: s => {
          this.status.set(s);
          if (!this.saving()) {
            this.logDirInput.set(s.logDir);
          }
          if (!this.savingMinDate()) {
            if (s.minDate) {
              this.minDateValue.set(new Date(s.minDate));
            } else {
              // Default: Today - 14 days
              const d = new Date();
              d.setDate(d.getDate() - 14);
              d.setHours(0, 0, 0, 0);
              this.minDateValue.set(d);
            }
          }
          if (!this.savingIdleTimeout()) {
            this.idleTimeoutValue.set(s.idleTimeoutSeconds);
          }
        },
      });

    this.api.getLogDirPresets().subscribe({ next: p => this.presets.set(p) });
  }

  ngOnDestroy(): void {
    this.pollSub?.unsubscribe();
  }

  protected usePreset(path: string): void {
    this.logDirInput.set(path);
  }

  protected saveLogDir(): void {
    const dir = this.logDirInput().trim();
    if (!dir) return;
    this.saving.set(true);
    this.saveError.set('');
    this.api.setLogDir(dir).subscribe({
      next: s => {
        this.status.set(s);
        this.saving.set(false);
      },
      error: () => {
        this.saveError.set('Failed to update log directory.');
        this.saving.set(false);
      },
    });
  }

  protected setQuickMinDate(days: number | null): void {
    if (days === null) {
      // "All time" — clear the filter entirely (no minimum date).
      this.minDateValue.set(null);
    } else {
      const d = new Date();
      d.setDate(d.getDate() - days);
      d.setHours(0, 0, 0, 0);
      this.minDateValue.set(d);
    }
  }

  protected saveMinDate(): void {
    const date = this.minDateValue();
    this.savingMinDate.set(true);
    this.saveMinDateError.set('');
    // An empty string clears the filter (parse all logs regardless of date).
    this.api.setMinDate(date ? date.toISOString() : '').subscribe({
      next: s => {
        this.status.set(s);
        this.savingMinDate.set(false);
      },
      error: () => {
        this.saveMinDateError.set('Failed to update start date.');
        this.savingMinDate.set(false);
      },
    });
  }

  protected setQuickIdleTimeout(seconds: number): void {
    this.idleTimeoutValue.set(seconds);
  }

  protected saveIdleTimeout(): void {
    let seconds = Math.round(this.idleTimeoutValue());
    if (!Number.isFinite(seconds) || seconds < 0) {
      seconds = 0;
    }
    this.savingIdleTimeout.set(true);
    this.saveIdleTimeoutError.set('');
    this.api.setIdleTimeout(seconds).subscribe({
      next: s => {
        this.status.set(s);
        this.idleTimeoutValue.set(s.idleTimeoutSeconds);
        this.savingIdleTimeout.set(false);
      },
      error: () => {
        this.saveIdleTimeoutError.set('Failed to update auto-shutdown timeout.');
        this.savingIdleTimeout.set(false);
      },
    });
  }

  protected flushBuffer(): void {
    this.flushingBuffer.set(true);
    this.flushBufferMsg.set('');
    this.api.flushEvents().subscribe({
      next: r => {
        this.status.set(r.status);
        this.flushBufferMsg.set(`Wrote ${r.flushed.toLocaleString()} event(s) to the database.`);
        this.flushingBuffer.set(false);
      },
      error: () => {
        this.flushBufferMsg.set('Failed to flush buffered events.');
        this.flushingBuffer.set(false);
      },
    });
  }
}
