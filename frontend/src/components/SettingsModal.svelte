<script lang="ts">
  import { onMount } from 'svelte';
  import { X } from '@lucide/svelte';
  import * as TwitchService from "../../bindings/github.com/GennoBou/gnb-twview/backend/twitchservice";
  import { i18n } from '../i18n.svelte';

  // Svelte 5 Runes for Props
  let {
    show = $bindable(false),
    initialWatchTime = $bindable(600), // in seconds
    rotationTime = $bindable(300),     // in seconds
    autoStartOnLogin = $bindable(true),
    language = $bindable("en"),
    isInline = false,
    onsave = () => {},
    onclose = () => {}
  } = $props();

  // Local state for edits (converted to minutes for user convenience)
  let initialWatchMin = $state(Math.round(initialWatchTime / 60));
  let rotationMin = $state(Math.round(rotationTime / 60));
  let localAutoStart = $state(autoStartOnLogin);
  let localLanguage = $state(language);

  // --- Injection States ---
  let cssText = $state("");
  let htmlText = $state("");
  let jsText = $state("");

  // Accordion Open/Close states
  let cssOpen = $state(false);
  let htmlOpen = $state(false);
  let jsOpen = $state(false);

  // Validation Error states
  let cssError = $state<string | null>(null);
  let htmlError = $state<string | null>(null);
  let jsError = $state<string | null>(null);

  // Derived state to check if any validation error exists
  let hasAnyError = $derived(!!cssError || !!htmlError || !!jsError);

  // Force accordion to stay open if there's an error
  $effect(() => {
    if (cssError) cssOpen = true;
  });
  $effect(() => {
    if (htmlError) htmlOpen = true;
  });
  $effect(() => {
    if (jsError) jsOpen = true;
  });

  // Sync back local state when modal opens or is inline (initialized)
  $effect(() => {
    if (show || isInline) {
      initialWatchMin = Math.round(initialWatchTime / 60);
      rotationMin = Math.round(rotationTime / 60);
      localAutoStart = autoStartOnLogin;
      localLanguage = language;
      i18n.lang = language;
    }
  });

  // Load existing injection texts from Go backend on mount
  onMount(() => {
    TwitchService.GetInjectionTexts()
      .then((texts) => {
        if (texts) {
          cssText = texts.css || "";
          htmlText = texts.html || "";
          jsText = texts.js || "";
          // Perform initial validation
          validateCSS(cssText);
          validateHTML(htmlText);
          validateJS(jsText);
        }
      })
      .catch((e) => console.error("Failed to load injection texts:", e));
  });

  // --- Validation Logics ---

  // JS Validator
  function validateJS(code: string) {
    if (!code.trim()) {
      jsError = null;
      return;
    }
    try {
      new Function(code);
      jsError = null;
    } catch (err: any) {
      const errMsg = err.message || "構文エラーが発生しました。";
      let lineInfo = "位置不明";
      if (err.stack) {
        const match = err.stack.match(/<anonymous>:(\d+):/);
        if (match) {
          lineInfo = `${match[1]}行目付近で発生`;
        }
      }
      jsError = `${errMsg}\n${lineInfo}`;
    }
  }

  // HTML Validator
  function validateHTML(code: string) {
    if (!code.trim()) {
      htmlError = null;
      return;
    }
    try {
      const parser = new DOMParser();
      const doc = parser.parseFromString(code, 'text/html');
      const parserError = doc.querySelector('parsererror');
      if (parserError) {
        const fullMsg = parserError.textContent || "HTMLの構文に問題があります。";
        const cleanMsg = fullMsg.split('\n')[0] || "構文エラー";
        htmlError = `${cleanMsg}\n(HTMLパース時に検出)`;
      } else {
        htmlError = null;
      }
    } catch (err: any) {
      htmlError = `パース失敗: ${err.message || "不明なエラー"}\n(解析エラー)`;
    }
  }

  // CSS Validator
  function validateCSS(code: string) {
    if (!code.trim()) {
      cssError = null;
      return;
    }

    let openBraces = 0;
    let errorFound = false;
    let lastOpenLine = 0;
    const lines = code.split('\n');

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];
      for (let char of line) {
        if (char === '{') {
          openBraces++;
          lastOpenLine = i + 1;
        } else if (char === '}') {
          openBraces--;
          if (openBraces < 0) {
            cssError = `閉じ括弧 '}' が多すぎます。\n${i + 1}行目付近で検出`;
            errorFound = true;
            break;
          }
        }
      }
      if (errorFound) break;
    }

    if (!errorFound) {
      if (openBraces > 0) {
        cssError = `閉じられていない括弧 '{' が存在します。\n${lastOpenLine}行目付近で開始`;
      } else {
        cssError = null;
      }
    }
  }

  // Constant real-time validation via $effect runes
  $effect(() => {
    validateCSS(cssText);
  });
  $effect(() => {
    validateHTML(htmlText);
  });
  $effect(() => {
    validateJS(jsText);
  });

  function handleLanguageChange() {
    i18n.lang = localLanguage;
  }

  function handleSave() {
    if (hasAnyError) return;

    initialWatchTime = initialWatchMin * 60;
    rotationTime = rotationMin * 60;
    autoStartOnLogin = localAutoStart;
    language = localLanguage;
    onsave();

    // Save injection texts to Go backend (will auto-reapply to active subwindow)
    TwitchService.SaveInjectionTexts(cssText, htmlText, jsText)
      .then(() => {
        console.log("Settings and injection texts saved successfully.");
        if (isInline) {
          onclose();
        } else {
          show = false;
        }
      })
      .catch((e) => console.error("Failed to save settings/injections:", e));
  }

  // Handle Cancel / Close button
  function handleClose() {
    if (!isInline) {
      show = false;
    }
    onclose();
  }
