import React from "react";
import { AbsoluteFill, Img, staticFile } from "remotion";
import { loadFont } from "@remotion/google-fonts/Inter";

const { fontFamily } = loadFont("normal", {
  weights: ["400", "500", "600", "700"],
  subsets: ["latin"],
});

type Provider = "aws" | "azure" | "gcp" | "hetzner";

type ServerDef = {
  provider: Provider;
  hasGpu: boolean;
};

// Provider metadata — logo/logoDark follow the files in public/logos/
const PROVIDER_META: Record<
  Provider,
  { label: string; color: string; logo: string; logoDark?: string }
> = {
  aws: {
    label: "AWS",
    color: "#C47B12",
    logo: "logos/aws.svg",
    logoDark: "logos/aws-dark.svg",
  },
  azure: { label: "Azure", color: "#0078D4", logo: "logos/azure.svg" },
  gcp: { label: "GCP", color: "#4285F4", logo: "logos/gcp.svg" },
  hetzner: { label: "Hetzner", color: "#D50C2D", logo: "logos/hetzner.svg" },
};

// Theme-aware palette
const THEMES = {
  light: {
    bg: "#ffffff",
    k8sBorder: "#326CE5",
    k8sBg: "#F0F4FF",
    k8sText: "#1E3A5F",
    controlBorder: "#94A3B8",
    controlBg: "#F8FAFC",
    controlText: "#475569",
    computeBorder: "#86EFAC",
    computeBg: "#F0FDF4",
    computeText: "#166534",
    stroke: "#94A3B8",
    text: "#1E293B",
    textSecondary: "#64748B",
    serverFill: (c: string) => `${c}12`,
    serverStroke: (c: string) => c,
    shadow: "0 1px 4px rgba(15, 23, 42, 0.05)",
  },
  dark: {
    bg: "#1a1d23",
    k8sBorder: "#4A8AF4",
    k8sBg: "#1E2433",
    k8sText: "#8AB4F8",
    controlBorder: "#4A5568",
    controlBg: "#1E2430",
    controlText: "#94A3B8",
    computeBorder: "#2D5A3D",
    computeBg: "#1A2E22",
    computeText: "#6EE7A0",
    stroke: "#7A8A9C",
    text: "#E2E8F0",
    textSecondary: "#94A3B8",
    serverFill: (c: string) => `${c}20`,
    serverStroke: (c: string) => `${c}AA`,
    shadow: "0 1px 4px rgba(0, 0, 0, 0.3)",
  },
  transparent: {
    bg: "transparent",
    k8sBorder: "#326CE5",
    k8sBg: "rgba(240, 244, 255, 0.5)",
    k8sText: "#1E3A5F",
    controlBorder: "#94A3B8",
    controlBg: "rgba(248, 250, 252, 0.5)",
    controlText: "#475569",
    computeBorder: "#86EFAC",
    computeBg: "rgba(240, 253, 244, 0.5)",
    computeText: "#166534",
    stroke: "#94A3B8",
    text: "#1E293B",
    textSecondary: "#64748B",
    serverFill: (c: string) => `${c}12`,
    serverStroke: (c: string) => c,
    shadow: "0 1px 4px rgba(15, 23, 42, 0.05)",
  },
};

const GPU_COLOR = "#4A7A4A";

// Control plane: 3 servers, each a different provider, no GPUs
const CONTROL_SERVERS: Provider[] = ["aws", "gcp", "hetzner"];

// Compute: 12 servers, randomly mixed providers, 3 with GPUs — 6x2 grid
const COMPUTE_ROWS: ServerDef[][] = [
  [
    { provider: "aws", hasGpu: false },
    { provider: "gcp", hasGpu: true },
    { provider: "hetzner", hasGpu: false },
    { provider: "azure", hasGpu: false },
    { provider: "gcp", hasGpu: false },
    { provider: "aws", hasGpu: true },
  ],
  [
    { provider: "hetzner", hasGpu: false },
    { provider: "azure", hasGpu: false },
    { provider: "azure", hasGpu: false },
    { provider: "hetzner", hasGpu: false },
    { provider: "gcp", hasGpu: false },
    { provider: "aws", hasGpu: true },
  ],
];

const COL_W = 120;
const CONTROL_COL_W = 80;

