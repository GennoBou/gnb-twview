[日本語版 (Japanese)](./README_ja.md)

# gnb-twview

A specialized desktop and mobile browser designed for efficient Twitch viewing. 

## Overview

`gnb-twview` is a lightweight browser dedicated strictly to watching Twitch streams. It strips away all generic browser features to provide a distraction-free, automated, and streamlined viewing experience.

### Key Features (v0.1.0 Plan)
- **Auto Mode**: Automatically rotates through live followed streamers every X minutes based on a smart queue system.
- **Skip Button**: Instantly jump to the next streamer in the queue.
- **Minimalist UI**: Simple top navigation bar with stream rotation controls, URL field, and settings.
- **Multi-Window Integration**: Consists of a main control window, an overlaid Twitch stream window (`TwitchWindow`) utilizing custom CSS injection, and a dedicated popup window for settings to prevent rendering lag.
- **Integrated Twitch View**: A clean fullscreen display of the official Twitch stream and chat.
- **Custom Injections (CSS/HTML/JS)**: Advanced customization settings allowing users to inject custom CSS styles, HTML elements, and JavaScript logic directly into the Twitch window, complete with real-time syntax validation.
- **Multi-language GUI**: Built-in support for both English and Japanese. Automatically detects the system locale on first startup and dynamically switches interface language in real-time when configured in Settings.

## Tech Stack
- **Framework**: [Wails v3](https://v3.wails.io/)
- **Frontend**: Svelte v5 + Skeleton v4 + TailwindCSS (for Skeleton v4 integration)
- **Backend**: Go
- **Target Platforms**: Windows (Primary), Android (Tablet/Mobile)

## Repository Structure
- `/README.md` - English version (this file)
- `/README_ja.md` - Japanese version
- `/docs/` - Public documentation
- `/docs/local/` - Local scratchpad & AI thinking processes (Git-ignored)

## Documentation Policy
This project enforces dual-language documentation (English and Japanese) with cross-links. Please refer to [.cursorrules](./.cursorrules) for more information.
