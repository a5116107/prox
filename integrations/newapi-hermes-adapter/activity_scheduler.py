"""Small proactive activity scheduler used by GameDirectorIntegration."""

from __future__ import annotations

import time


class ActivityScheduler:
    def __init__(self, send_fn=None, site_id="default"):
        self.send_fn = send_fn
        self.site_id = site_id
        self._groups = set()
        self._last_tick = 0.0

    def register_group(self, group_id):
        gid = str(group_id or "").strip()
        if gid:
            self._groups.add(gid)

    def tick(self):
        # Placeholder for scheduled campaigns. Keep it intentionally quiet:
        # config-driven campaigns should be added through New API Ops policy.
        self._last_tick = time.time()
        return []
