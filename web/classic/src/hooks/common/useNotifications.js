/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import { useState, useEffect } from 'react';

const normalizeBoolean = (value) =>
  value === true || value === 'true' || value === 1 || value === '1';

const isForcePopupAnnouncement = (announcement) =>
  normalizeBoolean(announcement?.forcePopup) ||
  normalizeBoolean(announcement?.force_popup) ||
  normalizeBoolean(announcement?.forced);

const getForcePopupSeenKeys = () => {
  try {
    return (
      JSON.parse(
        localStorage.getItem('notice_force_popup_seen_keys_strong_v3'),
      ) || []
    );
  } catch (_) {
    return [];
  }
};

const markForcePopupSeen = (key) => {
  if (!key) return;
  const seenKeys = getForcePopupSeenKeys();
  localStorage.setItem(
    'notice_force_popup_seen_keys_strong_v3',
    JSON.stringify(Array.from(new Set([...seenKeys, key]))),
  );
};

export const useNotifications = (statusState) => {
  const [noticeVisible, setNoticeVisible] = useState(false);
  const [unreadCount, setUnreadCount] = useState(0);

  const announcements = statusState?.status?.announcements || [];

  // Helper functions
  const getAnnouncementKey = (a) =>
    `${a?.id || ''}-${a?.publishDate || ''}-${(a?.content || '').slice(0, 30)}`;

  const forcePopupKey = announcements
    .filter(isForcePopupAnnouncement)
    .map(getAnnouncementKey)
    .join('|');

  const calculateUnreadCount = () => {
    if (!announcements.length) return 0;
    let readKeys = [];
    try {
      readKeys = JSON.parse(localStorage.getItem('notice_read_keys')) || [];
    } catch (_) {
      readKeys = [];
    }
    const readSet = new Set(readKeys);
    return announcements.filter((a) => !readSet.has(getAnnouncementKey(a)))
      .length;
  };

  const getUnreadKeys = () => {
    if (!announcements.length) return [];
    let readKeys = [];
    try {
      readKeys = JSON.parse(localStorage.getItem('notice_read_keys')) || [];
    } catch (_) {
      readKeys = [];
    }
    const readSet = new Set(readKeys);
    return announcements
      .filter((a) => !readSet.has(getAnnouncementKey(a)))
      .map(getAnnouncementKey);
  };

  // Effects
  useEffect(() => {
    setUnreadCount(calculateUnreadCount());
  }, [announcements]);

  useEffect(() => {
    if (!forcePopupKey) return;
    if (!getForcePopupSeenKeys().includes(forcePopupKey)) {
      setNoticeVisible(true);
    }
  }, [forcePopupKey]);

  // Actions
  const handleNoticeOpen = () => {
    setNoticeVisible(true);
  };

  const handleNoticeClose = () => {
    setNoticeVisible(false);
    if (forcePopupKey) {
      markForcePopupSeen(forcePopupKey);
    }
    if (announcements.length) {
      let readKeys = [];
      try {
        readKeys = JSON.parse(localStorage.getItem('notice_read_keys')) || [];
      } catch (_) {
        readKeys = [];
      }
      const mergedKeys = Array.from(
        new Set([...readKeys, ...announcements.map(getAnnouncementKey)]),
      );
      localStorage.setItem('notice_read_keys', JSON.stringify(mergedKeys));
    }
    setUnreadCount(0);
  };

  return {
    noticeVisible,
    unreadCount,
    announcements,
    handleNoticeOpen,
    handleNoticeClose,
    getUnreadKeys,
    hasForcePopup: Boolean(forcePopupKey),
    forcePopupKey,
  };
};
