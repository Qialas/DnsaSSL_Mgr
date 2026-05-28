import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [react()],
  build: {
    chunkSizeWarningLimit: 1000,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) {
            return undefined;
          }

          if (id.includes('/react/') || id.includes('/react-dom/') || id.includes('/scheduler/')) {
            return 'vendor-react';
          }

          if (id.includes('/@ant-design/icons')) {
            return 'vendor-ant-icons';
          }

          if (id.includes('/@ant-design/pro-components') || id.includes('/@ant-design/pro-')) {
            return 'vendor-ant-pro';
          }

          if (id.includes('/antd/')) {
            return 'vendor-antd';
          }

          if (id.includes('/@rc-component/') || id.includes('/rc-')) {
            return 'vendor';
          }

          if (id.includes('/lodash') || id.includes('/dayjs/') || id.includes('/classnames/')) {
            return 'vendor-utils';
          }

          return 'vendor';
        },
      },
    },
  },
  server: {
    host: '0.0.0.0',
  },
});
