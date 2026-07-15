"""Game Plugin Registry."""

import importlib, importlib.util, inspect, os, sys
from .base import GamePlugin, GameContext, GameResponse

_registry = {}


def discover_plugins(d=None):
    global _registry
    if d is None:
        d = os.path.dirname(__file__)
    _registry.clear()
    for f in sorted(os.listdir(d)):
        if f.startswith(("_", ".")) or not f.endswith(".py") or f == "base.py":
            continue
        try:
            spec = importlib.util.spec_from_file_location(
                "game_plugins." + f[:-3], os.path.join(d, f)
            )
            m = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(m)
            for n, o in inspect.getmembers(m):
                if (
                    inspect.isclass(o)
                    and issubclass(o, GamePlugin)
                    and o is not GamePlugin
                ):
                    p = o()
                    _registry[p.name] = p
        except Exception as e:
            print("RegFail " + f + ": " + str(e))
    return _registry


def get_plugin(n):
    return _registry.get(n)


def list_plugins():
    return [
        {
            "name": p.name,
            "display": p.display_name,
            "tier": p.tier,
            "on": p.config.get("enabled", 1),
        }
        for p in _registry.values()
    ]
