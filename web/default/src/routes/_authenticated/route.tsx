/*
Copyright (C) 2023-2026 QuantumNous

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
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { getSelf } from '@/lib/api'
import { AuthenticatedLayout } from '@/components/layout'

// 内存中的验证标记，避免同一会话中重复验证
let sessionVerified = false

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: async ({ location }) => {
    const { auth } = useAuthStore.getState()

    // 先以服务端 session 为准恢复登录态。
    // OAuth 登录后浏览器有时已经拿到 session cookie，但 localStorage 尚未写入/被并发 401 清空；
    // 这时不能直接踢回登录页，否则会出现“登录成功后又回 /sign-in”。
    const restoreFromSession = async () => {
      const res = await getSelf().catch(() => null)
      if (res?.success && res.data) {
        auth.setUser(res.data)
        sessionVerified = true
        try {
          if (typeof window !== 'undefined' && res.data.id != null) {
            window.localStorage.setItem('uid', String(res.data.id))
          }
        } catch (_error) {
          void _error
        }
        return true
      }
      return false
    }

    if (!auth.user) {
      if (await restoreFromSession()) return
      throw redirect({
        to: '/sign-in',
        search: { redirect: location.href },
      })
    }

    // 本地有用户信息，但需要验证 session 是否有效（每个会话只验证一次）
    if (!sessionVerified) {
      if (!(await restoreFromSession())) {
        // 验证失败或 API 调用失败，清除本地缓存并跳转登录页
        auth.reset()
        throw redirect({
          to: '/sign-in',
          search: { redirect: location.href },
        })
      }
    }
  },
  component: AuthenticatedLayout,
})
