# bongocat

A lightweight BongoCat desktop widget for Linux/X11. The cat sits in a corner of your screen and taps its paws in time with your keyboard and mouse clicks.

## Inspiration

Ported from [BongoCat-mac](https://github.com/Gamma-Software/BongoCat-mac) by Gamma-Software. Original concept from the BongoCat meme.

## Features

- **Keyboard tracking** — left-side keys trigger the left paw, right-side keys trigger the right paw
- **Mouse tracking** — left click triggers the left paw, right click triggers the right paw
- **Per-pixel alpha transparency** — the window blends into your desktop (requires a compositing manager such as picom or compton)
- **Click-through** — by default the window passes all clicks to whatever is underneath it
- **Draggable and resizable** — enable "Enable Move and Resize" from the system tray icon; drag the window to reposition it or drag a corner to resize (aspect ratio locked)
- **System tray icon** — right-click the tray icon to toggle move/resize mode or quit

## Getting Started

### Requirements

- Linux with X11 (not Wayland)
- A compositing manager running (e.g. picom) for alpha transparency
- `libayatana-appindicator3-dev` (system tray support)

```
sudo apt-get install libayatana-appindicator3-dev
```

### Install

The install script builds from source, installs the binary to `~/.local/bin`, adds an application icon, and registers a desktop entry so BongoCat appears in your launcher.

```
git clone https://github.com/randspace0/bongocat
cd bongocat
./install.sh
```

For a system-wide install to `/usr/local`:

```
sudo ./install.sh --system
```

To uninstall:

```
./install.sh --uninstall
```

### Build and run manually

Requires Go 1.21+.

```
go build -o bongocat .
./bongocat
```

### Usage

| Action | Effect |
|--------|--------|
| Type on keyboard | Paws tap left / right |
| Click mouse | Paws tap left / right |
| Tray → Enable Move and Resize | Unlocks drag and resize |
| Drag window (non-corner) | Repositions the cat |
| Drag a corner | Resizes the window (ratio locked) |
| Tray → Quit | Exits |

> **Note:** XRecord requires that no other application is holding an exclusive XRecord context. If the cat does not react to input, check that no other input-monitoring tool is blocking XRecord.

## Disclaimer

Significant portions of this project — including the pure-Go X11 window management, XRecord input monitoring, and sprite rendering pipeline — were implemented with the assistance of [Claude Code](https://claude.ai/code) (Anthropic). The human author directed the architecture, reviewed all generated code, and is responsible for the final result.

## License

```
DO WHAT THE FUCK YOU WANT TO PUBLIC LICENSE
Version 2, December 2004

Copyright (C) 2004 Sam Hocevar <sam@hocevar.net>

Everyone is permitted to copy and distribute verbatim or modified
copies of this license document, and changing it is allowed as long
as the name is changed.

DO WHAT THE FUCK YOU WANT TO PUBLIC LICENSE
TERMS AND CONDITIONS FOR COPYING, DISTRIBUTION AND MODIFICATION

0. You just DO WHAT THE FUCK YOU WANT TO.
```
