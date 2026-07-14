<script lang="ts">
  import { X, Copy, ExternalLink, Check, Loader2 } from '@lucide/svelte';
  import * as TwitchService from "../../bindings/github.com/GennoBou/gnb-twview/backend/twitchservice";
  import { i18n } from '../i18n.svelte';

  // Svelte 5 Runes for Props
  let {
    show = $bindable(false),
    userCode = '',
    verificationUri = '',
    errorMsg = $bindable(''),
    onClose = () => {}
  } = $props();

  let copied = $state(false);

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(userCode);
      copied = true;
      setTimeout(() => {
        copied = false;
      }, 2000);
    } catch (err) {
      console.error('Failed to copy text: ', err);
    }
  }

  function handleCancel() {
    TwitchService.CancelLogin()
      .then(() => {
        show = false;
        onClose();
      })
      .catch(e => console.error(e));
  }
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm transition-opacity duration-300">
    <!-- Modal Card -->
    <div class="w-full max-w-md overflow-hidden bg-surface-800 border border-surface-700 rounded-xl shadow-2xl animate-in fade-in zoom-in-95 duration-200">
      
      <!-- Modal Header -->
      <div class="flex items-center justify-between p-5 border-b border-surface-700 bg-surface-900/50">
        <h3 class="h3 text-white font-semibold tracking-wide">{i18n.t('authTitle')}</h3>
        <button 
          onclick={handleCancel} 
          class="p-1.5 text-surface-400 hover:text-white rounded-lg hover:bg-surface-700 transition-colors"
          aria-label="Close"
        >
          <X size={20} />
        </button>
      </div>

      <!-- Modal Body -->
      <div class="p-6 space-y-6">
        
        <div class="text-center space-y-2">
          <p class="text-sm text-surface-300">
            {i18n.t('authInstructions')}
          </p>
          <a
            href={verificationUri}
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-1.5 text-primary-400 hover:text-primary-300 hover:underline text-sm font-medium transition-all"
          >
            {verificationUri}
            <ExternalLink size={14} />
          </a>
        </div>

        <!-- Code Box -->
        <div class="bg-surface-900 border border-surface-700 rounded-xl p-5 flex flex-col items-center justify-center relative overflow-hidden">
          <span class="text-xs text-surface-400 font-semibold tracking-wider mb-2">{i18n.t('codeLabel')}</span>
          <span class="text-3xl font-bold tracking-widest text-white font-mono select-all">
            {userCode}
          </span>
          
          <button
            onclick={handleCopy}
            class="mt-4 flex items-center gap-1.5 px-3 py-1.5 bg-surface-700 hover:bg-surface-600 active:bg-surface-500 text-surface-200 text-xs font-semibold rounded-lg border border-surface-600 transition-all"
          >
            {#if copied}
              <Check size={13} class="text-emerald-400" />
              <span class="text-emerald-400 font-medium">{i18n.t('copied')}</span>
            {:else}
              <Copy size={13} />
              <span>{i18n.t('copyCode')}</span>
            {/if}
          </button>
        </div>

        <!-- Waiting Indicator -->
        <div class="flex flex-col items-center justify-center gap-2 pt-2">
          {#if errorMsg}
            <p class="text-xs text-rose-400 text-center font-medium bg-rose-950/20 border border-rose-900/30 rounded-lg px-3 py-2 w-full">
              {errorMsg}
            </p>
          {:else}
            <div class="flex items-center gap-2 text-surface-400 text-xs font-medium">
              <Loader2 size={14} class="animate-spin text-primary-400" />
              <span>{i18n.t('waitingAuth')}</span>
            </div>
          {/if}
        </div>

      </div>

      <!-- Modal Footer -->
      <div class="flex justify-end gap-3 p-5 border-t border-surface-700 bg-surface-900/30">
        <button
          onclick={handleCancel}
          class="px-5 py-2 text-sm font-medium text-surface-300 hover:text-white bg-surface-700 hover:bg-surface-600 rounded-lg transition-colors w-full"
        >
          {i18n.t('cancel')}
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
  .text-primary-400 {
    color: var(--color-primary-400, #c084fc);
  }
  .text-primary-400:hover {
    color: var(--color-primary-300, #d8b4fe);
  }
  .bg-surface-700 {
    background-color: var(--color-surface-700, #343e56);
  }
  .bg-surface-700:hover {
    background-color: var(--color-surface-600, #404c6a);
  }
  .border-surface-600 {
    border-color: var(--color-surface-600, #404c6a);
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
  .text-rose-400 {
    color: #fb7185;
  }
  .bg-rose-950\/20 {
    background-color: rgba(136, 19, 55, 0.2);
  }
  .border-rose-900\/30 {
    border-color: rgba(136, 19, 55, 0.3);
  }
</style>
