import { mount } from 'svelte';
import { initTheme } from '@kenn-io/kit-ui';
import '@kenn-io/kit-ui/theme.css';
import App from './App.svelte';
import './app.css';
initTheme({ storageKey: 'ciview-theme' });
mount(App, { target: document.getElementById('app')! });
