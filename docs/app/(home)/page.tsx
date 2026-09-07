import Link from 'next/link';

export default function HomePage() {
  return (
    <div className="flex flex-col items-center justify-center text-center flex-1 px-4 py-16">
      <h1 className="text-4xl font-extrabold tracking-tight sm:text-5xl mb-4">
        Alexandria
      </h1>
      <p className="max-w-2xl text-lg text-fd-muted-foreground mb-8">
        Decentralized identity, wallet integration, and verifiable credentials.
      </p>
      <div className="flex flex-row gap-4">
        <Link
          href="/docs"
          className="rounded-lg bg-fd-primary px-5 py-2.5 text-sm font-medium text-fd-primary-foreground shadow transition-colors hover:bg-fd-primary/90"
        >
          Explore Documentation
        </Link>
        <a
          href="https://github.com/caparicio-esd/alexandria"
          target="_blank"
          rel="noreferrer"
          className="rounded-lg border border-fd-border px-5 py-2.5 text-sm font-medium transition-colors hover:bg-fd-accent hover:text-fd-accent-foreground"
        >
          GitHub
        </a>
      </div>
    </div>
  );
}
