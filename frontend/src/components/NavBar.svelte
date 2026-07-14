<script lang="ts">
  import { Play, Square, SkipForward, Settings, LogIn, LogOut, User, RefreshCw } from '@lucide/svelte';
  import { i18n } from '../i18n.svelte';

  // Svelte 5 Runes for Props
  let {
    currentChannel = $bindable(''),
    autoMode = $bindable(false),
    isLoggedIn = false,
    userDisplayName = '',
    currentChannelDisplayName = '',
    timeRemaining = 0,
    onSkip = () => {},
    onOpenSettings = () => {},
    onLogin = () => {},
    onLogout = () => {}
  } = $props();

  // Local input value
  let inputVal = $state(currentChannel);
  let isFocused = $state(false);

  // Keep input field updated when channel changes from outside (e.g., auto mode)
  $effect(() => {
    inputVal = currentChannel;
  });

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      const trimmed = inputVal.trim();
      if (trimmed) {
        currentChannel = trimmed;
        // Manual change stops auto mode
        autoMode = false;
      }
    }
  }

  function toggleAuto() {
    autoMode = !autoMode;
  }
</script>

<header class="flex items-center justify-between h-14 px-4 bg-surface-900 border-b border-surface-700 select-none">
  
  <!-- Left: Action Buttons -->
  <div class="flex items-center gap-2">
    <!-- Auto Mode Button -->
    <button
      onclick={toggleAuto}
      disabled={!isLoggedIn}
      class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold tracking-wider transition-all duration-200 shadow-sm
        {autoMode 
          ? 'bg-rose-600 hover:bg-rose-500 text-white shadow-rose-900/30' 
          : 'bg-surface-700 hover:bg-surface-600 text-surface-200'}
        disabled:opacity-45 disabled:cursor-not-allowed disabled:pointer-events-none"
      title={autoMode ? i18n.t('titleAutoStop') : i18n.t('titleAutoStart')}
    >
      {#if autoMode}
        <Square size={14} fill="currentColor" />
        <span>{i18n.t('autoStop')}</span>
      {:else}
        <Play size={14} fill="currentColor" />
        <span>{i18n.t('autoStart')}</span>
      {/if}
    </button>

    <!-- Skip Button -->
    <button
      onclick={onSkip}
      disabled={!isLoggedIn}
      class="p-2 rounded-lg bg-surface-800 text-surface-200 hover:bg-surface-700 transition-colors duration-200 disabled:opacity-45 disabled:cursor-not-allowed disabled:pointer-events-none"
      title={i18n.t('titleSkip')}
    >
      <SkipForward size={16} />
    </button>

  </div>

  <!-- Center: URL / Channel Input Area -->
  <div class="flex-1 max-w-xl mx-6">
    <div class="relative flex items-center">
      <span class="absolute left-3 text-xs font-semibold tracking-wider text-surface-400">twitch.tv/</span>
      <input
        type="text"
        bind:value={inputVal}
        onkeydown={handleKeydown}
        onfocus={() => isFocused = true}
        onblur={() => isFocused = false}
        disabled={!isLoggedIn}
        style="padding-right: {!isFocused && isLoggedIn && currentChannel ? (autoMode ? '11rem' : '6rem') : '1rem'}"
        placeholder={isLoggedIn ? i18n.t('placeholderInput') : i18n.t('placeholderLoginNeeded')}
        class="w-full pl-20 py-1.5 text-sm text-white bg-surface-950 border border-surface-700 rounded-lg focus:outline-none focus:ring-1 focus:ring-primary-500 focus:border-transparent transition-all font-mono disabled:opacity-45 disabled:cursor-not-allowed"
      />
      
      <!-- Integrated channel details (Display Name & Auto Countdown HUD) -->
      {#if !isFocused && isLoggedIn && currentChannel}
        <div class="absolute right-3 top-1/2 -translate-y-1/2 flex items-center gap-3 text-xs pointer-events-none select-none">
          <!-- Display Name (日本語など) -->
          {#if currentChannelDisplayName}
            <span class="text-surface-400 font-sans">({currentChannelDisplayName})</span>
          {/if}
 
          <!-- Auto Mode Timer HUD -->
          {#if autoMode}
            <div class="flex items-center gap-1.5 px-2 py-0.5 bg-rose-950/40 border border-rose-800/40 rounded text-rose-400 font-mono">
              <SkipForward size={12} class="animate-pulse" />
              <span>{Math.floor(timeRemaining / 60)}:{(timeRemaining % 60).toString().padStart(2, "0")}</span>
            </div>
          {/if}
        </div>
      {/if}
    </div>
  </div>

  <!-- Right: Twitch Login, Settings & Panels Controls -->
  <div class="flex items-center gap-2">
    
    <!-- Twitch Login Section -->
    {#if isLoggedIn}
      <div class="flex items-center gap-2 px-3 py-1 bg-surface-850 border border-surface-700 rounded-lg text-xs text-surface-300">
        <User size={13} class="text-primary-400" />
        <span class="font-medium text-white truncate max-w-[80px]" title={userDisplayName}>{userDisplayName}</span>
        <button 
          onclick={onLogin}
          class="ml-1 p-0.5 text-surface-400 hover:text-primary-400 transition-colors"
          title={i18n.t('titleReconnect')}
        >
          <RefreshCw size={13} />
        </button>
        <button 
          onclick={onLogout}
          class="ml-1 p-0.5 text-surface-400 hover:text-rose-400 transition-colors"
          title={i18n.t('titleLogout')}
        >
          <LogOut size={13} />
        </button>
      </div>
    {:else}
      <button
        onclick={onLogin}
        class="flex items-center gap-1.5 px-3 py-1.5 bg-primary-600 hover:bg-primary-500 text-white rounded-lg text-xs font-semibold shadow-md transition-all duration-200"
        title={i18n.t('titleLogin')}
      >
        <LogIn size={13} />
        <span>{i18n.t('btnLogin')}</span>
      </button>
    {/if}

    <!-- Settings Button -->
    <button
      onclick={onOpenSettings}
      disabled={!isLoggedIn}
      class="p-2 rounded-lg bg-surface-800 text-surface-200 hover:bg-surface-700 hover:text-white transition-all duration-200 disabled:opacity-45 disabled:cursor-not-allowed disabled:pointer-events-none"
      title={i18n.t('titleSettings')}
    >
      <Settings size={16} />
    </button>

  </div>

</header>

<style>
  /* Local Skeleton V4 colors mappings fallback */
  .bg-surface-900 {
    background-color: var(--color-surface-900, #141a26);
  }
  .bg-surface-950 {
    background-color: var(--color-surface-950, #0a0e17);
  }
  .bg-surface-800 {
    background-color: var(--color-surface-800, #242c3d);
  }
  .bg-surface-850 {
    background-color: var(--color-surface-850, #1b212f);
  }
  .bg-surface-700 {
    background-color: var(--color-surface-700, #343e56);
  }
  .bg-surface-700:hover {
    background-color: var(--color-surface-600, #404c6a);
  }
  .bg-surface-800:hover {
    background-color: var(--color-surface-700, #343e56);
  }
  .border-surface-700 {
    border-color: var(--color-surface-700, #343e56);
  }
  .text-surface-400 {
    color: var(--color-surface-400, #94a3b8);
  }
  .text-surface-300 {
    color: var(--color-surface-300, #cbd5e1);
  }
  .text-surface-200 {
    color: var(--color-surface-200, #e2e8f0);
  }
  .bg-primary-600 {
    background-color: var(--color-primary-600, #9333ea);
  }
  .bg-primary-600:hover {
    background-color: var(--color-primary-500, #a855f7);
  }
  .text-primary-400 {
    color: var(--color-primary-400, #c084fc);
  }
</style>
