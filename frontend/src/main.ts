import { mount } from 'svelte';
import { initTheme, setThemeMode } from '@kenn-io/kit-ui';
import '@kenn-io/kit-ui/theme.css';
import './theme.css';
import App from './App.svelte';
import './app.css';
import './components.css';

initTheme({ storageKey: 'ciview-theme' });
// Default to charcoal while respecting explicit light and system preferences.
try {
 if (!localStorage.getItem('ciview-theme')) setThemeMode('dark');
} catch {
 setThemeMode('dark');
}
mount(App, { target: document.getElementById('app')! });
