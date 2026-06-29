import React from "react";
import {
  AbsoluteFill,
  Img,
  interpolate,
  spring,
  staticFile,
  useCurrentFrame,
  useVideoConfig,
} from "remotion";
import { loadFont } from "@remotion/google-fonts/Inter";

const { fontFamily } = loadFont("normal", {
  weights: ["400", "500", "600", "700"],
  subsets: ["latin"],
});

// ── Types ──────────────────────────────────────────────────

type Provider = "aws" | "azure" | "gcp" | "hetzner";

type ServerDef = {
  provider: Provider;
  hasGpu: boolean;
};

// ── Provider metadata ──────────────────────────────────────

const PROVIDER_META: Record<
  Provider,
  { color: string; logo: string; logoDark?: string }
> = {
  aws: {
    color: "#C47B12",
    logo: "logos/aws.svg",
    logoDark: "logos/aws-dark.svg",
  },
  azure: { color: "#0078D4", logo: "logos/azure.svg" },
  gcp: { color: "#4285F4", logo: "logos/gcp.svg" },
  hetzner: { color: "#D50C2D", logo: "logos/hetzner.svg" },
};

// ── Theme palette ──────────────────────────────────────────

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
    serverFill: (c: string) => `${c}12`,
    serverStroke: (c: string) => c,
    shadow: "0 1px 4px rgba(15, 23, 42, 0.05)",
  },
  "transparent-dark": {
    bg: "transparent",
    k8sBorder: "#4A8AF4",
    k8sBg: "rgba(30, 36, 51, 0.5)",
    k8sText: "#8AB4F8",
    controlBorder: "#4A5568",
    controlBg: "rgba(30, 36, 48, 0.5)",
    controlText: "#94A3B8",
    computeBorder: "#2D5A3D",
    computeBg: "rgba(26, 46, 34, 0.5)",
    computeText: "#6EE7A0",
    stroke: "#7A8A9C",
    text: "#E2E8F0",
    serverFill: (c: string) => `${c}20`,
    serverStroke: (c: string) => `${c}AA`,
    shadow: "0 1px 4px rgba(0, 0, 0, 0.3)",
  },
};

type Theme = keyof typeof THEMES;

// ── Shared visual components ───────────────────────────────

const GPU_COLOR = "#4A7A4A";
const COL_W = 165;
const CONTROL_COL_W = 110;

const GpuCard: React.FC = () => (
  <svg width="88" height="25" viewBox="0 0 64 18">
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

const ServerNode: React.FC<{ server: ServerDef; theme: Theme }> = ({
  server,
  theme,
}) => {
  const meta = PROVIDER_META[server.provider];
  const t = THEMES[theme];
  const isDark = theme === "dark" || theme === "transparent-dark";
  const logo = isDark && meta.logoDark ? meta.logoDark : meta.logo;

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
      <Img
        src={staticFile(logo)}
        style={{ width: 31, height: 31, objectFit: "contain" }}
      />
      <div style={{ height: 6 }} />
      <svg width="77" height="58" viewBox="0 0 56 42">
        <rect x="2" y="1" width="52" height="19" rx="3.5"
          fill={t.serverFill(meta.color)} stroke={t.serverStroke(meta.color)} strokeWidth="1.5" />
        <circle cx="44" cy="10.5" r="3" fill={t.serverStroke(meta.color)} opacity="0.55" />
        <line x1="8" y1="10.5" x2="34" y2="10.5"
          stroke={t.serverStroke(meta.color)} strokeWidth="1.2" opacity="0.35" />
        <rect x="2" y="22" width="52" height="19" rx="3.5"
          fill={t.serverFill(meta.color)} stroke={t.serverStroke(meta.color)} strokeWidth="1.5" />
        <circle cx="44" cy="31.5" r="3" fill={t.serverStroke(meta.color)} opacity="0.55" />
        <line x1="8" y1="31.5" x2="34" y2="31.5"
          stroke={t.serverStroke(meta.color)} strokeWidth="1.2" opacity="0.35" />
      </svg>
      {server.hasGpu && <GpuCard />}
    </div>
  );
};

