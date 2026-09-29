import { defineConfig } from '@rsbuild/core';
import { pluginReact } from '@rsbuild/plugin-react';
import { pluginTailwindcss } from '@rsbuild/plugin-tailwindcss';

const backendPort = process.env.NASVIA_PORT ?? '3720';

export default defineConfig({
  plugins: [pluginReact(), pluginTailwindcss()],
  html: {
    template: './index.html',
  },
  source: {
    entry: {
      index: './src/main.tsx',
    },
  },
  output: {
    distPath: {
      root: 'dist',
      js: '',
      css: '',
      media: '',
      image: '',
      svg: '',
      font: '',
    },
    // 资源统一放到 /assets/ 下，便于服务端设置长缓存
    filename: {
      js: 'assets/[name].[contenthash:8].js',
      css: 'assets/[name].[contenthash:8].css',
    },
    assetPrefix: '/',
    cleanDistPath: true,
    // 生产构建不生成 source map，保持产物轻量
    sourceMap: false,
  },
  performance: {
    removeConsole: ['log'],
    chunkSplit: {
      strategy: 'split-by-experience',
    },
  },
  server: {
    port: 5180,
    proxy: {
      '/api': `http://127.0.0.1:${backendPort}`,
    },
  },
});
