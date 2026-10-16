import { mount } from 'svelte';
import { initTheme, setThemeMode } from '@kenn-io/kit-ui';
import '@kenn-io/kit-ui/theme.css';
import App from './App.svelte';
import './app.css';
initTheme({ storageKey: 'ciview-theme' });
// Start in charcoal; retain explicit light or system preferences.
try {
 if (!localStorage.getItem('ciview-theme')) setThemeMode('dark');
} catch {
 setThemeMode('dark');
}
mount(App, { target: document.getElementById('app')! });
