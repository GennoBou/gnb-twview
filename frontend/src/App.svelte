<script lang="ts">
  import { onMount } from "svelte";
  import NavBar from "./components/NavBar.svelte";
  import SettingsModal from "./components/SettingsModal.svelte";
  import DeviceAuthModal from "./components/DeviceAuthModal.svelte";
  import { Events } from "@wailsio/runtime";
  import * as TwitchService from "../bindings/github.com/GennoBou/gnb-twview/backend/twitchservice";
  import { i18n } from "./i18n.svelte";

  // TypeScript declaration for Twitch SDK global
  declare let Twitch: any;

  // --- Reactive States (Svelte 5 Runes) ---
  let currentChannel = $state(""); // Default initial channel
  let autoMode = $state(false);
  let isSettingsWindowMode = $state(false);
  let hostname = $state("localhost");
  let hasAutoStarted = $state(false);
  let debugOrigin = $state("");
  let debugHostname = $state("");

  // Device Auth State
  let showDeviceAuth = $state(false);
  let authUserCode = $state("");
  let authVerificationUri = $state("");
  let authErrorMsg = $state("");

  // Twitch Authentication State
  let isLoggedIn = $state(false);
  let userDisplayName = $state("");

  // Derived rune to compute the display name of the currently watched channel
  let currentChannelDisplayName = $derived(
    liveStreamers.find((s) => s.user_login === currentChannel)?.user_name || ""
  );

  // Auto mode settings & queue state
  let initialWatchTime = $state(600); // in seconds
  let rotationTime = $state(300); // in seconds
  let autoStartOnLogin = $state(true);
  let language = $state("en");
  let timeRemaining = $state(600); // Countdown
  let activeQueue = $state<string[]>([]);
  let liveStreamers = $state<any[]>([]);

  // Monitor channel changes to sync Go sub windows
  $effect(() => {
    if (currentChannel && !isSettingsWindowMode) {
      TwitchService.SetCurrentChannel(currentChannel).catch((e) => console.error("Error syncing channel to sub windows:", e));
    }
  });

  // Monitor autoMode changes to toggle Go timer
  $effect(() => {
    if (isSettingsWindowMode) return;
    if (autoMode) {
      // If currentChannel is default 'fps_shaka' or not in liveStreamers, and we have liveStreamers, switch to first one
      if ((currentChannel === "fps_shaka" || !liveStreamers.some((s) => s.user_login === currentChannel)) && liveStreamers.length > 0) {
        currentChannel = liveStreamers[0].user_login;
      }
      TwitchService.StartAutoMode(currentChannel)
        .then((state) => handleAutoStateChange(state))
        .catch((e) => console.error(e));
    } else {
      TwitchService.StopAutoMode().catch((e) => console.error(e));
    }
  });

  // --- Handlers & API wrappers ---

  function handleAutoStateChange(state: any) {
    if (!state) return;
    autoMode = state.auto_mode;
    if (state.current_channel !== undefined && state.current_channel !== "") {
      currentChannel = state.current_channel;
    }
    timeRemaining = state.time_remaining;
    activeQueue = state.queue || [];
  }

  function handleSkip() {
    TwitchService.SkipStreamer()
      .then((state) => handleAutoStateChange(state))
      .catch((e) => console.error(e));
  }

  function handleLogin() {
    authErrorMsg = "";
    TwitchService.Login()
      .then((res) => {
        if (res) {
          authUserCode = res.user_code;
          authVerificationUri = res.verification_uri;
          showDeviceAuth = true;
        }
      })
      .catch((e) => {
        console.error(e);
        authErrorMsg = i18n.t('loginInitFailed');
        showDeviceAuth = true;
      });
  }

  function handleLogout() {
    TwitchService.Logout()
      .then(() => {
        isLoggedIn = false;
        userDisplayName = "";
        currentChannel = "";
        autoMode = false;
        activeQueue = [];
        hasAutoStarted = false;
      })
      .catch((e) => console.error(e));
  }

  function handleSettingsSave() {
    TwitchService.SaveSettings(initialWatchTime, rotationTime, autoStartOnLogin ? 1 : 0, language)
      .then(() => {
        console.log("Settings saved on Go backend");
        if (isSettingsWindowMode) {
          TwitchService.SetSettingsOpen(false);
        }
      })
      .catch((e) => console.error(e));
  }

  // --- Lifecycle Events ---
  onMount(() => {
    hostname = window.location.hostname || "localhost";
    debugOrigin = window.location.origin || "unknown";
    debugHostname = window.location.hostname || "unknown";

    // Detect settings window mode
    const params = new URLSearchParams(window.location.search);
    if (params.get("mode") === "settings") {
      isSettingsWindowMode = true;
    }

    // 1. Get initial settings
    TwitchService.GetSettings()
      .then((settings) => {
        initialWatchTime = settings.initial_watch_time;
        rotationTime = settings.rotation_time;
        autoStartOnLogin = settings.auto_start_on_login === 1;
        language = settings.language || "en";
        i18n.lang = language;

        if (isSettingsWindowMode) {
          return null;
        }

        // 2. Get initial Auth status
        return TwitchService.GetLoginStatus();
      })
      .then((status) => {
        if (!status) return;
        isLoggedIn = status.logged_in;
        userDisplayName = status.display_name || "";

        // Trigger token loading after frontend listeners are fully registered.
        // This guarantees that 'login-status-changed' and 'streamers-updated' events are not lost.
        TwitchService.LoadSavedToken();
      })
      .catch((e) => console.error(e));

    if (isSettingsWindowMode) {
      Events.On("settings-saved", (ev: any) => {
        const data = ev.data;
        initialWatchTime = data.initial_watch_time;
        rotationTime = data.rotation_time;
        autoStartOnLogin = data.auto_start_on_login === 1;
        language = data.language || "en";
        i18n.lang = language;
        console.log("[DEBUG] settings-saved event received, state synchronized.");
      });
      return;
    }

    // 3. Listen to Backend Events
    Events.On("login-status-changed", (ev: any) => {
      const data = ev.data;
      isLoggedIn = data.logged_in;
      userDisplayName = data.display_name || "";
      if (!isLoggedIn) {
        handleLogout();
        hasAutoStarted = false;
      }
    });

    Events.On("settings-saved", (ev: any) => {
      const data = ev.data;
      initialWatchTime = data.initial_watch_time;
      rotationTime = data.rotation_time;
      autoStartOnLogin = data.auto_start_on_login === 1;
      language = data.language || "en";
      i18n.lang = language;
      console.log("[DEBUG] settings-saved event received, state synchronized.");
    });

    Events.On("streamers-updated", (ev: any) => {
      const streamers = ev.data;
      liveStreamers = streamers || [];
      console.log("[DEBUG] streamers-updated received, count:", liveStreamers.length);

      if (autoStartOnLogin && !autoMode && !hasAutoStarted) {
        if (streamers && streamers.length > 0) {
          console.log("[DEBUG] Triggering auto start with first streamer:", streamers[0].user_login);
          hasAutoStarted = true;
          const firstStreamer = streamers[0].user_login;
          currentChannel = firstStreamer;
          setTimeout(() => {
            autoMode = true;
          }, 100);
        } else {
          console.log("[DEBUG] Auto start skipped: No live streamers online.");
        }
      } else {
        console.log("[DEBUG] Auto start skipped. autoStartOnLogin:", autoStartOnLogin, "autoMode:", autoMode, "hasAutoStarted:", hasAutoStarted);
      }
    });

    Events.On("device-auth-status", (ev: any) => {
      const data = ev.data;
      if (data.status === "success") {
        showDeviceAuth = false;
        authErrorMsg = "";
      } else if (data.status === "expired" || data.status === "error") {
        authErrorMsg = data.error || i18n.t('authError');
      }
    });

    Events.On("auto-state-changed", (ev: any) => {
      const state = ev.data;
      handleAutoStateChange(state);
    });

    Events.On("streamer-switched", (ev: any) => {
      const data = ev.data;
      currentChannel = data.channel;
      timeRemaining = data.time_remaining;
    });
  });
