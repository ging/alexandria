import Image from 'next/image';
import Link from 'next/link';
import bannerImg from '@/public/banner.png';
import {
  BookOpen,
  Globe,
  FileCheck,
  Wallet,
  ShieldCheck,
  Layers,
  ArrowRight,
  Sparkles,
  Terminal,
  ExternalLink,
} from 'lucide-react';

export default function HomePage() {
  const features = [
    {
      icon: BookOpen,
      category: 'Profile DSL & Compiler',
      title: 'eunomia.profiles/v1 & vhubc',
      description:
        'Author ODRL policy profiles and SpaceProfiles in YAML. Validate across N1–N5 and deterministically generate RDF, JSON-LD, SHACL, JSON Schema, and Cedar artifacts.',
      link: '/docs/vhub/profiles-dsl',
    },
    {
      icon: Globe,
      category: 'Vocabulary Hub (C-HUB)',
      title: 'Governance & Signed Distribution',
      description:
        'Manage draft-to-release lifecycles, compute transition grace windows, sign eunomia.versions/v1 listings with Ed25519 over JCS, and serve immutable /pub artifacts.',
      link: '/docs/vhub/server',
    },
    {
      icon: ShieldCheck,
      category: 'Edge Sidecar (C-CLI)',
      title: 'Zero-Latency DSP Proxy',
      description:
        'Synchronize verified packages in the background (S1–S13), enforce policy gates on DSP traffic, attach RFC 9651 VHub-Verdict headers, and track agreements.',
      link: '/docs/vhub/client',
    },
    {
      icon: Wallet,
      category: 'Trust & SSI Anchor',
      title: 'Unified Wallet Port',
      description:
        'Cryptographic keys never reside in the node. Pluggable adapters decouple domain logic from Fafnir and Eclipse EDC IdentityHub, supporting DCP and VC 1.0/2.0.',
      link: '/docs/architecture/wallet-port',
    },
    {
      icon: FileCheck,
      category: 'Conformance & F6',
      title: '61 REQ-* & 64 F5 Codes Verified',
      description:
        'Automated conformance suite covering 61 normative requirements, 63 negative N-<CODE> tests, real-binary Scenario F6 reproduction, and TRACEABILITY.md.',
      link: '/docs/development/conformance',
    },
    {
      icon: Layers,
      category: 'Hands-On Learning',
      title: '5 Incremental Tutorials',
      description:
        'Step-by-step tutorials taking you from your first YAML profile in vhubc to vhub-server governance, vhub-client edge enforcement, F6 time travel, and wallets.',
      link: '/docs/tutorials',
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
            <span>Vocabulary Hub v0.2.1 · eunomia.profiles/v1 · Sovereign SSI</span>
          </div>

          {/* Headline */}
          <h1 className="text-4xl sm:text-6xl font-extrabold tracking-tight mb-6 leading-[1.15]">
            The Vocabulary Hub for{' '}
            <span className="text-alexandria-gradient">Sovereign Dataspaces</span>
          </h1>

          {/* Subtitle */}
          <p className="max-w-3xl mx-auto text-base sm:text-lg text-fd-muted-foreground leading-relaxed mb-10">
            Compile dataspace profiles from YAML, govern signed vocabulary lifecycles with vhub-server, enforce DSP compliance at the edge with vhub-client, and manage decentralized identities with Fafnir and IdentityHub.
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
              href="/docs/tutorials"
              className="inline-flex items-center gap-2 rounded-xl border border-fd-border bg-fd-card px-6 py-3 text-sm font-semibold transition-colors hover:bg-fd-accent hover:text-fd-accent-foreground shadow-xs"
            >
              <BookOpen className="size-4 text-[#ff3366]" />
              <span>Incremental Tutorials</span>
            </Link>

            <Link
              href="/docs/vhub"
              className="inline-flex items-center gap-2 rounded-xl border border-fd-border bg-fd-card px-6 py-3 text-sm font-semibold transition-colors hover:bg-fd-accent hover:text-fd-accent-foreground shadow-xs"
            >
              <FileCheck className="size-4 text-[#ff3366]" />
              <span>vhub v0.2.1 & DSL Spec</span>
            </Link>

            <a
              href="https://github.com/ging/alexandria"
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
                alt="Alexandria Vocabulary Hub Ecosystem"
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
            Core Semantic & Architectural Pillars
          </h2>
          <p className="text-sm sm:text-base text-fd-muted-foreground">
            Enabling frictionless semantic interoperability and verified compliance across heterogeneous dataspace connectors.
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
                  <span>Explore documentation</span>
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
                alexandria — vhub & dev:auto
              </span>
            </div>
            <span className="text-[11px] font-mono text-fd-muted-foreground">bash</span>
          </div>
          <div className="p-5 font-mono text-xs sm:text-sm bg-black/90 text-neutral-200 overflow-x-auto leading-relaxed">
            <div className="text-neutral-400"># 1. Build vhub binaries & run the end-to-end F6 lifecycle demo:</div>
            <div className="text-[#ff3e73] mt-1 font-semibold">$ make build && make demo</div>
            <div className="text-neutral-400 mt-2">✔ vhub-server (C-HUB):      Listening with eunomia.profiles/v1 DSL compiler</div>
            <div className="text-neutral-400">✔ vhub-client (C-CLI):      Synced S1–S13 & enforcing DSP gates</div>
            <div className="text-emerald-400">✨ F6 scenario completed:   19/19 rows verified (1.0.0 → 2.0.0 → Revoked)</div>
            <div className="text-neutral-400 mt-4"># 2. Or start the full SSI stack with TLS, Zitadel IAM & Wallet link:</div>
            <div className="text-[#ff3e73] mt-1 font-semibold">$ task dev:auto</div>
            <div className="text-emerald-400 mt-2">✨ Alexandria Vocabulary Hub running at https://alexandria.127.0.0.1.nip.io:8443</div>
          </div>
        </div>
      </section>

      {/* Bottom CTA Banner */}
      <section className="max-w-5xl mx-auto px-4 sm:px-6 pt-12">
        <div className="relative rounded-2xl overflow-hidden p-8 sm:p-12 text-center border border-fd-border bg-fd-card">
          <div className="absolute -top-24 left-1/2 -translate-x-1/2 w-96 h-96 bg-alexandria-gradient rounded-full blur-3xl opacity-10 pointer-events-none" />
          <h2 className="text-2xl sm:text-3xl font-bold tracking-tight mb-4">
            Ready to Build and Govern Sovereign Dataspaces?
          </h2>
          <p className="max-w-2xl mx-auto text-sm sm:text-base text-fd-muted-foreground mb-8">
            Follow our 5-part incremental tutorial series to compile YAML profiles, publish signed releases on vhub-server, validate DSP traffic at the edge, and manage decentralized wallets.
          </p>
          <div className="flex flex-wrap items-center justify-center gap-4">
            <Link
              href="/docs/tutorials"
              className="inline-flex items-center gap-2 rounded-xl bg-fd-primary text-fd-primary-foreground px-6 py-2.5 text-sm font-semibold shadow-xs hover:bg-fd-primary/90 transition-colors"
            >
              <span>Start Tutorial 1</span>
              <ArrowRight className="size-4" />
            </Link>
            <Link
              href="/docs/vhub"
              className="inline-flex items-center gap-2 rounded-xl border border-fd-border bg-fd-card px-6 py-2.5 text-sm font-semibold hover:bg-fd-accent hover:text-fd-accent-foreground transition-colors shadow-xs"
            >
              <span>Vocabulary Hub v0.2.1 Spec</span>
            </Link>
          </div>
        </div>
      </section>
    </div>
  );
}
