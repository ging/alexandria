# Alexandria Documentation Site

Documentation portal for Alexandria, built with [Fumadocs](https://fumadocs.dev) and [Next.js](https://nextjs.org), deployed statically to GitHub Pages.

## Local Development

From the repository root:
```bash
task docs:dev
```
Or directly from `docs/`:
```bash
pnpm dev
```
Open [http://localhost:3000](http://localhost:3000) in your browser.

## Static Build

To build the static export for GitHub Pages:
```bash
task docs:build
```
Or:
```bash
pnpm build
```
This generates the static files in `docs/out/` with the `/alexandria` base path.

To test the static build locally:
```bash
pnpm start
```