const SmallServerNode: React.FC<{ provider: Provider; theme: Theme }> = ({
  provider,
  theme,
}) => {
  const meta = PROVIDER_META[provider];
  const t = THEMES[theme];
  const isDark = theme === "dark" || theme === "transparent-dark";
  const logo = isDark && meta.logoDark ? meta.logoDark : meta.logo;

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
      <Img
        src={staticFile(logo)}
        style={{ width: 28, height: 28, objectFit: "contain" }}
      />
      <div style={{ height: 6 }} />
      <svg width="69" height="31" viewBox="0 0 50 22">
        <rect x="2" y="1" width="46" height="20" rx="3.5"
          fill={t.serverFill(meta.color)} stroke={t.serverStroke(meta.color)} strokeWidth="1.5" />
        <circle cx="40" cy="11" r="3" fill={t.serverStroke(meta.color)} opacity="0.55" />
        <line x1="8" y1="11" x2="30" y2="11"
          stroke={t.serverStroke(meta.color)} strokeWidth="1.2" opacity="0.35" />
      </svg>
    </div>
  );
};

// ── Layout constants ───────────────────────────────────────

const CLAUDIE_LOGO_SIZE = 70;
const VERTICAL_GAP = 20;
const K8S_BOX_H = 420;
const BOX_PADDING = 12;
const INNER_GAP = 10;
const CONTROL_BOX_W = 165;
const COMPUTE_PADDING = 10;
// Height of a server node: logo(31) + spacer(6) + rack(58) = 95, with GPU +25 = 120
const SERVER_H = 95;
const SERVER_GPU_H = 120;

// K8s header row height (icon is 28px, text line-height matches)
const K8S_HEADER_H = 28;
// "COMPUTE" label height (fontSize 11, ~16px line height)
const COMPUTE_LABEL_H = 16;

// Inner box row height = K8s box minus outer padding (both sides), header, and gap
const INNER_BOX_H = K8S_BOX_H - BOX_PADDING * 2 - K8S_HEADER_H - 10;
// Compute content area = inner box minus compute padding (both sides), label, and gap
const COMPUTE_CONTENT_H = INNER_BOX_H - COMPUTE_PADDING * 2 - COMPUTE_LABEL_H - 8;
// Reserve bottom margin so second row doesn't hug the box border
const BOTTOM_MARGIN = 20;
// Row height: half the usable area (after margin)
const ROW_H = (COMPUTE_CONTENT_H - BOTTOM_MARGIN) / 2;

const CONTROL_SERVERS: Provider[] = ["aws", "gcp", "hetzner"];

// ── Derived dimension helpers ──────────────────────────────

function computeContentW(numCols: number): number {
  return numCols * COL_W;
}

function computeBoxW(numCols: number): number {
  return computeContentW(numCols) + 2 * COMPUTE_PADDING;
}

function k8sBoxW(numCols: number): number {
  return CONTROL_BOX_W + INNER_GAP + computeBoxW(numCols) + 2 * BOX_PADDING;
}

// Server X within the compute content area (space-evenly style)
function serverX(col: number, numCols: number, contentW: number): number {
  const gap = (contentW - numCols * COL_W) / (numCols + 1);
  return gap * (col + 1) + COL_W * col;
}

// Server Y within the compute content area (bottom-aligned within row)
function serverY(row: number, hasGpu: boolean): number {
  const h = hasGpu ? SERVER_GPU_H : SERVER_H;
  return row * ROW_H + (ROW_H - h);
}

// ── Phase timing (frames at 30fps) ────────────────────────

const P2_START = 30;   // spawn 4 hetzner
const P3_START = 90;   // remove 3 original
const P4_START = 135;  // compact to 5+4
const P5_START = 180;  // spawn 3 GPU
const P6_START = 270;  // remove non-originals (return journey)
const P7_START = 315;  // compact + slide to 4-col original layout
const P8_START = 360;  // spawn s3, s5, s7 back

// ── Signal animation ──────────────────────────────────────
// A signal fires before each modification phase: glow on Claudie logo
// then a dot travels down the dashed line to the K8s box.
const SIGNAL_DUR = 18;  // total signal duration in frames
const SIGNAL_PHASES = [P2_START, P3_START, P4_START, P5_START, P6_START, P7_START, P8_START];
const SIGNAL_COLOR = "#4A8AF4"; // blue neon

// ── Server definitions per phase ───────────────────────────

type AnimServer = {
  id: string;
  provider: Provider;
  hasGpu: boolean;
};

// Phase 1: 8 initial servers (some with GPUs)
const INITIAL_SERVERS: AnimServer[] = [
  { id: "s0", provider: "aws", hasGpu: false },
  { id: "s1", provider: "gcp", hasGpu: true },
  { id: "s2", provider: "hetzner", hasGpu: false },
  { id: "s3", provider: "azure", hasGpu: false },
  { id: "s4", provider: "hetzner", hasGpu: false },
  { id: "s5", provider: "azure", hasGpu: false },
  { id: "s6", provider: "gcp", hasGpu: false },
  { id: "s7", provider: "aws", hasGpu: true },
];

