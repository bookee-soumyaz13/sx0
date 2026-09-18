// Copy px0.* browser keys to sx0.* once so a renamed install keeps theme,
// wrap, vim, tabs, and the rest of the previous session.
function migratePrefixedStore(store, fromPrefix, toPrefix) {
  try {
    const keys = [];
    for (let i = 0; i < store.length; i++) {
      const k = store.key(i);
      if (k) keys.push(k);
    }
    for (const k of keys) {
      if (!k.startsWith(fromPrefix)) continue;
      const next = toPrefix + k.slice(fromPrefix.length);
      if (store.getItem(next) === null) store.setItem(next, store.getItem(k));
    }
  } catch {}
}

export function migrateBrowserState() {
  migratePrefixedStore(localStorage, 'px0.', 'sx0.');
  migratePrefixedStore(sessionStorage, 'px0.', 'sx0.');
}