</script>

{#if isSettingsWindowMode}
  <!-- Inline Settings Window View -->
  <SettingsModal
    isInline={true}
    bind:initialWatchTime
    bind:rotationTime
    bind:autoStartOnLogin
    bind:language
    onsave={handleSettingsSave}
    onclose={() => TwitchService.SetSettingsOpen(false)}
  />
{:else}
  <div class="flex flex-col h-screen overflow-hidden bg-surface-950 text-white font-sans">
    <!-- Navigation Bar -->
    <NavBar bind:currentChannel bind:autoMode {isLoggedIn} {userDisplayName} {currentChannelDisplayName} {timeRemaining} onSkip={handleSkip} onLogin={handleLogin} onLogout={handleLogout} onOpenSettings={() => TwitchService.SetSettingsOpen(true)} />

    <!-- Main View Area (Remote Control Panel) -->
    <!-- Main View Area (Placeholder Layout for Window Overlay) -->
    <main class="flex-grow flex relative overflow-hidden bg-black">
      {#if isLoggedIn && currentChannel}
        <!-- Stream Player View Placeholder (fills space below navbar) -->
        <div class="flex-1 h-full relative bg-black">
          <!-- Just an empty dark div acting as space placeholder for Go window overlay -->
          <div class="w-full h-full bg-black"></div>
        </div>
      {:else}
        <!-- Twitch Home Page View (pure black area for loading/login dialogs) -->
        <div class="flex-1 h-full w-full bg-black"></div>
      {/if}
    </main>

    <!-- Device Auth Modal Dialog -->
    <DeviceAuthModal
      bind:show={showDeviceAuth}
      userCode={authUserCode}
      verificationUri={authVerificationUri}
      bind:errorMsg={authErrorMsg}
      onClose={() => {
        showDeviceAuth = false;
        authErrorMsg = "";
      }}
    />
  </div>
{/if}

<style>
  /* Layout utilities for dark palette */
  .bg-surface-950 {
    background-color: var(--color-surface-950, #0a0e17);
  }
</style>