// Grid positions for Phase 1: 4x2
const PHASE1_POS: Record<string, [number, number]> = {
  s0: [0, 0], s1: [1, 0], s2: [2, 0], s3: [3, 0],
  s4: [0, 1], s5: [1, 1], s6: [2, 1], s7: [3, 1],
};

// Phase 2: add 4 Hetzner servers
const PHASE2_NEW: AnimServer[] = [
  { id: "s8", provider: "hetzner", hasGpu: false },
  { id: "s9", provider: "hetzner", hasGpu: false },
  { id: "s10", provider: "hetzner", hasGpu: false },
  { id: "s11", provider: "hetzner", hasGpu: false },
];

// Grid positions after Phase 2: 6x2
const PHASE2_POS: Record<string, [number, number]> = {
  s0: [0, 0], s1: [1, 0], s2: [2, 0], s3: [3, 0], s8: [4, 0], s9: [5, 0],
  s4: [0, 1], s5: [1, 1], s6: [2, 1], s7: [3, 1], s10: [4, 1], s11: [5, 1],
};

// Phase 3: IDs to remove
const PHASE3_REMOVE = ["s3", "s5", "s7"];

// Phase 4: compacted positions (5+4 grid)
const PHASE4_POS: Record<string, [number, number]> = {
  s0: [0, 0], s1: [1, 0], s2: [2, 0], s8: [3, 0], s9: [4, 0],
  s4: [0, 1], s6: [1, 1], s10: [2, 1], s11: [3, 1],
};

// Phase 5: add 3 GCP GPU servers
const PHASE5_NEW: AnimServer[] = [
  { id: "g0", provider: "gcp", hasGpu: true },
  { id: "g1", provider: "gcp", hasGpu: true },
  { id: "g2", provider: "gcp", hasGpu: true },
];

// Grid positions after Phase 5: 6x2 (5+4 + 3 new)
const PHASE5_POS: Record<string, [number, number]> = {
  s0: [0, 0], s1: [1, 0], s2: [2, 0], s8: [3, 0], s9: [4, 0], g0: [5, 0],
  s4: [0, 1], s6: [1, 1], s10: [2, 1], s11: [3, 1], g1: [4, 1], g2: [5, 1],
};

// Phase 6: IDs to remove (all non-original servers)
const PHASE6_REMOVE = ["s8", "s9", "s10", "s11", "g0", "g1", "g2"];

// Phase 7: compacted positions for the 5 surviving originals (in 4-col grid)
// s6 slides from (1,1) in PHASE5_POS to (2,1) in PHASE1_POS
const PHASE7_POS: Record<string, [number, number]> = {
  s0: [0, 0], s1: [1, 0], s2: [2, 0],
  s4: [0, 1], s6: [2, 1],
};

// Phase 8: re-spawn these original servers
const PHASE8_RESPAWN: AnimServer[] = [
  { id: "s3", provider: "azure", hasGpu: false },
  { id: "s5", provider: "azure", hasGpu: false },
  { id: "s7", provider: "aws", hasGpu: true },
];

// ── Renderable server state ────────────────────────────────

type RenderServer = {
  id: string;
  provider: Provider;
  hasGpu: boolean;
  x: number;
  y: number;
  opacity: number;
  scale: number;
};

// ── Main component ─────────────────────────────────────────

export type ClusterDiagramAnimationProps = {
  theme: Theme;
};

export const ClusterDiagramAnimation: React.FC<
  ClusterDiagramAnimationProps
