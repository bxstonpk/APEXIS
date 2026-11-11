import { BrowserRouter, Navigate, Route, Routes } from 'react-router'
import { appConfig } from '@/config'
import { AuthProvider, RequireAuth } from '@/features/auth'
import { HomePage } from '@/pages/HomePage'
import { LoginPage } from '@/pages/LoginPage'

/** ราก (root) ของแอป: auth context + router — path ทั้งหมดมาจาก appConfig.routes */
export function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route path={appConfig.routes.login} element={<LoginPage />} />
          <Route
            path={appConfig.routes.home}
            element={
              <RequireAuth>
                <HomePage />
              </RequireAuth>
            }
          />
          <Route path="*" element={<Navigate to={appConfig.routes.home} replace />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  )
}
