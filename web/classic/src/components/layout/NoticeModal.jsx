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

import React, { useEffect, useState, useContext, useMemo } from 'react';
import {
  Button,
  Modal,
  Empty,
  Tabs,
  TabPane,
  Timeline,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, getRelativeTime } from '../../helpers';
import { marked } from 'marked';
import {
  IllustrationNoContent,
  IllustrationNoContentDark,
} from '@douyinfe/semi-illustrations';
import { StatusContext } from '../../context/Status';
import { Bell, Megaphone, Send, Users } from 'lucide-react';

const normalizeBoolean = (value) =>
  value === true || value === 'true' || value === 1 || value === '1';

const isForcePopupAnnouncement = (item) =>
  normalizeBoolean(item?.forcePopup) ||
  normalizeBoolean(item?.force_popup) ||
  normalizeBoolean(item?.forced);

const QQ_GROUP_PATTERN =
  /(^|[\s，。；;、（(【\[])(Q群|QQ群|qq群|qq\s*群|QQ\s*群|qq|QQ)\s*[：: ]\s*(\d{5,12})/g;
const QQ_GROUP_EXTRACT_PATTERN =
  /(?:Q群|QQ群|qq群|qq\s*群|QQ\s*群|qq|QQ)\s*[：: ]\s*(\d{5,12})/i;

const buildQqGroupLink = (group) =>
  `mqqapi://card/show_pslcard?src_type=internal&version=1&uin=${encodeURIComponent(
    group,
  )}&card_type=group&source=qrcode`;

const buildTelegramLink = (value) => {
  const raw = String(value || '').trim();
  if (!raw) return '';
  if (/^https?:\/\//i.test(raw) || /^tg:\/\//i.test(raw)) return raw;
  return `https://t.me/${raw.replace(/^@+/, '')}`;
};

const extractQqGroup = (text = '') =>
  String(text || '').match(QQ_GROUP_EXTRACT_PATTERN)?.[1] || '';

const escapeAttr = (value = '') =>
  String(value).replace(/&/g, '&amp;').replace(/"/g, '&quot;');

const normalizeCommunityType = (value) => {
  const normalized = String(value || '').toLowerCase();
  return normalized === 'tg' || normalized === 'telegram' ? 'tg' : 'qq';
};

const getCommunityInfo = (item = {}) => {
  const inferredType =
    item.tgGroup ||
    item.tg_group ||
    item.telegram ||
    item.telegramLink ||
    item.telegram_link
      ? 'tg'
      : 'qq';
  const type = normalizeCommunityType(
    item.communityType || item.community_type || inferredType,
  );
  const value =
    item.communityValue ||
    item.community_value ||
    (type === 'tg'
      ? item.tgGroup || item.tg_group || item.telegram || ''
      : item.qqGroup ||
        item.qq_group ||
        extractQqGroup(item.content) ||
        extractQqGroup(item.extra) ||
        '');
  const link =
    item.communityLink ||
    item.community_link ||
    (type === 'tg'
      ? item.tgGroupLink ||
        item.tg_group_link ||
        item.telegramLink ||
        item.telegram_link ||
        buildTelegramLink(value)
      : item.communityLink ||
        item.qq_group_link ||
        (value ? buildQqGroupLink(value) : ''));
  const joinText = type === 'tg' ? '立即加入 TG 群' : '立即加入 QQ 群';
  return { type, value, link, joinText };
};

const linkifyQqGroups = (text = '', item = {}) => {
  const source = String(text || '');
  if (!source) return source;

  return source.replace(QQ_GROUP_PATTERN, (match, prefix, label, group) => {
    const configured = getCommunityInfo({ ...item, communityType: 'qq' });
    const link =
      configured.link && (!configured.value || configured.value === group)
        ? configured.link
        : buildQqGroupLink(group);
    return `${prefix}<a href="${escapeAttr(link)}" target="_blank" rel="noopener noreferrer" class="qq-group-link">${label}：${group}</a>`;
  });
};

const openCommunityLink = (link) => {
  if (!link) return;
  if (/^mqqapi:/i.test(link) || /^tg:\/\//i.test(link)) {
    window.location.href = link;
    return;
  }
  window.open(link, '_blank', 'noopener,noreferrer');
};

const SITE_PRIMARY_COMMUNITY_INFO = {
  type: 'qq',
  value: '925249987',
  link: buildQqGroupLink('925249987'),
  joinText: '立即加入 QQ 群',
};

const NoticeModal = ({
  visible,
  onClose,
  isMobile,
  defaultTab = 'inApp',
  unreadKeys = [],
  forcePopup = false,
}) => {
  const { t } = useTranslation();
  const [noticeContent, setNoticeContent] = useState('');
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState(defaultTab);

  const [statusState] = useContext(StatusContext);

  const announcements = statusState?.status?.announcements || [];

  const unreadSet = useMemo(() => new Set(unreadKeys), [unreadKeys]);

  const getKeyForItem = (item) =>
    `${item?.publishDate || ''}-${(item?.content || '').slice(0, 30)}`;

  const processedAnnouncements = useMemo(() => {
    return (announcements || []).slice(0, 20).map((item) => {
      const pubDate = item?.publishDate ? new Date(item.publishDate) : null;
      const absoluteTime =
        pubDate && !isNaN(pubDate.getTime())
          ? `${pubDate.getFullYear()}-${String(pubDate.getMonth() + 1).padStart(2, '0')}-${String(pubDate.getDate()).padStart(2, '0')} ${String(pubDate.getHours()).padStart(2, '0')}:${String(pubDate.getMinutes()).padStart(2, '0')}`
          : item?.publishDate || '';
      return {
        key: getKeyForItem(item),
        type: item.type || 'default',
        time: absoluteTime,
        content: item.content,
        extra: item.extra,
        forcePopup: isForcePopupAnnouncement(item),
        communityType: getCommunityInfo(item).type,
        communityValue: getCommunityInfo(item).value,
        communityLink: getCommunityInfo(item).link,
        communityJoinText: getCommunityInfo(item).joinText,
        qqGroup: getCommunityInfo(item).value,
        qqGroupLink: getCommunityInfo(item).link,
        relative: getRelativeTime(item.publishDate),
        isUnread: unreadSet.has(getKeyForItem(item)),
      };
    });
  }, [announcements, unreadSet]);

  const primaryCommunityInfo = useMemo(() => {
    const item = processedAnnouncements.find(
      (announcement) => announcement.communityLink,
    );
    return item
      ? {
          type: item.communityType || 'qq',
          value: item.communityValue || '',
          link: item.communityLink || '',
          joinText: item.communityJoinText || '立即加入 QQ 群',
        }
      : { ...SITE_PRIMARY_COMMUNITY_INFO };
  }, [processedAnnouncements]);

  const handleCloseTodayNotice = () => {
    if (forcePopup) {
      onClose();
      return;
    }
    const today = new Date().toDateString();
    localStorage.setItem('notice_close_date', today);
    onClose();
  };

  const displayNotice = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/notice');
      const { success, message, data } = res.data;
      if (success) {
        if (data !== '') {
          const htmlNotice = marked.parse(linkifyQqGroups(data));
          setNoticeContent(htmlNotice);
        } else {
          setNoticeContent('');
        }
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (visible) {
      displayNotice();
    }
  }, [visible]);

  useEffect(() => {
    if (visible) {
      setActiveTab(defaultTab);
    }
  }, [defaultTab, visible]);

  const renderMarkdownNotice = () => {
    if (loading) {
      return (
        <div className='py-12'>
          <Empty description={t('加载中...')} />
        </div>
      );
    }

    if (!noticeContent) {
      return (
        <div className='py-12'>
          <Empty
            image={
              <IllustrationNoContent style={{ width: 150, height: 150 }} />
            }
            darkModeImage={
              <IllustrationNoContentDark style={{ width: 150, height: 150 }} />
            }
            description={t('暂无公告')}
          />
        </div>
      );
    }

    return (
      <div
        dangerouslySetInnerHTML={{ __html: noticeContent }}
        className='notice-content-scroll max-h-[55vh] overflow-y-auto pr-2'
      />
    );
  };

  const renderAnnouncementTimeline = () => {
    if (processedAnnouncements.length === 0) {
      return (
        <div className='py-12'>
          <Empty
            image={
              <IllustrationNoContent style={{ width: 150, height: 150 }} />
            }
            darkModeImage={
              <IllustrationNoContentDark style={{ width: 150, height: 150 }} />
            }
            description={t('暂无系统公告')}
          />
        </div>
      );
    }

    return (
      <div className='max-h-[62vh] overflow-y-auto pr-2 card-content-scroll strong-announcement-timeline'>
        <Timeline mode='left'>
          {processedAnnouncements.map((item, idx) => {
            const htmlContent = marked.parse(
              linkifyQqGroups(item.content || '', item),
            );
            const htmlExtra = item.extra
              ? marked.parse(linkifyQqGroups(item.extra, item))
              : '';
            return (
              <Timeline.Item
                key={idx}
                type={item.type}
                time={`${item.relative ? item.relative + ' ' : ''}${item.time}`}
                extra={
                  item.extra ? (
                    <div
                      className='text-xs text-gray-500'
                      dangerouslySetInnerHTML={{ __html: htmlExtra }}
                    />
                  ) : null
                }
                className={item.isUnread ? '' : ''}
              >
                <div className='strong-announcement-item'>
                  <div
                    className={
                      item.isUnread
                        ? 'shine-text strong-announcement-content'
                        : 'strong-announcement-content'
                    }
                    dangerouslySetInnerHTML={{ __html: htmlContent }}
                  />
                </div>
              </Timeline.Item>
            );
          })}
        </Timeline>
      </div>
    );
  };

  const renderBody = () => {
    if (activeTab === 'inApp') {
      return renderMarkdownNotice();
    }
    return renderAnnouncementTimeline();
  };

  return (
    <Modal
      className='strong-notice-modal prox-terminal-notice'
      maskStyle={{
        backdropFilter: 'blur(12px) saturate(1.25)',
        background:
          'radial-gradient(circle at 30% 10%, rgba(14,165,233,.22), transparent 32%), rgba(2, 6, 23, 0.72)',
      }}
      title={
        <div className='notice-title-shell prox-title-shell flex items-center justify-between w-full'>
          <span className='prox-title-copy'>
            <span className='prox-title-kicker' translate='no'>
              {'AI PROX CONTROL'}
            </span>
            <span className='prox-title-main'>
              <Bell size={18} /> {t('系统公告')}
            </span>
          </span>
          <Tabs activeKey={activeTab} onChange={setActiveTab} type='button'>
            <TabPane
              tab={
                <span className='flex items-center gap-1'>
                  <Bell size={14} /> {t('通知')}
                </span>
              }
              itemKey='inApp'
            />
            <TabPane
              tab={
                <span className='flex items-center gap-1'>
                  <Megaphone size={14} /> {t('系统公告')}
                </span>
              }
              itemKey='system'
            />
          </Tabs>
        </div>
      }
      visible={visible}
      onCancel={onClose}
      footer={
        <div className='strong-notice-footer'>
          {primaryCommunityInfo.link ? (
            <div className='strong-footer-qq'>
              <div className='strong-footer-copy'>
                <div className='strong-footer-title'>
                  {t('第一时间获取线路和模型状态')}
                </div>
                <div className='strong-footer-desc'>
                  {primaryCommunityInfo.type === 'tg'
                    ? t('加入 Telegram，公告更新会同步通知')
                    : t('加入官方群，公告更新会同步通知')}
                </div>
              </div>
              <Button
                size='large'
                type='primary'
                theme='solid'
                className='strong-qq-button strong-footer-qq-button'
                icon={
                  primaryCommunityInfo.type === 'tg' ? (
                    <Send size={18} />
                  ) : (
                    <Users size={18} />
                  )
                }
                onClick={() => openCommunityLink(primaryCommunityInfo.link)}
              >
                {primaryCommunityInfo.value
                  ? `${t(primaryCommunityInfo.type === 'tg' ? '立即加 TG' : '立即加群')} ${primaryCommunityInfo.value}`
                  : t(
                      primaryCommunityInfo.type === 'tg'
                        ? '立即加 TG'
                        : '立即加群',
                    )}
              </Button>
            </div>
          ) : (
            <span />
          )}
          <div className='strong-footer-actions'>
            {!forcePopup ? (
              <Button type='secondary' onClick={handleCloseTodayNotice}>
                {t('今日关闭')}
              </Button>
            ) : null}
            <Button type='primary' onClick={onClose}>
              {t('关闭公告')}
            </Button>
          </div>
        </div>
      }
      size={isMobile ? 'full-width' : 'large'}
      style={isMobile ? undefined : { maxWidth: 860 }}
    >
      <style>{`
        /* NOTICE_SKIN_PROX_TERMINAL_V2 */
        .prox-terminal-notice .semi-modal-content {
          position: relative;
          overflow: hidden;
          border: 1px solid rgba(34,211,238,.34);
          border-radius: 30px;
          color: #e0f2fe;
          background:
            radial-gradient(circle at 16% 0%, rgba(56,189,248,.28), transparent 34%),
            radial-gradient(circle at 90% 10%, rgba(52,211,153,.17), transparent 32%),
            linear-gradient(145deg, rgba(8,13,30,.97), rgba(4,9,20,.96));
          box-shadow: 0 34px 140px -48px rgba(0,0,0,.98), 0 0 0 1px rgba(103,232,249,.10), inset 0 1px 0 rgba(255,255,255,.10);
        }
        .prox-terminal-notice .semi-modal-content::before {
          content: '';
          position: absolute;
          inset: 0;
          pointer-events: none;
          background-image: linear-gradient(rgba(125,211,252,.055) 1px, transparent 1px), linear-gradient(90deg, rgba(125,211,252,.05) 1px, transparent 1px);
          background-size: 28px 28px;
          mask-image: linear-gradient(to bottom, rgba(0,0,0,.9), rgba(0,0,0,.18));
        }
        .prox-terminal-notice .semi-modal-header,
        .prox-terminal-notice .semi-modal-footer {
          position: relative;
          border-color: rgba(103,232,249,.18);
          background: rgba(2,6,23,.20);
        }
        .prox-title-copy { display: flex; flex-direction: column; gap: 4px; }
        .prox-title-kicker {
          color: #67e8f9;
          font-size: 11px;
          font-weight: 900;
          letter-spacing: .18em;
          text-transform: uppercase;
        }
        .prox-title-main {
          display: flex;
          align-items: center;
          gap: 10px;
          color: #f8fafc;
          font-size: 20px;
          font-weight: 950;
          letter-spacing: -.02em;
        }
        .prox-terminal-notice .semi-tabs-tab-button.semi-tabs-tab-active {
          color: #061018;
          background: linear-gradient(90deg, #38bdf8, #22d3ee, #34d399);
          box-shadow: 0 10px 34px -16px rgba(34,211,238,.95);
        }
        .prox-terminal-notice .notice-content-scroll,
        .prox-terminal-notice .card-content-scroll { color: #e5e7eb; }
        .prox-terminal-notice .semi-timeline-item-content { padding-bottom: 16px; }
        .prox-terminal-notice .strong-announcement-item {
          border: 1px solid rgba(103,232,249,.18);
          border-radius: 22px;
          padding: 16px;
          background: linear-gradient(135deg, rgba(15,23,42,.86), rgba(8,47,73,.24));
          box-shadow: 0 18px 54px -34px rgba(2,132,199,.95), inset 0 0 0 1px rgba(255,255,255,.045);
        }
        .prox-terminal-notice .strong-announcement-content {
          color: #e2e8f0;
          font-size: 15px;
          font-weight: 750;
          line-height: 1.78;
        }
        .prox-terminal-notice .strong-announcement-content a,
        .prox-terminal-notice .qq-group-link {
          color: #67e8f9;
          font-weight: 950;
          text-decoration: underline;
          text-underline-offset: 4px;
        }
        .prox-terminal-notice .strong-footer-qq {
          border: 1px solid rgba(103,232,249,.34);
          border-radius: 20px;
          background: linear-gradient(135deg, rgba(34,211,238,.15), rgba(16,185,129,.10));
          box-shadow: 0 22px 70px -34px rgba(34,211,238,.95), inset 0 0 0 1px rgba(255,255,255,.06);
        }
        .prox-terminal-notice .strong-qq-button {
          min-height: 48px;
          border: 0 !important;
          border-radius: 999px !important;
          color: #04111d !important;
          font-size: 15px !important;
          font-weight: 950 !important;
          background: linear-gradient(90deg, #38bdf8, #22d3ee, #34d399) !important;
          box-shadow: 0 0 0 1px rgba(255,255,255,.42), 0 20px 58px -12px rgba(34,211,238,.95) !important;
          transition: transform .18s ease, filter .18s ease;
          animation: proxTerminalGlow 2.6s ease-in-out infinite;
        }
        .prox-terminal-notice .strong-qq-button:hover { transform: translateY(-1px) scale(1.012); filter: brightness(1.06); }
        .prox-terminal-notice .strong-notice-footer { display: flex; align-items: center; justify-content: space-between; gap: 14px; width: 100%; }
        .prox-terminal-notice .strong-footer-qq { display: flex; align-items: center; justify-content: space-between; gap: 14px; flex: 1; padding: 12px; }
        .prox-terminal-notice .strong-footer-title { color: #cffafe; font-size: 14px; font-weight: 950; }
        .prox-terminal-notice .strong-footer-desc { color: rgba(226,232,240,.78); font-size: 12px; font-weight: 650; }
        .prox-terminal-notice .strong-footer-qq-button { width: auto; min-width: 196px; }
        .prox-terminal-notice .strong-footer-actions { display: flex; justify-content: flex-end; gap: 8px; flex-shrink: 0; }
        @keyframes proxTerminalGlow {
          0%, 100% { box-shadow: 0 0 0 1px rgba(255,255,255,.38), 0 20px 56px -12px rgba(34,211,238,.88); }
          50% { box-shadow: 0 0 0 1px rgba(255,255,255,.50), 0 24px 78px -8px rgba(52,211,153,1); }
        }
        @media (prefers-reduced-motion: reduce) { .prox-terminal-notice .strong-qq-button { animation: none; } }
        @media (max-width: 640px) {
          .prox-terminal-notice .notice-title-shell { flex-direction: column; align-items: stretch; gap: 12px; }
          .prox-terminal-notice .strong-notice-footer,
          .prox-terminal-notice .strong-footer-qq { flex-direction: column; align-items: stretch; }
          .prox-terminal-notice .strong-footer-qq-button,
          .prox-terminal-notice .strong-footer-actions { width: 100%; }
          .prox-terminal-notice .strong-footer-actions > button { flex: 1; }
        }
      `}</style>
      {renderBody()}
    </Modal>
  );
};

export default NoticeModal;
