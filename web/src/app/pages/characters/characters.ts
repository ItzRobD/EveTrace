import { Component, inject, OnInit, signal } from '@angular/core';
import { DatePipe } from '@angular/common';
import { RouterLink } from '@angular/router';
import {
  Accordion,
  AccordionContent,
  AccordionHeader,
  AccordionPanel,
} from 'primeng/accordion';
import { Button } from 'primeng/button';
import { ConfirmDialog } from 'primeng/confirmdialog';
import { Skeleton } from 'primeng/skeleton';
import { TableModule } from 'primeng/table';
import { Tag } from 'primeng/tag';
import { ConfirmationService } from 'primeng/api';
import { Character } from '../../models/character.model';
import { Session } from '../../models/session.model';
import { ApiService } from '../../services/api.service';

@Component({
  standalone: true,
  selector: 'app-characters',
  templateUrl: './characters.html',
  styleUrl: './characters.scss',
  imports: [
    Accordion,
    AccordionPanel,
    AccordionHeader,
    AccordionContent,
    Button,
    ConfirmDialog,
    DatePipe,
    RouterLink,
    Skeleton,
    TableModule,
    Tag,
  ],
  providers: [ConfirmationService],
})
export class CharactersComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly confirm = inject(ConfirmationService);

  protected readonly characters = signal<Character[]>([]);
  protected readonly loading = signal(true);
  protected readonly sessionsMap = signal(new Map<number, Session[]>());
  protected readonly loadingSessionIds = signal(new Set<number>());

  ngOnInit(): void {
    this.loadCharacters();
  }

  private loadCharacters(): void {
    this.loading.set(true);
    this.api.getCharacters().subscribe({
      next: chars => {
        this.characters.set(chars);
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }

  protected loadSessionsIfNeeded(characterId: number): void {
    if (this.sessionsMap().has(characterId) || this.loadingSessionIds().has(characterId)) {
      return;
    }
    this.loadingSessionIds.update(s => new Set(s).add(characterId));
    this.api.getCharacterSessions(characterId).subscribe({
      next: sessions => {
        this.sessionsMap.update(m => new Map(m).set(characterId, sessions));
        this.loadingSessionIds.update(s => {
          const next = new Set(s);
          next.delete(characterId);
          return next;
        });
      },
      error: () => {
        this.loadingSessionIds.update(s => {
          const next = new Set(s);
          next.delete(characterId);
          return next;
        });
      },
    });
  }

  protected confirmDeleteCharacter(event: MouseEvent, character: Character): void {
    event.stopPropagation();
    this.confirm.confirm({
      message: `Delete "${character.Name}" and all their data? This cannot be undone.`,
      header: 'Delete Character',
      icon: 'pi pi-exclamation-triangle',
      acceptLabel: 'Delete',
      rejectLabel: 'Cancel',
      accept: () => {
        this.api.deleteCharacter(character.ID).subscribe(() => {
          this.characters.update(c => c.filter(ch => ch.ID !== character.ID));
        });
      },
    });
  }

  protected confirmDeleteSession(session: Session, characterId: number): void {
    this.confirm.confirm({
      message: `Delete session "${session.SessionKey}"? All events will be lost.`,
      header: 'Delete Session',
      icon: 'pi pi-exclamation-triangle',
      acceptLabel: 'Delete',
      rejectLabel: 'Cancel',
      accept: () => {
        this.api.deleteSession(session.ID).subscribe(() => {
          this.sessionsMap.update(m => {
            const updated = new Map(m);
            updated.set(
              characterId,
              (updated.get(characterId) ?? []).filter(s => s.ID !== session.ID),
            );
            return updated;
          });
        });
      },
    });
  }

  protected getSessions(characterId: number): Session[] {
    return this.sessionsMap().get(characterId) ?? [];
  }

  protected isLoadingSessions(characterId: number): boolean {
    return this.loadingSessionIds().has(characterId);
  }
}
