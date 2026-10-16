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
  weights: ["400", "700", "900"],
  subsets: ["latin"],
});

const USE_CASES = [
  "Cost Optimization",
  "Data Residency / GDPR",
  "Cloud Bursting",
  "GPU Nodes for AI",
  "Vendor Lock-in Freedom",
];

export const OutroScene: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  const titleScale = spring({ frame, fps, config: { damping: 12 } });
  const titleOpacity = spring({ frame, fps, config: { damping: 200 } });

  const urlOpacity = spring({ frame, fps, delay: 60, config: { damping: 200 } });

  // Pulsing glow
  const glowScale = interpolate(
    Math.sin(frame * 0.05),
    [-1, 1],
    [0.95, 1.05]
  );

  return (
    <AbsoluteFill
      style={{
        backgroundColor: COLORS.bg,
        fontFamily,
        justifyContent: "center",
        alignItems: "center",
      }}
    >
      {/* Background glow */}
      <div
        style={{
          position: "absolute",
          width: 700,
          height: 700,
          borderRadius: "50%",
          background: `radial-gradient(circle, ${COLORS.primary}15 0%, transparent 70%)`,
          top: "50%",
          left: "50%",
          transform: `translate(-50%, -50%) scale(${glowScale})`,
        }}
      />

      {/* Title */}
      <Sequence from={0} layout="none">
        <div
          style={{
            fontSize: 64,
            fontWeight: 900,
            color: COLORS.text,
            opacity: titleOpacity,
            transform: `scale(${titleScale})`,
            textAlign: "center",
            marginBottom: 24,
          }}
        >
          Why{" "}
          <span
            style={{
              background: `linear-gradient(135deg, ${COLORS.primary}, ${COLORS.accent})`,
              WebkitBackgroundClip: "text",
              WebkitTextFillColor: "transparent",
            }}
          >
            Claudie
          </span>
          ?
        </div>
      </Sequence>

      {/* Use cases */}
      <div style={{ display: "flex", flexWrap: "wrap", gap: 12, justifyContent: "center", maxWidth: 700, marginBottom: 40 }}>
        {USE_CASES.map((uc, i) => {
          const chipOpacity = spring({
            frame,
            fps,
            delay: 15 + i * 8,
            config: { damping: 200 },
          });
          const chipScale = spring({
            frame,
            fps,
            delay: 15 + i * 8,
            config: { damping: 15 },
          });

          return (
            <Sequence key={uc} from={0} layout="none">
              <div
                style={{
                  fontSize: 18,
                  fontWeight: 700,
                  color: COLORS.text,
                  background: COLORS.card,
                  border: `1px solid ${COLORS.border}`,
                  borderRadius: 12,
                  padding: "10px 22px",
                  opacity: chipOpacity,
                  transform: `scale(${chipScale})`,
                }}
              >
                {uc}
              </div>
            </Sequence>
          );
        })}
      </div>

      {/* GitHub URL */}
      <Sequence from={0} layout="none">
        <div
          style={{
            opacity: urlOpacity,
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            gap: 12,
          }}
        >
          <div
            style={{
              fontSize: 16,
              color: COLORS.textMuted,
              textTransform: "uppercase",
              letterSpacing: 3,
            }}
          >
            Open Source — Apache 2.0
          </div>
          <div
            style={{
              fontSize: 28,
              fontWeight: 700,
              color: COLORS.primary,
              background: `${COLORS.primary}12`,
              border: `2px solid ${COLORS.primary}44`,
              borderRadius: 14,
              padding: "12px 32px",
            }}
          >
            github.com/berops/claudie
          </div>
        </div>
      </Sequence>
    </AbsoluteFill>
  );
};
