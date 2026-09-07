import Image from 'next/image';
import Link from 'next/link';
import bannerImg from '@/public/banner.png';
import {
  Layers,
  Wallet,
  ShieldCheck,
  Network,
  Sliders,
  CheckCircle2,
  ArrowRight,
  Sparkles,
  BookOpen,
  Terminal,
  ExternalLink,
} from 'lucide-react';

export default function HomePage() {
  const features = [
    {
      icon: Layers,
      category: 'Architecture',
      title: 'In-Process Hexagonal Modules',
      description:
        'Engineered with strict Go package boundaries (ssi-auth, auth-proxy) that assemble into an in-process modular monolith with zero network hops.',
      link: '/docs/architecture/bounded-contexts',
    },
    {
      icon: Wallet,
      category: 'Key Custody',
      title: 'Unified Wallet Port',
      description:
        'Private keys never reside in the node. Pluggable adapters decouple domain logic from Fafnir and Eclipse EDC IdentityHub.',
      link: '/docs/architecture/wallet-port',
    },
    {
      icon: ShieldCheck,
      category: 'Security',
      title: 'Node-Terminated OIDC',
      description:
        'Authentication terminates at the node with encrypted HttpOnly session cookies and PKCE. Downstream APIs remain strictly protected.',
      link: '/docs/development/authentication',
    },
    {
      icon: Network,
      category: 'Networking',
      title: 'Complete TLS Parity',
      description:
        'Reverse proxy (Caddy) terminates TLS across all deployment shapes, including development with wildcard nip.io domain routing.',
      link: '/docs/adr/0005-tls-terminated-by-a-proxy-in-development-too',
    },
    {
      icon: Sliders,
      category: 'Configuration',
      title: 'Single Document with Viper',
      description:
        'One zero-secret baseline YAML document discovered automatically, with hierarchical environment variable overrides (ALEXANDRIA_*).',
      link: '/docs/getting-started/configuration',
    },
    {
      icon: CheckCircle2,
      category: 'Verification',
      title: 'Rigorous Testing & Automation',
      description:
        'Single-command automation with Taskfile, race-condition detectors, live wallet integration suites, and GitHub Actions pipelines.',
      link: '/docs/development/testing-and-ci',
    },
  ];

  return (
    <div className="flex flex-col flex-1 pb-20">
      {/* Hero Section */}
      <section className="relative overflow-hidden pt-12 sm:pt-20 pb-16 px-4 sm:px-6">
        <div className="max-w-5xl mx-auto text-center">
          {/* Badge */}
          <div className="inline-flex items-center gap-2 px-3.5 py-1.5 mb-8 rounded-full border border-fd-border bg-fd-card/80 backdrop-blur-md text-xs font-medium text-fd-foreground shadow-xs">
            <Sparkles className="size-3.5 text-[#e6007e]" />
            <span>International Data Spaces (IDS) Vocabulary Hub</span>
          </div>

          {/* Headline */}
          <h1 className="text-4xl sm:text-6xl font-extrabold tracking-tight mb-6 leading-[1.15]">
            The Semantic Core of{' '}
            <span className="text-alexandria-gradient">Alexandria</span>
          </h1>

          {/* Subtitle */}
          <p className="max-w-2xl mx-auto text-base sm:text-lg text-fd-muted-foreground leading-relaxed mb-10">
            Decentralized identity, cryptographic key delegation, and verifiable semantic ontologies engineered for sovereign dataspace participants.
          </p>

          {/* CTAs */}
          <div className="flex flex-wrap items-center justify-center gap-4">
            <Link
              href="/docs/getting-started"
              className="inline-flex items-center gap-2 rounded-xl bg-alexandria-gradient text-white px-6 py-3 text-sm font-semibold shadow-md shadow-[#e6007e]/20 transition-all hover:scale-[1.02] hover:shadow-lg hover:shadow-[#e6007e]/30"
            >
              <span>Get Started</span>
              <ArrowRight className="size-4" />
            </Link>

            <Link
              href="/docs/architecture"
              className="inline-flex items-center gap-2 rounded-xl border border-fd-border bg-fd-card px-6 py-3 text-sm font-semibold transition-colors hover:bg-fd-accent hover:text-fd-accent-foreground shadow-xs"
            >
              <BookOpen className="size-4" />
              <span>Architecture & Wallets</span>
            </Link>

            <a
              href="https://github.com/caparicio-esd/alexandria"
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-2 rounded-xl border border-fd-border/70 bg-fd-card/50 px-5 py-3 text-sm font-medium text-fd-muted-foreground hover:text-fd-foreground hover:border-fd-border transition-colors"
            >
              <span>GitHub</span>
              <ExternalLink className="size-3.5" />
            </a>
          </div>

          {/* Hero Banner Visual */}
          <div className="relative mt-14 sm:mt-18 mx-auto max-w-5xl group">
            <div className="absolute -inset-1 bg-gradient-to-r from-[#e6007e] via-[#ff3366] to-[#ff7a00] rounded-2xl blur-xl opacity-20 dark:opacity-30 transition duration-700 group-hover:opacity-40" />
            <div className="relative overflow-hidden rounded-2xl border border-fd-border bg-fd-card shadow-2xl p-2 sm:p-3">
              <Image
                src={bannerImg}
                alt="Alexandria Architecture Ecosystem Banner"
                priority
                className="w-full h-auto rounded-xl object-cover"
              />
            </div>
          </div>
        </div>
      </section>

      {/* Feature Cards Section */}
      <section className="max-w-6xl mx-auto px-4 sm:px-6 py-16">
        <div className="text-center max-w-2xl mx-auto mb-14">
          <h2 className="text-2xl sm:text-3xl font-bold tracking-tight mb-3">
            Architectural Highlights
          </h2>
          <p className="text-sm sm:text-base text-fd-muted-foreground">
            A cohesive dataspace node built for security, operational clarity, and modular evolution.
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {features.map((feature, idx) => {
            const Icon = feature.icon;
            return (
              <Link
                key={idx}
                href={feature.link}
                className="group relative flex flex-col justify-between p-6 rounded-2xl border border-fd-border bg-fd-card transition-all hover:border-[#ff3366]/50 hover:shadow-lg hover:shadow-[#ff3366]/5"
              >
                <div>
                  <div className="flex items-center justify-between mb-4">
                    <span className="p-2.5 rounded-xl bg-fd-accent text-[#e6007e] dark:text-[#ff3e73] transition-colors group-hover:scale-110">
                      <Icon className="size-5" />
                    </span>
                    <span className="text-xs font-medium text-fd-muted-foreground uppercase tracking-wider">
                      {feature.category}
                    </span>
                  </div>
                  <h3 className="text-base font-semibold mb-2 group-hover:text-[#e6007e] dark:group-hover:text-[#ff3e73] transition-colors">
                    {feature.title}
                  </h3>
                  <p className="text-sm text-fd-muted-foreground leading-relaxed">
                    {feature.description}
                  </p>
                </div>
                <div className="mt-5 flex items-center gap-1.5 text-xs font-semibold text-[#e6007e] dark:text-[#ff3e73]">
                  <span>Explore guide</span>
                  <ArrowRight className="size-3 transition-transform group-hover:translate-x-1" />
                </div>
              </Link>
            );
          })}
        </div>
      </section>

      {/* Quick Launch Terminal Preview */}
      <section className="max-w-4xl mx-auto px-4 sm:px-6 py-8">
        <div className="rounded-2xl border border-fd-border bg-fd-card overflow-hidden shadow-xl">
          <div className="flex items-center justify-between px-4 py-3 border-b border-fd-border bg-fd-muted/50">
            <div className="flex items-center gap-2">
              <span className="size-3 rounded-full bg-red-500/80" />
              <span className="size-3 rounded-full bg-yellow-500/80" />
              <span className="size-3 rounded-full bg-green-500/80" />
              <span className="ml-2 text-xs font-mono text-fd-muted-foreground flex items-center gap-1.5">
                <Terminal className="size-3" />
                alexandria — dev:auto
              </span>
            </div>
            <span className="text-[11px] font-mono text-fd-muted-foreground">bash</span>
          </div>
          <div className="p-5 font-mono text-xs sm:text-sm bg-black/90 text-neutral-200 overflow-x-auto leading-relaxed">
            <div className="text-neutral-400"># Start the full stack with hot reload and automatic dependency wiring:</div>
            <div className="text-[#ff3e73] mt-1 font-semibold">$ task dev:auto</div>
            <div className="text-neutral-400 mt-3">✔ Trusting local root Certificate Authority...</div>
            <div className="text-neutral-400">✔ PostgreSQL 17 active on port 1500</div>
            <div className="text-neutral-400">✔ Zitadel IAM running on https://auth.127.0.0.1.nip.io:8443</div>
            <div className="text-neutral-400">✔ Fafnir wallet connected on port 7003</div>
            <div className="text-emerald-400 mt-2">✨ Node listening on https://alexandria.127.0.0.1.nip.io:8443</div>
            <div className="text-neutral-400">   DID Document ready at /.well-known/did.json</div>
          </div>
        </div>
      </section>

      {/* Bottom CTA Banner */}
      <section className="max-w-5xl mx-auto px-4 sm:px-6 pt-12">
        <div className="relative rounded-2xl overflow-hidden p-8 sm:p-12 text-center border border-fd-border bg-fd-card">
          <div className="absolute -top-24 left-1/2 -translate-x-1/2 w-96 h-96 bg-alexandria-gradient rounded-full blur-3xl opacity-10 pointer-events-none" />
          <h2 className="text-2xl sm:text-3xl font-bold tracking-tight mb-4">
            Dive into the Technical Architecture
          </h2>
          <p className="max-w-xl mx-auto text-sm sm:text-base text-fd-muted-foreground mb-8">
            Read our 7 Architectural Decision Records (ADRs) or learn how to integrate custom wallets and credential schemas.
          </p>
          <div className="flex flex-wrap items-center justify-center gap-4">
            <Link
              href="/docs/adr"
              className="inline-flex items-center gap-2 rounded-xl bg-fd-primary text-fd-primary-foreground px-6 py-2.5 text-sm font-semibold shadow-xs hover:bg-fd-primary/90 transition-colors"
            >
              <span>Read ADRs</span>
              <ArrowRight className="size-4" />
            </Link>
            <Link
              href="/docs/getting-started/quickstart"
              className="inline-flex items-center gap-2 rounded-xl border border-fd-border bg-fd-card px-6 py-2.5 text-sm font-semibold hover:bg-fd-accent hover:text-fd-accent-foreground transition-colors shadow-xs"
            >
              <span>Run Locally</span>
            </Link>
          </div>
        </div>
      </section>
    </div>
  );
}
