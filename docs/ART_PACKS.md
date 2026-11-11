# Art packs

An **art pack** is a complete set of illustrations for the game, painted once in
one style and shipped with the server. With a pack in place, play needs no image
generation at all: no waiting, and no image cost.

The game ships one pack, **Candlelit Oils** (`candlelit`): dark-fantasy oil
painting by candlelight. This guide covers generating a pack, making one in your
own style, and using hand-made or bought art instead.

## What's in a pack

A pack is a folder, `web/public/stock/<pack>/`, with 129 images:

| Folder | Count | Size | One per | Used for |
|---|---|---|---|---|
| `scenes/<scene>.webp` | 14 | 1536×1024 | scene key (`tavern`, `forest`, `crypt`…) | the backdrop above the story |
| `battlemaps/<scene>.webp` | 14 | 1536×1024 | scene key | the floor of the 12×8 combat grid, seen from above |
| `monsters/<monster>.webp` | 18 | 1024×1024 | monster type (`goblin`, `giant-rat`…) | monster tokens and the Foes list |
| `portraits/<race>-<class>.webp` | 63 | 1024×1024 | race and class (`elf-wizard`, `half-orc-bard`…) | hero portraits and tokens |
| `npcs/<type>.webp` | 20 | 1024×1024 | person type (`innkeeper`, `guard`, `priest`…) | NPCs the DM puts on the table; creature NPCs (a kobold guide) use the monster portrait |

`make stock-list` prints every file a pack needs and whether it's there yet.

The list comes from the game's own data, so it grows with it. A new scene key,
monster, race or class adds an entry, and the next `make stock-paint` paints just
that one.

## How the game uses it

`-art` picks the mode (env `TABLETOP_ART`):