// GPU card SVG — sits directly below the server rack as its "base"
const GpuCard: React.FC = () => (
  <svg width="64" height="18" viewBox="0 0 64 18">
    <rect
      x="2"
      y="1"
      width="60"
      height="16"
      rx="3"
      fill={`${GPU_COLOR}15`}
      stroke={GPU_COLOR}
      strokeWidth="1.5"
    />
    <rect x="6" y="4" width="9" height="10" rx="1.5" fill={GPU_COLOR} opacity="0.3" />
    <rect x="18" y="4" width="9" height="10" rx="1.5" fill={GPU_COLOR} opacity="0.3" />
    <text
      x="44"
      y="13"
      textAnchor="middle"
      fill={GPU_COLOR}
      fontSize="10"
      fontWeight="700"
      fontFamily="Inter, Arial, sans-serif"
    >
      GPU
    </text>
  </svg>
);

// 2U server node (for compute)
const ServerNode: React.FC<{
  server: ServerDef;
  theme: "light" | "dark" | "transparent";
}> = ({ server, theme }) => {
  const meta = PROVIDER_META[server.provider];
  const t = THEMES[theme];
  const logo =
    theme === "dark" && meta.logoDark ? meta.logoDark : meta.logo;

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        gap: 0,
        width: COL_W,
      }}
    >
      {/* Provider logo */}
      <Img
        src={staticFile(logo)}
        style={{ width: 22, height: 22, objectFit: "contain" }}
      />

      <div style={{ height: 4 }} />

      {/* Server rack icon — 2U */}
      <svg width="56" height="42" viewBox="0 0 56 42">
        <rect
          x="2"
          y="1"
          width="52"
          height="19"
          rx="3.5"
          fill={t.serverFill(meta.color)}
          stroke={t.serverStroke(meta.color)}
          strokeWidth="1.5"
        />
        <circle cx="44" cy="10.5" r="3" fill={t.serverStroke(meta.color)} opacity="0.55" />
        <line
          x1="8"
          y1="10.5"
          x2="34"
          y2="10.5"
          stroke={t.serverStroke(meta.color)}
          strokeWidth="1.2"
          opacity="0.35"
        />
        <rect
          x="2"
          y="22"
          width="52"
          height="19"
          rx="3.5"
          fill={t.serverFill(meta.color)}
          stroke={t.serverStroke(meta.color)}
          strokeWidth="1.5"
        />
        <circle cx="44" cy="31.5" r="3" fill={t.serverStroke(meta.color)} opacity="0.55" />
        <line
          x1="8"
          y1="31.5"
          x2="34"
          y2="31.5"
          stroke={t.serverStroke(meta.color)}
          strokeWidth="1.2"
          opacity="0.35"
        />
      </svg>

      {/* GPU card — directly below server rack as its base */}
      {server.hasGpu && <GpuCard />}
    </div>
  );
};

// 1U server node (for control plane)
const SmallServerNode: React.FC<{
  provider: Provider;
  theme: "light" | "dark" | "transparent";
}> = ({ provider, theme }) => {
  const meta = PROVIDER_META[provider];
  const t = THEMES[theme];
  const logo =
    theme === "dark" && meta.logoDark ? meta.logoDark : meta.logo;

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        gap: 0,
        width: CONTROL_COL_W,
      }}
    >
      {/* Provider logo */}
      <Img
        src={staticFile(logo)}
        style={{ width: 20, height: 20, objectFit: "contain" }}
      />

      <div style={{ height: 4 }} />

      {/* Server rack icon — 1U */}
      <svg width="50" height="22" viewBox="0 0 50 22">
        <rect
          x="2"
          y="1"
          width="46"
          height="20"
          rx="3.5"
          fill={t.serverFill(meta.color)}
          stroke={t.serverStroke(meta.color)}
          strokeWidth="1.5"
        />
        <circle cx="40" cy="11" r="3" fill={t.serverStroke(meta.color)} opacity="0.55" />
        <line
          x1="8"
          y1="11"
          x2="30"
          y2="11"
          stroke={t.serverStroke(meta.color)}
          strokeWidth="1.2"
          opacity="0.35"
        />
      </svg>
    </div>
  );
};

// Layout
const CLAUDIE_LOGO_W = 120;
const GAP = 50;
const K8S_BOX_W = 1060;
const K8S_BOX_H = 420;
const TOTAL_W = CLAUDIE_LOGO_W + GAP + K8S_BOX_W;

export type ClusterDiagramProps = {
  theme: "transparent" | "light" | "dark";
};