</script>

{#if show || isInline}
  <div class={isInline ? "w-full h-full flex flex-col bg-surface-800 text-white select-none overflow-hidden" : "fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm transition-opacity duration-300"}>
    <!-- Modal Card / Container -->
    <div class={isInline ? "flex-grow flex flex-col h-full w-full overflow-hidden" : "w-full max-w-2xl max-h-[85vh] overflow-hidden bg-surface-800 border border-surface-700 rounded-xl shadow-2xl animate-in fade-in zoom-in-95 duration-200 flex flex-col"}>
      
      <!-- Modal Header -->
      <div class="flex items-center justify-between p-4 border-b border-surface-700 bg-surface-900/50 flex-shrink-0">
        <h3 class="text-lg text-white font-semibold tracking-wide">{i18n.t('settingsTitle')}</h3>
        {#if !isInline}
          <button 
            onclick={handleClose} 
            class="p-1.5 text-surface-400 hover:text-white rounded-lg hover:bg-surface-700 transition-colors"
            aria-label="Close"
          >
            <X size={20} />
          </button>
        {/if}
      </div>

      <!-- Modal Body -->
      <div class="p-5 space-y-5 overflow-y-auto flex-grow">
        
        <!-- Setting 1 -->
        <div class="space-y-1">
          <label class="block text-sm font-medium text-surface-200" for="initial-watch-time">
            {i18n.t('initialWatchTimeLabel')}
          </label>
          <div class="flex items-center gap-3">
            <input
              id="initial-watch-time"
              type="number"
              min="1"
              max="120"
              bind:value={initialWatchMin}
              disabled
              class="w-20 px-2.5 py-1.5 text-white bg-surface-900 border border-surface-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent transition-all opacity-50 cursor-not-allowed"
            />
            <span class="text-surface-300 text-sm">{i18n.t('minuteFixed')}</span>
          </div>
          <p class="text-xs text-surface-400">
            {i18n.t('initialWatchTimeDesc')}
          </p>
        </div>

        <!-- Setting 2 -->
        <div class="space-y-1">
          <label class="block text-sm font-medium text-surface-200" for="rotation-time">
            {i18n.t('rotationTimeLabel')}
          </label>
          <div class="flex items-center gap-3">
            <input
              id="rotation-time"
              type="number"
              min="1"
              max="120"
              bind:value={rotationMin}
              class="w-20 px-2.5 py-1.5 text-white bg-surface-900 border border-surface-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent transition-all"
            />
            <span class="text-surface-300 text-sm">{i18n.t('minuteDefault')}</span>
          </div>
          <p class="text-xs text-surface-400">
            {i18n.t('rotationTimeDesc')}
          </p>
        </div>

        <!-- Setting 3 -->
        <div class="flex items-start gap-3 pt-1">
          <div class="flex items-center h-5">
            <input
              id="auto-start-on-login"
              type="checkbox"
              bind:checked={localAutoStart}
              class="w-4 h-4 text-primary-600 bg-surface-900 border-surface-700 rounded focus:ring-primary-500 focus:ring-2 cursor-pointer"
            />
          </div>
          <div class="space-y-0.5 select-none">
            <label class="block text-sm font-medium text-surface-200 cursor-pointer" for="auto-start-on-login">
              {i18n.t('autoStartLabel')}
            </label>
            <p class="text-xs text-surface-400">
              {i18n.t('autoStartDesc')}
            </p>
          </div>
        </div>

        <!-- Divider -->
        <hr class="border-surface-700 my-4" />

        <h4 class="text-sm font-semibold text-surface-200 tracking-wide mb-1">{i18n.t('customInjections')}</h4>
        <p class="text-xs text-surface-400 mb-3">{i18n.t('customInjectionsDesc')}</p>

        <div class="space-y-3">
          
          <!-- Accordion 1: CSS -->
          <div class="border border-surface-700 rounded-lg overflow-hidden bg-surface-900/10">
            <button
              type="button"
              onclick={() => { if (!cssError) cssOpen = !cssOpen; }}
              disabled={!!cssError}
              class="w-full flex items-center justify-between p-3.5 text-sm font-medium text-surface-200 hover:bg-surface-700/20 transition-colors disabled:cursor-not-allowed select-none"
            >
              <span class="flex items-center gap-2">
                <span>{i18n.t('addCSS')}</span>
                {#if cssError}
                  <span class="px-2 py-0.5 text-xs font-semibold bg-rose-950 text-rose-300 rounded border border-rose-800/60 animate-pulse">{i18n.t('statusError')}</span>
                {:else if cssText.trim()}
                  <span class="px-2 py-0.5 text-xs font-semibold bg-emerald-950 text-emerald-300 rounded border border-emerald-800/60">{i18n.t('statusNormal')}</span>
                {/if}
              </span>
              <span class="transform transition-transform duration-200 text-xs text-surface-400" class:rotate-180={cssOpen}>▼</span>
            </button>

            {#if cssOpen}
              <div class="p-3.5 border-t border-surface-700 space-y-2 bg-surface-950/20">
                <textarea
                  bind:value={cssText}
                  placeholder={i18n.t('placeholderCSS')}
                  class="w-full h-36 p-2.5 font-mono text-xs text-white bg-surface-900 border rounded-lg focus:outline-none focus:ring-2 transition-all resize-y"
                  class:border-rose-500={!!cssError}
                  class:focus:ring-rose-500={!!cssError}
                  class:border-emerald-500={!cssError && cssText.trim()}
                  class:focus:ring-emerald-500={!cssError && cssText.trim()}
                  class:border-surface-700={!cssError && !cssText.trim()}
                  class:focus:ring-primary-500={!cssError && !cssText.trim()}
                ></textarea>
                {#if cssError}
                  <div class="text-xs text-rose-400 font-medium leading-relaxed bg-rose-950/20 border border-rose-900/30 p-2 rounded-md">
                    {#each cssError.split('\n') as line}
                      <div>{line}</div>
                    {/each}
                  </div>
                {/if}
              </div>
            {/if}
          </div>

          <!-- Accordion 2: HTML -->
          <div class="border border-surface-700 rounded-lg overflow-hidden bg-surface-900/10">
            <button
              type="button"
              onclick={() => { if (!htmlError) htmlOpen = !htmlOpen; }}
              disabled={!!htmlError}
              class="w-full flex items-center justify-between p-3.5 text-sm font-medium text-surface-200 hover:bg-surface-700/20 transition-colors disabled:cursor-not-allowed select-none"
            >
              <span class="flex items-center gap-2">
                <span>{i18n.t('addHTML')}</span>
                {#if htmlError}
                  <span class="px-2 py-0.5 text-xs font-semibold bg-rose-950 text-rose-300 rounded border border-rose-800/60 animate-pulse">{i18n.t('statusError')}</span>
                {:else if htmlText.trim()}
                  <span class="px-2 py-0.5 text-xs font-semibold bg-emerald-950 text-emerald-300 rounded border border-emerald-800/60">{i18n.t('statusNormal')}</span>
                {/if}
              </span>
              <span class="transform transition-transform duration-200 text-xs text-surface-400" class:rotate-180={htmlOpen}>▼</span>
            </button>

            {#if htmlOpen}
              <div class="p-3.5 border-t border-surface-700 space-y-2 bg-surface-950/20">
                <textarea
                  bind:value={htmlText}
                  placeholder={i18n.t('placeholderHTML')}
                  class="w-full h-36 p-2.5 font-mono text-xs text-white bg-surface-900 border rounded-lg focus:outline-none focus:ring-2 transition-all resize-y"
                  class:border-rose-500={!!htmlError}
                  class:focus:ring-rose-500={!!htmlError}
                  class:border-emerald-500={!htmlError && htmlText.trim()}
                  class:focus:ring-emerald-500={!htmlError && htmlText.trim()}
                  class:border-surface-700={!htmlError && !htmlText.trim()}
                  class:focus:ring-primary-500={!htmlError && !htmlText.trim()}
                ></textarea>
                {#if htmlError}
                  <div class="text-xs text-rose-400 font-medium leading-relaxed bg-rose-950/20 border border-rose-900/30 p-2 rounded-md">
                    {#each htmlError.split('\n') as line}
                      <div>{line}</div>
                    {/each}
                  </div>
                {/if}
              </div>
            {/if}
          </div>

          <!-- Accordion 3: JS -->
          <div class="border border-surface-700 rounded-lg overflow-hidden bg-surface-900/10">
            <button
              type="button"
              onclick={() => { if (!jsError) jsOpen = !jsOpen; }}
              disabled={!!jsError}
              class="w-full flex items-center justify-between p-3.5 text-sm font-medium text-surface-200 hover:bg-surface-700/20 transition-colors disabled:cursor-not-allowed select-none"
            >
              <span class="flex items-center gap-2">
                <span>{i18n.t('addJS')}</span>
                {#if jsError}
                  <span class="px-2 py-0.5 text-xs font-semibold bg-rose-950 text-rose-300 rounded border border-rose-800/60 animate-pulse">{i18n.t('statusError')}</span>
                {:else if jsText.trim()}
                  <span class="px-2 py-0.5 text-xs font-semibold bg-emerald-950 text-emerald-300 rounded border border-emerald-800/60">{i18n.t('statusNormal')}</span>
                {/if}
              </span>
              <span class="transform transition-transform duration-200 text-xs text-surface-400" class:rotate-180={jsOpen}>▼</span>
            </button>

            {#if jsOpen}
              <div class="p-3.5 border-t border-surface-700 space-y-2 bg-surface-950/20">
                <textarea
                  bind:value={jsText}
                  placeholder={i18n.t('placeholderJS')}
                  class="w-full h-36 p-2.5 font-mono text-xs text-white bg-surface-900 border rounded-lg focus:outline-none focus:ring-2 transition-all resize-y"
                  class:border-rose-500={!!jsError}
                  class:focus:ring-rose-500={!!jsError}
                  class:border-emerald-500={!jsError && jsText.trim()}
                  class:focus:ring-emerald-500={!jsError && jsText.trim()}
                  class:border-surface-700={!jsError && !jsText.trim()}
                  class:focus:ring-primary-500={!jsError && !jsText.trim()}
                ></textarea>
                {#if jsError}
                  <div class="text-xs text-rose-400 font-medium leading-relaxed bg-rose-950/20 border border-rose-900/30 p-2 rounded-md">
                    {#each jsError.split('\n') as line}
                      <div>{line}</div>
                    {/each}
                  </div>
                {/if}
              </div>
            {/if}
          </div>

        </div>

        <!-- Divider -->
        <hr class="border-surface-700 my-4" />

        <!-- Language Settings -->
        <div class="space-y-1">
          <label class="block text-sm font-medium text-surface-200" for="language-select">
            {i18n.t('languageLabel')}
          </label>
          <div class="flex items-center gap-3">
            <select
              id="language-select"
              bind:value={localLanguage}
              onchange={handleLanguageChange}
              class="px-2.5 py-1.5 text-white bg-surface-900 border border-surface-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent transition-all cursor-pointer"
            >
              <option value="en">English</option>
              <option value="ja">日本語</option>
            </select>
          </div>
          <p class="text-xs text-surface-400">
            {i18n.t('languageDesc')}
          </p>
        </div>

      </div>

      <!-- Modal Footer -->
      <div class="flex justify-end gap-3 p-4 border-t border-surface-700 bg-surface-900/30 flex-shrink-0">
        <button
          onclick={handleClose}
          class="px-4 py-2 text-sm font-medium text-surface-300 hover:text-white bg-surface-700 hover:bg-surface-600 rounded-lg transition-colors"
        >
          {isInline ? i18n.t('close') : i18n.t('cancel')}
        </button>
        <button
          onclick={handleSave}
          disabled={hasAnyError}
          class="px-5 py-2 text-sm font-medium text-white bg-primary-600 hover:bg-primary-500 rounded-lg shadow-lg hover:shadow-primary-500/20 transition-all duration-200 disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:bg-primary-600 disabled:shadow-none"
        >
          {i18n.t('save')}
        </button>
      </div>

    </div>
  </div>
{/if}

<style>
  /* Layout style fixes to work nicely with Skeleton V4 */
  .bg-surface-800 {
    background-color: var(--color-surface-800, #242c3d);
  }
  .bg-surface-900\/50 {
    background-color: rgba(var(--color-surface-900-rgb, 20, 26, 38), 0.5);
  }
  .bg-surface-900\/30 {
    background-color: rgba(var(--color-surface-900-rgb, 20, 26, 38), 0.3);
  }
  .bg-surface-900 {
    background-color: var(--color-surface-900, #141a26);
  }
  .border-surface-700 {
    border-color: var(--color-surface-700, #343e56);
  }
  .bg-primary-600 {
    background-color: var(--color-primary-600, #9333ea);
  }
  .bg-primary-600:hover {
    background-color: var(--color-primary-500, #a855f7);
  }
  .bg-surface-700 {
    background-color: var(--color-surface-700, #343e56);
  }
  .bg-surface-700:hover {
    background-color: var(--color-surface-600, #404c6a);
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
</style>