| Mode | What players see | Image cost in play |
|---|---|---|
| `stock` (default) | the pack's images | none |
| `paint` | custom paintings (one per hero, monster type and scene, on each player's OpenAI key); the pack until each is ready | per new hero, monster type and place |
| `off` | the built-in drawings only | none |

The game looks for each picture in this order:

1. A custom painting of that exact subject (for example, a portrait painted for
   your hero in `paint` mode).
2. The pack's image for its kind: `portraits/dwarf-cleric` for any dwarf cleric,
   `scenes/crypt` for any crypt.
3. The built-in drawings.

So a missing file never breaks anything. It just shows the drawing.

`-art-pack <key>` (env `TABLETOP_ART_PACK`) picks which pack to show. The images
are built into the binary when you run `make web`.

## Generating a pack

Painting uses OpenAI's image model (`-image-model`, default `gpt-image-2`). The
key comes from `OPENAI_API_KEY`, or else from the OpenAI key the `local`
account saved in Settings (`-stock-account` picks another account). No player
is billed: a pack is painted once, by whoever builds the server.

```sh
make stock-list                                   # what's painted, what's missing
make stock-paint KINDS=scenes LIMIT=2             # a small sample: check the style first
make stock-paint KINDS=scenes,battlemaps          # then a kind at a time
make stock-paint KINDS=monsters
make stock-paint KINDS=portraits
make stock-paint                                  # or everything still missing
make web && make build                            # bundle the images into the binary
```

| Option | Make variable | Flag | Default |
|---|---|---|---|
| Pack | `PACK=` | `-art-pack` | `candlelit` |
| Kinds | `KINDS=` (`scenes,battlemaps,monsters,portraits,npcs` or `all`) | `-paint-stock` | `all` |
| Cap per run | `LIMIT=` | `-stock-limit` | `0` (no cap) |
| Quality | (the target uses `medium`) | `-image-quality` (`low`, `medium`, `high`) | `medium` |
| Parallel paintings | | `-art-workers` | `2` |
| Output folder | | `-stock-dir` | `web/public/stock` |

- **A run only paints what's missing.** Stop it with Ctrl-C and run it again to
  carry on.
- **To repaint one image,** delete its file and run again.
- **A failed image is logged and skipped.** Look for `stock art failed` with its
  key; the rest of the run carries on.
- **Cost** scales with the number of images and the quality: 129 images per pack
  at your chosen quality. Paint a sample at `LIMIT=2` first, and use
  `-image-quality low` while experimenting with a style.

## Making your own style

A style is one entry in `ArtPacks` in `internal/domain/art.go`:

```go
var ArtPacks = []ArtPack{
	{Key: "candlelit", Name: "Candlelit Oils", Style: ArtStyle},
	{Key: "inkwash", Name: "Ink and Wash", Style: "Style: loose black ink linework with " +
		"muted watercolour washes on cream paper, like a hand-illustrated field journal. " +
		"No text, lettering, captions, watermark, border or frame."},
}
```

- **Key:** the folder name and the `-art-pack` value (lowercase, dashes).
- **Name:** shown to people.
- **Style:** added to the end of every prompt in the pack.

Then:

```sh
make stock-paint PACK=inkwash KINDS=scenes LIMIT=2       # try it on two scenes
open web/public/stock/inkwash/scenes/
# tweak Style, delete the samples, repeat until you like it
make stock-paint PACK=inkwash                            # paint the whole pack
make web && make build && ./bin/tabletop -local -art-pack inkwash
```

### Writing a good style prompt

- **Describe the medium, the light and the palette,** in that order. For
  example: "gouache on toned paper, flat afternoon light, a limited palette of
  ochre, slate blue and brick red".
- **Keep the last sentence** ("No text, lettering, captions, watermark, border
  or frame."). Image models love adding titles and frames.
- **Avoid naming living artists or copyrighted franchises.** Describe the look
  instead.
- **Test on varied subjects:** a bright scene (`village`), a dark one (`crypt`),
  a battle map, a monster and a portrait. A style that flatters one can fail
  another.
- **Battle maps sit under tokens and a grid,** so a style with deep shadows can
  make them hard to read. The battle-map prompt already asks for even lighting.
  If yours still come out dark, say so in the style ("bright, even daylight").

Some ideas to start from:

| Look | Style line |
|---|---|
| Ink and wash | loose black ink linework with muted watercolour washes on cream paper |
| Woodcut | bold black-and-white woodcut print, heavy carved lines, a single red accent colour |
| Storybook | soft gouache storybook illustration, warm daylight, gentle rounded shapes |
| Grimdark | desaturated, gritty charcoal and oil, cold moonlight, heavy texture |
| Pixel art | detailed 32-bit pixel art, limited palette, crisp edges, no anti-aliasing |

### What each prompt says

The subject half of each prompt is built in `internal/domain/art.go` and shared
by every pack:

| Kind | Built by | Edit to change |
|---|---|---|
| Scenes | `sceneRequest` with `stockScenes` | one description per scene key |
| Battle maps | `battlemapRequest` with `stockBattlemaps` | one top-down description per scene key; the "open middle, no grid" rules |
| Monsters | `monsterRequest` | one shared template ("close-up portrait of a …") |
| Portraits | `portraitRequest` with `classLook` | gear per class |
| NPCs | `npcRequest` (`internal/domain/npc.go`) | one shared template; the types are `domain.NPCTypes` |

Changes apply to images painted afterwards. Delete the files you want redone.

## Using your own images

You don't have to generate anything. Any image saved under the right name works:

- **Format:** WebP, named exactly as `make stock-list` shows (for example
  `web/public/stock/candlelit/monsters/orc.webp`). Convert other formats with
  any image tool, for example `cwebp -q 85 orc.png -o orc.webp`.
- **Sizes:** scenes and battle maps are 3:2 (1536×1024 is ideal), portraits and
  monsters are square (1024×1024).
- **Battle maps** must be top-down, with no grid drawn in. The game draws its
  own 12×8 grid, so each square is 1/12 of the width.
- Make sure you have the rights to ship the image. Packs are committed to the
  repository and built into the binary.

## Committing a pack

A pack is about 16 MB, which is fine in plain git:

```sh
git add web/public/stock/<pack>
git commit -m "Add the <pack> art pack"
```

Every repaint keeps the old copy in git history. If you plan several packs or
lots of repainting, track the images with Git LFS first:

```sh
git lfs track "web/public/stock/**/*.webp"
```

Each pack folder has a `README.md` describing its style and anything special
about it (for example, which images were hand-picked).

## Troubleshooting

| Symptom | Fix |
|---|---|
| `moderation_blocked` (OpenAI's safety filter) | Run again; the filter is not consistent. If it keeps failing, soften the subject's wording (for example, "fierce" instead of "menacing") or drop in your own image. |
| A grid, frame or text appears in the image | Repaint it. If it keeps happening, make the style's "No text… border or frame" line stronger. |
| Battle maps too dark under tokens | Add "bright, even lighting" to the pack's style, delete them, repaint. |
| A portrait is reused for heroes it doesn't suit | That's how stock works: one face per race and class. Use `-art paint` for a portrait per hero. |
| New images don't appear | Run `make web && make build`; the binary carries the images. |

## Where this is heading

Packs are a natural thing for players to make and share. The plan for that is
in [ROADMAP.md](ROADMAP.md) under "Player-made art packs".
