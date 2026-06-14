import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { LiveStateService } from '../../services/live-state.service';
import { CharacterCardComponent } from '../../components/character-card/character-card';
import { Skeleton } from 'primeng/skeleton';

@Component({
  standalone: true,
  selector: 'app-popout-character',
  template: `
    @if (loading()) {
      <p-skeleton height="400px" />
    } @else if (!state()) {
      <div class="not-found">
        <i class="pi pi-exclamation-circle"></i>
        <h3>Character not found or offline</h3>
        <p>This window will update automatically when the character comes online.</p>
      </div>
    } @else {
      <app-character-card [state]="state()!" [isPopout]="true" />
    }
  `,
  styles: [`
    :host {
      display: block;
      padding: 1rem;
      height: 100vh;
      overflow: auto;
    }
    .not-found {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      height: 100%;
      color: var(--p-text-muted-color);
      text-align: center;
      i { font-size: 3rem; margin-bottom: 1rem; }
    }
  `],
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CharacterCardComponent, Skeleton],
})
export class PopoutCharacterComponent {
  private readonly route = inject(ActivatedRoute);
  private readonly liveStateService = inject(LiveStateService);

  protected readonly loading = this.liveStateService.loading;
  protected readonly charName = computed(() => this.route.snapshot.paramMap.get('name'));

  protected readonly state = computed(() => {
    const name = this.charName();
    if (!name) return null;
    return this.liveStateService.liveState().get(name) ?? null;
  });
}
