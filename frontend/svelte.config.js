import { vitePreprocess } from '@sveltejs/vite-plugin-svelte'

export default {
  preprocess: vitePreprocess(),
  // Runes everywhere: no legacy reactive statements or `export let`.
  compilerOptions: { runes: true },
}
