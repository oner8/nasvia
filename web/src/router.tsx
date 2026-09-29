import { Outlet, createRootRoute, createRoute, createRouter } from '@tanstack/react-router'

import { Toaster } from './components/toaster'
import { AdminPage } from './pages/admin-page'
import { HomePage } from './pages/home-page'

/** 根路由：渲染子路由并提供全局 toast 容器。 */
const rootRoute = createRootRoute({
  component: () => (
    <>
      <Outlet />
      <Toaster />
    </>
  ),
})

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: HomePage,
})

const adminRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/admin',
  component: AdminPage,
})

const routeTree = rootRoute.addChildren([indexRoute, adminRoute])

export const router = createRouter({
  routeTree,
  defaultPreload: 'intent',
  scrollRestoration: false,
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