> = ({ theme }) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const t = THEMES[theme];

  const springConf = { damping: 200 };

  // ── Signal animation state ────────────────────────────────
  let signalProgress = -1;
  for (const phaseStart of SIGNAL_PHASES) {
    const sigStart = phaseStart - SIGNAL_DUR;
    if (frame >= sigStart && frame < phaseStart) {
      signalProgress = (frame - sigStart) / SIGNAL_DUR;
      break;
    }
  }

  // Glow intensity: peaks in first half (0→1→0), biased early
  const glowIntensity =
    signalProgress >= 0
      ? signalProgress < 0.5
        ? signalProgress * 2
        : (1 - signalProgress) * 2
      : 0;

  // Dot position along the dashed line: 0→1 during second half of signal
  const dotT =
    signalProgress >= 0.3
      ? Math.min(1, (signalProgress - 0.3) / 0.7)
      : -1;

  // Helper: spring that goes from 0 to 1
  const sp = (start: number, dur: number, delay = 0) =>
    spring({
      frame: Math.max(0, frame - start - delay),
      fps,
      config: springConf,
      durationInFrames: dur,
    });

  // Helper: pixel position for a col/row in an N-col grid
  const posAt = (
    col: number,
    row: number,
    hasGpu: boolean,
    nCols: number,
  ) => ({
    x: serverX(col, nCols, computeContentW(nCols)),
    y: serverY(row, hasGpu),
  });

  // Helper: interpolate between two positions
  const lerpPos = (
    a: { x: number; y: number },
    b: { x: number; y: number },
    t: number,
  ) => ({
    x: interpolate(t, [0, 1], [a.x, b.x]),
    y: interpolate(t, [0, 1], [a.y, b.y]),
  });

  // ── Animated column count ──────────────────────────────

  let animCols: number;
  if (frame < P2_START) {
    animCols = 4;
  } else if (frame < P3_START) {
    animCols = interpolate(sp(P2_START, 50), [0, 1], [4, 6]);
  } else if (frame < P4_START) {
    animCols = 6;
  } else if (frame < P5_START) {
    animCols = interpolate(sp(P4_START, 40), [0, 1], [6, 5]);
  } else if (frame < P6_START) {
    animCols = interpolate(sp(P5_START, 50), [0, 1], [5, 6]);
  } else if (frame < P7_START) {
    animCols = 6;
  } else if (frame < P8_START) {
    animCols = interpolate(sp(P7_START, 40), [0, 1], [6, 4]);
  } else {
    animCols = 4;
  }

  const currentK8sW = k8sBoxW(animCols);
  const currentComputeW = computeBoxW(animCols);

  // ── Build renderable servers ───────────────────────────

  const servers: RenderServer[] = [];

  // --- Original 8 servers (INITIAL_SERVERS) ---
  for (const srv of INITIAL_SERVERS) {
    const isP3Removed = PHASE3_REMOVE.includes(srv.id);
    const p1 = PHASE1_POS[srv.id];
    const p2 = PHASE2_POS[srv.id];

    let opacity = 1;
    let scale = 1;
    let pos: { x: number; y: number };

    if (isP3Removed) {
      // --- Servers removed in Phase 3: s3, s5, s7 ---
      // Visible in Phases 1-3, invisible in 4-7, re-appear in Phase 8

      if (frame < P3_START) {
        // Phases 1-2: visible
        if (frame < P2_START) {
          pos = posAt(p1[0], p1[1], srv.hasGpu, 4);
        } else {
          const ac = Math.max(animCols, 4);
          pos = posAt(p2[0], p2[1], srv.hasGpu, ac);
        }
      } else if (frame < P8_START) {
        // Phase 3: fading out; Phases 4-7: invisible
        const idx = PHASE3_REMOVE.indexOf(srv.id);
        const exit = sp(P3_START, 25, idx * 6);
        opacity = 1 - exit;
        scale = 1 - exit;
        pos = posAt(p2[0], p2[1], srv.hasGpu, 6);
        if (opacity <= 0.001) continue;
      } else {
        // Phase 8: re-spawning at Phase 1 positions
        const idx = PHASE8_RESPAWN.findIndex((s) => s.id === srv.id);
        const enter = sp(P8_START, 25, idx * 8);
        opacity = enter;
        scale = enter;
        pos = posAt(p1[0], p1[1], srv.hasGpu, 4);
      }
    } else {
      // --- Servers that survive Phase 3: s0, s1, s2, s4, s6 ---
      const p4 = PHASE4_POS[srv.id]!;
      const p5 = PHASE5_POS[srv.id]!;
      const p7 = PHASE7_POS[srv.id]!;

      if (frame < P2_START) {
        // Phase 1
        pos = posAt(p1[0], p1[1], srv.hasGpu, 4);
      } else if (frame < P4_START) {
        // Phases 2-3: in 6-col grid at Phase 2 positions
        const ac = frame < P3_START ? Math.max(animCols, 4) : 6;
        pos = posAt(p2[0], p2[1], srv.hasGpu, ac);
      } else if (frame < P5_START) {
        // Phase 4: slide from Phase 2 pos to Phase 4 pos
        const t = sp(P4_START, 40);
        const from = posAt(p2[0], p2[1], srv.hasGpu, 6);
        const ac = Math.max(animCols, 5);
        const to = posAt(p4[0], p4[1], srv.hasGpu, ac);
        pos = lerpPos(from, to, t);
      } else if (frame < P6_START) {
        // Phase 5 + hold: at Phase 5 positions
        const ac = animCols;
        pos = posAt(p5[0], p5[1], srv.hasGpu, ac);
      } else if (frame < P7_START) {
        // Phase 6: stay at Phase 5 positions (others are fading out)
        pos = posAt(p5[0], p5[1], srv.hasGpu, 6);
      } else {
        // Phase 7+: slide from Phase 5 pos to Phase 1 pos
        const t = sp(P7_START, 40);
        const from = posAt(p5[0], p5[1], srv.hasGpu, 6);
        const ac = frame < P8_START ? Math.max(animCols, 4) : 4;
        const to = posAt(p7[0], p7[1], srv.hasGpu, ac);
        pos = lerpPos(from, to, t);
      }
    }

    servers.push({
      id: srv.id,
      provider: srv.provider,
      hasGpu: srv.hasGpu,
      x: pos.x,
      y: pos.y,
      opacity,
      scale,
    });
  }

  // --- Phase 2 new servers (Hetzner: s8-s11) ---
  if (frame >= P2_START) {
    for (let i = 0; i < PHASE2_NEW.length; i++) {
      const srv = PHASE2_NEW[i];
      const p2 = PHASE2_POS[srv.id];
      const p4 = PHASE4_POS[srv.id]!;
      const p5 = PHASE5_POS[srv.id]!;

      // Enter animation
      const enter = sp(P2_START, 25, i * 8);

      // Exit animation (Phase 6)
      let exit = 0;
      if (frame >= P6_START) {
        const idx = PHASE6_REMOVE.indexOf(srv.id);
        exit = sp(P6_START, 25, idx * 5);
      }

      const opacity = Math.min(enter, 1 - exit);
      const scl = Math.min(enter, 1 - exit);
      if (opacity <= 0.001) continue;

      let pos: { x: number; y: number };

      if (frame < P4_START) {
        const ac = Math.max(animCols, 4);
        pos = posAt(p2[0], p2[1], srv.hasGpu, ac);
      } else if (frame < P5_START) {
        const t = sp(P4_START, 40);
        const from = posAt(p2[0], p2[1], srv.hasGpu, 6);
        const ac = Math.max(animCols, 5);
        const to = posAt(p4[0], p4[1], srv.hasGpu, ac);
        pos = lerpPos(from, to, t);
      } else {
        // Phase 5+: at Phase 5 positions
        pos = posAt(p5[0], p5[1], srv.hasGpu, frame < P7_START ? animCols : 6);
      }

      servers.push({
        id: srv.id,
        provider: srv.provider,
        hasGpu: srv.hasGpu,
        x: pos.x,
        y: pos.y,
        opacity,
        scale: scl,
      });
    }
  }

  // --- Phase 5 new servers (GCP GPU: g0-g2) ---
  if (frame >= P5_START) {
    for (let i = 0; i < PHASE5_NEW.length; i++) {
      const srv = PHASE5_NEW[i];
      const p5 = PHASE5_POS[srv.id];

      // Enter animation
      const enter = sp(P5_START, 25, i * 8);

      // Exit animation (Phase 6)
      let exit = 0;
      if (frame >= P6_START) {
        const idx = PHASE6_REMOVE.indexOf(srv.id);
        exit = sp(P6_START, 25, idx * 5);
      }

      const opacity = Math.min(enter, 1 - exit);
      const scl = Math.min(enter, 1 - exit);
      if (opacity <= 0.001) continue;

      const ac = frame < P7_START ? animCols : 6;
      const pos = posAt(p5[0], p5[1], srv.hasGpu, ac);

      servers.push({
        id: srv.id,
        provider: srv.provider,
        hasGpu: srv.hasGpu,
        x: pos.x,
        y: pos.y,
        opacity,
        scale: scl,
      });
    }
  }

  // ── Render ─────────────────────────────────────────────

  const maxK8sW = k8sBoxW(6);
  const k8sTopY = CLAUDIE_LOGO_SIZE + VERTICAL_GAP;
  const totalH = k8sTopY + K8S_BOX_H;
  const centerX = maxK8sW / 2;

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
          width: maxK8sW,
          height: totalH,
        }}
      >
        {/* Dashed connection line (vertical) + signal dot */}
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
          <defs>
            <filter id="signal-glow" x="-100%" y="-100%" width="300%" height="300%">
              <feGaussianBlur in="SourceGraphic" stdDeviation="3" result="blur" />
              <feMerge>
                <feMergeNode in="blur" />
                <feMergeNode in="SourceGraphic" />
              </feMerge>
            </filter>
          </defs>
          <line
            x1={centerX}
            y1={CLAUDIE_LOGO_SIZE}
            x2={centerX}
            y2={k8sTopY}
            stroke={t.stroke}
            strokeWidth="1.5"
            strokeDasharray="6 4"
          />
          {/* Traveling signal dot */}
          {dotT >= 0 && dotT <= 1 && (
            <circle
              cx={centerX}
              cy={interpolate(dotT, [0, 1], [CLAUDIE_LOGO_SIZE, k8sTopY])}
              r={4}
              fill={SIGNAL_COLOR}
              filter="url(#signal-glow)"
              opacity={dotT > 0.85 ? interpolate(dotT, [0.85, 1], [1, 0]) : 1}
            />
          )}
        </svg>

        {/* Claudie logo with signal glow */}
        <div
          style={{
            position: "absolute",
            left: centerX - CLAUDIE_LOGO_SIZE / 2,
            top: 0,
            width: CLAUDIE_LOGO_SIZE,
            height: CLAUDIE_LOGO_SIZE,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            borderRadius: 12,
            boxShadow:
              glowIntensity > 0
                ? `0 0 ${8 + glowIntensity * 16}px ${glowIntensity * 8}px ${SIGNAL_COLOR}${Math.round(glowIntensity * 180).toString(16).padStart(2, "0")}`
                : "none",
          }}
        >
          <Img
            src={staticFile("logos/claudie.svg")}
            style={{
              width: CLAUDIE_LOGO_SIZE,
              height: CLAUDIE_LOGO_SIZE,
              borderRadius: 12,
              objectFit: "contain",
            }}
          />
        </div>

        {/* K8s outer box */}
        <div
          style={{
            position: "absolute",
            left: (maxK8sW - currentK8sW) / 2,
            top: k8sTopY,
            width: currentK8sW,
            height: K8S_BOX_H,
            borderRadius: 16,
            border: `2px solid ${t.k8sBorder}`,
            backgroundColor: t.k8sBg,
            padding: BOX_PADDING,
            display: "flex",
            flexDirection: "column",
            gap: 10,
            boxShadow: t.shadow,
            overflow: "hidden",
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
                whiteSpace: "nowrap",
              }}
            >
              Single Multi-Cloud Kubernetes Cluster
            </span>
          </div>

          {/* Control + Compute side by side */}
          <div style={{ flex: 1, display: "flex", gap: INNER_GAP }}>
            {/* Control box (static) */}
            <div
              style={{
                width: CONTROL_BOX_W,
                borderRadius: 12,
                border: `1.5px solid ${t.controlBorder}`,
                backgroundColor: t.controlBg,
                padding: 10,
                display: "flex",
                flexDirection: "column",
                gap: 8,
              }}
            >
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
                  Control
                </span>
              </div>
              <div
                style={{
                  flex: 1,
                  display: "flex",
                  flexDirection: "column",
                  justifyContent: "space-evenly",
                  alignItems: "center",
                }}
              >
                {CONTROL_SERVERS.map((provider, i) => (
                  <SmallServerNode key={i} provider={provider} theme={theme} />
                ))}
              </div>
            </div>

            {/* Compute box (animated width) */}
            <div
              style={{
                width: currentComputeW,
                borderRadius: 12,
                border: `1.5px solid ${t.computeBorder}`,
                backgroundColor: t.computeBg,
                padding: COMPUTE_PADDING,
                display: "flex",
                flexDirection: "column",
                gap: 8,
                overflow: "visible",
              }}
            >
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

              {/* Server area: absolute positioning */}
              <div
                style={{
                  flex: 1,
                  position: "relative",
                  overflow: "visible",
                }}
              >
                {servers.map((srv) => (
                  <div
                    key={srv.id}
                    style={{
                      position: "absolute",
                      left: srv.x,
                      top: srv.y,
                      opacity: srv.opacity,
                      transform: `scale(${srv.scale})`,
                      transformOrigin: "center bottom",
                    }}
                  >
                    <ServerNode
                      server={{
                        provider: srv.provider,
                        hasGpu: srv.hasGpu,
                      }}
                      theme={theme}
                    />
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
