import React from "react";
import {
  AbsoluteFill,
  useCurrentFrame,
  useVideoConfig,
  interpolate,
  spring,
  Sequence,
} from "remotion";
import { loadFont } from "@remotion/google-fonts/Inter";
import { COLORS } from "../colors";

const { fontFamily } = loadFont("normal", {
  weights: ["400", "700"],
  subsets: ["latin"],
});

const PROVIDERS = [
  { name: "AWS", color: COLORS.aws, letter: "A" },
  { name: "Azure", color: COLORS.azure, letter: "Az" },
  { name: "GCP", color: COLORS.gcp, letter: "G" },
  { name: "OCI", color: COLORS.oci, letter: "O" },
  { name: "Hetzner", color: COLORS.hetzner, letter: "H" },
  { name: "Exoscale", color: "#da291c", letter: "E" },
];

const FEATURES = [
  { title: "Multi-Cloud Clusters", desc: "Mix providers in a single cluster", icon: "🌐" },
  { title: "WireGuard VPN", desc: "Secure inter-node communication", icon: "🔒" },
  { title: "Managed LB", desc: "Built-in load balancing via Envoy", icon: "⚖️" },
  { title: "Longhorn Storage", desc: "Persistent volumes out of the box", icon: "💿" },
];

export const ProvidersScene: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  const headerOpacity = spring({ frame, fps, config: { damping: 200 } });

  return (
    <AbsoluteFill
      style={{
        backgroundColor: COLORS.bg,
        fontFamily,
        padding: 60,
      }}
    >
      {/* Header */}
      <div
        style={{
          opacity: headerOpacity,
          display: "flex",
          alignItems: "center",
          gap: 16,
          marginBottom: 40,
        }}
      >
        <div
          style={{
            width: 8,
            height: 40,
            borderRadius: 4,
            background: `linear-gradient(180deg, ${COLORS.primary}, ${COLORS.secondary})`,
          }}
        />
        <div style={{ fontSize: 42, fontWeight: 700, color: COLORS.text }}>
          Multi-Cloud Ecosystem
        </div>
      </div>

      {/* Providers ring */}
      <div style={{ display: "flex", justifyContent: "center", marginBottom: 50 }}>
        <div style={{ position: "relative", width: 400, height: 220 }}>
          {/* Central K8s icon */}
          <div
            style={{
              position: "absolute",
              left: "50%",
              top: "50%",
              transform: "translate(-50%, -50%)",
              width: 80,
              height: 80,
              borderRadius: "50%",
              background: `${COLORS.primary}22`,
              border: `3px solid ${COLORS.primary}`,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              fontSize: 40,
              opacity: spring({ frame, fps, delay: 5, config: { damping: 200 } }),
            }}
          >
            ☸️
          </div>

          {/* Provider nodes arranged in arc */}
          {PROVIDERS.map((p, i) => {
            const angle = -Math.PI / 2 + (i / (PROVIDERS.length - 1)) * Math.PI;
            const rx = 180;
            const ry = 90;
            const cx = 200 + rx * Math.cos(angle);
            const cy = 110 + ry * Math.sin(angle);

            const nodeProgress = spring({
              frame,
              fps,
              delay: 10 + i * 6,
              config: { damping: 12 },
            });

            // Animated connecting line
            const lineOpacity = spring({
              frame,
              fps,
              delay: 15 + i * 6,
              config: { damping: 200 },
            });

            return (
              <Sequence key={p.name} from={0} layout="none">
                {/* Connection line */}
                <svg
                  style={{ position: "absolute", top: 0, left: 0, width: "100%", height: "100%" }}
                >
                  <line
                    x1={200}
                    y1={110}
                    x2={cx}
                    y2={cy}
                    stroke={p.color}
                    strokeWidth="1.5"
                    strokeDasharray="4 3"
                    opacity={lineOpacity * 0.4}
                  />
                </svg>
                <div
                  style={{
                    position: "absolute",
                    left: cx - 28,
                    top: cy - 28,
                    width: 56,
                    height: 56,
                    borderRadius: 14,
                    background: `${p.color}22`,
                    border: `2px solid ${p.color}`,
                    display: "flex",
                    flexDirection: "column",
                    alignItems: "center",
                    justifyContent: "center",
                    transform: `scale(${nodeProgress})`,
                  }}
                >
                  <div style={{ fontSize: 16, fontWeight: 700, color: p.color }}>
                    {p.letter}
                  </div>
                  <div style={{ fontSize: 9, color: COLORS.textMuted }}>{p.name}</div>
                </div>
              </Sequence>
            );
          })}
        </div>
      </div>

      {/* Features grid */}
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "1fr 1fr",
          gap: 20,
        }}
      >
        {FEATURES.map((feat, i) => {
          const cardOpacity = spring({
            frame,
            fps,
            delay: 50 + i * 10,
            config: { damping: 200 },
          });
          const cardY = interpolate(
            spring({ frame, fps, delay: 50 + i * 10, config: { damping: 200 } }),
            [0, 1],
            [20, 0]
          );

          return (
            <Sequence key={feat.title} from={0} layout="none">
              <div
                style={{
                  background: COLORS.card,
                  borderRadius: 14,
                  padding: "20px 24px",
                  border: `1px solid ${COLORS.border}`,
                  opacity: cardOpacity,
                  transform: `translateY(${cardY}px)`,
                  display: "flex",
                  alignItems: "center",
                  gap: 16,
                }}
              >
                <div style={{ fontSize: 32 }}>{feat.icon}</div>
                <div>
                  <div style={{ fontSize: 18, fontWeight: 700, color: COLORS.text }}>
                    {feat.title}
                  </div>
                  <div style={{ fontSize: 14, color: COLORS.textMuted }}>{feat.desc}</div>
                </div>
              </div>
            </Sequence>
          );
        })}
      </div>
    </AbsoluteFill>
  );
};
