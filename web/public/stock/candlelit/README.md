# Candlelit Oils (`candlelit`)

The default art pack: painted dark-fantasy tabletop illustration, rich
oil-painting brushwork, warm candlelight and deep shadows, a muted earthy palette
with touches of gold.

The style prompt and pack list live in `internal/domain/art.go` (`ArtPacks`).
`make stock-list` shows what is painted; `make stock-paint` fills the gaps.

- `scenes/<scene-key>.webp`: 1536x1024 backdrops, one per `domain.SceneKeys`
- `battlemaps/<scene-key>.webp`: 1536x1024 top-down floors for the 12x8 combat
  grid (3:2), no grid drawn in, scenery at the edges and an open middle
- `npcs/<type>.webp`: 1024x1024, one per `domain.NPCTypes` (innkeeper, guard…)
- `monsters/<monster-key>.webp`: 1024x1024, one per SRD monster type
- `portraits/<race>-<class>.webp`: 1024x1024, one per race and class

Seeded from earlier custom paintings: scenes tavern, road, temple, crypt;
monsters cultist, goblin, zombie; portraits human-wizard, halfling-rogue,
dwarf-cleric, human-barbarian (painted for specific heroes).
