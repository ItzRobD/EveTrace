import {
  ChangeDetectionStrategy,
  Component,
  computed,
  inject,
  signal,
} from '@angular/core';
import { Skeleton } from 'primeng/skeleton';
import { Button } from 'primeng/button';
import { CharacterCardComponent } from '../../components/character-card/character-card';
import { LiveStateService } from '../../services/live-state.service';

@Component({
  standalone: true,
  selector: 'app-live',
  templateUrl: './live.html',
  styleUrl: './live.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [Button, CharacterCardComponent, Skeleton],
})
export class LiveComponent {
  private readonly liveStateService = inject(LiveStateService);

  protected readonly loading = this.liveStateService.loading;
  protected readonly hiddenChars = signal<Set<string>>(new Set());

  protected readonly allChars = computed(() => [...this.liveStateService.liveState().values()]);
  protected readonly activeChars = computed(() => {
    const hidden = this.hiddenChars();
    return this.allChars().filter(s => !hidden.has(s.characterName));
  });
  protected readonly showFilter = computed(() => this.allChars().length > 1);

  protected toggleChar(name: string): void {
    this.hiddenChars.update(set => {
      const next = new Set(set);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  }
}
