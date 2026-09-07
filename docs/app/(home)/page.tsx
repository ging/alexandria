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
      category: 'IDSA Vocabulary Hub',
      title: 'Single Source of Semantic Truth',
      description:
        'Author, version, document, and publish domain ontologies, RDF vocabularies, and dataset profiles extending the core IDS Information Model.',
      link: '/docs/getting-started',
    },
    {
      icon: Globe,
      category: 'Runtime Semantics',
      title: 'Dynamic Term Dereferencing',
      description:
        'Connectors resolve unknown attribute IRIs in Self-Descriptions on the fly, dereferencing machine-readable RDF schemas, property classes, and multilingual labels.',
      link: '/docs/getting-started',
    },
    {
      icon: FileCheck,
      category: 'Compliance & Validation',
      title: 'Remote Conformance Tests',
      description:
        'Automated semantic verification using SHACL constraint shapes. Validates that connector payloads, catalogs, and dataset offerings strictly conform to dataspace rules.',
      link: '/docs/development',
    },
    {
      icon: Wallet,
      category: 'Trust & SSI Anchor',
      title: 'Unified Wallet Port',
      description:
        'Cryptographic keys and signing material never reside in the node. Pluggable adapters decouple domain logic from Fafnir and Eclipse EDC IdentityHub.',
      link: '/docs/architecture/wallet-port',
    },
    {
      icon: ShieldCheck,
      category: 'Access Control',
      title: 'Node-Terminated OIDC IAM',
      description:
        'Authentication terminates at the node with encrypted HttpOnly session cookies and PKCE via Zitadel. Protects vocabulary authoring and remote testing endpoints.',
      link: '/docs/development/authentication',
    },
    {
      icon: Layers,
      category: 'Architecture',
      title: 'In-Process Seams & Full TLS',
      description:
        'Engineered as an in-process modular monolith with strict Go package seams, full TLS reverse proxy parity via Caddy, and one-command startup.',
      link: '/docs/architecture/bounded-contexts',
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
            <span>IDSA Reference Architecture · International Data Spaces</span>
          </div>

          {/* Headline */}
          <h1 className="text-4xl sm:text-6xl font-extrabold tracking-tight mb-6 leading-[1.15]">
            The IDS Vocabulary Hub for{' '}
            <span className="text-alexandria-gradient">Sovereign Dataspaces</span>
          </h1>

          {/* Subtitle */}
          <p className="max-w-3xl mx-auto text-base sm:text-lg text-fd-muted-foreground leading-relaxed mb-10">
            Maintain and publish domain ontologies, dereference semantic terms in real-time for dataspace connectors, and run automated remote conformance tests with verifiable trust.
          </p>

          {/* CTAs */}
          <div className="flex flex-wrap items-center justify-center gap-4">
            <Link
              href="/docs/getting-started"
              className="inline-flex items-center gap-2 rounded-xl bg-alexandria-gradient text-white px-6 py-3 text-sm font-semibold shadow-md shadow-[#e6007e]/20 transition-all hover:scale-[1.02] hover:shadow-lg hover:shadow-[#e6007e]/30"
            >
              <span>Explore Vocabulary Hub</span>
              <ArrowRight className="size-4" />
            </Link>

            <Link
              href="/docs/development"
              className="inline-flex items-center gap-2 rounded-xl border border-fd-border bg-fd-card px-6 py-3 text-sm font-semibold transition-colors hover:bg-fd-accent hover:text-fd-accent-foreground shadow-xs"
            >
              <FileCheck className="size-4 text-[#ff3366]" />
              <span>Remote Conformance & API</span>
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
                alt="Alexandria IDSA Vocabulary Hub Ecosystem"
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
                alexandria — dev:auto
              </span>
            </div>
            <span className="text-[11px] font-mono text-fd-muted-foreground">bash</span>
          </div>
          <div className="p-5 font-mono text-xs sm:text-sm bg-black/90 text-neutral-200 overflow-x-auto leading-relaxed">
            <div className="text-neutral-400"># Start the full IDSA stack with TLS, IAM, and wallet link:</div>
            <div className="text-[#ff3e73] mt-1 font-semibold">$ task dev:auto</div>
            <div className="text-neutral-400 mt-3">✔ Caddy Reverse Proxy:      https://alexandria.127.0.0.1.nip.io:8443</div>
            <div className="text-neutral-400">✔ Zitadel IAM Provider:     https://auth.127.0.0.1.nip.io:8443</div>
            <div className="text-neutral-400">✔ Vocabulary Engine:        Active (RDF / OWL / SHACL registry)</div>
            <div className="text-neutral-400">✔ Remote Conformance Suite: Ready for connector validation</div>
            <div className="text-neutral-400">✔ Identity & Wallet Link:   DID resolvable at /.well-known/did.json</div>
            <div className="text-emerald-400 mt-2">✨ Alexandria IDSA Vocabulary Hub running with live reload</div>
          </div>
        </div>
      </section>

      {/* Bottom CTA Banner */}
      <section className="max-w-5xl mx-auto px-4 sm:px-6 pt-12">
        <div className="relative rounded-2xl overflow-hidden p-8 sm:p-12 text-center border border-fd-border bg-fd-card">
          <div className="absolute -top-24 left-1/2 -translate-x-1/2 w-96 h-96 bg-alexandria-gradient rounded-full blur-3xl opacity-10 pointer-events-none" />
          <h2 className="text-2xl sm:text-3xl font-bold tracking-tight mb-4">
            Powering Semantic Interoperability in IDSA Dataspaces
          </h2>
          <p className="max-w-2xl mx-auto text-sm sm:text-base text-fd-muted-foreground mb-8">
            Learn how Alexandria manages vocabulary lifecycles, dereferences term IRIs in real time, and executes remote SHACL conformance tests to certify dataspace data models.
          </p>
          <div className="flex flex-wrap items-center justify-center gap-4">
            <Link
              href="/docs/getting-started"
              className="inline-flex items-center gap-2 rounded-xl bg-fd-primary text-fd-primary-foreground px-6 py-2.5 text-sm font-semibold shadow-xs hover:bg-fd-primary/90 transition-colors"
            >
              <span>Explore the Guide</span>
              <ArrowRight className="size-4" />
            </Link>
            <Link
              href="/docs/development"
              className="inline-flex items-center gap-2 rounded-xl border border-fd-border bg-fd-card px-6 py-2.5 text-sm font-semibold hover:bg-fd-accent hover:text-fd-accent-foreground transition-colors shadow-xs"
            >
              <span>Remote Conformance & Testing</span>
            </Link>
          </div>
        </div>
      </section>
    </div>
  );
}
