import '@testing-library/jest-dom';

// Mock localStorage
const localStorageMock = (() => {
  let store = {};
  return {
    getItem: (key) => store[key] ?? null,
    setItem: (key, value) => { store[key] = String(value); },
    removeItem: (key) => { delete store[key]; },
    clear: () => { store = {}; },
  };
})();

Object.defineProperty(window, 'localStorage', { value: localStorageMock });

// Mock import.meta.env
if (!import.meta.env.VITE_API_BASE) {
  import.meta.env.VITE_API_BASE = 'http://localhost:8080';
}
if (!import.meta.env.VITE_WS_BASE) {
  import.meta.env.VITE_WS_BASE = 'ws://localhost:8080/ws';
}
