export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  ssr: false,
  devtools: { enabled: true },

  modules: ['@pinia/nuxt', '@vueuse/nuxt', '@nuxt/icon'],

  css: ['~/assets/main.css'],

  runtimeConfig: {
    public: {
      apiBase: process.env.API_BASE ?? '',
    },
  },

  nitro: {
    prerender: {
      ignore: ['/api/'],
      failOnError: false,
    },
  },

  routeRules: {
    '/api/**': {
      proxy: {
        to: (process.env.API_BASE ?? 'http://localhost:8080') + '/**',
      },
    },
  },

  app: {
    head: {
      title: 'Scutum',
      meta: [{ name: 'description', content: 'Sovereign P2P orchestration' }],
      link: [
        { rel: 'icon', type: 'image/png', href: '/favicon.png' }
      ]
    },
  },
})
