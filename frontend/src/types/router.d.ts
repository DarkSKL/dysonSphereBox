import 'vue-router'

export type RouteLayout = 'main' | 'blank'

declare module 'vue-router' {
  interface RouteMeta {
    layout?: RouteLayout
    keepAlive?: boolean
    title?: string
  }
}
