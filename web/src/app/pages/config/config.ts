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
import { Tag } from 'primeng/tag';
import { ApiService } from '../../services/api.service';
import { EventStreamService, ConnectionStatus } from '../../services/event-stream.service';

interface Status {
  logDir: string;
  eventsProcessed: number;
  sessionsOpened: number;
  wsClients: number;
}

@Component({
  standalone: true,
  selector: 'app-config',
  templateUrl: './config.html',
  styleUrl: './config.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [Button, DecimalPipe, FormsModule, InputText, Tag],
})
export class ConfigComponent implements OnInit, OnDestroy {
  private readonly api = inject(ApiService);
  private readonly eventStream = inject(EventStreamService);

  protected readonly status = signal<Status | null>(null);
  protected readonly logDirInput = signal('');
  protected readonly saving = signal(false);
  protected readonly saveError = signal('');
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
}
