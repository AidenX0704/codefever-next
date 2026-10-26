import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from 'react-router'

import { setUnauthorizedHandler } from '@/api/client'
import { ConfirmProvider } from '@/components/unit/Confirm'
import { ToastProvider } from '@/components/unit/Toast'
import { I18nProvider } from '@/i18n'

import { router } from './router'

import '@/styles/index.css'


const container = document.getElementById('root')
if (!container) {
  throw new Error('挂载点 #root 不存在')
}

// 登录失效（code 0x0101）时由 api 层回调这里跳转；集中一处，避免每个页面自己判断。
setUnauthorizedHandler(() => {
  window.location.href = '/user/login'
})

createRoot(container).render(
  <StrictMode>
    <I18nProvider>
      <ToastProvider>
        <ConfirmProvider>
          <RouterProvider router={router} />
        </ConfirmProvider>
      </ToastProvider>
    </I18nProvider>
  </StrictMode>,
)
