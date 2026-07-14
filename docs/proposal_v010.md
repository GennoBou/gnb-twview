[日本語版 (Japanese)](./proposal_v010_ja.md)

# gnb-twview v0.1.0 Development Plan & Technical Analysis Report

This document outlines the technical research, algorithm analysis, and concrete implementation proposals for the Twitch-specific browser `gnb-twview` (v0.1.0).

---

## 1. Twitch Embedded Player and `parent` Parameter Specifications

The Twitch Embedded Player (`player.twitch.tv`) requires the `parent` parameter to specify the host name (domain) of the embedding parent page for security reasons (e.g., clickjacking prevention).

### Behavior and Workaround in Wails v3
- **Windows (WebView2)**: The Wails v3 asset server hosts the frontend on `localhost` or `wails.localhost` by default.
  - Example: `https://player.twitch.tv/?channel=<username>&parent=localhost`
  - By pinning the Wails asset server hostname to `localhost`, this parameter works reliably.
- **Android (WebView)**: Loading local assets via the `file://` scheme results in a `null` Origin, causing the Twitch player to reject playback.
  - **Workaround**: Host the frontend on a virtual domain (such as `localhost` or `appassets.androidplatform.net`) using Android's `WebViewAssetLoader` (or Wails v3's Android asset serving). Passing this hostname to `parent` resolves the error.

---

## 2. Mute Function (Abandoned Implementation)

We initially planned to implement a dedicated mute button that would control the browser's audio output directly. However, we have decided to abandon this feature and leave audio management to Twitch's native controls.

### Rationale
Since the application rendering has shifted to directly display official `twitch.tv` pages, the player's mute state is natively maintained and stored in `localStorage` by Twitch itself (via the `video-muted` key). Thus, implementing a separate, custom mute function in the application wrapper is redundant and unnecessary. All volume and muting actions are managed natively within the Twitch player.

---

## 3. Twitch API, OAuth Login, and Credential Management

Handling user authentication and Client ID management securely.

### Authentication Flow (OAuth 2.0 PKCE)
- **Client ID & Secret Management**:
  - The `Client ID` can be public, but the `Client Secret` must never be hardcoded in the binary or exposed in a public repository.
  - Therefore, we will use **OAuth 2.0 Authorization Code Flow with PKCE** to ensure secure authentication without requiring a Client Secret (the previously considered Implicit Grant was discarded due to security deprecations).
- **Login Flow**:
  1. The app generates a temporary `code_verifier` and its corresponding `code_challenge`.
  2. Open the Twitch auth URL (`https://id.twitch.tv/oauth2/authorize`) in the default system browser, passing parameters including the `code_challenge` and `code_challenge_method=S256`.
  3. Spin up a temporary local HTTP server (e.g., `http://localhost:8520/callback`) within the Wails app to catch the redirected authorization `code`.
  4. Send a POST request directly from the Go backend to Twitch's token endpoint to exchange the authorization `code` and the `code_verifier` for an Access Token.
  5. Stop the local server and store the token securely.
- **Secure Token Storage**:
  - Currently, tokens are saved to `~/.gnb-twview/token.json` with restricted permissions. In the future, we plan to leverage platform-native secure storage, such as `Windows Credential Manager` for Windows and `EncryptedSharedPreferences` for Android.

---

## 4. Proposed Auto Mode Rotation Algorithm

To solve the issue where the front of the queue is dominated by streamers below the "initial watch target," we propose a "Smart Round-Robin" queue algorithm.

### Smart Round-Robin Algorithm
Track viewing history as "total watched seconds" and construct the queue with the following rules:

1. **State variables**:
   - `TargetTime` (Initial watch target, fixed at 6 minutes)
   - `TotalWatchedTime` per streamer (Accumulated viewing seconds, restored from file if the last watched time is within 3 hours on startup, otherwise reset to 0)
   - `LastWatchedAt` per streamer (Tracked individually and cleaned up after 3 hours of inactivity)
2. **Priority Sorting (Queue Construction)**:
   - Every time the live streamer list is updated, sort the queue using these priorities:
     - **Priority 1**: Streamers with `TotalWatchedTime < TargetTime` (Goal Unachieved Group)
       - Sort by `TotalWatchedTime` in **ascending order** (prioritizing streamers with the least watch time).
     - **Priority 2**: Streamers with `TotalWatchedTime >= TargetTime` (Goal Achieved Group)
       - Sort by `TotalWatchedTime` in ascending order or randomize.
3. **Dynamic Stay Duration (Rotation Timer)**:
   - When viewing a streamer:
     - If the streamer is **Goal Unachieved** (`TotalWatchedTime < TargetTime`): Stay for the remaining duration, calculated as **`TargetTime (6 minutes) - TotalWatchedTime`**.
     - If the streamer is **Goal Achieved** (`TotalWatchedTime >= TargetTime`): Stay for **Rotation Time** (3 minutes by default, customizable).
4. **Benefits**:
   - Streamers who haven't been watched yet are prioritized fairly, avoiding rotation bias.
   - Even if the channel is skipped or the application is restarted, the previously watched duration is preserved and the timer resumes from where it left off, dramatically improving rotation efficiency.
   - Once all active streamers have been watched for at least 6 minutes, the app automatically transitions to a faster rotation cycle (customizable, default 3 minutes).

---

## 5. Stream Rendering Method (CSS Injection vs `player.twitch.tv`)

### Comparison and Final Design Decision
- **Option 1: `player.twitch.tv` (Embedded Player) [Discarded]**
  - **Cons**: Embedding streams blocks streak points and channel points acquisition. Furthermore, users cannot easily log into the embedded frame since Twitch limits sign-ins inside third-party embedding frames, requiring complex session sharing.
- **Option 2 (Selected): `twitch.tv/<channel>` with Custom CSS Injection (TwitchWindow)**
  - **Pros**: Since it loads the official Twitch website directly, users can obtain viewing streak points and channel points natively. Users can easily log in using Twitch's native header controls, and the session is automatically shared.
  - **Mitigating Cons (Hiding Elements)**: We resolved layout breakage and maintenance issues by externalizing CSS into `/assets/inject.css` for dynamic loading, and implementing a **self-healing CSS injection** script (utilizing `MutationObserver` and `setInterval`) to prevent SPA/React DOM rerenders from wiping the injected styles. Additionally, a Wails ready-signal spoofing hack (`HandleMessage("wails:runtime:ready")`) is used to bypass the Wails v3 IPC execution boundary.
  - **Multi-Window Design (Settings Window Separation)**: To completely eliminate rendering lag and flickering on the main Twitch window during settings toggling, the settings screen is separated into its own borderless window (`settingsWindow`). It loads the same Svelte frontend with query parameters (`/?mode=settings`) and stays on top (`AlwaysOnTop: true`), ensuring the main Twitch window remains untouched during interaction.

---

## 6. Cross-Platform Strategy (Wails v3 & Android)

While Wails v3 has mobile support, compiling identical Go code across desktop and Android remains complex due to platform-specific WebView differences.

### Recommended Strategy
1. **Frontend-Heavy Logic**:
   - Implement core logic (queue management, timers, Twitch API requests, OAuth) within the **Svelte (TypeScript) layer**.
   - Keep the Go layer thin, focusing on Windows-specific window controls.
2. **Android Porting**:
   - After releasing the Windows version (v0.1.0), reuse the Svelte codebase and wrap it using **Capacitor** or **Tauri v2** for Android.
   - This ensures rapid development without debugging low-level Android Go bindings.
