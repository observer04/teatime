/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        primary: {
          50: '#f0fdf4',
          100: '#dcfce7',
          200: '#bbf7d0',
          300: '#86efac',
          400: '#4ade80',
          500: '#5d8a66',
          600: '#4a7054',
          700: '#3d5c45',
          800: '#334d3a',
          900: '#2a4030',
        },
      },
    },
  },
  plugins: [],
}
