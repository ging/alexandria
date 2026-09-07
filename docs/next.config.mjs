import { createMDX } from 'fumadocs-mdx/next';

const withMDX = createMDX();

const isProd = process.env.NODE_ENV === 'production';
const basePath = process.env.BASE_PATH !== undefined
  ? process.env.BASE_PATH
  : (isProd ? '/alexandria' : '');

/** @type {import('next').NextConfig} */
const config = {
  output: 'export',
  basePath,
  env: {
    NEXT_PUBLIC_BASE_PATH: basePath,
  },
  trailingSlash: true,
  images: {
    unoptimized: true,
  },
  reactStrictMode: true,
};

export default withMDX(config);
