import Image from 'next/image';
import Link from 'next/link';
import bannerImg from '@/public/banner.png';
import {
  BookOpen,
  Globe,
  FileCheck,
  KeyRound,
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
      title: 'eunomia.profiles/v1 & alexandriac',
      description:
        'Author ODRL policy profiles and SpaceProfiles in YAML. Validate across N1–N5 and deterministically generate RDF, JSON-LD, SHACL, JSON Schema, and Cedar artifacts.',
      link: '/docs/vhub/profiles-dsl',
    },
    {
      icon: Globe,
      category: 'Vocabulary Hub (C-HUB)',
      title: 'alexandria-server Governance & /pub',
      description:
        'Manage namespaces, stewards, hosted & proxy vocabularies, draft-to-release lifecycles, W3C VC 2.0 eddsa-jcs-2022 listings, and immutable /pub distribution.',
      link: '/docs/vhub/server',
    },
    {
      icon: ShieldCheck,
      category: 'Edge Sidecar (C-CLI)',
      title: 'alexandria-client L7 DSP Proxy',
      description:
        'Synchronize verified packages in the background (S1–S13), enforce dynamic policy rules on DSP traffic, attach RFC 9651 VHub-Verdict headers, and track agreements.',
      link: '/docs/vhub/client',
    },
    {
      icon: KeyRound,
      category: 'Cryptographic Trust',
      title: 'VC 2.0, DID & OAuth 2.0 JWKS',
      description:
        'Verify Data Integrity eddsa-jcs-2022 proofs offline via did:key and did:web (V0–V4), and guard /v1 management APIs with RFC 9068 at+jwt access tokens (A1–A5).',
      link: '/docs/architecture/credentials-and-trust',
    },
    {
      icon: FileCheck,
      category: 'Conformance & F6',
      title: '81 REQ-* & 77 F5 Codes Verified',
      description:
        'Automated conformance suite covering all 81 normative requirements, 76/77 negative N-<CODE> tests, real-binary 29-row Scenario F6 replay, and TRACEABILITY.md.',
      link: '/docs/development/conformance',
    },
    {
      icon: Layers,
      category: 'Hands-On Learning',
      title: '5 Interactive Jupyter Tutorials',
      description:
        'Incremental notebooks and web guides with interactive HTML widgets covering DSL compilation, Hub governance, DSP edge proxy, F6 time travel, and resilience.',
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
            <span>Specification v0.3.0 · W3C VC 2.0 eddsa-jcs-2022 · eunomia.profiles/v1</span>
          </div>

          {/* Headline */}
          <h1 className="text-4xl sm:text-6xl font-extrabold tracking-tight mb-6 leading-[1.15]">
            The Vocabulary Hub for{' '}
            <span className="text-alexandria-gradient">Interoperable Dataspaces</span>
          </h1>

          {/* Subtitle */}
          <p className="max-w-3xl mx-auto text-base sm:text-lg text-fd-muted-foreground leading-relaxed mb-10">
            Compile dataspace profiles from YAML with alexandriac, govern W3C VC 2.0 signed vocabulary lifecycles with alexandria-server, and enforce zero-hub-latency DSP compliance at the edge with alexandria-client.
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
              <span>Interactive Tutorials</span>
            </Link>

            <Link
              href="/docs/vhub"
              className="inline-flex items-center gap-2 rounded-xl border border-fd-border bg-fd-card px-6 py-3 text-sm font-semibold transition-colors hover:bg-fd-accent hover:text-fd-accent-foreground shadow-xs"
            >
              <FileCheck className="size-4 text-[#ff3366]" />
              <span>Specification v0.3.0</span>
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
            Enabling frictionless semantic interoperability and cryptographically verified compliance across Dataspace Protocol (DSP) connectors.
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
                alexandria — build, demo & jupyter tutorials
              </span>
            </div>
            <span className="text-[11px] font-mono text-fd-muted-foreground">bash</span>
          </div>
          <div className="p-5 font-mono text-xs sm:text-sm bg-black/90 text-neutral-200 overflow-x-auto leading-relaxed">
            <div className="text-neutral-400"># 1. Build all binaries & run the end-to-end F6 lifecycle demo:</div>
            <div className="text-[#ff3e73] mt-1 font-semibold">$ make build && make demo</div>
            <div className="text-neutral-400 mt-2">✔ alexandria-server (C-HUB):  Listening on https://127.0.0.1:8443 (did:key)</div>
            <div className="text-neutral-400">✔ alexandria-client (C-CLI):  Synced S1–S13 & enforcing DSP gates</div>
            <div className="text-emerald-400">✨ F6 scenario completed:     29/29 rows verified (1.0.0 → 2.0.0 → Revoked)</div>
            <div className="text-neutral-400 mt-4"># 2. Or launch the interactive JupyterLab notebook environment:</div>
            <div className="text-[#ff3e73] mt-1 font-semibold">$ make tutorial</div>
            <div className="text-emerald-400 mt-2">✨ JupyterLab running at http://127.0.0.1:8888/lab/tree/01_dsl_and_alexandriac_compiler.ipynb</div>
          </div>
        </div>
      </section>

      {/* Bottom CTA Banner */}
      <section className="max-w-5xl mx-auto px-4 sm:px-6 pt-12">
        <div className="relative rounded-2xl overflow-hidden p-8 sm:p-12 text-center border border-fd-border bg-fd-card">
          <div className="absolute -top-24 left-1/2 -translate-x-1/2 w-96 h-96 bg-alexandria-gradient rounded-full blur-3xl opacity-10 pointer-events-none" />
          <h2 className="text-2xl sm:text-3xl font-bold tracking-tight mb-4">
            Ready to Govern Semantic Profiles Across Your Dataspace?
          </h2>
          <p className="max-w-2xl mx-auto text-sm sm:text-base text-fd-muted-foreground mb-8">
            Follow our 5-part incremental tutorial series to compile YAML profiles, publish VC 2.0 signed releases on alexandria-server, validate DSP traffic at the edge, and replay breaking transitions.
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
              <span>Specification v0.3.0 Overview</span>
            </Link>
          </div>
        </div>
      </section>
    </div>
  );
}
