// jsdom does not implement ResizeObserver; provide a minimal stub for all tests.
global.ResizeObserver = class {
  observe() {}
  unobserve() {}
  disconnect() {}
}
