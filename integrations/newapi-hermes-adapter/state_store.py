"""Unified file-based state persistence for game plugins.
Survives adapter restarts. Each namespace gets its own JSON file."""

import os, json, threading, time

_DEFAULT_DIR = os.environ.get("HERMES_STATE_DIR") or os.path.join(
    os.path.dirname(os.path.abspath(__file__)), "game_state"
)


class StateStore:
    def __init__(self, store_dir=None):
        self.dir = store_dir or _DEFAULT_DIR
        os.makedirs(self.dir, exist_ok=True)
        self._data = {}
        self._lock = threading.Lock()
        self._dirty = set()
        self._load_all()

    def _path(self, ns):
        return os.path.join(self.dir, f"{ns}.json")

    def _load_all(self):
        for f in os.listdir(self.dir):
            if f.endswith(".json"):
                ns = f[:-5]
                try:
                    with open(os.path.join(self.dir, f)) as fh:
                        self._data[ns] = json.load(fh)
                except Exception:
                    self._data[ns] = {}

    def get(self, ns, key, default=None):
        with self._lock:
            return self._data.get(ns, {}).get(key, default)

    def set(self, ns, key, value):
        with self._lock:
            self._data.setdefault(ns, {})[key] = value
            self._dirty.add(ns)

    def delete(self, ns, key):
        with self._lock:
            if ns in self._data and key in self._data[ns]:
                del self._data[ns][key]
                self._dirty.add(ns)

    def get_all(self, ns):
        with self._lock:
            return dict(self._data.get(ns, {}))

    def set_all(self, ns, data):
        with self._lock:
            self._data[ns] = dict(data)
            self._dirty.add(ns)

    def save(self, ns=None):
        with self._lock:
            targets = [ns] if ns else list(self._dirty)
            for n in targets:
                if n in self._data:
                    try:
                        tmp = self._path(n) + ".tmp"
                        with open(tmp, "w") as f:
                            json.dump(self._data[n], f, ensure_ascii=False, indent=1)
                        os.replace(tmp, self._path(n))
                    except Exception as e:
                        print(f"[StateStore] save {n} failed: {e}", flush=True)
            self._dirty -= set(targets)

    def save_all(self):
        self.save()

    def flush_if_dirty(self):
        if self._dirty:
            self.save()


_instance = None
_instance_lock = threading.Lock()


def get_store(store_dir=None):
    global _instance
    with _instance_lock:
        if _instance is None:
            _instance = StateStore(store_dir)
    return _instance
