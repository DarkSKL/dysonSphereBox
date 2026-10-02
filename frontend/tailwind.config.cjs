/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './index.html',
    './src/**/*.{vue,js,ts,jsx,tsx}',
  ],
  theme: {
    extend: {
      fontFamily: {
        sans: ['var(--ds-font-ui)'],
        display: ['var(--ds-font-display)'],
        mono: ['var(--ds-font-mono)'],
      },
    },
  },
  plugins: [],
}