export const ClusterDiagram: React.FC<ClusterDiagramProps> = ({ theme }) => {
  const t = THEMES[theme];
  const claudieCenterY = K8S_BOX_H / 2;
  const k8sLeft = CLAUDIE_LOGO_W + GAP;

  return (
    <AbsoluteFill
      style={{
        backgroundColor: t.bg,
        fontFamily,
        justifyContent: "center",
        alignItems: "center",
      }}
    >
      <div
        style={{
          position: "relative",
          width: TOTAL_W,
          height: K8S_BOX_H,
        }}
      >
        {/* Dashed connection line: Claudie -- K8s */}
        <svg
          style={{
            position: "absolute",
            top: 0,
            left: 0,
            width: "100%",
            height: "100%",
            pointerEvents: "none",
          }}
        >
          <line
            x1={CLAUDIE_LOGO_W}
            y1={claudieCenterY}
            x2={k8sLeft}
            y2={claudieCenterY}
            stroke={t.stroke}
            strokeWidth="1.5"
            strokeDasharray="8 5"
          />
        </svg>

        {/* ── Claudie logo (left) ── */}
        <div
          style={{
            position: "absolute",
            left: 0,
            top: claudieCenterY - CLAUDIE_LOGO_W / 2,
            width: CLAUDIE_LOGO_W,
            height: CLAUDIE_LOGO_W,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          <Img
            src={staticFile("logos/claudie.svg")}
            style={{
              width: CLAUDIE_LOGO_W,
              height: CLAUDIE_LOGO_W,
              borderRadius: 16,
              objectFit: "contain",
            }}
          />
        </div>

        {/* ── Outer box: Kubernetes Cluster ── */}
        <div
          style={{
            position: "absolute",
            left: k8sLeft,
            top: 0,
            width: K8S_BOX_W,
            height: K8S_BOX_H,
            borderRadius: 16,
            border: `2px solid ${t.k8sBorder}`,
            backgroundColor: t.k8sBg,
            padding: 14,
            display: "flex",
            flexDirection: "column",
            gap: 10,
            boxShadow: t.shadow,
          }}
        >
          {/* K8s header */}
          <div
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              gap: 10,
            }}
          >
            <Img
              src={staticFile("logos/kubernetes.svg")}
              style={{ width: 28, height: 28 }}
            />
            <span
              style={{
                fontSize: 16,
                fontWeight: 700,
                color: t.k8sText,
                letterSpacing: 0.5,
              }}
            >
              Single Multi-Cloud Kubernetes Cluster
            </span>
          </div>

          {/* Control + Compute side by side */}
          <div
            style={{
              flex: 1,
              display: "flex",
              gap: 12,
            }}
          >
            {/* ── Control box ── */}
            <div
              style={{
                width: 280,
                borderRadius: 12,
                border: `1.5px solid ${t.controlBorder}`,
                backgroundColor: t.controlBg,
                padding: 10,
                display: "flex",
                flexDirection: "column",
                gap: 8,
              }}
            >
              {/* Control header */}
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                }}
              >
                <span
                  style={{
                    fontSize: 11,
                    fontWeight: 700,
                    color: t.controlText,
                    letterSpacing: 1,
                    textTransform: "uppercase" as const,
                  }}
                >
                  Control Plane
                </span>
              </div>

              {/* Control servers — single row of 3 */}
              <div
                style={{
                  flex: 1,
                  display: "flex",
                  justifyContent: "space-evenly",
                  alignItems: "center",
                }}
              >
                {CONTROL_SERVERS.map((provider, i) => (
                  <SmallServerNode key={i} provider={provider} theme={theme} />
                ))}
              </div>
            </div>

            {/* ── Compute box ── */}
            <div
              style={{
                flex: 1,
                borderRadius: 12,
                border: `1.5px solid ${t.computeBorder}`,
                backgroundColor: t.computeBg,
                padding: 10,
                display: "flex",
                flexDirection: "column",
                gap: 8,
              }}
            >
              {/* Compute header */}
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                }}
              >
                <span
                  style={{
                    fontSize: 11,
                    fontWeight: 700,
                    color: t.computeText,
                    letterSpacing: 1,
                    textTransform: "uppercase" as const,
                  }}
                >
                  Compute
                </span>
              </div>

              {/* Compute servers — 2 rows x 6 cols */}
              <div
                style={{
                  flex: 1,
                  display: "flex",
                  flexDirection: "column",
                  justifyContent: "space-evenly",
                }}
              >
                {COMPUTE_ROWS.map((row, ri) => (
                  <div
                    key={ri}
                    style={{
                      display: "flex",
                      justifyContent: "space-evenly",
                      alignItems: "flex-end",
                    }}
                  >
                    {row.map((server, ci) => (
                      <ServerNode key={ci} server={server} theme={theme} />
                    ))}
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>
    </AbsoluteFill>
  );
};
